package gbtb

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/buildkite/interpolate"
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
	for _, ue := range cmd.Env {
		ekey := ue[:strings.Index(ue, "=")+1]
		pop := -1
		for i := 0; i < len(envs) && pop == -1; i++ {
			if strings.HasPrefix(envs[i], ekey) {
				pop = i
			}
		}
		if pop != -1 {
			copy(envs[pop:], envs[pop+1:])
			envs = envs[:len(envs)-1]
		}
		envs = append(envs, ue)
	}
	cmd.Env = envs
}

// RunCommand runs a single command. For more information check out PipeCommands
func RunCommand(cmd string, args ...string) error {
	return PipeCommands(exec.Command(cmd, args...))
}

func pipeCommands(cmds ...*exec.Cmd) (err error) {
	defer func() {
		if err != nil {
			for _, cmd := range cmds {
				if cmd.Process != nil {
					cmd.Process.Kill()
				}
			}
		}
	}()
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
		env := interpolate.NewSliceEnv(cmd.Env)
		cmd.Path, err = interpolate.Interpolate(env, cmd.Path)
		if err != nil {
			return
		}
		for i := range cmd.Args {
			cmd.Args[i], err = interpolate.Interpolate(env, cmd.Args[i])
			if err != nil {
				return
			}
		}
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
		if err = cmd.Start(); err != nil {
			return
		}
	}
	return nil
}

// PipeCommands check out documentation for PipeCommandsContext
func PipeCommands(cmds ...*exec.Cmd) (err error) {
	if err := pipeCommands(cmds...); err != nil {
		return err
	}
	for _, cmd := range cmds {
		if cerr := cmd.Wait(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return
}

// PipeCommandsContext runs a list of commands piping input from previous command to
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
// On context done function attempts to terminate proccess with os.Interrupt, if process
// does not terminate in 10 seconds, it gets killed.
func PipeCommandsContext(ctx context.Context, cmds ...*exec.Cmd) (err error) {
	if err := pipeCommands(cmds...); err != nil {
		return err
	}
	wait := make(chan error)
	go func() {
		var err error
		for _, cmd := range cmds {
			if cerr := cmd.Wait(); cerr != nil && err == nil {
				err = cerr
			}
		}
		wait <- err
	}()
	select {
	case err = <-wait:
	case <-ctx.Done():
		for _, cmd := range cmds {
			cmd.Process.Signal(os.Interrupt)
		}
		timer := time.NewTimer(time.Second * 10)
		select {
		case <-timer.C:
			for _, cmd := range cmds {
				cmd.Process.Kill()
			}
			err = <-wait
		case err = <-wait:
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

// RunStoppable Runs a command with an ability to stop it
func RunStoppable(stop chan struct{}, name string, args ...string) error {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command(name, args...)
	go func() {
		<-stop
		cancel()
	}()
	return PipeCommandsContext(ctx, cmd)
}
