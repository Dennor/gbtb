package gbtb

import (
	"os/exec"
	"strings"
	"time"
)

// ModTime returns the last time artifacts resulting from task
// were modified
type ModTime func() (time.Time, error)

// CommandModTime is a convienience function that simply runs a command
// which must only return a time with given format
func CommandModTime(format, cmd string, args ...string) ModTime {
	return CommandModTimePipe(format, exec.Command(cmd, args...))
}

// CommandModTimePipe is a convienience function that runs a pipe of commands
// which must only return a time with given format
func CommandModTimePipe(format string, cmds ...*exec.Cmd) ModTime {
	return func() (time.Time, error) {
		b, err := OutputPipe(cmds...)
		if err != nil {
			return time.Time{}, nil
		}
		return time.Parse(format, strings.TrimSpace(string(b)))
	}
}

// MultiTargetModTime for a multitarget task
type MultiTargetModTime func(string) (time.Time, error)
