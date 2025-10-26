package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DetectRemoteURL attempts to detect the Git remote URL from the current directory
func DetectRemoteURL() (string, error) {
	return DetectRemoteURLFromDir(".")
}

// DetectRemoteURLFromDir attempts to detect the Git remote URL from the specified directory
func DetectRemoteURLFromDir(dir string) (string, error) {
	configPath := filepath.Join(dir, ".git", "config")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("not a git repository or no remote 'origin' configured")
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "[remote \"origin\"]") {
			for _, subLine := range lines[i+1:] {
				if strings.HasPrefix(subLine, "\turl =") {
					return strings.TrimSpace(strings.TrimPrefix(subLine, "\turl =")), nil
				}
				if strings.HasPrefix(subLine, "[") {
					break
				}
			}
		}
	}

	return "", fmt.Errorf("git remote 'origin' is empty")
}

// DetectCurrentBranch attempts to detect the current Git branch
func DetectCurrentBranch() (string, error) {
	return DetectCurrentBranchFromDir(".")
}

// DetectCurrentBranchFromDir attempts to detect the current Git branch from the specified directory
func DetectCurrentBranchFromDir(dir string) (string, error) {
	headPath := filepath.Join(dir, ".git", "HEAD")
	content, err := os.ReadFile(headPath)
	if err != nil {
		return "", fmt.Errorf("failed to detect current branch")
	}

	head := strings.TrimSpace(string(content))
	if strings.HasPrefix(head, "ref: refs/heads/") {
		return strings.TrimPrefix(head, "ref: refs/heads/"), nil
	}

	return "", fmt.Errorf("not on a named branch (detached HEAD?)")
}

// IsGitRepository checks if the current directory is a Git repository
func IsGitRepository() bool {
	return IsGitRepositoryAt(".")
}

// IsGitRepositoryAt checks if the specified directory is a Git repository
func IsGitRepositoryAt(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// GetRepositoryInfo returns both the remote URL and current branch
func GetRepositoryInfo() (remoteURL string, branch string, err error) {
	return GetRepositoryInfoFrom(".")
}

// GetRepositoryInfoFrom returns both the remote URL and current branch from the specified directory
func GetRepositoryInfoFrom(dir string) (remoteURL string, branch string, err error) {
	if !IsGitRepositoryAt(dir) {
		return "", "", fmt.Errorf("not a git repository")
	}

	remoteURL, err = DetectRemoteURLFromDir(dir)
	if err != nil {
		return "", "", err
	}

	branch, err = DetectCurrentBranchFromDir(dir)
	if err != nil {
		// Not being on a branch is not fatal - default to "main"
		branch = "main"
	}

	return remoteURL, branch, nil
}