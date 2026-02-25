"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getToken } from "@/lib/api";

/**
 * useAuth — Client-side auth guard hook.
 * Redirects to /login if no token is present.
 * Returns { token, isLoading } so pages can show loading state.
 */
export function useAuth() {
    const router = useRouter();
    const [token, setToken] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(true);

    useEffect(() => {
        const stored = getToken();
        if (!stored) {
            router.replace("/login");
        } else {
            setToken(stored);
            setIsLoading(false);
        }
    }, [router]);

    return { token, isLoading };
}
