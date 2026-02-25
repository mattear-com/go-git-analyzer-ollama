import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "CodeLens AI — Code Observability Platform",
  description: "Premium code observability platform with RAG-powered analysis, real-time insights, and AI-driven code intelligence.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" data-theme="dark" suppressHydrationWarning>
      <body>
        <div className="bg-glow bg-glow-1" />
        <div className="bg-glow bg-glow-2" />
        {children}
      </body>
    </html>
  );
}
