package edmain

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"
)

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

	for ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "running...")
		output, kill := startcmd(argv, env)
		fittedPrint(output)
		select {
		case <-time.After(5 * time.Second):
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
