package gitops

import (
	"fmt"
	"os/exec"
	"strings"
)

// Run executes a git subcommand inside repoDir and returns combined output.
func Run(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, s)
	}
	return s, nil
}

// AddCommit stages the given paths (relative to repoDir) and creates a commit.
// Returns ErrNoChanges if nothing was staged.
func AddCommit(repoDir, message string, paths ...string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths to commit")
	}
	addArgs := append([]string{"add", "--"}, paths...)
	if _, err := Run(repoDir, addArgs...); err != nil {
		return err
	}
	// Anything actually staged?
	status, err := Run(repoDir, "status", "--porcelain")
	if err != nil {
		return err
	}
	if status == "" {
		return ErrNoChanges
	}
	if _, err := Run(repoDir, "commit", "-m", message); err != nil {
		return err
	}
	return nil
}

// Push pushes the current branch to origin.
func Push(repoDir string) error {
	branch, err := Run(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if _, err := Run(repoDir, "push", "origin", branch); err != nil {
		return err
	}
	return nil
}

// HasRemote reports whether the repo has any remote configured.
func HasRemote(repoDir string) bool {
	out, err := Run(repoDir, "remote")
	return err == nil && out != ""
}

var ErrNoChanges = fmt.Errorf("git: no changes to commit")
