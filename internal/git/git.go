package git

import (
	"errors"
	"os/exec"
	"strings"
)

// git diff --staged
func Diff() (string, error) {
	// Get diff
	cmd := exec.Command("git", "diff", "--staged")
	diff, err := cmd.Output()
	if err != nil {
		return "", errors.Join(err, errors.New(string(diff)))
	}

	// Remove unnecessary '+' prefix
	diffStr := ""
	for line := range strings.SplitSeq(strings.TrimSuffix(string(diff), "\n"), "\n") {
		diffStr += strings.TrimPrefix(line, "+") + "\n"
	}

	return diffStr, nil
}

// git commit -m
func Commit(message string) (string, error) {
	cmd := exec.Command("git", "commit", "-m", message)
	gitCommit, err := cmd.Output()
	if err != nil {
		return "", errors.Join(err, errors.New(string(gitCommit)))
	}

	return string(gitCommit), nil
}
