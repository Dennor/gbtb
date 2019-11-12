package gbtb

import (
	"fmt"
	"sync"
)

// Runner executes jobs in parallel
type Runner struct {
	jobs  chan func()
	queue chan job

	running bool
	l       sync.Mutex
}

type job struct {
	f   func() error
	err chan error
}

func (r *Runner) run() {
	defer func() {
		r.l.Lock()
		r.running = false
		r.l.Unlock()
		close(r.jobs)
	}()
	for j := range r.queue {
		jc := j
		f := func() {
			var err error
			if jc.f != nil {
				err = jc.f()
			}
			jc.err <- err
		}
		r.jobs <- f
	}
}

// Put queues a job for runner. Return error is the same as an error returned by f.
// User must ensure that all Put calls finished before calling Stop
func (r *Runner) Put(f func() error) error {
	errCh := make(chan error)
	r.queue <- job{f, errCh}
	return <-errCh
}

type worker interface {
	run()
}

type synchronusWorker chan func()

func (s synchronusWorker) run() {
	for j := range s {
		j()
	}
}

type asynchronusWorker chan func()

func (a asynchronusWorker) run() {
	for j := range a {
		go j()
	}
}

// Start begins running tasks
// It is an error to call Start a second time if runner was not stopped.
func (r *Runner) Start() error {
	r.l.Lock()
	defer r.l.Unlock()
	if r.running {
		return fmt.Errorf("runner is running")
	}
	r.running = true
	r.queue = make(chan job)
	r.jobs = make(chan func())
	if Jobs > 0 {
		for i := 0; i < Jobs; i++ {
			go synchronusWorker(r.jobs).run()
		}
	} else {
		go asynchronusWorker(r.jobs).run()
	}
	go r.run()
	return nil
}

// Stop cleans up after a runner. All Put calls must have finished before Stop is called
func (r *Runner) Stop() {
	close(r.queue)
}
