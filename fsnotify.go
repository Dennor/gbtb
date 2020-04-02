package gbtb

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"gopkg.in/fsnotify.v1"
)

// Notify task is a blocking task that runs named task when change in file system is detected
type Notify struct {
	// Name of notify task
	Name string
	// Job is a name of task to run on change
	Job string

	Task TaskLike
}

func (n *Notify) GetNames() []string {
	return []string{n.Name}
}

type watcher struct {
	*fsnotify.Watcher
	fileChange chan struct{}
}

func (w *watcher) watch(fdeps map[string]struct{}, done chan error, stop chan struct{}) {
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
		n.Task.Reset()
		_, err := n.Task.Do(tasks, runner)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func (n *Notify) job(
	tasks Tasks,
	runner *Runner,
	fdeps map[string]struct{},
	tDeps map[string]struct{},
	stop chan struct{},
) error {
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

	done := make(chan error)
	go watcher.watch(fdeps, done, stop)

	for d := range fdeps {
		err := watcher.Add(d)
		if err != nil {
			return err
		}
	}
	for d := range dWatch {
		err := watcher.Add(d)
		if err != nil {
			return err
		}
	}

	n.build(tasks, runner, tDeps, fileChange)
	return <-done
}

func (n *Notify) deps(tasks Tasks, task TaskLike) (
	fileDeps map[string]struct{},
	taskDeps map[string]struct{},
	err error) {
	if task == nil {
		err = fmt.Errorf("task %s does not exist", n.Job)
		return
	}
	fileDeps = make(map[string]struct{})
	taskDeps = make(map[string]struct{})
	deps, err := task.DependsOn().Get()
	if err == nil {
		for _, dep := range deps {
			if tDep := tasks.getTask(dep); tDep != nil {
				var fDeps map[string]struct{}
				var tDeps map[string]struct{}
				fDeps, tDeps, err = n.deps(tasks, tDep)
				if err != nil {
					break
				}
				for k := range fDeps {
					fileDeps[k] = struct{}{}
				}
				taskDeps[dep] = struct{}{}
				for k := range tDeps {
					taskDeps[k] = struct{}{}
				}
			} else {
				fileDeps[dep] = struct{}{}
			}
		}
	}
	return
}

func (n *Notify) Do(tasks Tasks, runner *Runner) (time.Time, error) {
	if n.Task == nil {
		n.Task = tasks.getTask(n.Job)
		if n.Task == nil {
			return time.Time{}, fmt.Errorf("task \"%s\" does not exist", n.Job)
		}
	}
	fdeps, tdeps, err := n.deps(tasks, n.Task)
	if err != nil {
		return time.Time{}, err
	}
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		close(stop)
	}()
	err = n.job(
		tasks,
		runner,
		fdeps,
		tdeps,
		stop,
	)
	close(c)
	// Notify is always out of date
	return time.Time{}, err
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
