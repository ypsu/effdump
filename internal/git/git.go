// Package git implements the git interaction for effdump.
package git

import "context"

// VersionSystem implements git version lookup via parsing `git` CLI tool's output.
// It uses the first 8 characters of the full commit ID for the version strings.
type VersionSystem struct{}

// New returns a new git VersionSystem.
func New() *VersionSystem { return &VersionSystem{} }

// Fetch returns the HEAD commit and whether working dir is clean or not.
func (*VersionSystem) Fetch(context.Context) (version string, clean bool, err error) {
	// TODO: implement
	return "1234abcd", true, nil
}

// Resolve resolves references such as HEAD or "ab12" to the full version string.
func (*VersionSystem) Resolve(context.Context, string) (version string, err error) {
	// TODO: implement
	return "1234abcd", nil
}
