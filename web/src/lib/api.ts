export const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

type RequestOptions = {
    method?: string;
    body?: unknown;
    token?: string;
};

export async function api<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
    const { method = "GET", body, token } = options;

    const headers: Record<string, string> = {
        "Content-Type": "application/json",
    };

    if (token) {
        headers["Authorization"] = `Bearer ${token}`;
    }

    const res = await fetch(`${API_BASE}${endpoint}`, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
    });

    if (!res.ok) {
        const error = await res.json().catch(() => ({ error: "Request failed" }));
        throw new Error(error.error || `HTTP ${res.status}`);
    }

    return res.json();
}

export function getToken(): string | null {
    if (typeof window === "undefined") return null;
    return localStorage.getItem("codelens_token");
}

export function setToken(token: string): void {
    localStorage.setItem("codelens_token", token);
}

export function clearToken(): void {
    localStorage.removeItem("codelens_token");
}
