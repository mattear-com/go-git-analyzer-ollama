"use client";

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import rehypeRaw from "rehype-raw";
import mermaid from "mermaid";
import { useEffect, useRef, useState, useCallback } from "react";
import "highlight.js/styles/github-dark.css";

// Detect theme and initialize mermaid
const isDark = typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: dark)")?.matches;
mermaid.initialize({
    startOnLoad: false,
    theme: isDark ? "dark" : "default",
    securityLevel: "loose",
    fontFamily: "-apple-system, BlinkMacSystemFont, 'SF Pro Text', system-ui, sans-serif",
    themeVariables: isDark ? {} : {
        primaryColor: "#e8eaed",
        primaryTextColor: "#1a1a2e",
        primaryBorderColor: "#c0c0c0",
        lineColor: "#6b7280",
        secondaryColor: "#f3f4f6",
        tertiaryColor: "#ffffff",
        noteBkgColor: "#fff3cd",
        noteTextColor: "#1a1a2e",
        noteBorderColor: "#e0c97a",
    },
});

// Pre-process mermaid code to fix common AI-generated syntax issues
function sanitizeMermaid(code: string): string {
    let cleaned = code;

    // Fix participant labels with parentheses — must be quoted
    // e.g., "participant Client as Client (e.g., web app)" → 'participant Client as "Client (e.g., web app)"'
    cleaned = cleaned.replace(
        /^(\s*participant\s+\w+\s+as\s+)(.+\(.+\).*)$/gm,
        (_, prefix, label) => `${prefix}"${label.replace(/"/g, "'")}"`,
    );

    // Fix note labels with parentheses
    cleaned = cleaned.replace(
        /^(\s*Note\s+(?:right|left|over)\s+of\s+\w+:\s*)(.+\(.+\).*)$/gm,
        (_, prefix, label) => `${prefix}${label.replace(/[()]/g, "")}`,
    );

    // Replace special Unicode characters that mermaid can't handle
    cleaned = cleaned.replace(/≠/g, "!=");
    cleaned = cleaned.replace(/≤/g, "<=");
    cleaned = cleaned.replace(/≥/g, ">=");
    cleaned = cleaned.replace(/→/g, "->");
    cleaned = cleaned.replace(/←/g, "<-");
    cleaned = cleaned.replace(/⟶/g, "-->");
    cleaned = cleaned.replace(/⟵/g, "<--");

    // Fix "return" keyword used as a message (invalid in sequenceDiagram)
    // Turn "return error: ..." into a normal arrow or note
    cleaned = cleaned.replace(
        /^\s*return\s+(.+)$/gm,
        (_, msg) => `    Note right of Sender: ${msg}`,
    );

    // Remove empty alt/else/end blocks that can cause issues
    cleaned = cleaned.replace(/^\s*alt\s*$/gm, "alt condition");
    cleaned = cleaned.replace(/^\s*else\s*$/gm, "else otherwise");

    // Escape quotes inside labels (double quotes in messages)
    // Leave alone lines that are already properly formatted

    return cleaned;
}

