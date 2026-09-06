package runctx

import (
	"os/exec"
	"syscall"
	"time"
)

// termGrace is how long a process group is given to exit on SIGTERM before it
// is killed.
const termGrace = 2 * time.Second

// isolateProcessGroup places a command in its own process group so the whole
// tree can be signalled, not just the direct child.
//
// The software under test spawns children that outlive their parent, so
// signalling the PID alone leaves them running.
func isolateProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup terminates an entire process group, escalating to SIGKILL.
//
// exited is closed by the caller once the child has been reaped; escalation is
// skipped when the group goes away during the grace period.
func killProcessGroup(pgid int, exited <-chan struct{}) {
	syscall.Kill(-pgid, syscall.SIGTERM)

	select {
	case <-exited:
		return
	case <-time.After(termGrace):
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
}
