//go:build !windows

package runtime

import (
	"os/exec"
	"syscall"
	"time"
)

// killSweepDelay covers a race that shows up whenever a turn is cancelled
// early: the agent may be forking a child at the instant the group is
// signalled, so that child is created just after the kernel enumerated the
// group and survives as an orphan — still holding the output pipes open. A
// second sweep shortly after catches it.
const killSweepDelay = 250 * time.Millisecond

// configureProcAttr puts the agent in its own process group so cancelling a
// turn takes down the whole tree, not just the process the server launched.
func configureProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// If a descendant still holds the pipes after cancellation, give up on it
	// rather than blocking the turn: exec closes the pipes and Wait returns.
	cmd.WaitDelay = 3 * time.Second

	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		pid := cmd.Process.Pid
		// A negative pid signals the whole process group.
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		err := syscall.Kill(-pid, syscall.SIGKILL)
		time.AfterFunc(killSweepDelay, func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })
		return err
	}
}
