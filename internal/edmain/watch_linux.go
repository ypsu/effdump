package edmain

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
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

// fittedPrint prints the top of s to terminal such that it fits the terminal's current size.
// It trims excess lines and excess width.
func fittedPrint(s []byte) {
	width, height := termsize()
	if width < 10 || height < 10 {
		// Not a terminal or too small.
		fmt.Fprintf(os.Stderr, "%s\n", s)
		return
	}

	// Trim.
	ss := bytes.Split(s, []byte("\n"))
	if len(ss) > height-4 {
		ss = append(ss[:height-6], []byte(fmt.Sprintf("… (%d lines omitted)\n", len(ss)-(height-6))))
	}
	for i, line := range ss {
		trimmed, w := make([]byte, 0, len(line)), 0
		for _, rune := range bytes.Split(line, nil) {
			k := 1
			if rune[0] == '\t' {
				k = 8
			}
			if w+k >= width-1 {
				trimmed = append(trimmed, []byte("…")...)
				break
			}
			trimmed, w = append(trimmed, rune...), w+k
		}
		ss[i] = trimmed
	}

	// Clear screen and print.
	msg := bytes.Join(ss, []byte("\n"))
	if !bytes.HasSuffix(msg, []byte("\n")) {
		msg = append(msg, '\n')
	}
	fmt.Fprintf(os.Stderr, "\033[H\033[J%s\n", msg)
}

// startcmd starts command and returns its output after it exited or sent us SIGUSR1, whichever happens sooner.
// The resulting subprocess must be collected via the returned kill command.
func startcmd(argv []string, env []string) (output []byte, kill func()) {
	donesig := make(chan os.Signal, 2)
	signal.Notify(donesig, syscall.SIGUSR1)
	defer signal.Stop(donesig)

	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Appendf(nil, "ERROR: effdump: create pipe for subprocess output: %v.\n", err), func() {}
	}
	defer pr.Close()
	defer pw.Close()
	iodone := make(chan bool)
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	go func() {
		io.Copy(buf, pr)
		close(iodone)
	}()

	process, err := os.StartProcess(argv[0], argv, &os.ProcAttr{
		Env:   env,
		Files: []*os.File{nil, pw, pw},
		Sys:   &syscall.SysProcAttr{Setpgid: true},
	})
	if err != nil {
		return fmt.Appendf(nil, "ERROR: effdump: start subprocess: %v.\n", err), func() {}
	}
	pw.Close()
	go func() {
		process.Wait()
		donesig <- syscall.SIGUSR1
	}()

	select {
	case <-donesig:
		// Await child to finish / mark output finished with SIGUSR1 + some grace time for IO to finish copying.
		// This is needed because effdump could run indefinitely in case web serving (e.g. webdiff) was requested.
		// Note that there are multiple children ("go run" and its child, the effdump binary).
		// So it's not possible for the effdump binary to close pw after it started serving http and have pr unblock; the go run will keep pw open.
		// Work around this via this signaling.
		pr.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	case <-iodone:
	}
	<-iodone

	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		buf.WriteByte('\n')
	}
	return buf.Bytes(), func() {
		syscall.Kill(-process.Pid, syscall.SIGTERM)
		process.Wait()
	}
}

// reportchanges reports file modifications from under the current directory.
// Sends a true on the channel whenever it detects some fs events.
func reportchanges(changed chan<- bool) {
	watches := map[int]string{}
	ifd, err := syscall.InotifyInit()
	if err != nil {
		log.Fatalf("InotifyInit: %v.", err)
	}

	var watchpath func(string)
	watchpath = func(p string) {
		if p == ".git" {
			return
		}
		mask := uint32(0) |
			syscall.IN_CLOSE_WRITE |
			syscall.IN_CREATE |
			syscall.IN_DELETE |
			syscall.IN_MOVED_FROM |
			syscall.IN_MOVED_TO |
			syscall.IN_DONT_FOLLOW |
			syscall.IN_EXCL_UNLINK |
			syscall.IN_ONLYDIR
		wd, err := syscall.InotifyAddWatch(ifd, p, mask)
		if err != nil {
			log.Fatalf("InotifyAddWatch(%q): %v.", p, err)
		}
		watches[wd] = p
		filepath.WalkDir(p, func(childpath string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() || childpath == p {
				return nil
			}
			watchpath(childpath)
			return fs.SkipDir
		})
	}
	watchpath(".")

	for {
		const bufsize = 16384
		eventbuf := [bufsize]byte{}
		n, err := syscall.Read(ifd, eventbuf[:])
		if n <= 0 || err != nil {
			log.Fatalf("Read inotify fd: %v.", err)
		}
		for offset := 0; offset < n; {
			if n-offset < syscall.SizeofInotifyEvent {
				log.Fatalf("Invalid inotify read.")
			}
			event := (*syscall.InotifyEvent)(unsafe.Pointer(&eventbuf[offset]))
			wd, mask, namelen := int(event.Wd), int(event.Mask), int(event.Len)
			namebytes := (*[syscall.PathMax]byte)(unsafe.Pointer(&eventbuf[offset+syscall.SizeofInotifyEvent]))
			name := string(bytes.TrimRight(namebytes[:namelen], "\000"))
			dir, watchExists := watches[wd]
			if !watchExists {
				log.Fatalf("Unknown watch descriptor %d.", wd)
			}
			fullname := filepath.Join(dir, name)
			if mask&syscall.IN_IGNORED != 0 {
				delete(watches, wd)
			}
			if mask&syscall.IN_CREATE != 0 || mask&syscall.IN_MOVED_TO != 0 {
				if fi, err := os.Stat(fullname); err == nil && fi.IsDir() { // on success and if fullname is dir
					watchpath(fullname)
				}
			}
			offset += syscall.SizeofInotifyEvent + namelen
		}
		changed <- true
	}
}

// watch runs the current command repeatedly after each filesystem change (with -watch flag removed).
func (p *Params) watch(ctx context.Context) error {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("edmain/watch: couldn't fetch buildinfo")
	}

	gobin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("edmain/find go binary: %v", err)
	}
	argv := append([]string{gobin, "run", bi.Path}, os.Args[1:]...)
	env := append(os.Environ(), "EFFDUMP_WATCHERPID="+strconv.Itoa(os.Getpid()))

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGINT)

	fsch := make(chan bool, 64)
	go reportchanges(fsch)

	for ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "running...")
		output, kill := startcmd(argv, env)
		fittedPrint(output)
		select {
		case <-fsch:
			// Wait a little bit to settle and then drain fsch.
			time.Sleep(100 * time.Millisecond)
			for len(fsch) > 0 {
				<-fsch
			}
			kill()
		case <-sigch:
			kill()
			return fmt.Errorf("edmain/watch: signal interrupt")
		}
	}
	return ctx.Err()
}

func (p *Params) notifyWatcher() {
	pid, err := strconv.Atoi(p.watcherpid)
	if err != nil || pid <= 0 {
		return
	}
	syscall.Kill(pid, syscall.SIGUSR1)
}
