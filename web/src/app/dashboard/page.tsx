"use client";

import { useAuth } from "@/lib/use-auth";
import { api } from "@/lib/api";
import { useEffect, useState } from "react";

interface AuditLog {
    timestamp: string;
    action: string;
    resource: string;
    user_id: string;
    details: string;
}

export default function DashboardPage() {
    const { token } = useAuth();
    const [logs, setLogs] = useState<AuditLog[]>([]);
    const [repoCount, setRepoCount] = useState(0);

    useEffect(() => {
        if (!token) return;

        // Fetch real audit logs for the live feed
        api<{ logs: AuditLog[]; count: number }>("/api/v1/stream/logs", { token })
            .then((data) => setLogs(data.logs || []))
            .catch(() => { });

        // Fetch real repos count
        api<{ repos: unknown[] }>("/api/v1/repos", { token })
            .then((data) => setRepoCount(data.repos?.length || 0))
            .catch(() => { });
    }, [token]);

    return (
        <>
            <div style={{ marginBottom: "2rem" }}>
                <h1 className="page-title">Dashboard</h1>
                <p className="page-subtitle">Overview of your code observability metrics</p>
            </div>

            {/* Stats Grid */}
            <div className="stats-grid">
                <div className="card stat-card hover-lift">
                    <div className="stat-label">Repositories</div>
                    <div className="stat-value">{repoCount}</div>
                    <div className="stat-change">tracked</div>
                </div>
                <div className="card stat-card hover-lift">
                    <div className="stat-label">Audit Events</div>
                    <div className="stat-value">{logs.length}</div>
                    <div className="stat-change">recent</div>
                </div>
                <div className="card stat-card hover-lift">
                    <div className="stat-label">Embeddings</div>
                    <div className="stat-value">—</div>
                    <div className="stat-change">pending setup</div>
                </div>
                <div className="card stat-card hover-lift">
                    <div className="stat-label">AI Queries</div>
                    <div className="stat-value">—</div>
                    <div className="stat-change">pending setup</div>
                </div>
            </div>

            {/* Live Log Console — real data */}
            <div className="log-console">
                <div className="log-header">
                    <div className="log-header-title">
                        <span className="log-dot" />
                        Live Audit Feed
                    </div>
                    <a href="/dashboard/audit" style={{ fontSize: "12px", color: "var(--accent)" }}>
                        View all →
                    </a>
                </div>
                <div className="log-body">
                    {logs.length === 0 && (
                        <div className="log-line">
                            <span className="log-message" style={{ color: "#484f58" }}>
                                No audit events yet. Login activity and API calls will appear here.
                            </span>
                        </div>
                    )}
                    {logs.map((log, i) => (
                        <div className="log-line" key={i}>
                            <span className="log-time">
                                {new Date(log.timestamp).toLocaleTimeString()}
                            </span>
                            <span className="log-level-info">
                                {log.action.toUpperCase().padEnd(14)}
                            </span>
                            <span className="log-message">
                                {log.resource} {log.user_id !== "anonymous" ? `(${log.user_id.slice(0, 8)}…)` : ""}
                            </span>
                        </div>
                    ))}
                </div>
            </div>
        </>
    );
}
