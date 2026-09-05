package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ai-skope/aiss/internal/store"
)

// abandonGrace is how long a cancelled turn waits for its output readers
// before giving up on them.
const abandonGrace = 1500 * time.Millisecond

// maxLine caps one line of agent output. Anything longer is truncated rather
// than buffered without bound.
const maxLine = 4 << 20

// baseEnvKeys are the only variables inherited from the server's own
// environment. Everything else is dropped so a key in the user's shell never
// reaches an agent unless a provider explicitly injects it.
var baseEnvKeys = []string{
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "TERM",
	"LANG", "LC_ALL", "LC_CTYPE", "TZ",
	"SystemRoot", "COMSPEC", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "PATHEXT",
}

// XDG_* is deliberately absent. Those variables point at *this server's*
// config and data when aiss is run with them set, and agents keep their
// credentials under the same paths — opencode reads
// $XDG_DATA_HOME/opencode/auth.json, for instance. Passing the server's values
// down makes an authenticated agent look unauthenticated. Agents fall back to
// HOME, which is what the user's own shell would have given them; anyone who
// genuinely needs one can name it in passthroughEnv.

// BaseEnv returns the scrubbed environment for an agent, plus any variables
// the user chose to pass through.
func BaseEnv(passthrough []string) []string {
	var env []string
	keys := append(append([]string{}, baseEnvKeys...), passthrough...)
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			continue
		}
		seen[k] = true
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// procTurn is a running agent process exposed as a Turn.
type procTurn struct {
	events chan Event
	cancel context.CancelFunc
	once   sync.Once
}

func (t *procTurn) Events() <-chan Event { return t.events }

func (t *procTurn) Cancel() { t.once.Do(func() { t.cancel() }) }

// spawn starts the agent described by spec and streams its output as events.
func spawn(parent context.Context, spec Spec, req TurnRequest, bin string) (Turn, error) {
	args := spec.Args(req)
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = req.WorkDir
	cmd.Env = req.Env
	configureProcAttr(cmd)

	if spec.PromptViaStdin {
		cmd.Stdin = strings.NewReader(req.Prompt)
	} else {
		cmd.Stdin = strings.NewReader("")
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}

	t := &procTurn{events: make(chan Event, 128), cancel: cancel}
	started := time.Now()

	var errTail strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		readLines(stdout, func(line []byte) {
			for _, ev := range spec.parse(line) {
				select {
				case t.events <- ev:
				case <-ctx.Done():
					return
				}
			}
		})
	}()

	go func() {
		defer wg.Done()
		readLines(stderr, func(line []byte) {
			if errTail.Len() < 4096 {
				errTail.Write(line)
				errTail.WriteByte('\n')
			}
		})
	}()

	readersDone := make(chan struct{})
	go func() { wg.Wait(); close(readersDone) }()

	go func() {
		// Normally the readers finish when the agent exits and its pipes
		// close. After a cancellation a descendant may still hold a pipe
		// open, so waiting for the readers is bounded: past the grace period
		// they are abandoned and cmd.Wait (with WaitDelay set) closes the
		// pipes itself. Without this the turn would stay open for as long as
		// the stray process lives.
		select {
		case <-readersDone:
		case <-ctx.Done():
			select {
			case <-readersDone:
			case <-time.After(abandonGrace):
			}
		}
		waitErr := cmd.Wait()
		elapsed := time.Since(started).Milliseconds()

		if waitErr != nil {
			msg := describeExit(waitErr, ctx, bin, errTail.String())
			t.events <- Event{Kind: EventError, Err: msg, Retryable: ctx.Err() != context.Canceled}
		}
		t.events <- Event{Kind: EventUsage, Usage: &store.Usage{MS: elapsed}}
		t.events <- Event{Kind: EventDone}
		close(t.events)
		cancel()
	}()

	return t, nil
}

// describeExit turns a process failure into something a person can act on.
func describeExit(err error, ctx context.Context, bin, stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if len(stderr) > 600 {
		stderr = stderr[len(stderr)-600:]
	}
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return "The runtime took too long and was stopped."
	case ctx.Err() == context.Canceled:
		return "Cancelled."
	}
	if stderr != "" {
		return stderr
	}
	return fmt.Sprintf("%s exited: %v", bin, err)
}

// readLines feeds complete lines to fn, truncating any line over maxLine.
func readLines(r io.Reader, fn func([]byte)) {
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		line, err := br.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			buf := append([]byte{}, line...)
			for err == bufio.ErrBufferFull {
				line, err = br.ReadSlice('\n')
				if len(buf) < maxLine {
					buf = append(buf, line...)
				}
			}
			if len(buf) > 0 {
				fn(trimEOL(buf))
			}
			if err != nil {
				return
			}
			continue
		}
		if len(line) > 0 {
			fn(trimEOL(append([]byte{}, line...)))
		}
		if err != nil {
			return
		}
	}
}

func trimEOL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
