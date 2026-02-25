package port

import (
	"context"
	"encoding/json"
)

// AnalysisStrategy defines a pluggable analysis engine (Strategy Pattern).
// Each strategy performs a specific kind of code analysis.
type AnalysisStrategy interface {
	// Name returns the unique name of this strategy (e.g. "architecture", "code_quality").
	Name() string

	// Description returns a human-readable description of what this strategy analyzes.
	Description() string

	// Analyze executes the analysis on the given request and returns results.
	Analyze(ctx context.Context, req AnalysisRequest) (*AnalysisResult, error)
}

// AnalysisRequest contains everything a strategy needs to perform analysis.
type AnalysisRequest struct {
	RepoID     string   `json:"repo_id"`
	RepoName   string   `json:"repo_name"`
	CommitHash string   `json:"commit_hash"`
	Chunks     []string `json:"chunks"`
	FileTree   []string `json:"file_tree"`
	Language   string   `json:"language,omitempty"`
}

// AnalysisResult holds the output of an analysis strategy.
type AnalysisResult struct {
	Strategy    string          `json:"strategy"`
	Summary     string          `json:"summary"`
	Details     json.RawMessage `json:"details"`
	Score       float64         `json:"score"`
	Suggestions []string        `json:"suggestions,omitempty"`
	Diagrams    []Diagram       `json:"diagrams,omitempty"`
}

// Diagram represents a generated diagram (e.g. Mermaid, PlantUML).
type Diagram struct {
	Title  string `json:"title"`
	Type   string `json:"type"` // mermaid, plantuml
	Source string `json:"source"`
}

// AnalysisEngine orchestrates multiple strategies.
type AnalysisEngine struct {
	strategies map[string]AnalysisStrategy
}

// NewAnalysisEngine creates a new engine with the given strategies.
func NewAnalysisEngine(strategies ...AnalysisStrategy) *AnalysisEngine {
	m := make(map[string]AnalysisStrategy, len(strategies))
	for _, s := range strategies {
		m[s.Name()] = s
	}
	return &AnalysisEngine{strategies: m}
}

// Run executes the named strategy.
func (e *AnalysisEngine) Run(ctx context.Context, strategyName string, req AnalysisRequest) (*AnalysisResult, error) {
	s, ok := e.strategies[strategyName]
	if !ok {
		return nil, ErrStrategyNotFound
	}
	return s.Analyze(ctx, req)
}

// RunAll executes all registered strategies and returns their results.
func (e *AnalysisEngine) RunAll(ctx context.Context, req AnalysisRequest) ([]*AnalysisResult, error) {
	results := make([]*AnalysisResult, 0, len(e.strategies))
	for _, s := range e.strategies {
		r, err := s.Analyze(ctx, req)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

// AvailableStrategies returns the names of all registered strategies.
func (e *AnalysisEngine) AvailableStrategies() []string {
	names := make([]string, 0, len(e.strategies))
	for name := range e.strategies {
		names = append(names, name)
	}
	return names
}
