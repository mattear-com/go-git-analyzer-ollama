package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/port"
)

// AnalysisService orchestrates running analysis strategies on repositories.
type AnalysisService struct {
	engine *port.AnalysisEngine
}

// NewAnalysisService creates a new analysis service with the given engine.
func NewAnalysisService(engine *port.AnalysisEngine) *AnalysisService {
	return &AnalysisService{engine: engine}
}

// RunStrategy executes a specific analysis strategy.
func (s *AnalysisService) RunStrategy(ctx context.Context, strategyName string, req port.AnalysisRequest) (*port.AnalysisResult, error) {
	slog.Info("running analysis strategy", "strategy", strategyName, "repo", req.RepoID)
	result, err := s.engine.Run(ctx, strategyName, req)
	if err != nil {
		return nil, fmt.Errorf("run strategy %s: %w", strategyName, err)
	}
	return result, nil
}

// RunAll executes all registered strategies on a repository.
func (s *AnalysisService) RunAll(ctx context.Context, req port.AnalysisRequest) ([]*port.AnalysisResult, error) {
	slog.Info("running all analysis strategies", "repo", req.RepoID)
	return s.engine.RunAll(ctx, req)
}

// ListStrategies returns the available strategy names.
func (s *AnalysisService) ListStrategies() []string {
	return s.engine.AvailableStrategies()
}
