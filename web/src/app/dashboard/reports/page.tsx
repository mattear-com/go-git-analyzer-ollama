"use client";

import { useAuth } from "@/lib/use-auth";
import { api } from "@/lib/api";
import { useEffect, useState, useCallback } from "react";
import { MarkdownViewer } from "@/components/MarkdownViewer";

interface AnalysisReport {
    id: string;
    repo_id: string;
    strategy: string;
    summary: string;
    summary_translated: string;
    details: string;
    score: number;
    created_at: string;
}

export default function ReportsPage() {
    const { token } = useAuth();
    const [reports, setReports] = useState<AnalysisReport[]>([]);
    const [repoMap, setRepoMap] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(true);
    const [expandedId, setExpandedId] = useState<string | null>(null);
    const [filterRepo, setFilterRepo] = useState<string>("all");
    const [showTranslated, setShowTranslated] = useState<Record<string, boolean>>({});
    const [searchQuery, setSearchQuery] = useState("");
    const [expandedStrategy, setExpandedStrategy] = useState<Record<string, boolean>>({});
    const [deleting, setDeleting] = useState<Record<string, boolean>>({});

    const handleDeleteReports = async (repoId: string, repoName: string) => {
        if (!token) return;
        if (!confirm(`Delete ALL reports and embeddings for "${repoName}"? This cannot be undone.`)) return;
        setDeleting((prev) => ({ ...prev, [repoId]: true }));
        try {
            await api(`/api/v1/reports/${repoId}`, { method: "DELETE", token });
            fetchReports();
        } catch {
            alert("Failed to delete reports");
        }
        setDeleting((prev) => ({ ...prev, [repoId]: false }));
    };

    const fetchReports = useCallback(async () => {
        if (!token) return;
        setLoading(true);
        try {
            const data = await api<{ results: AnalysisReport[]; repo_map: Record<string, string> }>("/api/v1/reports", { token });
            setReports(data.results || []);
            setRepoMap(data.repo_map || {});
        } catch { /* */ }
        setLoading(false);
    }, [token]);

    useEffect(() => { fetchReports(); }, [fetchReports]);

    // Debounced search
    useEffect(() => {
        if (!token) return;
        if (!searchQuery.trim()) {
            fetchReports();
            return;
        }
        const timer = setTimeout(async () => {
            try {
                const params = new URLSearchParams({ q: searchQuery.trim() });
                if (filterRepo !== "all") params.set("repo_id", filterRepo);
                const data = await api<{ results: AnalysisReport[]; repo_map: Record<string, string> }>(
                    `/api/v1/reports/search?${params}`, { token }
                );
                setReports(data.results || []);
                setRepoMap((prev) => ({ ...prev, ...(data.repo_map || {}) }));
            } catch { /* */ }
        }, 300);
        return () => clearTimeout(timer);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [searchQuery, filterRepo, token]);

    const groupedByRepo = reports.reduce<Record<string, AnalysisReport[]>>((acc, r) => {
        if (!acc[r.repo_id]) acc[r.repo_id] = [];
        acc[r.repo_id].push(r);
        return acc;
    }, {});

    const repoIds = Object.keys(groupedByRepo);
    const filteredRepoIds = filterRepo === "all" ? repoIds : repoIds.filter((id) => id === filterRepo);

    const scoreBadge = (score: number) => score >= 7 ? "badge-success" : score >= 4 ? "badge-warning" : "badge-danger";

    const toggleTranslated = (id: string) => {
        setShowTranslated((prev) => ({ ...prev, [id]: !prev[id] }));
    };

    return (
        <>
            <div style={{ marginBottom: "2rem", display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                <div>
                    <h1 className="page-title">Analysis Reports</h1>
                    <p className="page-subtitle">{reports.length} reports across {repoIds.length} repositories</p>
                </div>
                <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                    <div style={{ position: "relative" }}>
                        <span style={{ position: "absolute", left: "0.65rem", top: "50%", transform: "translateY(-50%)", color: "var(--text-tertiary)", fontSize: "13px", pointerEvents: "none" }}>⌕</span>
                        <input
                            className="input"
                            type="text"
                            placeholder="Search reports..."
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                            style={{ fontSize: "12px", padding: "0.4rem 0.6rem 0.4rem 1.75rem", width: "220px" }}
                        />
                    </div>
                    <select value={filterRepo} onChange={(e) => setFilterRepo(e.target.value)} className="input"
                        style={{ fontSize: "12px", padding: "0.4rem 0.6rem", width: "auto" }}>
                        <option value="all">All Repos</option>
                        {repoIds.map((id) => <option key={id} value={id}>{repoMap[id] || id.slice(0, 8)}</option>)}
                    </select>
                    <button className="btn btn-secondary" onClick={fetchReports} style={{ fontSize: "12px" }}>↻ Refresh</button>
                </div>
            </div>

            {loading ? (
                <div className="card" style={{ textAlign: "center", padding: "3rem" }}>
                    <p style={{ color: "var(--text-secondary)" }}>Loading reports...</p>
                </div>
            ) : reports.length === 0 ? (
                <div className="card" style={{ textAlign: "center", padding: "3rem", color: "var(--text-tertiary)" }}>
                    <p style={{ fontSize: "16px", marginBottom: "0.5rem" }}>No reports yet</p>
                    <p style={{ fontSize: "13px" }}>Go to <strong>Repositories</strong>, clone a repo, and click <strong>▶ Analyze</strong>.</p>
                </div>
            ) : (
                <div style={{ display: "flex", flexDirection: "column", gap: "1.5rem" }}>
                    {filteredRepoIds.map((repoId) => {
                        const repoReports = groupedByRepo[repoId];
                        const repoName = repoMap[repoId] || repoId.slice(0, 8);

                        // Group by run (within 10 min = same run)
                        const runs: AnalysisReport[][] = [];
                        let currentRun: AnalysisReport[] = [];
                        repoReports.forEach((r, i) => {
                            if (i === 0) { currentRun.push(r); }
                            else {
                                const prev = new Date(repoReports[i - 1].created_at).getTime();
                                const curr = new Date(r.created_at).getTime();
                                if (Math.abs(prev - curr) < 10 * 60 * 1000) { currentRun.push(r); }
                                else { runs.push(currentRun); currentRun = [r]; }
                            }
                        });
                        if (currentRun.length > 0) runs.push(currentRun);

                        return (
                            <div key={repoId}>
                                <h2 style={{ fontSize: "16px", fontWeight: 600, marginBottom: "0.75rem", display: "flex", alignItems: "center", gap: "0.5rem" }}>
                                    📦 {repoName}
                                    <span className="badge badge-accent" style={{ fontSize: "10px" }}>{repoReports.length} reports</span>
                                    <div style={{ marginLeft: "auto", display: "flex", gap: "0.5rem" }}>
                                        <a href={`/dashboard/chat?repo=${repoId}`} className="btn btn-secondary" style={{ fontSize: "11px", padding: "0.25rem 0.5rem", textDecoration: "none" }}>
                                            💬 Chat about this repo
                                        </a>
                                        <button
                                            className="btn btn-secondary"
                                            onClick={() => handleDeleteReports(repoId, repoName)}
                                            disabled={deleting[repoId]}
                                            style={{ fontSize: "11px", padding: "0.25rem 0.5rem", color: "var(--danger)" }}
                                        >
                                            {deleting[repoId] ? "⏳ Deleting..." : "🗑️ Delete Reports"}
                                        </button>
                                    </div>
                                </h2>

                                {runs.map((run, runIdx) => {
                                    const avgScore = run.reduce((sum, r) => sum + r.score, 0) / Math.max(run.length, 1);
                                    const runDate = new Date(run[0].created_at);
                                    const runKey = `${repoId}-${runIdx}`;

                                    return (
                                        <div key={runIdx} className="card" style={{ marginBottom: "0.75rem" }}>
                                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", cursor: "pointer" }}
                                                onClick={() => setExpandedId(expandedId === runKey ? null : runKey)}>
                                                <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
                                                    <span style={{ fontSize: "13px", color: "var(--text-secondary)" }}>{expandedId === runKey ? "▼" : "▶"}</span>
                                                    <span style={{ fontSize: "14px", fontWeight: 500 }}>
                                                        Run {runs.length - runIdx} — {runDate.toLocaleDateString()} {runDate.toLocaleTimeString()}
                                                    </span>
                                                    {avgScore > 0 && <span className={`badge ${scoreBadge(avgScore)}`}>avg {avgScore.toFixed(1)}/10</span>}
                                                    <span className="badge badge-accent" style={{ fontSize: "10px" }}>{run.length} strategies</span>
                                                </div>
                                            </div>

                                            {expandedId === runKey && (
                                                <div style={{ marginTop: "1rem", borderTop: "1px solid var(--border-color)", paddingTop: "1rem" }}>
                                                    {run.map((result) => {
                                                        const hasTranslation = !!result.summary_translated;
                                                        const showing = showTranslated[result.id] && hasTranslation ? result.summary_translated : result.summary;
                                                        const strategyKey = `${runKey}-${result.id}`;
                                                        const isStrategyExpanded = expandedStrategy[strategyKey] ?? false;
                                                        const strategyIcons: Record<string, string> = {
                                                            architecture: "🏗️", code_quality: "🔍", functionality: "⚙️",
                                                            devops: "🚀", security: "🛡️",
                                                        };

                                                        return (
                                                            <div key={result.id} style={{ marginBottom: "0.5rem" }}>
                                                                <div
                                                                    onClick={() => setExpandedStrategy(prev => ({ ...prev, [strategyKey]: !isStrategyExpanded }))}
                                                                    style={{
                                                                        display: "flex", alignItems: "center", gap: "0.75rem",
                                                                        padding: "0.65rem 0.85rem", cursor: "pointer",
                                                                        background: "var(--bg-tertiary)", borderRadius: "8px",
                                                                        border: "1px solid var(--border-color)",
                                                                        transition: "background 0.15s",
                                                                    }}
                                                                >
                                                                    <span style={{ fontSize: "13px", color: "var(--text-secondary)", width: "14px" }}>
                                                                        {isStrategyExpanded ? "▼" : "▶"}
                                                                    </span>
                                                                    <span style={{ fontSize: "16px" }}>{strategyIcons[result.strategy] || "📋"}</span>
                                                                    <h4 style={{ fontSize: "14px", fontWeight: 600, textTransform: "capitalize", margin: 0, flex: 1 }}>
                                                                        {result.strategy.replace(/_/g, " ")}
                                                                    </h4>
                                                                    {result.score > 0 && <span className={`badge ${scoreBadge(result.score)}`}>{result.score.toFixed(1)}/10</span>}
                                                                    {hasTranslation && (
                                                                        <button className="btn btn-secondary" onClick={(e) => { e.stopPropagation(); toggleTranslated(result.id); }}
                                                                            style={{ fontSize: "10px", padding: "0.2rem 0.5rem" }}>
                                                                            {showTranslated[result.id] ? "🇺🇸 EN" : "🌐 TR"}
                                                                        </button>
                                                                    )}
                                                                </div>
                                                                {isStrategyExpanded && (
                                                                    <div style={{ padding: "1rem", borderLeft: "2px solid var(--border-color)", marginLeft: "7px", marginTop: "0.25rem" }}>
                                                                        <MarkdownViewer content={showing} />
                                                                    </div>
                                                                )}
                                                            </div>
                                                        );
                                                    })}
                                                </div>
                                            )}
                                        </div>
                                    );
                                })}
                            </div>
                        );
                    })}
                </div>
            )}
        </>
    );
}
