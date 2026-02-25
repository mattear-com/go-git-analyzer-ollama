package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
)

type FunctionalityStrategy struct {
	ai port.AIProvider
}

func NewFunctionalityStrategy(ai port.AIProvider) *FunctionalityStrategy {
	return &FunctionalityStrategy{ai: ai}
}

func (s *FunctionalityStrategy) Name() string { return "functionality" }
func (s *FunctionalityStrategy) Description() string {
	return "Business logic and functional flow mapping"
}

func (s *FunctionalityStrategy) Analyze(ctx context.Context, req port.AnalysisRequest) (*port.AnalysisResult, error) {
	systemPrompt := `You are an expert business analyst and software engineer. Analyze the provided codebase and produce a beautiful Markdown report.

Your report MUST include:
1. **Use Cases** — distinct business use cases identified from the code
2. **API Endpoints** — all endpoints found with HTTP methods and descriptions
3. **Business Rules** — core domain rules and validations discovered
4. **Flow Diagram** — a Mermaid sequence diagram of the main user flow
5. **Gaps & Missing Functionality** — what appears incomplete or missing
6. **Functionality Score** — rate 0-10 with justification

Format rules:
- Use Markdown headings (##), bullet points, tables for endpoints
- Include exactly ONE Mermaid sequence diagram
- Be specific about actual code found, not generic
- End with: **Score: X/10**`

	codeContext := make([]string, 0, len(req.Chunks)+1)
	codeContext = append(codeContext, fmt.Sprintf("Repository: %s\n\nFile tree:\n%s", req.RepoName, formatFileTree(req.FileTree)))
	codeContext = append(codeContext, req.Chunks...)

	response, err := s.ai.Chat(ctx, systemPrompt, "Map the business functionality of this codebase and produce a Markdown report with Mermaid diagrams.", codeContext)
	if err != nil {
		return nil, fmt.Errorf("functionality analysis: %w", err)
	}

	return &port.AnalysisResult{
		Strategy: s.Name(),
		Summary:  response,
		Details:  json.RawMessage("{}"),
		Score:    extractScore(response),
	}, nil
}