// Mermaid code block renderer
function MermaidBlock({ code }: { code: string }) {
    const ref = useRef<HTMLDivElement>(null);
    const [svg, setSvg] = useState<string>("");
    const [error, setError] = useState(false);

    useEffect(() => {
        let cancelled = false;
        const id = `mermaid-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
        const sanitized = sanitizeMermaid(code);

        (async () => {
            try {
                // Validate syntax first — avoids error DOM injection
                await mermaid.parse(sanitized);
                const result = await mermaid.render(id, sanitized);
                if (!cancelled) setSvg(result.svg);
            } catch {
                if (!cancelled) setError(true);
                // Clean up any error elements mermaid may have injected
                const errorEl = document.getElementById(`d${id}`);
                if (errorEl) errorEl.remove();
            }
        })();

        return () => { cancelled = true; };
    }, [code]);

    if (error) {
        return (
            <pre className="md-code-block" style={{ color: "var(--text-secondary)", fontSize: "12px" }}>
                <code>{code}</code>
            </pre>
        );
    }

    if (!svg) return null; // loading

    return (
        <div
            ref={ref}
            className="md-mermaid"
            dangerouslySetInnerHTML={{ __html: svg }}
        />
    );
}

interface MarkdownViewerProps {
    content: string;
    compact?: boolean; // for chat messages
}

export function MarkdownViewer({ content, compact = false }: MarkdownViewerProps) {
    const containerRef = useRef<HTMLDivElement>(null);

    const renderMermaidBlocks = useCallback(() => {
        if (!containerRef.current) return;
        // Re-render any pending mermaid blocks after DOM update
        containerRef.current.querySelectorAll(".mermaid-pending").forEach(async (el, idx) => {
            try {
                const id = `mermaid-post-${Date.now()}-${idx}`;
                const { svg } = await mermaid.render(id, el.textContent || "");
                el.innerHTML = svg;
                el.classList.remove("mermaid-pending");
                el.classList.add("mermaid-rendered");
            } catch {
                el.classList.remove("mermaid-pending");
            }
        });
    }, []);

    useEffect(() => {
        renderMermaidBlocks();
    }, [content, renderMermaidBlocks]);

    return (
        <div ref={containerRef} className={compact ? "md-viewer md-compact" : "md-viewer"}>
            <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                rehypePlugins={[rehypeHighlight, rehypeRaw]}
                components={{
                    code({ className, children, ...props }) {
                        const match = /language-(\w+)/.exec(className || "");
                        const lang = match?.[1];
                        const codeStr = String(children).replace(/\n$/, "");

                        // Mermaid code blocks
                        if (lang === "mermaid" || lang === "sequence" || lang === "flowchart" || lang === "gantt" || lang === "er" || lang === "erd") {
                            return <MermaidBlock code={codeStr} />;
                        }

                        // Check if it's an inline code
                        const isInline = !className && !codeStr.includes("\n");
                        if (isInline) {
                            return <code className="md-inline-code" {...props}>{children}</code>;
                        }

                        return (
                            <code className={className} {...props}>
                                {children}
                            </code>
                        );
                    },
                    pre({ children }) {
                        return <pre className="md-code-block">{children}</pre>;
                    },
                    table({ children }) {
                        return (
                            <div className="md-table-wrapper">
                                <table className="md-table">{children}</table>
                            </div>
                        );
                    },
                    blockquote({ children }) {
                        return <blockquote className="md-blockquote">{children}</blockquote>;
                    },
                    a({ href, children }) {
                        return <a href={href} target="_blank" rel="noopener noreferrer" className="md-link">{children}</a>;
                    },
                    hr() {
                        return <hr className="md-hr" />;
                    },
                    img({ src, alt }) {
                        return <img src={src} alt={alt || ""} className="md-img" loading="lazy" />;
                    },
                }}
            >
                {content}
            </ReactMarkdown>

            <style jsx global>{`
                /* ── Premium Markdown Viewer ─────────────────────────── */
                .md-viewer {
                    font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Inter', system-ui, sans-serif;
                    font-size: 14px;
                    line-height: 1.75;
                    color: var(--text-primary);
                    -webkit-font-smoothing: antialiased;
                }

                .md-compact {
                    font-size: 13px;
                    line-height: 1.6;
                }

                /* Headings */
                .md-viewer h1 {
                    font-size: 24px;
                    font-weight: 700;
                    margin: 28px 0 14px;
                    letter-spacing: -0.5px;
                    border-bottom: 1px solid var(--border-color);
                    padding-bottom: 8px;
                }

                .md-viewer h2 {
                    font-size: 20px;
                    font-weight: 700;
                    margin: 24px 0 12px;
                    letter-spacing: -0.3px;
                    border-bottom: 1px solid var(--border-color);
                    padding-bottom: 6px;
                }

                .md-viewer h3 {
                    font-size: 17px;
                    font-weight: 600;
                    margin: 20px 0 10px;
                }

                .md-viewer h4 {
                    font-size: 15px;
                    font-weight: 600;
                    margin: 16px 0 8px;
                }

                .md-viewer h5 {
                    font-size: 14px;
                    font-weight: 600;
                    margin: 14px 0 6px;
                    color: var(--text-secondary);
                }

                .md-viewer h1:first-child,
                .md-viewer h2:first-child {
                    margin-top: 0;
                }

                .md-compact h1 { font-size: 16px; margin: 12px 0 6px; border: none; padding: 0; }
                .md-compact h2 { font-size: 15px; margin: 10px 0 5px; border: none; padding: 0; }
                .md-compact h3 { font-size: 14px; margin: 8px 0 4px; }

                /* Paragraphs */
                .md-viewer p {
                    margin: 0 0 12px;
                }

                .md-compact p {
                    margin: 0 0 6px;
                }

                /* Lists */
                .md-viewer ul, .md-viewer ol {
                    margin: 8px 0 14px 20px;
                    padding: 0;
                }

                .md-viewer li {
                    margin: 4px 0;
                }

                .md-viewer li::marker {
                    color: var(--text-tertiary);
                }

                /* Task lists */
                .md-viewer li input[type="checkbox"] {
                    margin-right: 6px;
                    accent-color: var(--accent);
                }

                /* Links */
                .md-link {
                    color: var(--accent) !important;
                    text-decoration: none;
                    font-weight: 500;
                }

                .md-link:hover {
                    text-decoration: underline;
                }

                /* Inline code */
                .md-inline-code {
                    font-family: 'SF Mono', Menlo, 'Fira Code', monospace;
                    font-size: 0.875em;
                    background: var(--bg-tertiary);
                    padding: 2px 6px;
                    border-radius: 4px;
                    color: #e5a00d;
                    border: 1px solid var(--border-color);
                }

                /* Code blocks */
                .md-code-block {
                    background: var(--bg-tertiary) !important;
                    border-radius: 10px;
                    padding: 16px 20px;
                    margin: 14px 0;
                    overflow-x: auto;
                    border: 1px solid var(--border-color);
                    font-size: 13px;
                    line-height: 1.55;
                }

                .md-code-block code {
                    font-family: 'SF Mono', Menlo, 'Fira Code', monospace !important;
                    background: none !important;
                    padding: 0 !important;
                    border: none !important;
                    font-size: 13px !important;
                    color: var(--text-primary) !important;
                }

                /* hljs override for dark backgrounds */
                .md-code-block .hljs {
                    background: transparent !important;
                    padding: 0 !important;
                }

                /* Tables */
                .md-table-wrapper {
                    overflow-x: auto;
                    margin: 14px 0;
                    border-radius: 8px;
                    border: 1px solid var(--border-color);
                }

                .md-table {
                    border-collapse: collapse;
                    width: 100%;
                    font-size: 13px;
                }

                .md-table th, .md-table td {
                    border: 1px solid var(--border-color);
                    padding: 10px 14px;
                    text-align: left;
                }

                .md-table th {
                    background: var(--bg-tertiary);
                    font-weight: 600;
                    font-size: 12px;
                    text-transform: uppercase;
                    letter-spacing: 0.3px;
                    color: var(--text-secondary);
                }

                .md-table tr:hover td {
                    background: rgba(255, 255, 255, 0.02);
                }

                /* Blockquotes */
                .md-blockquote {
                    border-left: 3px solid var(--accent);
                    padding: 8px 16px;
                    margin: 14px 0;
                    color: var(--text-secondary);
                    background: rgba(var(--accent-rgb, 59, 130, 246), 0.05);
                    border-radius: 0 6px 6px 0;
                    font-style: italic;
                }

                .md-blockquote p {
                    margin: 4px 0;
                }

                /* Horizontal rule */
                .md-hr {
                    border: none;
                    border-top: 1px solid var(--border-color);
                    margin: 24px 0;
                }

                /* Images */
                .md-img {
                    max-width: 100%;
                    border-radius: 8px;
                    margin: 14px 0;
                    display: block;
                    border: 1px solid var(--border-color);
                }

                /* Mermaid diagrams */
                .md-mermaid {
                    margin: 16px 0;
                    text-align: center;
                    background: var(--bg-tertiary);
                    border-radius: 10px;
                    padding: 20px;
                    border: 1px solid var(--border-color);
                    overflow-x: auto;
                }

                .md-mermaid svg {
                    max-width: 100%;
                    height: auto;
                }

                /* Light mode: ensure mermaid text is readable */
                @media (prefers-color-scheme: light) {
                    .md-mermaid {
                        background: #fafafa;
                        border-color: #e0e0e0;
                    }
                    .md-mermaid text {
                        fill: #1a1a2e !important;
                    }
                    .md-mermaid .nodeLabel,
                    .md-mermaid .edgeLabel,
                    .md-mermaid .label {
                        color: #1a1a2e !important;
                    }
                }

                /* Strong / Bold */
                .md-viewer strong {
                    font-weight: 600;
                    color: var(--text-primary);
                }

                /* Emphasis */
                .md-viewer em {
                    font-style: italic;
                }

                /* Strikethrough */
                .md-viewer del {
                    text-decoration: line-through;
                    color: var(--text-tertiary);
                }

                /* Emoji severity */
                .md-viewer .severity-high { color: #ef4444; }
                .md-viewer .severity-medium { color: #eab308; }
                .md-viewer .severity-low { color: #22c55e; }
            `}</style>
        </div>
    );
}
