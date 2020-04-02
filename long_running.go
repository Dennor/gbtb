package gbtb

import (
	"fmt"
	"os"
	"os/signal"
	"time"
)

// StoppableLongRunning adds support for long running stoppable tasks
type StoppableLongRunning struct {
	// Name of long running target
	Name string
	// Dependencies of long running target
	Dependencies Dependencies
	// Job is stoppable job for long running task
	Job func(chan struct{}) error

	Stop chan struct{}
}

// Do runs long running task and waits for an interrupt from os
func (l *StoppableLongRunning) Do(tasks Tasks, runner *Runner) (time.Time, error) {
	stop := l.Stop
	job := l.Job
	t := Task{
		Name:         l.Name,
		Dependencies: l.Dependencies,
		Job: func() error {
			return job(stop)
		},
	}
	return t.Do(tasks, runner)
}

// DependsOn for long running task
func (l *StoppableLongRunning) DependsOn() Dependencies {
	return l.Dependencies
}

// Reset the long running task
func (l *StoppableLongRunning) Reset() {}

func (l *StoppableLongRunning) GetNames() []string { return []string{l.Name} }
func (l *StoppableLongRunning) GetTask(name string) TaskLike {
	if name != l.Name {
		return nil
	}
	return l
}

// LongRunning adds support for long running tasks stopped on os.Interrupt
type LongRunning struct {
	// Name of long running target
	Name string
	// Dependencies of long running target
	Dependencies Dependencies
	// Job is stoppable job for long running task
	Job func(chan struct{}) error
}

// Do runs long running task and waits for an interrupt from os
func (l *LongRunning) Do(tasks Tasks, runner *Runner) (time.Time, error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	stop := make(chan struct{})
	go func() {
		<-c
		close(stop)
	}()
	stoppable := StoppableLongRunning{
		Name:         l.Name,
		Dependencies: l.Dependencies,
		Job:          l.Job,
		Stop:         stop,
	}
	return stoppable.Do(tasks, runner)
}

// DependsOn for long running task
func (l *LongRunning) DependsOn() Dependencies {
	return l.Dependencies
}

func (l *LongRunning) GetNames() []string { return []string{l.Name} }
func (l *LongRunning) GetTask(name string) TaskLike {
	if name != l.Name {
		return nil
	}
	return l
}

// Reset the long running task
func (l *LongRunning) Reset() {}

// NotifyLongRunning start a long running task and restarts if there's a change detected
// in file system
type NotifyLongRunning struct {
	// Name of long running target
	Name string
	// Dependencies of long running target
	Dependencies Dependencies
	// Job is stoppable job for long running task
	Job func(chan struct{}) error
}

func doStoppableAndLogError(f func(chan struct{}) error, stop chan struct{}) {
}

type stoppableJob struct {
	j    func(chan struct{}) error
	wait chan struct{}
	stop chan struct{}
}

func (s *stoppableJob) Start() {
	stop := make(chan struct{})
	go func() {
		if err := s.j(stop); err != nil {
			fmt.Println(err)
		}
		s.wait <- struct{}{}
	}()
	s.stop = stop
}

func (s *stoppableJob) Wait() {
	close(s.stop)
	<-s.wait
}

func newStoppableJob(f func(chan struct{}) error) stoppableJob {
	return stoppableJob{
		j:    f,
		wait: make(chan struct{}),
	}
}

func (l *NotifyLongRunning) doStoppable(tasks Tasks, runner *Runner) (chan struct{}, chan struct{}) {
	restart := make(chan struct{})
	j := l.Job
	stoppable := StoppableLongRunning{
		Name:         l.Name,
		Dependencies: l.Dependencies,
		Job: func(restart chan struct{}) error {
			stoppable := newStoppableJob(j)
			stoppable.Start()
			for range restart {
				stoppable.Wait()
				stoppable.Start()
			}
			stoppable.Wait()
			return nil
		},
		Stop: restart,
	}
	wait := make(chan struct{})
	go func() {
		stoppable.Do(tasks, runner)
		close(wait)
	}()
	return wait, restart
}

// Do runs long running task and waits for an interrupt from os
func (l *NotifyLongRunning) Do(tasks Tasks, runner *Runner) (time.Time, error) {
	wait, restart := l.doStoppable(tasks, runner)
	defer func() {
		close(restart)
		<-wait
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	stop := make(chan struct{})
	go func() {
		<-c
		close(stop)
	}()
	notify := Notify{
		Task: &Task{
			Name:         l.Name,
			Dependencies: l.Dependencies,
			Job: func() error {
				restart <- struct{}{}
				return nil
			},
		},
	}
	fdeps, tdeps, err := notify.deps(tasks, notify.Task)
	if err != nil {
		return time.Time{}, err
	}
	return time.Time{}, notify.job(
		tasks,
		runner,
		fdeps,
		tdeps,
		stop,
	)
}

// DependsOn for long running task
func (l *NotifyLongRunning) DependsOn() Dependencies {
	return l.Dependencies
}

// Reset the long running task
func (l *NotifyLongRunning) Reset() {}

func (l *NotifyLongRunning) GetNames() []string { return []string{l.Name} }
func (l *NotifyLongRunning) GetTask(name string) TaskLike {
	if name != l.Name {
		return nil
	}
	return l
}
