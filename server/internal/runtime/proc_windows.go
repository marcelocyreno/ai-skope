//go:build windows

package runtime

import (
	"os/exec"
	"time"
)

// configureProcAttr keeps Windows behaviour simple: CommandContext kills the
// process, and WaitDelay stops a stray descendant from holding the turn open.
func configureProcAttr(cmd *exec.Cmd) {
	cmd.WaitDelay = 3 * time.Second
}
