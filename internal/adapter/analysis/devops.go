package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
)

type DevOpsStrategy struct {
	ai port.AIProvider
}

func NewDevOpsStrategy(ai port.AIProvider) *DevOpsStrategy {
	return &DevOpsStrategy{ai: ai}
}

func (s *DevOpsStrategy) Name() string        { return "devops" }
func (s *DevOpsStrategy) Description() string { return "DevOps, CI/CD, and infrastructure analysis" }

func (s *DevOpsStrategy) Analyze(ctx context.Context, req port.AnalysisRequest) (*port.AnalysisResult, error) {
	systemPrompt := `You are an expert in DevOps, CI/CD, and infrastructure. Analyze the provided codebase and produce a beautiful Markdown report.

Your report MUST include:
1. **Infrastructure Overview** — what infrastructure files exist (Docker, CI, config)
2. **CI/CD Pipeline** — describe any CI/CD configuration found, or suggest one
3. **Deployment Architecture** — how the app is deployed or should be deployed
4. **DevOps Diagram** — a Mermaid diagram of the deployment/CI pipeline
5. **Recommendations** — specific DevOps improvements
6. **DevOps Score** — rate 0-10 with justification

Format rules:
- Use Markdown headings (##), bullet points, code blocks
- Include ONE Mermaid diagram (graph or flowchart)
- Reference actual files found (Dockerfile, docker-compose.yml, Makefile, etc.)
- End with: **Score: X/10**`

	codeContext := make([]string, 0, len(req.Chunks)+1)
	codeContext = append(codeContext, fmt.Sprintf("Repository: %s\n\nFile tree:\n%s", req.RepoName, formatFileTree(req.FileTree)))
	codeContext = append(codeContext, req.Chunks...)

	response, err := s.ai.Chat(ctx, systemPrompt, "Analyze the DevOps and infrastructure of this codebase and produce a Markdown report with Mermaid diagrams.", codeContext)
	if err != nil {
		return nil, fmt.Errorf("devops analysis: %w", err)
	}

	return &port.AnalysisResult{
		Strategy: s.Name(),
		Summary:  response,
		Details:  json.RawMessage("{}"),
		Score:    extractScore(response),
	}, nil
}
