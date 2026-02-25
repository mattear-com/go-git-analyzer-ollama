"use client";

import { useEffect, useState, useCallback, createContext, useContext } from "react";
import { API_BASE, api } from "@/lib/api";
import { useAuth } from "@/lib/use-auth";

// --- Job Context (shared across pages) ---
interface JobStatus {
    id: string;
    repo_id: string;
    repo_name?: string;
    status: string; // running, complete, error
    progress: number;
    total: number;
    current_strategy: string;
    completed_strategies: string[];
}

interface RepoEvent {
    repo_id: string;
    name: string;
    status: string;
}

interface JobContextType {
    jobs: Record<string, JobStatus>;
    repoEvents: RepoEvent[];
    startJob: (repoId: string, repoName: string, jobId: string) => void;
}

const JobContext = createContext<JobContextType>({
    jobs: {},
    repoEvents: [],
    startJob: () => { },
});

export function useJobs() {
    return useContext(JobContext);
}

export function JobProvider({ children }: { children: React.ReactNode }) {
    const { token } = useAuth();
    const [jobs, setJobs] = useState<Record<string, JobStatus>>({});
    const [repoEvents, setRepoEvents] = useState<RepoEvent[]>([]);

    // SSE for repo status changes
    useEffect(() => {
        if (!token) return;
        const evtSource = new EventSource(`${API_BASE}/api/v1/repos/events?token=${token}`);

        evtSource.addEventListener("repo_status", (e) => {
            const evt = JSON.parse(e.data) as RepoEvent;
            setRepoEvents((prev) => [evt, ...prev.slice(0, 19)]);
        });

        evtSource.onerror = () => {
            // Will auto-reconnect
        };

        return () => evtSource.close();
    }, [token]);

    const startJob = useCallback((repoId: string, repoName: string, jobId: string) => {
        if (!token) return;

        setJobs((prev) => ({
            ...prev,
            [repoId]: {
                id: jobId,
                repo_id: repoId,
                repo_name: repoName,
                status: "running",
                progress: 0,
                total: 4,
                current_strategy: "",
                completed_strategies: [],
            },
        }));

        // Subscribe to SSE for this job's progress
        const evtSource = new EventSource(`${API_BASE}/api/v1/jobs/${jobId}/stream?token=${token}`);

        evtSource.addEventListener("progress", (e) => {
            const data = JSON.parse(e.data) as JobStatus;
            setJobs((prev) => ({ ...prev, [repoId]: { ...data, repo_name: repoName } }));
        });

        evtSource.addEventListener("complete", (e) => {
            const data = JSON.parse(e.data) as JobStatus;
            setJobs((prev) => ({ ...prev, [repoId]: { ...data, repo_name: repoName } }));
            evtSource.close();
            // Auto-remove after 30s
            setTimeout(() => {
                setJobs((prev) => {
                    const next = { ...prev };
                    delete next[repoId];
                    return next;
                });
            }, 30000);
        });

        evtSource.addEventListener("error", () => {
            evtSource.close();
            // Fallback: poll status
            const poll = setInterval(async () => {
                try {
                    const job = await api<JobStatus>(`/api/v1/jobs/${jobId}`, { token });
                    setJobs((prev) => ({ ...prev, [repoId]: { ...job, repo_name: repoName } }));
                    if (job.status === "complete" || job.status === "error") clearInterval(poll);
                } catch { clearInterval(poll); }
            }, 3000);
        });
    }, [token]);

    return (
        <JobContext.Provider value={{ jobs, repoEvents, startJob }}>
            {children}
        </JobContext.Provider>
    );
}

