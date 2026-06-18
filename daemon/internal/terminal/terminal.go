package terminal

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

type ExitInfo struct {
	Code   int
	Signal string
}

type OutputFunc func(data []byte)

type TerminalProcess struct {
	ID      string
	Name    string
	Cwd     string
	Title   *string
	Command string
	Args    []string

	ptmx    *os.File
	process *os.Process
	rows    uint16
	cols    uint16

	mu          sync.Mutex
	subscribers map[uint64]OutputFunc
	nextSubID   uint64
	onExitFuncs []func(ExitInfo)
	done        chan struct{}
	exited      bool
	logger      *slog.Logger
}

func NewTerminalProcess(id, name, cwd, command string, args []string, rows, cols uint16, logger *slog.Logger) *TerminalProcess {
	return &TerminalProcess{
		ID:          id,
		Name:        name,
		Cwd:         cwd,
		Command:     command,
		Args:        args,
		rows:        rows,
		cols:        cols,
		subscribers: make(map[uint64]OutputFunc),
		done:        make(chan struct{}),
		logger:      logger.With("terminalId", id),
	}
}

func (t *TerminalProcess) Start() error {
	shell := t.Command
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}

	args := t.Args
	cmd := exec.Command(shell, args...)
	cmd.Dir = t.Cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	winsize := &pty.Winsize{
		Rows: t.rows,
		Cols: t.cols,
	}

	ptmx, err := pty.StartWithSize(cmd, winsize)
	if err != nil {
		return fmt.Errorf("pty start: %w", err)
	}

	t.ptmx = ptmx
	t.process = cmd.Process

	go t.readLoop()

	go func() {
		err := cmd.Wait()
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.exited {
			return
		}
		t.exited = true
		close(t.done)

		exitInfo := ExitInfo{}
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitInfo.Code = exitErr.ExitCode()
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					if status.Signaled() {
						exitInfo.Signal = status.Signal().String()
					}
				}
			}
		}

		for _, fn := range t.onExitFuncs {
			fn(exitInfo)
		}
	}()

	return nil
}

func (t *TerminalProcess) WriteInput(data []byte) error {
	t.mu.Lock()
	ptmx := t.ptmx
	t.mu.Unlock()
	if ptmx == nil {
		return fmt.Errorf("terminal not running")
	}
	_, err := ptmx.Write(data)
	return err
}

func (t *TerminalProcess) Resize(rows, cols uint16) error {
	t.mu.Lock()
	ptmx := t.ptmx
	t.rows = rows
	t.cols = cols
	t.mu.Unlock()
	if ptmx == nil {
		return fmt.Errorf("terminal not running")
	}

	winsize := &pty.Winsize{Rows: rows, Cols: cols}
	if err := pty.Setsize(ptmx, winsize); err != nil {
		return fmt.Errorf("pty setsize: %w", err)
	}

	if t.process != nil {
		_ = t.process.Signal(syscall.SIGWINCH)
	}
	return nil
}

func (t *TerminalProcess) Kill() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.exited {
		return
	}
	if t.process != nil {
		_ = t.process.Signal(syscall.SIGTERM)
	}
	go func() {
		select {
		case <-t.done:
		case <-time.After(2 * time.Second):
			t.mu.Lock()
			if !t.exited && t.process != nil {
				_ = t.process.Signal(syscall.SIGKILL)
			}
			t.mu.Unlock()
		}
	}()
}

func (t *TerminalProcess) Subscribe(fn OutputFunc) func() {
	t.mu.Lock()
	defer t.mu.Unlock()
	id := t.nextSubID
	t.nextSubID++
	t.subscribers[id] = fn
	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		delete(t.subscribers, id)
	}
}

func (t *TerminalProcess) OnExit(fn func(ExitInfo)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onExitFuncs = append(t.onExitFuncs, fn)
}

func (t *TerminalProcess) Done() <-chan struct{} {
	return t.done
}

func (t *TerminalProcess) Close() {
	t.Kill()
	<-t.done
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ptmx != nil {
		_ = t.ptmx.Close()
		t.ptmx = nil
	}
}

func (t *TerminalProcess) Rows() uint16 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.rows
}

func (t *TerminalProcess) Cols() uint16 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cols
}

func (t *TerminalProcess) readLoop() {
	ptmx := t.ptmx
	buf := make([]byte, 4096)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			t.mu.Lock()
			subs := make(map[uint64]OutputFunc, len(t.subscribers))
			for k, v := range t.subscribers {
				subs[k] = v
			}
			t.mu.Unlock()
			for _, fn := range subs {
				fn(data)
			}
		}
		if err != nil {
			return
		}
	}
}
