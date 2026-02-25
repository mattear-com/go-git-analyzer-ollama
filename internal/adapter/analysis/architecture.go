package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
)

// ArchitectureStrategy analyzes design patterns, module structure, and generates diagrams.
type ArchitectureStrategy struct {
	ai port.AIProvider
}

func NewArchitectureStrategy(ai port.AIProvider) *ArchitectureStrategy {
	return &ArchitectureStrategy{ai: ai}
}

func (s *ArchitectureStrategy) Name() string { return "architecture" }
func (s *ArchitectureStrategy) Description() string {
	return "Architecture analysis with Mermaid diagrams"
}

func (s *ArchitectureStrategy) Analyze(ctx context.Context, req port.AnalysisRequest) (*port.AnalysisResult, error) {
	systemPrompt := `You are an expert software architect. Analyze the provided codebase and produce a beautiful Markdown report.

Your report MUST include:
1. **Architecture Overview** — identify architectural patterns (MVC, Clean, Hexagonal, etc.)
2. **Module Dependencies** — which modules depend on which
3. **Architecture Diagram** — a Mermaid diagram (use graph TD or flowchart) showing components and dependencies
4. **Issues & Improvements** — architectural issues found with recommendations
5. **Architecture Score** — rate 0-10 with justification

Format rules:
- Use Markdown headings (##), bullet points, bold text
- Include exactly ONE Mermaid diagram wrapped in triple backticks with "mermaid" language tag
- Be specific about the actual files/packages found, not generic
- End with a score line: **Score: X/10**`

	codeContext := make([]string, 0, len(req.Chunks)+1)
	codeContext = append(codeContext, fmt.Sprintf("Repository: %s\n\nFile tree:\n%s", req.RepoName, formatFileTree(req.FileTree)))
	codeContext = append(codeContext, req.Chunks...)

	response, err := s.ai.Chat(ctx, systemPrompt, "Analyze the architecture of this codebase and produce a Markdown report with Mermaid diagrams.", codeContext)
	if err != nil {
		return nil, fmt.Errorf("architecture analysis: %w", err)
	}

	return &port.AnalysisResult{
		Strategy: s.Name(),
		Summary:  response,
		Details:  json.RawMessage("{}"),
		Score:    extractScore(response),
	}, nil
}

func formatFileTree(files []string) string {
	result := ""
	for _, f := range files {
		result += "  " + f + "\n"
	}
	return result
}

// extractScore tries to find "Score: X/10" in text.
func extractScore(response string) float64 {
	// Try JSON first
	var parsed struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err == nil && parsed.Score > 0 {
		return parsed.Score
	}

	// Try to find "Score: X/10" pattern
	for i := len(response) - 1; i >= 0; i-- {
		if i+10 < len(response) && response[i:i+6] == "Score:" {
			for j := i + 6; j < len(response) && j < i+12; j++ {
				if response[j] >= '0' && response[j] <= '9' {
					score := float64(response[j] - '0')
					if j+1 < len(response) && response[j+1] == '0' {
						return 10
					}
					return score
				}
			}
		}
	}
	return 0
}
