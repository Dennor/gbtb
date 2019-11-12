package gbtb

import "os/exec"

// Job is an operation in build
type Job func() error

// CommandJob is a convienience function that simply runs a command as a job
func CommandJob(cmd string, args ...string) Job {
	return CommandJobPipe(exec.Command(cmd, args...))
}

// CommandJobPipe is a convienience function that runs a pipe of commands as a job
func CommandJobPipe(cmds ...*exec.Cmd) Job {
	return func() error {
		return PipeCommands(cmds...)
	}
}

// MultiTargetJob for a multitarget task
type MultiTargetJob func(string) error
