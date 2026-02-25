package vcs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arturoeanton/go-git-analyzer-ollama/internal/domain"
)

// GitProvider implements port.VCSProvider using the git CLI.
type GitProvider struct{}

// NewGitProvider creates a new Git VCS provider.
func NewGitProvider() *GitProvider {
	return &GitProvider{}
}

// Clone clones a repository into dest.
func (g *GitProvider) Clone(ctx context.Context, url string, dest string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone %s: %w", url, err)
	}
	return nil
}

// Pull fetches the latest changes for an existing repository.
func (g *GitProvider) Pull(ctx context.Context, repoPath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull %s: %w", repoPath, err)
	}
	return nil
}

// Log returns the commit history.
func (g *GitProvider) Log(ctx context.Context, repoPath string, limit int) ([]domain.CommitInfo, error) {
	format := "%H|%an|%s|%aI|%m"
	args := []string{"-C", repoPath, "log", fmt.Sprintf("--format=%s", format), "--shortstat"}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []domain.CommitInfo

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 4 {
			continue
		}

		ts, _ := time.Parse(time.RFC3339, parts[3])
		ci := domain.CommitInfo{
			Hash:      parts[0],
			Author:    parts[1],
			Message:   parts[2],
			Timestamp: ts,
		}

		// Try to parse shortstat from next non-empty line
		if i+1 < len(lines) {
			statLine := strings.TrimSpace(lines[i+1])
			if strings.Contains(statLine, "file") {
				parts := strings.Fields(statLine)
				if len(parts) > 0 {
					n, _ := strconv.Atoi(parts[0])
					ci.Files = n
				}
				i++ // skip the stat line
			}
		}

		commits = append(commits, ci)
	}

	return commits, nil
}

// Diff returns the unified diff between two commits.
func (g *GitProvider) Diff(ctx context.Context, repoPath, fromHash, toHash string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", fromHash, toHash)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(output), nil
}

// ListFiles returns all file paths in the repository at a given commit.
func (g *GitProvider) ListFiles(ctx context.Context, repoPath string, commitHash string) ([]string, error) {
	args := []string{"-C", repoPath, "ls-tree", "-r", "--name-only"}
	if commitHash != "" {
		args = append(args, commitHash)
	} else {
		args = append(args, "HEAD")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-tree: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// ReadFile reads a file's content at a specific commit hash.
func (g *GitProvider) ReadFile(ctx context.Context, repoPath string, commitHash string, filePath string) ([]byte, error) {
	if commitHash == "" {
		// Read from working tree
		fullPath := filepath.Join(repoPath, filePath)
		return os.ReadFile(fullPath)
	}

	ref := fmt.Sprintf("%s:%s", commitHash, filePath)
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "show", ref)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s: %w", ref, err)
	}
	return output, nil
}

// gitCommitEntry represents a parsed git log entry for graph building.
type gitCommitEntry struct {
	Hash    string
	Parents []string
	Refs    []string // branch/tag decorations
	Message string
	Author  string
}

