// Package git implements the git interaction for effdump.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// VersionSystem implements git version lookup via parsing `git` CLI tool's output.
// It uses the first ~7 characters of the full commit ID for the version strings.
type VersionSystem struct{}

// New returns a new git VersionSystem.
func New() *VersionSystem { return &VersionSystem{} }

// Fetch returns the HEAD commit and whether working dir is clean or not.
func (vs *VersionSystem) Fetch(ctx context.Context) (version string, clean bool, err error) {
	head, err := vs.Resolve(ctx, "HEAD")
	if err != nil {
		return "", false, fmt.Errorf("git/fetch HEAD: %v", err)
	}
	clean = true
	_, err = exec.Command("git", "diff-index", "--quiet", "HEAD").Output()
	if err != nil {
		clean = false
	}
	return head, clean, nil
}

// Resolve resolves references such as HEAD or "ab12" to the full version string.
func (*VersionSystem) Resolve(_ context.Context, rev string) (version string, err error) {
	output, err := exec.Command("git", "rev-parse", "--short", rev).Output()
	if err != nil {
		return "", fmt.Errorf("git/exec rev-parse: %v (not in git directory?)", err)
	}
	fields := bytes.Fields(output)
	if len(fields) != 1 {
		return "", fmt.Errorf("git/split rev-parse: expected one field, got %q", output)
	}
	return string(bytes.TrimSpace(fields[0])), nil
}
