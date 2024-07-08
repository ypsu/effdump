//go:build !linux

package edmain

import "fmt"

func (p *Params) watch() error {
	return fmt.Errorf("edmain/watch: watch is only supported on linux")
}

func (p *Params) notifyWatcher() {}
