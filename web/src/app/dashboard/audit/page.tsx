"use client";

import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/lib/use-auth";
import { api } from "@/lib/api";

interface AuditLog {
    timestamp: string;
    action: string;
    resource: string;
    user_id: string;
    details: string;
}

const actionBadge: Record<string, string> = {
    login: "badge-info",
    logout: "badge-info",
    http_request: "badge-accent",
    repo_access: "badge-accent",
    repo_clone: "badge-success",
    analysis_run: "badge-warning",
    rag_query: "badge-accent",
    mcp_call: "badge-danger",
};

export default function AuditPage() {
    const { token } = useAuth();
    const [logs, setLogs] = useState<AuditLog[]>([]);
    const [isLive, setIsLive] = useState(true);

    const fetchLogs = useCallback(() => {
        if (!token) return;
        api<{ logs: AuditLog[]; count: number }>("/api/v1/stream/logs", { token })
            .then((data) => setLogs(data.logs || []))
            .catch(() => { });
    }, [token]);

    useEffect(() => {
        fetchLogs();
    }, [fetchLogs]);

    // Live polling
    useEffect(() => {
        if (!isLive) return;
        const interval = setInterval(fetchLogs, 5000);
        return () => clearInterval(interval);
    }, [isLive, fetchLogs]);

    return (
        <>
            <div style={{ marginBottom: "2rem", display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                <div>
                    <h1 className="page-title">Audit Logs</h1>
                    <p className="page-subtitle">Compliance & security event tracking — real-time</p>
                </div>
                <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                    <button
                        className={`btn ${isLive ? "btn-primary" : "btn-secondary"}`}
                        onClick={() => setIsLive(!isLive)}
                        style={{ fontSize: "12px", padding: "0.5rem 1rem" }}
                    >
                        <span className="log-dot" style={{ background: isLive ? "#22c55e" : "var(--text-tertiary)", animation: isLive ? "pulse 2s infinite" : "none" }} />
                        {isLive ? "Live" : "Paused"}
                    </button>
                </div>
            </div>

            {/* Audit Table */}
            <div className="table-container">
                <table className="table">
                    <thead>
                        <tr>
                            <th>Timestamp</th>
                            <th>User</th>
                            <th>Action</th>
                            <th>Resource</th>
                        </tr>
                    </thead>
                    <tbody>
                        {logs.length === 0 && (
                            <tr>
                                <td colSpan={4} style={{ textAlign: "center", color: "var(--text-tertiary)", padding: "2rem" }}>
                                    No audit events yet
                                </td>
                            </tr>
                        )}
                        {logs.map((log, i) => (
                            <tr key={i}>
                                <td style={{ fontVariantNumeric: "tabular-nums", whiteSpace: "nowrap" }}>
                                    {new Date(log.timestamp).toLocaleString()}
                                </td>
                                <td>
                                    <code style={{ fontSize: "12px", background: "var(--bg-tertiary)", padding: "0.125rem 0.375rem", borderRadius: "4px" }}>
                                        {log.user_id === "anonymous" ? "anon" : log.user_id.slice(0, 8) + "…"}
                                    </code>
                                </td>
                                <td>
                                    <span className={`badge ${actionBadge[log.action] || "badge-accent"}`}>
                                        {log.action}
                                    </span>
                                </td>
                                <td style={{ color: "var(--text-secondary)" }}>{log.resource}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>

            <div style={{ marginTop: "1rem", fontSize: "12px", color: "var(--text-tertiary)" }}>
                Showing {logs.length} events {isLive && "• Auto-refresh every 5s"}
            </div>
        </>
    );
}