// BuildMermaidGitGraph generates a Mermaid gitGraph diagram from the repo's git history.
// It reads branch topology, commits, and merges to build a valid gitGraph string.
// Returns the mermaid diagram string and a list of unique authors.
func (g *GitProvider) BuildMermaidGitGraph(ctx context.Context, repoPath string, maxCommits int) (string, []string, error) {
	if maxCommits <= 0 {
		maxCommits = 100
	}

	// Get all commits in topological (reverse) order with parent hashes, decorations, and author
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "log", "--all",
		"--topo-order", "--reverse",
		fmt.Sprintf("--format=%%H|%%P|%%D|%%s|%%an"),
		fmt.Sprintf("-n%d", maxCommits),
	)
	output, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("git log for graph: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return "", nil, fmt.Errorf("no commits found")
	}

	// Parse commits
	var commits []gitCommitEntry
	authorSet := map[string]bool{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		author := strings.TrimSpace(parts[4])
		authorSet[author] = true
		entry := gitCommitEntry{
			Hash:    parts[0],
			Message: sanitizeMermaidText(parts[3]),
			Author:  author,
		}
		if parts[1] != "" {
			entry.Parents = strings.Fields(parts[1])
		}
		if parts[2] != "" {
			refs := strings.Split(parts[2], ",")
			for _, r := range refs {
				r = strings.TrimSpace(r)
				// Clean ref names: remove "HEAD -> ", "origin/", "tag: " prefixes
				r = strings.TrimPrefix(r, "HEAD -> ")
				r = strings.TrimPrefix(r, "tag: ")
				if strings.HasPrefix(r, "origin/") {
					continue // skip remote tracking refs
				}
				if r != "" {
					entry.Refs = append(entry.Refs, sanitizeBranchName(r))
				}
			}
		}
		commits = append(commits, entry)
	}

	if len(commits) == 0 {
		return "", nil, fmt.Errorf("no commits parsed")
	}

	// Build unique author list (ordered by first appearance)
	var authors []string
	seen := map[string]bool{}
	for _, c := range commits {
		if !seen[c.Author] {
			authors = append(authors, c.Author)
			seen[c.Author] = true
		}
	}

	// Build the Mermaid gitGraph
	var sb strings.Builder
	sb.WriteString("gitGraph TB:\n")

	// Track state
	currentBranch := "main"
	createdBranches := map[string]bool{"main": true}
	commitBranch := map[string]string{} // hash -> branch

	// First pass: determine the default branch name
	defaultBranch := "main"
	if len(commits) > 0 && len(commits[0].Refs) > 0 {
		for _, ref := range commits[0].Refs {
			if ref == "main" || ref == "master" {
				defaultBranch = ref
				break
			}
		}
	}
	currentBranch = defaultBranch
	createdBranches = map[string]bool{defaultBranch: true}

	for _, c := range commits {
		isMerge := len(c.Parents) > 1

		// Determine which branch this commit belongs to
		targetBranch := ""
		for _, ref := range c.Refs {
			if ref != "" {
				targetBranch = ref
				break
			}
		}

		if targetBranch == "" {
			// No decoration — infer from parent
			if len(c.Parents) > 0 {
				if b, ok := commitBranch[c.Parents[0]]; ok {
					targetBranch = b
				}
			}
		}
		if targetBranch == "" {
			targetBranch = currentBranch
		}

		// Create branch if needed
		if !createdBranches[targetBranch] {
			sb.WriteString(fmt.Sprintf("    branch %s\n", targetBranch))
			createdBranches[targetBranch] = true
		}

		// Checkout if different from current
		if targetBranch != currentBranch {
			sb.WriteString(fmt.Sprintf("    checkout %s\n", targetBranch))
			currentBranch = targetBranch
		}

		// Build commit label with author prefix
		label := fmt.Sprintf("%s: %s", c.Author, truncate(c.Message, 35))

		if isMerge {
			// Find which branch was merged — the second parent's branch
			mergeSrc := currentBranch
			if len(c.Parents) > 1 {
				if b, ok := commitBranch[c.Parents[1]]; ok {
					mergeSrc = b
				}
			}
			if mergeSrc != currentBranch {
				sb.WriteString(fmt.Sprintf("    merge %s\n", mergeSrc))
			} else {
				sb.WriteString(fmt.Sprintf("    commit id: \"%s\"\n", label))
			}
		} else {
			sb.WriteString(fmt.Sprintf("    commit id: \"%s\"\n", label))
		}

		commitBranch[c.Hash] = targetBranch
	}

	return sb.String(), authors, nil
}

// sanitizeMermaidText removes characters that break Mermaid syntax.
func sanitizeMermaidText(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return strings.TrimSpace(s)
}

// sanitizeBranchName makes branch names safe for Mermaid.
func sanitizeBranchName(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\"", "")
	return s
}

// truncate shortens a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
