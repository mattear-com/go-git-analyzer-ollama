"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { setToken } from "@/lib/api";

function AuthCallbackContent() {
    const router = useRouter();
    const searchParams = useSearchParams();

    useEffect(() => {
        const token = searchParams.get("token");
        if (token) {
            setToken(token);
            router.push("/dashboard");
        } else {
            router.push("/login");
        }
    }, [router, searchParams]);

    return (
        <div className="page-center">
            <div style={{ textAlign: "center" }}>
                <div className="login-logo" style={{ margin: "0 auto 1rem" }}>⟐</div>
                <p style={{ color: "var(--text-secondary)" }}>Authenticating...</p>
            </div>
        </div>
    );
}

export default function AuthCallbackPage() {
    return (
        <Suspense fallback={
            <div className="page-center">
                <p style={{ color: "var(--text-secondary)" }}>Loading...</p>
            </div>
        }>
            <AuthCallbackContent />
        </Suspense>
    );
}
