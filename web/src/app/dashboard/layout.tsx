"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ThemeToggle } from "@/components/theme-toggle";
import { useAuth } from "@/lib/use-auth";
import { clearToken } from "@/lib/api";
import { useRouter } from "next/navigation";
import { JobProvider, FloatingJobTracker } from "@/components/FloatingJobTracker";

export default function DashboardLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const pathname = usePathname();
    const router = useRouter();
    const { isLoading } = useAuth();

    const navItems = [
        { href: "/dashboard", label: "Overview", icon: "◈" },
        { href: "/dashboard/repos", label: "Repositories", icon: "⊞" },
        { href: "/dashboard/reports", label: "Reports", icon: "◉" },
        { href: "/dashboard/audit", label: "Audit Logs", icon: "⊡" },
    ];

    const handleLogout = () => {
        clearToken();
        router.push("/login");
    };

    // Show loading while checking auth
    if (isLoading) {
        return (
            <div className="page-center">
                <div style={{ textAlign: "center" }}>
                    <div className="login-logo" style={{ margin: "0 auto 1rem" }}>⟐</div>
                    <p style={{ color: "var(--text-secondary)" }}>Loading...</p>
                </div>
            </div>
        );
    }

    return (
        <JobProvider>
            <div className="dashboard-layout">
                {/* Sidebar */}
                <aside className="sidebar">
                    <div className="sidebar-header">
                        <div className="sidebar-logo">⟐</div>
                        <span className="sidebar-title">CodeLens AI</span>
                    </div>

                    <nav className="sidebar-nav">
                        {navItems.map((item) => (
                            <Link
                                key={item.href}
                                href={item.href}
                                className={`nav-item ${pathname === item.href ? "active" : ""}`}
                            >
                                <span className="nav-icon">{item.icon}</span>
                                {item.label}
                            </Link>
                        ))}
                    </nav>

                    <div className="sidebar-footer">
                        <button
                            onClick={handleLogout}
                            className="nav-item"
                            style={{ width: "100%", marginBottom: "0.5rem", border: "none", background: "none", cursor: "pointer", font: "inherit" }}
                        >
                            <span className="nav-icon">⏻</span>
                            Logout
                        </button>
                        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                            <span style={{ fontSize: "12px", color: "var(--text-tertiary)" }}>v1.0.0</span>
                            <ThemeToggle />
                        </div>
                    </div>
                </aside>

                {/* Main */}
                <main className="main-content">
                    <header className="top-header">
                        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
                            <span style={{ color: "var(--text-secondary)", fontSize: "13px" }}>
                                {navItems.find((item) => pathname === item.href)?.label || "Dashboard"}
                            </span>
                        </div>
                        <div />
                    </header>

                    <div className="page-content fade-in">
                        {children}
                    </div>
                </main>

                {/* Floating job tracker — visible on all pages */}
                <FloatingJobTracker />
            </div>
        </JobProvider>
    );
}
