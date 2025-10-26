package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// DetectRemoteURL attempts to detect the Git remote URL from the current directory
func DetectRemoteURL() (string, error) {
	return DetectRemoteURLFromDir(".")
}

// DetectRemoteURLFromDir attempts to detect the Git remote URL from the specified directory
func DetectRemoteURLFromDir(dir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository or no remote 'origin' configured")
	}

	url := strings.TrimSpace(string(output))
	if url == "" {
		return "", fmt.Errorf("git remote 'origin' is empty")
	}

	return url, nil
}

// DetectCurrentBranch attempts to detect the current Git branch
func DetectCurrentBranch() (string, error) {
	return DetectCurrentBranchFromDir(".")
}

// DetectCurrentBranchFromDir attempts to detect the current Git branch from the specified directory
func DetectCurrentBranchFromDir(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect current branch")
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("not on a named branch (detached HEAD?)")
	}

	return branch, nil
}

// IsGitRepository checks if the current directory is a Git repository
func IsGitRepository() bool {
	return IsGitRepositoryAt(".")
}

// IsGitRepositoryAt checks if the specified directory is a Git repository
func IsGitRepositoryAt(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	
	err := cmd.Run()
	return err == nil
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
