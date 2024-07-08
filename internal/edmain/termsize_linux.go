package edmain

import (
	"os"
	"syscall"
	"unsafe"
)

// termsize() returns stderr's terminal size.
// Returns 0, 0 on error.
func termsize() (width, height int) {
	// Per https://stackoverflow.com/questions/1733155/how-do-you-get-the-terminal-size-in-go.
	var winsz [4]int16
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(os.Stderr.Fd()), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&winsz)))
	return int(winsz[1]), int(winsz[0])
}
