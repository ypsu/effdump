//go:build !linux

package edmain

import "fmt"

// isatty returns true iff stdout is a terminal.
func isatty() bool {
	return false
}

func (p *Params) watch() error {
	return fmt.Errorf("edmain/watch: watch is only supported on linux")
}

func (p *Params) notifyWatcher() {}
