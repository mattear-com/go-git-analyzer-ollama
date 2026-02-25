"use client";

import { useAuth } from "@/lib/use-auth";
import { api } from "@/lib/api";
import { useState, useRef, useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { MarkdownViewer } from "@/components/MarkdownViewer";

interface ChatMessage {
    role: "user" | "assistant";
    content: string;
}

function ChatContent() {
    const { token } = useAuth();
    const searchParams = useSearchParams();
    const repoId = searchParams.get("repo") || "";
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState("");
    const [loading, setLoading] = useState(false);
    const [repoName, setRepoName] = useState("");
    const messagesEndRef = useRef<HTMLDivElement>(null);

    // Fetch repo name
    useEffect(() => {
        if (!token || !repoId) return;
        api<{ repos: { id: string; name: string }[] }>("/api/v1/repos", { token }).then((data) => {
            const repo = data.repos?.find((r) => r.id === repoId);
            if (repo) setRepoName(repo.name);
        }).catch(() => { });
    }, [token, repoId]);

    useEffect(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }, [messages]);

    const handleSend = async () => {
        if (!input.trim() || !token || !repoId) return;

        const userMessage: ChatMessage = { role: "user", content: input };
        setMessages((prev) => [...prev, userMessage]);
        setInput("");
        setLoading(true);

        try {
            const result = await api<{ response: string }>(`/api/v1/chat/${repoId}`, {
                method: "POST", token,
                body: { message: input, history: messages.slice(-6) },
            });

            setMessages((prev) => [...prev, { role: "assistant", content: result.response }]);
        } catch {
            setMessages((prev) => [...prev, { role: "assistant", content: "❌ Error: Could not reach Ollama. Make sure it's running." }]);
        }
        setLoading(false);
    };

    if (!repoId) {
        return (
            <div className="card" style={{ textAlign: "center", padding: "3rem", color: "var(--text-tertiary)" }}>
                <p style={{ fontSize: "16px", marginBottom: "0.5rem" }}>No repository selected</p>
                <p style={{ fontSize: "13px" }}>Go to <a href="/dashboard/repos" style={{ color: "var(--accent)" }}>Repositories</a> and click <strong>💬 Chat</strong> on a repo.</p>
            </div>
        );
    }

    return (
        <div style={{ display: "flex", flexDirection: "column", height: "calc(100vh - 120px)" }}>
            <div style={{ marginBottom: "1rem" }}>
                <h1 className="page-title" style={{ fontSize: "18px" }}>💬 Chat with {repoName || "Repository"}</h1>
                <p className="page-subtitle">Ask questions about the codebase — powered by Ollama + analysis context</p>
            </div>

            {/* Messages area */}
            <div style={{
                flex: 1,
                overflowY: "auto",
                padding: "1rem",
                background: "var(--bg-secondary)",
                borderRadius: "12px",
                border: "1px solid var(--border-color)",
                display: "flex",
                flexDirection: "column",
                gap: "1rem",
            }}>
                {messages.length === 0 && (
                    <div style={{ textAlign: "center", color: "var(--text-tertiary)", padding: "3rem 0" }}>
                        <p style={{ fontSize: "24px", marginBottom: "0.5rem" }}>🤖</p>
                        <p style={{ fontSize: "14px" }}>Ask anything about <strong>{repoName}</strong></p>
                        <div style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem", justifyContent: "center", marginTop: "1rem" }}>
                            {["What's the architecture?", "Any security issues?", "Explain the main flow", "What tech stack is used?"].map((q) => (
                                <button key={q} className="btn btn-secondary" onClick={() => { setInput(q); }}
                                    style={{ fontSize: "12px", padding: "0.3rem 0.6rem" }}>
                                    {q}
                                </button>
                            ))}
                        </div>
                    </div>
                )}

                {messages.map((msg, i) => (
                    <div key={i} style={{
                        display: "flex",
                        justifyContent: msg.role === "user" ? "flex-end" : "flex-start",
                    }}>
                        <div style={{
                            maxWidth: "80%",
                            padding: "0.75rem 1rem",
                            borderRadius: msg.role === "user" ? "12px 12px 2px 12px" : "12px 12px 12px 2px",
                            background: msg.role === "user" ? "var(--accent)" : "var(--bg-tertiary)",
                            color: msg.role === "user" ? "white" : "var(--text-primary)",
                            fontSize: "13px",
                            lineHeight: "1.6",
                        }}>
                            {msg.role === "assistant" ? (
                                <MarkdownViewer content={msg.content} compact />
                            ) : (
                                msg.content
                            )}
                        </div>
                    </div>
                ))}

                {loading && (
                    <div style={{ display: "flex", justifyContent: "flex-start" }}>
                        <div style={{
                            padding: "0.75rem 1rem",
                            borderRadius: "12px 12px 12px 2px",
                            background: "var(--bg-tertiary)",
                            fontSize: "13px",
                            color: "var(--text-tertiary)",
                        }}>
                            <span className="typing-indicator">Thinking...</span>
                        </div>
                    </div>
                )}

                <div ref={messagesEndRef} />
            </div>

            {/* Input area */}
            <div style={{ marginTop: "1rem", display: "flex", gap: "0.5rem" }}>
                <input
                    type="text"
                    className="input"
                    placeholder={`Ask about ${repoName || "the repo"}...`}
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && handleSend()}
                    disabled={loading}
                    style={{ flex: 1 }}
                    autoFocus
                />
                <button className="btn btn-primary" onClick={handleSend} disabled={loading || !input.trim()}>
                    Send
                </button>
            </div>
        </div>
    );
}

export default function ChatPage() {
    return (
        <Suspense fallback={<div className="card" style={{ textAlign: "center", padding: "3rem" }}>Loading...</div>}>
            <ChatContent />
        </Suspense>
    );
}
