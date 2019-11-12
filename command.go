package gbtb

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type syncedIOWriteCloser struct {
	io.Writer
	lock sync.Locker
}

func (s *syncedIOWriteCloser) Write(p []byte) (int, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.Writer.Write(p)
}

func newLineSynchronizedWriter(lock *sync.Mutex, w io.Writer) io.WriteCloser {
	pr, pw := io.Pipe()
	go func() {
		io.Copy(&syncedIOWriteCloser{w, lock}, pr)
	}()
	return pw
}

var (
	stdout = newLineSynchronizedWriter(&sync.Mutex{}, os.Stdout)
	stderr = newLineSynchronizedWriter(&sync.Mutex{}, os.Stderr)
)

func addEnv(cmd *exec.Cmd) {
	envs := append([]string{}, os.Environ()...)
	for len(cmd.Env) > 0 {
		ekey := cmd.Env[0][:len(strings.Split(cmd.Env[0], "=")[0])+1]
		for i, e := range envs {
			if strings.HasPrefix(e, ekey) {
				envs[i] = cmd.Env[0]
			}
		}
		cmd.Env = cmd.Env[1:]
	}
	cmd.Env = envs
}

// RunCommand runs a single command. For more information check out PipeCommands
func RunCommand(cmd string, args ...string) error {
	return PipeCommands(exec.Command(cmd, args...))
}

// PipeCommands runs a list of commands piping input from previous command to
// output of the following one.
// If first command in a list has Stdin set, it will be used as a Stdin for the first
// command in the pipe. Otherwise reader that will always return io.EOF will be used.
// If caller provides a custom input, it is caller's responsibility to make sure that
// it will end reading with io.EOF, at some point, otherwise this function will block.
// If last command in a list has Stdout set, it will be used as a Stdout for the last
// command in the pipe, otherwise os.Stdout is used
// For each command stderr is piped to os.Stderr, unless command has non nil Stderr
// If any proccess in the pipe finished with an error, all proccesses that did not get
// waited on will be killed and function will return that error. If more than one
// proccess finishes with an error, it is not guaranteed which one will be returned
// except for the fact that atleast one will be returned
// Function appends current environment flags to command without overriding those
// already defined.
func PipeCommands(cmds ...*exec.Cmd) (err error) {
	if len(cmds) == 0 {
		return nil
	}
	first, last := cmds[0], cmds[len(cmds)-1]
	if last.Stdout == nil {
		last.Stdout = stdout
	}
	in := first.Stdin
	for _, cmd := range cmds {
		addEnv(cmd)
		cmd.Stdin = in
		if cmd != last {
			pr, pw := io.Pipe()
			defer pw.Close()
			cmd.Stdout = pw
			in = pr
		}
		if cmd.Stderr == nil {
			cmd.Stderr = stderr
		}
		if err := cmd.Start(); err != nil {
			return err
		}
	}
	for _, cmd := range cmds {
		if cerr := cmd.Wait(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return
}

// OutputPipe runs a command pipe and returns the stdout of last command panicing on error
func OutputPipe(cmds ...*exec.Cmd) ([]byte, error) {
	if len(cmds) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	cmds[len(cmds)-1].Stdout = &buf
	err := PipeCommands(cmds...)
	return buf.Bytes(), err
}

// MustOutputPipe runs a command and returns it's stdout panicing on error
func MustOutputPipe(cmds ...*exec.Cmd) []byte {
	b, err := OutputPipe(cmds...)
	if err != nil {
		panic(err)
	}
	return b
}

// Output runs a command and returns it's stdout
func Output(cmd string, args ...string) ([]byte, error) {
	return OutputPipe(exec.Command(cmd, args...))
}

// MustOutput runs a command and returns it's stdout panicing on error
func MustOutput(cmd string, args ...string) []byte {
	return MustOutputPipe(exec.Command(cmd, args...))
}
