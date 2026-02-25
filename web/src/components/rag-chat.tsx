"use client";

import { useState, useEffect, useRef } from "react";

interface Message {
    role: "user" | "assistant";
    content: string;
}

interface RAGChatProps {
    isOpen: boolean;
    onClose: () => void;
}

export function RAGChat({ isOpen, onClose }: RAGChatProps) {
    const [query, setQuery] = useState("");
    const [messages, setMessages] = useState<Message[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const inputRef = useRef<HTMLInputElement>(null);
    const messagesRef = useRef<HTMLDivElement>(null);

    // Focus input when opened
    useEffect(() => {
        if (isOpen) {
            setTimeout(() => inputRef.current?.focus(), 100);
        }
    }, [isOpen]);

    // Scroll to bottom on new messages
    useEffect(() => {
        if (messagesRef.current) {
            messagesRef.current.scrollTop = messagesRef.current.scrollHeight;
        }
    }, [messages]);

    // Handle keyboard shortcut
    useEffect(() => {
        const handler = (e: KeyboardEvent) => {
            if ((e.metaKey || e.ctrlKey) && e.key === "k") {
                e.preventDefault();
                if (isOpen) {
                    onClose();
                }
            }
            if (e.key === "Escape" && isOpen) {
                onClose();
            }
        };
        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, [isOpen, onClose]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!query.trim() || isLoading) return;

        const userMessage = query.trim();
        setQuery("");
        setMessages((prev) => [...prev, { role: "user", content: userMessage }]);
        setIsLoading(true);

        // Simulate AI response (in production, call /api/v1/rag/query)
        setTimeout(() => {
            setMessages((prev) => [
                ...prev,
                {
                    role: "assistant",
                    content: `Based on the codebase analysis, here's what I found about "${userMessage}":\n\nThe implementation follows a Clean Architecture pattern with clear separation between domain models, port interfaces, and adapter implementations. The relevant code can be found in \`internal/port/\` and \`internal/adapter/\` directories.\n\n**Sources:** auth.go, ai.go, analysis.go`,
                },
            ]);
            setIsLoading(false);
        }, 1500);
    };

    if (!isOpen) return null;

    return (
        <div className="chat-overlay" onClick={onClose}>
            <div className="chat-panel" onClick={(e) => e.stopPropagation()}>
                {/* Search Header */}
                <form className="chat-search" onSubmit={handleSubmit}>
                    <svg className="chat-search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                        <circle cx="11" cy="11" r="8" />
                        <path d="m21 21-4.35-4.35" />
                    </svg>
                    <input
                        ref={inputRef}
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        placeholder="Ask about your codebase..."
                        autoFocus
                    />
                    <span className="chat-search-shortcut">ESC</span>
                </form>

                {/* Messages */}
                {messages.length > 0 && (
                    <div className="chat-messages" ref={messagesRef}>
                        {messages.map((msg, i) => (
                            <div key={i} className={`chat-message ${msg.role}`}>
                                {msg.content}
                            </div>
                        ))}
                        {isLoading && (
                            <div className="chat-message assistant" style={{ opacity: 0.6 }}>
                                <span style={{ animation: "pulse 1s infinite" }}>⟐</span> Searching codebase...
                            </div>
                        )}
                    </div>
                )}

                {/* Empty state */}
                {messages.length === 0 && (
                    <div style={{ padding: "2rem", textAlign: "center", color: "var(--text-tertiary)" }}>
                        <p style={{ fontSize: "14px", marginBottom: "0.5rem" }}>
                            Search your codebase with AI
                        </p>
                        <p style={{ fontSize: "12px" }}>
                            Try: &quot;How does the auth middleware work?&quot; or &quot;Show me the analysis strategies&quot;
                        </p>
                    </div>
                )}
            </div>
        </div>
    );
}