// --- Floating Panel UI ---
export function FloatingJobTracker() {
    const { jobs, repoEvents } = useJobs();
    const [collapsed, setCollapsed] = useState(false);

    const activeJobs = Object.values(jobs);
    const runningJobs = activeJobs.filter((j) => j.status === "running");
    const hasActivity = activeJobs.length > 0 || repoEvents.length > 0;

    if (!hasActivity) return null;

    // Collapsed tab on right edge
    if (collapsed) {
        return (
            <button
                onClick={() => setCollapsed(false)}
                style={{
                    position: "fixed",
                    right: 0,
                    top: "50%",
                    transform: "translateY(-50%)",
                    zIndex: 9000,
                    background: runningJobs.length > 0 ? "var(--accent)" : "var(--bg-secondary)",
                    color: runningJobs.length > 0 ? "white" : "var(--text-primary)",
                    border: "1px solid var(--border-color)",
                    borderRight: "none",
                    borderRadius: "8px 0 0 8px",
                    padding: "0.75rem 0.4rem",
                    cursor: "pointer",
                    fontSize: "12px",
                    writingMode: "vertical-rl",
                    textOrientation: "mixed",
                    boxShadow: "0 2px 12px rgba(0,0,0,0.3)",
                }}
            >
                {runningJobs.length > 0 ? `⏳ ${runningJobs.length} job${runningJobs.length > 1 ? "s" : ""}` : "📋 Activity"}
            </button>
        );
    }

    return (
        <div style={{
            position: "fixed",
            right: "1rem",
            bottom: "1rem",
            zIndex: 9000,
            width: "320px",
            maxHeight: "400px",
            background: "var(--bg-secondary)",
            border: "1px solid var(--border-color)",
            borderRadius: "12px",
            boxShadow: "0 8px 32px rgba(0,0,0,0.4)",
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
        }}>
            {/* Header */}
            <div style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                padding: "0.75rem 1rem",
                borderBottom: "1px solid var(--border-color)",
                background: "var(--bg-tertiary)",
            }}>
                <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                    <span style={{ fontSize: "14px" }}>{runningJobs.length > 0 ? "⏳" : "✅"}</span>
                    <span style={{ fontSize: "13px", fontWeight: 600 }}>
                        {runningJobs.length > 0 ? `${runningJobs.length} Running` : "Activity"}
                    </span>
                </div>
                <button
                    onClick={() => setCollapsed(true)}
                    style={{
                        background: "none",
                        border: "none",
                        color: "var(--text-tertiary)",
                        cursor: "pointer",
                        fontSize: "16px",
                        padding: "0 0.25rem",
                    }}
                    title="Collapse to side tab"
                >
                    ▸
                </button>
            </div>

            {/* Job list */}
            <div style={{ overflowY: "auto", padding: "0.5rem" }}>
                {activeJobs.map((job) => (
                    <div key={job.repo_id} style={{
                        padding: "0.5rem 0.75rem",
                        borderRadius: "8px",
                        marginBottom: "0.25rem",
                        background: job.status === "running" ? "rgba(99,102,241,0.1)" : "transparent",
                    }}>
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "0.25rem" }}>
                            <span style={{ fontSize: "12px", fontWeight: 600 }}>
                                {job.repo_name || job.repo_id.slice(0, 8)}
                            </span>
                            <span className={`badge ${job.status === "running" ? "badge-warning" : job.status === "complete" ? "badge-success" : "badge-danger"}`}
                                style={{ fontSize: "9px" }}>
                                {job.status}
                            </span>
                        </div>

                        {job.status === "running" && (
                            <>
                                <div style={{
                                    background: "var(--bg-tertiary)",
                                    borderRadius: "3px",
                                    height: "3px",
                                    overflow: "hidden",
                                    marginBottom: "0.25rem",
                                }}>
                                    <div style={{
                                        width: `${(job.progress / Math.max(job.total, 1)) * 100}%`,
                                        height: "100%",
                                        background: "var(--accent)",
                                        transition: "width 0.5s ease",
                                    }} />
                                </div>
                                <p style={{ fontSize: "10px", color: "var(--text-tertiary)", margin: 0 }}>
                                    {job.current_strategy ? job.current_strategy.replace(/_/g, " ") : "Starting..."} • {job.progress}/{job.total}
                                </p>
                            </>
                        )}

                        {job.status === "complete" && (
                            <p style={{ fontSize: "10px", color: "var(--text-tertiary)", margin: 0 }}>
                                ✅ Done — <a href="/dashboard/reports" style={{ color: "var(--accent)" }}>View Reports</a>
                            </p>
                        )}
                    </div>
                ))}

                {/* Recent repo events */}
                {repoEvents.length > 0 && activeJobs.length > 0 && (
                    <div style={{ borderTop: "1px solid var(--border-color)", marginTop: "0.25rem", paddingTop: "0.25rem" }} />
                )}
                {repoEvents.slice(0, 5).map((evt, i) => (
                    <div key={`evt-${i}`} style={{ padding: "0.25rem 0.75rem" }}>
                        <p style={{ fontSize: "10px", color: "var(--text-tertiary)", margin: 0 }}>
                            {evt.status === "ready" ? "✅" : "❌"} <strong>{evt.name}</strong> → {evt.status}
                        </p>
                    </div>
                ))}
            </div>
        </div>
    );
}
