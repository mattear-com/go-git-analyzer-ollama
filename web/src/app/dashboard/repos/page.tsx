"use client";

import { useAuth } from "@/lib/use-auth";
import { api } from "@/lib/api";
import { useJobs } from "@/components/FloatingJobTracker";
import { useEffect, useState, useCallback, useRef } from "react";
import mermaid from "mermaid";

interface GitHubRepo {
    id: number;
    name: string;
    full_name: string;
    description: string;
    html_url: string;
    clone_url: string;
    default_branch: string;
    private: boolean;
    language: string;
    stargazers_count: number;
    updated_at: string;
}

interface LocalRepo {
    id: string;
    name: string;
    url: string;
    status: string;
    default_branch: string;
    report_language: string;
    created_at: string;
}

interface JobStatus {
    id: string;
    status: string;
    progress: number;
    total: number;
    current_strategy: string;
    completed_strategies: string[];
}

const LANGUAGES = [
    { code: "en", label: "English" },
    { code: "es", label: "Español" },
    { code: "pt", label: "Português" },
    { code: "fr", label: "Français" },
    { code: "de", label: "Deutsch" },
    { code: "ja", label: "日本語" },
    { code: "zh", label: "中文" },
];

export default function ReposPage() {
    const { token } = useAuth();
    const { jobs, repoEvents, startJob } = useJobs();
    const [githubRepos, setGithubRepos] = useState<GitHubRepo[]>([]);
    const [localRepos, setLocalRepos] = useState<LocalRepo[]>([]);
    const [loading, setLoading] = useState(true);
    const [cloning, setCloning] = useState<Record<string, boolean>>({});
    const [error, setError] = useState<string | null>(null);
    const [gitGraphData, setGitGraphData] = useState<Record<string, string>>({});
    const [gitGraphAuthors, setGitGraphAuthors] = useState<Record<string, string[]>>({});
    const [gitGraphLoading, setGitGraphLoading] = useState<Record<string, boolean>>({});
    const [gitGraphVisible, setGitGraphVisible] = useState<Record<string, boolean>>({});
    const [searchQuery, setSearchQuery] = useState("");
    const [filterStatus, setFilterStatus] = useState<"all" | "cloned" | "not_cloned">("all");
    const [cloneUrl, setCloneUrl] = useState("");
    const [cloningUrl, setCloningUrl] = useState(false);


    const fetchRepos = useCallback(async () => {
        if (!token) return;
        setLoading(true);
        try {
            const [ghData, localData] = await Promise.all([
                api<{ repos: GitHubRepo[] }>("/api/v1/repos/github", { token }).catch(() => ({ repos: [] })),
                api<{ repos: LocalRepo[] }>("/api/v1/repos", { token }).catch(() => ({ repos: [] })),
            ]);
            setGithubRepos(ghData.repos || []);
            setLocalRepos(localData.repos || []);
        } catch {
            setError("Failed to fetch repos");
        }
        setLoading(false);
    }, [token]);

    useEffect(() => { fetchRepos(); }, [fetchRepos]);

    // Refresh local repos when SSE repo events arrive (clone complete/error)
    useEffect(() => {
        if (repoEvents.length === 0) return;
        // Refetch local repos on any repo status change
        api<{ repos: LocalRepo[] }>("/api/v1/repos", { token: token || "" })
            .then((data) => setLocalRepos(data.repos || []))
            .catch(() => { });
    }, [repoEvents, token]);

    const isCloned = (ghRepo: GitHubRepo) => localRepos.some((lr) => lr.url === ghRepo.clone_url || lr.name === ghRepo.name);
    const getLocalRepo = (ghRepo: GitHubRepo) => localRepos.find((lr) => lr.url === ghRepo.clone_url || lr.name === ghRepo.name);

    const handleClone = async (ghRepo: GitHubRepo) => {
        if (!token) return;
        const key = String(ghRepo.id);
        setCloning((prev) => ({ ...prev, [key]: true }));
        try {
            await api("/api/v1/repos/clone", {
                method: "POST", token,
                body: { url: ghRepo.clone_url, name: ghRepo.name, branch: ghRepo.default_branch },
            });
            setTimeout(fetchRepos, 1000);
        } catch {
            setError("Failed to clone repo");
        }
        setCloning((prev) => ({ ...prev, [key]: false }));
    };

    const handleCloneByUrl = async () => {
        if (!token || !cloneUrl.trim()) return;
        setCloningUrl(true);
        try {
            // Extract repo name from URL
            const urlParts = cloneUrl.trim().replace(/\.git$/, "").split("/");
            const name = urlParts[urlParts.length - 1] || "repo";
            await api("/api/v1/repos/clone", {
                method: "POST", token,
                body: { url: cloneUrl.trim(), name, branch: "main" },
            });
            setCloneUrl("");
            setTimeout(fetchRepos, 1000);
        } catch {
            setError("Failed to clone from URL");
        }
        setCloningUrl(false);
    };

    const handleAnalyze = async (repoId: string, repoName: string) => {
        if (!token) return;
        try {
            const result = await api<{ job_id: string }>("/api/v1/analysis/run", {
                method: "POST", token,
                body: { repo_id: repoId },
            });
            startJob(repoId, repoName, result.job_id);
        } catch {
            setError("Failed to start analysis");
        }
    };

    const handleGitGraph = async (repoId: string) => {
        if (!token) return;
        const isVisible = gitGraphVisible[repoId];
        if (isVisible) {
            setGitGraphVisible((prev) => ({ ...prev, [repoId]: false }));
            return;
        }
        if (gitGraphData[repoId]) {
            setGitGraphVisible((prev) => ({ ...prev, [repoId]: true }));
            return;
        }
        setGitGraphLoading((prev) => ({ ...prev, [repoId]: true }));
        try {
            const data = await api<{ mermaid: string; authors: string[] }>(`/api/v1/repos/${repoId}/gitgraph`, { token });
            setGitGraphData((prev) => ({ ...prev, [repoId]: data.mermaid }));
            setGitGraphAuthors((prev) => ({ ...prev, [repoId]: data.authors || [] }));
            setGitGraphVisible((prev) => ({ ...prev, [repoId]: true }));
        } catch {
            setError("Failed to generate git graph");
        }
        setGitGraphLoading((prev) => ({ ...prev, [repoId]: false }));
    };

    // Initialize mermaid
    useEffect(() => {
        mermaid.initialize({
            startOnLoad: false,
            theme: "dark",
            gitGraph: {
                showCommitLabel: true,
                mainBranchName: "main",
                rotateCommitLabel: false,
                nodeSpacing: 150,
                mainBranchOrder: 2,
            } as any,
        });
    }, []);

    // Render mermaid diagrams when data changes
    useEffect(() => {
        Object.entries(gitGraphData).forEach(([repoId, diagram]) => {
            if (!gitGraphVisible[repoId]) return;
            const el = document.getElementById(`mermaid-${repoId}`);
            if (!el) return;
            el.innerHTML = diagram;
            el.removeAttribute("data-processed");
            const authors = gitGraphAuthors[repoId] || [];
            mermaid.run({ nodes: [el] }).then(() => {
                // Post-process: color commit labels by author
                colorCommitsByAuthor(el, authors);
            }).catch((err) => {
                console.error("Mermaid render error:", err);
                el.innerHTML = `<pre style="color: var(--text-secondary); font-size: 12px; white-space: pre-wrap;">${diagram}</pre>`;
            });
        });
    }, [gitGraphData, gitGraphVisible, gitGraphAuthors]);

    const handleSetLanguage = async (repoId: string, lang: string) => {
        if (!token) return;
        await api(`/api/v1/repos/${repoId}/language`, {
            method: "PUT", token, body: { language: lang },
        });
        fetchRepos();
    };

    const languageBadge: Record<string, string> = {
        Go: "badge-success", TypeScript: "badge-info", JavaScript: "badge-warning",
        Python: "badge-accent", Rust: "badge-danger", Java: "badge-warning", Swift: "badge-accent",
    };

    const statusBadge: Record<string, string> = {
        ready: "badge-success", cloning: "badge-warning", error: "badge-danger",
    };

    return (
        <>
            <div style={{ marginBottom: "2rem", display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                <div>
                    <h1 className="page-title">Repositories</h1>
                    <p className="page-subtitle">Connected to GitHub • {githubRepos.length} repos</p>
                </div>
                <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                    <div style={{ position: "relative" }}>
                        <span style={{ position: "absolute", left: "0.65rem", top: "50%", transform: "translateY(-50%)", color: "var(--text-tertiary)", fontSize: "13px", pointerEvents: "none" }}>⌕</span>
                        <input
                            className="input"
                            type="text"
                            placeholder="Search repos..."
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            style={{ fontSize: "12px", padding: "0.4rem 0.6rem 0.4rem 1.75rem", width: "220px" }}
                        />
                    </div>
                    <select
                        className="input"
                        value={filterStatus}
                        onChange={(e) => setFilterStatus(e.target.value as "all" | "cloned" | "not_cloned")}
                        style={{ fontSize: "12px", padding: "0.4rem 0.6rem", width: "auto" }}
                    >
                        <option value="all">All</option>
                        <option value="cloned">Cloned</option>
                        <option value="not_cloned">Not Cloned</option>
                    </select>
                    <button className="btn btn-secondary" onClick={fetchRepos} style={{ fontSize: "12px" }}>↻ Refresh</button>
                </div>
            </div>

            {/* Clone by URL */}
            <div className="card" style={{ marginBottom: "1rem", padding: "0.75rem 1rem", display: "flex", gap: "0.5rem", alignItems: "center" }}>
                <span style={{ color: "var(--text-tertiary)", fontSize: "13px", whiteSpace: "nowrap" }}>📎 Clone by URL:</span>
                <input
                    className="input"
                    type="text"
                    placeholder="https://github.com/user/repo.git"
                    value={cloneUrl}
                    onChange={(e) => setCloneUrl(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && handleCloneByUrl()}
                    style={{ flex: 1, fontSize: "12px", padding: "0.4rem 0.6rem" }}
                />
                <button
                    className="btn btn-primary"
                    onClick={handleCloneByUrl}
                    disabled={cloningUrl || !cloneUrl.trim()}
                    style={{ fontSize: "12px", padding: "0.4rem 0.75rem" }}
                >
                    {cloningUrl ? "Cloning..." : "⬇ Clone"}
                </button>
            </div>

            {error && (
                <div className="card" style={{ borderColor: "var(--danger)", marginBottom: "1rem", padding: "0.75rem", fontSize: "13px", color: "var(--danger)" }}>
                    {error}
                    <button onClick={() => setError(null)} style={{ float: "right", background: "none", border: "none", cursor: "pointer", color: "inherit" }}>✕</button>
                </div>
            )}

            {loading ? (
                <div className="card" style={{ textAlign: "center", padding: "3rem" }}>
                    <p style={{ color: "var(--text-secondary)" }}>Fetching GitHub repos...</p>
                </div>
            ) : (
                <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
                    {githubRepos
                        .filter((repo) => {
                            // Status filter
                            if (filterStatus === "cloned" && !isCloned(repo)) return false;
                            if (filterStatus === "not_cloned" && isCloned(repo)) return false;
                            // Search filter
                            if (!searchQuery.trim()) return true;
                            const q = searchQuery.toLowerCase();
                            return (
                                repo.full_name.toLowerCase().includes(q) ||
                                repo.name.toLowerCase().includes(q) ||
                                (repo.description || "").toLowerCase().includes(q) ||
                                (repo.language || "").toLowerCase().includes(q)
                            );
                        })
                        .map((repo) => {
                            const cloned = isCloned(repo);
                            const local = getLocalRepo(repo);
                            const localId = local?.id || "";
                            const key = String(repo.id);
                            const job = jobs[localId];
                            const isRunning = job?.status === "running";
                            const isComplete = job?.status === "complete";


                            return (
                                <div key={repo.id} className="card hover-lift" style={{ cursor: "default" }}>
                                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                                        <div style={{ flex: 1 }}>
                                            <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "0.25rem" }}>
                                                <h3 style={{ fontSize: "15px", fontWeight: 600, margin: 0 }}>
                                                    <a href={repo.html_url} target="_blank" rel="noopener noreferrer" style={{ color: "var(--text-primary)", textDecoration: "none" }}>
                                                        {repo.full_name}
                                                    </a>
                                                </h3>
                                                {repo.private && <span className="badge badge-danger" style={{ fontSize: "10px" }}>private</span>}
                                                {repo.language && <span className={`badge ${languageBadge[repo.language] || "badge-accent"}`} style={{ fontSize: "10px" }}>{repo.language}</span>}
                                                {local && <span className={`badge ${statusBadge[local.status] || "badge-accent"}`} style={{ fontSize: "10px" }}>{local.status}</span>}
                                            </div>
                                            <p style={{ color: "var(--text-tertiary)", fontSize: "12px", margin: "0.25rem 0 0" }}>
                                                {repo.description || "No description"}
                                                <span style={{ marginLeft: "1rem" }}>★ {repo.stargazers_count} • {repo.default_branch}</span>
                                            </p>
                                        </div>

                                        <div style={{ display: "flex", gap: "0.5rem", marginLeft: "1rem", flexShrink: 0, alignItems: "center" }}>
                                            {/* Language selector */}
                                            {local?.status === "ready" && (
                                                <select
                                                    value={local.report_language || "en"}
                                                    onChange={(e) => handleSetLanguage(localId, e.target.value)}
                                                    className="input"
                                                    style={{ fontSize: "11px", padding: "0.3rem 0.4rem", width: "auto", minWidth: "70px" }}
                                                    title="Report language"
                                                >
                                                    {LANGUAGES.map((l) => <option key={l.code} value={l.code}>{l.label}</option>)}
                                                </select>
                                            )}

                                            {(!cloned || local?.status === "error") ? (
                                                <button className="btn btn-secondary" onClick={() => handleClone(repo)} disabled={cloning[key]}
                                                    style={{ fontSize: "12px", padding: "0.4rem 0.8rem" }}>
                                                    {cloning[key] ? "Cloning..." : local?.status === "error" ? "↻ Retry" : "⬇ Clone"}
                                                </button>
                                            ) : local?.status === "cloning" ? (
                                                <span className="badge badge-warning" style={{ fontSize: "11px" }}>⏳ Cloning...</span>
                                            ) : (
                                                <>
                                                    {local?.status === "ready" && (
                                                        <>
                                                            <button className="btn btn-primary" onClick={() => handleAnalyze(localId, repo.name)} disabled={isRunning}
                                                                style={{ fontSize: "12px", padding: "0.4rem 0.8rem" }}>
                                                                {isRunning ? `⏳ ${job.progress}/${job.total}` : isComplete ? "↻ Re-Analyze" : "▶ Analyze"}
                                                            </button>
                                                            <button
                                                                className="btn btn-secondary"
                                                                onClick={() => handleGitGraph(localId)}
                                                                disabled={gitGraphLoading[localId]}
                                                                style={{ fontSize: "12px", padding: "0.4rem 0.8rem" }}
                                                            >
                                                                {gitGraphLoading[localId] ? "⏳..." : gitGraphVisible[localId] ? "▼ Hide Graph" : "🌳 Git Graph"}
                                                            </button>
                                                        </>
                                                    )}

                                                    <a href={`/dashboard/chat?repo=${localId}`} className="btn btn-secondary"
                                                        style={{ fontSize: "12px", padding: "0.4rem 0.8rem", textDecoration: "none" }}>
                                                        💬 Chat
                                                    </a>
                                                </>
                                            )}
                                        </div>
                                    </div>

                                    {/* Git Graph Panel */}
                                    {gitGraphVisible[localId] && gitGraphData[localId] && (
                                        <GitGraphViewer id={localId} authors={gitGraphAuthors[localId] || []} />
                                    )}

                                    {/* Progress bar */}
                                    {isRunning && job && (
                                        <div style={{ marginTop: "0.75rem" }}>
                                            <div style={{ background: "var(--bg-tertiary)", borderRadius: "4px", height: "4px", overflow: "hidden" }}>
                                                <div style={{
                                                    width: `${(job.progress / job.total) * 100}%`,
                                                    height: "100%",
                                                    background: "var(--accent)",
                                                    transition: "width 0.5s ease",
                                                }} />
                                            </div>
                                            <p style={{ fontSize: "11px", color: "var(--text-tertiary)", marginTop: "0.25rem" }}>
                                                {job.current_strategy ? `Analyzing: ${job.current_strategy.replace(/_/g, " ")}` : "Starting..."} •
                                                {job.progress}/{job.total} • You can navigate away — analysis continues in background.
                                            </p>
                                        </div>
                                    )}

                                    {isComplete && (
                                        <div style={{ marginTop: "0.5rem" }}>
                                            <span className="badge badge-success" style={{ fontSize: "11px" }}>
                                                ✅ Analysis complete — view in <a href="/dashboard/reports" style={{ color: "inherit" }}>Reports</a>
                                            </span>
                                        </div>
                                    )}
                                </div>
                            );
                        })}
                </div>
            )}
        </>
    );
}

// --- Deterministic vibrant color palette for authors ---
const AUTHOR_COLORS = [
    "#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7",
    "#DDA0DD", "#98D8C8", "#F7DC6F", "#BB8FCE", "#85C1E9",
    "#F1948A", "#82E0AA", "#F8C471", "#AED6F1", "#D7BDE2",
    "#A3E4D7", "#FAD7A0", "#A9CCE3", "#D5F5E3", "#FADBD8",
];

function getAuthorColor(author: string, authors: string[]): string {
    const idx = authors.indexOf(author);
    return AUTHOR_COLORS[idx >= 0 ? idx % AUTHOR_COLORS.length : 0];
}

function colorCommitsByAuthor(el: HTMLElement, authors: string[]) {
    if (!authors.length) return;
    const svg = el.querySelector("svg");
    if (!svg) return;

    // Find all text elements that contain commit labels (author: message format)
    const textEls = svg.querySelectorAll("text");
    textEls.forEach((textEl) => {
        const content = textEl.textContent || "";
        for (const author of authors) {
            if (content.startsWith(`${author}:`)) {
                const color = getAuthorColor(author, authors);
                textEl.setAttribute("fill", color);
                textEl.style.fill = color;
                textEl.style.fontWeight = "600";
                break;
            }
        }
    });
}

// --- GitGraphViewer: Zoomable + Pannable + Author Legend ---
function GitGraphViewer({ id, authors }: { id: string; authors: string[] }) {
    const [zoom, setZoom] = useState(1);
    const [pan, setPan] = useState({ x: 0, y: 0 });
    const [dragging, setDragging] = useState(false);
    const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
    const containerRef = useRef<HTMLDivElement>(null);

    const handleWheel = useCallback((e: React.WheelEvent) => {
        e.preventDefault();
        const delta = e.deltaY > 0 ? -0.1 : 0.1;
        setZoom((z) => Math.max(0.2, Math.min(5, z + delta)));
    }, []);

    const handleMouseDown = useCallback((e: React.MouseEvent) => {
        if (e.button !== 0) return;
        setDragging(true);
        setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
    }, [pan]);

    const handleMouseMove = useCallback((e: React.MouseEvent) => {
        if (!dragging) return;
        setPan({ x: e.clientX - dragStart.x, y: e.clientY - dragStart.y });
    }, [dragging, dragStart]);

    const handleMouseUp = useCallback(() => setDragging(false), []);

    const reset = () => { setZoom(1); setPan({ x: 0, y: 0 }); };

    return (
        <div style={{ marginTop: "1rem", borderTop: "1px solid var(--border-color)", paddingTop: "1rem" }} ref={containerRef}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "0.75rem" }}>
                <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                    <h4 style={{ fontSize: "14px", fontWeight: 600, margin: 0 }}>🌳 Git Graph</h4>
                    <span className="badge badge-accent" style={{ fontSize: "10px" }}>Scroll to zoom • Drag to pan</span>
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: "0.35rem" }}>
                    <button className="btn btn-secondary" onClick={() => setZoom((z) => Math.min(5, z + 0.25))}
                        style={{ fontSize: "13px", padding: "0.2rem 0.5rem", lineHeight: 1 }}>＋</button>
                    <span style={{ fontSize: "11px", color: "var(--text-tertiary)", minWidth: "40px", textAlign: "center" }}>
                        {Math.round(zoom * 100)}%
                    </span>
                    <button className="btn btn-secondary" onClick={() => setZoom((z) => Math.max(0.2, z - 0.25))}
                        style={{ fontSize: "13px", padding: "0.2rem 0.5rem", lineHeight: 1 }}>ー</button>
                    <button className="btn btn-secondary" onClick={reset}
                        style={{ fontSize: "11px", padding: "0.2rem 0.5rem" }}>Reset</button>
                </div>
            </div>

            {/* Author Legend */}
            {authors.length > 0 && (
                <div style={{
                    display: "flex", flexWrap: "wrap", gap: "0.5rem", marginBottom: "0.75rem",
                    padding: "0.5rem 0.75rem", background: "var(--bg-secondary)", borderRadius: "6px", alignItems: "center",
                }}>
                    <span style={{ fontSize: "11px", color: "var(--text-tertiary)", fontWeight: 600, marginRight: "0.25rem" }}>Authors:</span>
                    {authors.map((author, i) => (
                        <span key={author} style={{
                            display: "inline-flex", alignItems: "center", gap: "0.3rem",
                            fontSize: "11px", color: "var(--text-secondary)",
                        }}>
                            <span style={{
                                width: "10px", height: "10px", borderRadius: "50%",
                                background: AUTHOR_COLORS[i % AUTHOR_COLORS.length],
                                display: "inline-block", flexShrink: 0,
                            }} />
                            {author}
                        </span>
                    ))}
                </div>
            )}

            <div
                onWheel={handleWheel}
                onMouseDown={handleMouseDown}
                onMouseMove={handleMouseMove}
                onMouseUp={handleMouseUp}
                onMouseLeave={handleMouseUp}
                style={{
                    background: "var(--bg-tertiary)",
                    borderRadius: "8px",
                    overflow: "hidden",
                    cursor: dragging ? "grabbing" : "grab",
                    position: "relative",
                    height: "600px",
                    userSelect: "none",
                }}
            >
                <div style={{
                    transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
                    transformOrigin: "0 0",
                    transition: dragging ? "none" : "transform 0.15s ease",
                    padding: "1.5rem",
                    minWidth: "fit-content",
                }}>
                    <div
                        id={`mermaid-${id}`}
                        className="mermaid"
                    />
                </div>
            </div>
        </div>
    );
}

