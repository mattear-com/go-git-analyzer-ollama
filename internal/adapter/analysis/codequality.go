package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
)

type CodeQualityStrategy struct {
	ai port.AIProvider
}

func NewCodeQualityStrategy(ai port.AIProvider) *CodeQualityStrategy {
	return &CodeQualityStrategy{ai: ai}
}

func (s *CodeQualityStrategy) Name() string        { return "code_quality" }
func (s *CodeQualityStrategy) Description() string { return "Code quality, bugs, and security review" }

func (s *CodeQualityStrategy) Analyze(ctx context.Context, req port.AnalysisRequest) (*port.AnalysisResult, error) {
	systemPrompt := `You are an expert code reviewer specializing in security and quality. Analyze the provided codebase and produce a beautiful Markdown report.

Your report MUST include:
1. **Bugs & Logic Errors** — potential bugs found with file paths and line descriptions
2. **Security Vulnerabilities** — injection, auth bypasses, data leaks, etc.
3. **Code Smells** — anti-patterns, dead code, duplications
4. **Refactoring Opportunities** — specific improvements with before/after descriptions
5. **Quality Score** — rate 0-10 with justification

Format rules:
- Use Markdown headings (##), bullet points, bold, code blocks
- Reference specific files and functions you found in the code
- Use severity indicators: 🔴 Critical, 🟡 Warning, 🟢 Info
- End with: **Score: X/10**`

	codeContext := make([]string, 0, len(req.Chunks)+1)
	codeContext = append(codeContext, fmt.Sprintf("Repository: %s\n\nFile tree:\n%s", req.RepoName, formatFileTree(req.FileTree)))
	codeContext = append(codeContext, req.Chunks...)

	response, err := s.ai.Chat(ctx, systemPrompt, "Perform a comprehensive code quality and security review of this codebase. Produce a Markdown report.", codeContext)
	if err != nil {
		return nil, fmt.Errorf("code quality analysis: %w", err)
	}

	return &port.AnalysisResult{
		Strategy: s.Name(),
		Summary:  response,
		Details:  json.RawMessage("{}"),
		Score:    extractScore(response),
	}, nil
}
