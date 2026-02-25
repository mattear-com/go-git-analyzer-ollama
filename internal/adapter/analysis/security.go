package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
)

type SecurityStrategy struct {
	ai port.AIProvider
}

func NewSecurityStrategy(ai port.AIProvider) *SecurityStrategy {
	return &SecurityStrategy{ai: ai}
}

func (s *SecurityStrategy) Name() string { return "security" }
func (s *SecurityStrategy) Description() string {
	return "Security audit: secrets, injection, vulnerabilities"
}

func (s *SecurityStrategy) Analyze(ctx context.Context, req port.AnalysisRequest) (*port.AnalysisResult, error) {
	systemPrompt := `You are an elite application security auditor (OWASP certified). Analyze the provided codebase and produce a comprehensive security audit report in Markdown.

Your report MUST cover these categories:

## 🔑 Leaked Secrets & Sensitive Data
- Hardcoded API keys, tokens, passwords, connection strings
- .env files checked into version control
- Private keys, certificates, or credentials in code
- Sensitive data in logs, comments, or error messages

## 💉 Injection Vulnerabilities
- SQL injection (raw queries, string concatenation)
- NoSQL injection
- Command injection (os.exec, subprocess, etc.)
- XSS (Cross-Site Scripting) — unescaped user input in HTML/templates
- LDAP injection, XML injection, template injection

## 🔐 Authentication & Authorization
- Missing or weak authentication checks
- Broken access control (IDOR, privilege escalation)
- JWT issues (weak secrets, no expiration, algorithm confusion)
- Session management problems
- Missing CSRF protection

## 🌐 API & Network Security
- Missing input validation
- Missing rate limiting
- CORS misconfiguration
- Insecure HTTP (no TLS enforcement)
- Exposed internal endpoints

## 📦 Dependency & Configuration
- Known vulnerable dependencies
- Insecure default configurations
- Debug mode in production code
- Missing security headers

## 🛡️ Data Protection
- Sensitive data not encrypted at rest
- Missing data sanitization
- PII exposure risks
- Insecure file uploads

Format rules:
- Use severity indicators: 🔴 CRITICAL, 🟠 HIGH, 🟡 MEDIUM, 🟢 LOW
- Reference specific files, line descriptions, and functions
- Provide remediation suggestions for each finding
- Include code examples showing the vulnerable pattern and the fix
- End with: **Security Score: X/10** (10 = most secure)`

	codeContext := make([]string, 0, len(req.Chunks)+1)
	codeContext = append(codeContext, fmt.Sprintf("Repository: %s\n\nFile tree:\n%s", req.RepoName, formatFileTree(req.FileTree)))
	codeContext = append(codeContext, req.Chunks...)

	response, err := s.ai.Chat(ctx, systemPrompt, "Perform an exhaustive security audit of this codebase. Look for leaked secrets, injection vulnerabilities, authentication bypasses, and all OWASP Top 10 issues. Produce a detailed Markdown report.", codeContext)
	if err != nil {
		return nil, fmt.Errorf("security analysis: %w", err)
	}

	return &port.AnalysisResult{
		Strategy: s.Name(),
		Summary:  response,
		Details:  json.RawMessage("{}"),
		Score:    extractScore(response),
	}, nil
}
