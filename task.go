package gbtb

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Task in build
type Task struct {
	// Name of a task, with behaviour similar to make.
	// If a file matching task name exists, it is assumed that it's a result of this
	// task is that file, and like in make, mod times of resulting file
	// and it's dependencies will be compared.
	Name string
	// List of dependencies. A dependency can be a task name or a file, checked in that
	// order. If there's no task named as dependency or no file matching that name
	// build task will fail
	Dependencies Dependencies
	// Job ran by task
	Job Job
	// ModTime is a function that allows user to override default behaviour
	// testing when was the target updated last time. For example docker image
	// creation date. If not provided, a mod time of a file with the same
	// name as task name is used.
	ModTime ModTime

	modTime time.Time
	done    bool
	err     error
	lock    sync.Mutex
}

func (t *Task) getModtime() (tt time.Time, err error) {
	if t.ModTime != nil {
		tt, err = t.ModTime()
	} else {
		var fi os.FileInfo
		fi, err = os.Stat(t.Name)
		if err == nil {
			tt = fi.ModTime()
		}
		if os.IsNotExist(err) {
			err = nil
		}
	}
	return
}

// Do executes a task along with dependencies
func (t *Task) Do(tasks Tasks, runner *Runner) (time.Time, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.done {
		return t.modTime, t.err
	}
	wg := sync.WaitGroup{}
	var dependencyFailures []string
	dependencyFailureCh := make(chan string)
	var timestamps []time.Time
	timestampCh := make(chan time.Time)
	// monitor errors of dependant tasks
	waitForDependencies, waitForTimestamps := make(chan struct{}), make(chan struct{})
	go func() {
		for d := range dependencyFailureCh {
			dependencyFailures = append(dependencyFailures, d)
		}
		close(waitForDependencies)
	}()
	// monitor timestamps of dependencies
	go func() {
		for t := range timestampCh {
			timestamps = append(timestamps, t)
		}
		close(waitForTimestamps)
	}()
	var err error
	var dependencies []string
	if t.Dependencies != nil {
		dependencies, err = t.Dependencies.Get()
	}
	if err == nil {
		t.modTime, err = t.getModtime()
	}
	if err != nil {
		return t.modTime, err
	}
	for len(dependencies) > 0 {
		dep := dependencies[0]
		var depTask TaskLike
		if dep == t.Name {
			return t.modTime, fmt.Errorf("task %s depends on itself", t.Name)
		}
		for _, tg := range tasks {
			if depTask = tg.GetTask(dep); depTask != nil {
				break
			}
		}
		if depTask == nil {
			var t time.Time
			v, ok := fileModTimeCache.Load(dep)
			if !ok {
				t, err = fileDependency(dep)
				if err == nil {
					fileModTimeCache.Store(dep, t)
				} else {
					dependencyFailureCh <- dep
				}
			} else {
				t = v.(time.Time)
			}
			timestampCh <- t
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				t, err := depTask.Do(tasks, runner)
				if err != nil {
					dependencyFailureCh <- dep
					return
				}
				timestampCh <- t
			}()
		}
		dependencies = dependencies[1:]
	}
	wg.Wait()
	close(dependencyFailureCh)
	close(timestampCh)
	<-waitForDependencies
	<-waitForTimestamps
	if len(dependencyFailures) != 0 {
		t.err = fmt.Errorf("dependencies %v could not be satisified", dependencyFailures)
	} else {
		// zero time is always out of date
		upToDate := t.modTime != zeroTime
		for _, tt := range timestamps {
			upToDate = upToDate && !t.modTime.Before(tt)
			if !upToDate {
				break
			}
		}
		if !upToDate {
			fmt.Printf("building %s\n", t.Name)
			t.err = runner.Put(t.Job)
			if t.err == nil {
				// refresh modTime after update
				t.modTime, t.err = t.getModtime()
			}
		} else {
			fmt.Printf("task %s is up to date\n", t.Name)
		}
	}
	if t.err != nil {
		fmt.Println(t.err)
	}
	t.done = true
	return t.modTime, t.err
}

func (t *Task) DependsOn() Dependencies {
	return t.Dependencies
}

func (t *Task) Reset() {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.done = false
}

func (t *Task) GetNames() []string {
	return []string{t.Name}
}

func (t *Task) GetTask(name string) TaskLike {
	if name == t.Name {
		return t
	}
	return nil
}
