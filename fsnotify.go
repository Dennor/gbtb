package gbtb

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/fsnotify.v1"
)

// Notify task is a blocking task that runs named task when change in file system is detected
type Notify struct {
	// Name of notify task
	Name string
	// Job is a name of task to run on change
	Job string

	task TaskLike
	lock sync.Mutex
}

func (n *Notify) GetNames() []string {
	n.lock.Lock()
	defer n.lock.Unlock()
	return []string{n.Name}
}

type watcher struct {
	*fsnotify.Watcher
	fileChange chan struct{}
}

func (w *watcher) watch(fdeps map[string]struct{}, done chan error, stop chan os.Signal) {
	var err error
	defer func() {
		close(w.fileChange)
		done <- err
	}()
	for {
		select {
		case <-stop:
			return
		case ev, ok := <-w.Events:
			if !ok {
				err = fmt.Errorf("watcher error")
				return
			}
			if _, ok := fdeps[ev.Name]; ok {
				fileModTimeCache.Delete(ev.Name)
				if ev.Op == fsnotify.Create {
					w.Add(ev.Name)
				}
				w.fileChange <- struct{}{}
			}
		case _, ok := <-w.Errors:
			if !ok {
				err = fmt.Errorf("watcher error")
				return
			}
		}
	}
}

func newWatcher(fileChange chan struct{}) (*watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &watcher{w, fileChange}, nil
}

func (n *Notify) build(tasks Tasks, runner *Runner, tDeps map[string]struct{}, fileChange chan struct{}) {
	for range fileChange {
		// Wait a second to collect events because editors like vim
		// can generate quite a few events in very short period of time
		// for one write
		wait := true
		for wait {
			timer := time.NewTimer(time.Second)
			select {
			case <-timer.C:
				// timer expired, don't wait any longer
				wait = false
			case <-fileChange:
				// new change, wait another second
			}
		}
		for td := range tDeps {
			if t := tasks.getTask(td); t != nil {
				t.Reset()
			}
		}
		n.lock.Lock()
		n.task.Reset()
		_, err := n.task.Do(tasks, runner)
		n.lock.Unlock()
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (n *Notify) job(tasks Tasks, runner *Runner, fdeps map[string]struct{}, tDeps map[string]struct{}) error {
	// Watch base directories for each watch because certain
	// which recreate file on
	dWatch := make(map[string]struct{})
	for d := range fdeps {
		bd := filepath.Dir(d)
		if _, ok := fdeps[bd]; !ok {
			dWatch[bd] = struct{}{}
		}
	}

	fileChange := make(chan struct{})
	watcher, err := newWatcher(fileChange)
	if err != nil {
		return err
	}
	defer watcher.Close()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	done := make(chan error)
	go watcher.watch(fdeps, done, c)

	for d := range fdeps {
		err := watcher.Add(d)
		if err != nil {
			close(c)
			return err
		}
	}
	for d := range dWatch {
		err := watcher.Add(d)
		if err != nil {
			close(c)
			return err
		}
	}

	n.build(tasks, runner, tDeps, fileChange)
	return <-done
}

func (n *Notify) deps(tasks Tasks, task TaskLike) (
	fileDeps []string,
	taskDeps []string,
	err error) {
	if task == nil {
		err = fmt.Errorf("task %s does not exist", n.Job)
		return
	}
	deps, err := task.DependsOn().Get()
	if err == nil {
		for _, dep := range deps {
			if tDep := tasks.getTask(dep); tDep != nil {
				var fDeps []string
				var tDeps []string
				fDeps, tDeps, err = n.deps(tasks, tDep)
				if err != nil {
					break
				}
				fileDeps = append(fileDeps, fDeps...)
				taskDeps = append(taskDeps, dep)
				taskDeps = append(taskDeps, tDeps...)
			} else {
				fileDeps = append(fileDeps, dep)
			}
		}
	}
	return
}

func (n *Notify) Do(tasks Tasks, runner *Runner) (time.Time, error) {
	n.lock.Lock()
	n.task = tasks.getTask(n.Job)
	if n.task == nil {
		n.lock.Unlock()
		return time.Time{}, fmt.Errorf("task does not exist")
	}
	fdeps, tdeps, err := n.deps(tasks, n.task)
	n.lock.Unlock()
	if err != nil {
		return time.Time{}, err
	}
	uniqueDeps := make(map[string]struct{})
	for _, d := range fdeps {
		uniqueDeps[d] = struct{}{}
	}
	uniqueTaskDeps := make(map[string]struct{})
	for _, d := range tdeps {
		uniqueTaskDeps[d] = struct{}{}
	}
	// Notify is always out of date
	return time.Time{}, n.job(tasks, runner, uniqueDeps, uniqueTaskDeps)
}

func (n *Notify) DependsOn() Dependencies {
	// Notify does not depend on anything
	return nil
}

func (n *Notify) GetTask(name string) (t TaskLike) {
	if n.Name == name {
		t = n
	}
	return
}

func (n *Notify) Reset() {}
