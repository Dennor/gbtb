package gbtb

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

var (
	// Jobs is a global defining how many jobs can be run in parallel at once
	Jobs             = runtime.NumCPU()
	zeroTime         = time.Time{}
	fileModTimeCache sync.Map
)

func fileDependency(fn string) (t time.Time, err error) {
	st, err := os.Stat(fn)
	if err == nil {
		t = st.ModTime()
	}
	return
}

type TaskLike interface {
	// Do a tasks along with it's dependencies using Runner
	Do(Tasks, *Runner) (time.Time, error)
	// DependsOn is represents dependencies needed for a task
	DependsOn() Dependencies
	// Reset marks task as not done
	Reset()
}

// TaskGetter interface
type TaskGetter interface {
	GetNames() []string
	GetTask(name string) TaskLike
}

type failedTasks struct {
	failed   []string
	failedCh chan string
	done     chan struct{}
}

func newFailedTasks() failedTasks {
	return failedTasks{
		failedCh: make(chan string),
		done:     make(chan struct{}),
	}
}

func (f *failedTasks) run() {
	for t := range f.failedCh {
		f.failed = append(f.failed, t)
	}
	close(f.done)
}

func (f *failedTasks) get() []string {
	close(f.failedCh)
	<-f.done
	return f.failed
}

func (f *failedTasks) fail(name string) {
	f.failedCh <- name
}

// Tasks is a list of tasks defined by build
type Tasks []TaskGetter

func (tasks Tasks) getTask(name string) TaskLike {
	for _, tt := range tasks {
		if t := tt.GetTask(name); t != nil {
			return t
		}
	}
	return nil
}

func (tasks Tasks) execute(taskNames []string, allNames []string, failedTasks *failedTasks) error {
	wg := sync.WaitGroup{}
	runner := new(Runner)
	runner.Start()
	defer func() {
		wg.Wait()
		runner.Stop()
	}()
	for i := range taskNames {
		t := tasks.getTask(taskNames[i])
		if t == nil {
			return fmt.Errorf("task %s not found", taskNames[i])
		}
		tName := taskNames[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := t.Do(tasks, runner)
			if err != nil {
				failedTasks.fail(tName)
			}
		}()
	}
	return nil
}

func (tasks Tasks) definedTasks() ([]string, error) {
	var allNames []string
	var uniqueNames map[string]struct{}
	for _, tg := range tasks {
		for _, name := range tg.GetNames() {
			if _, ok := uniqueNames[name]; ok {
				return nil, fmt.Errorf("task %s redefined", name)
			}
			allNames = append(allNames, name)
		}
	}
	return allNames, nil
}

// Do executes a list of tasks out of all tasks defined, if no task
// is provided, just like make, execute the first task.
func (tasks Tasks) Do(taskNames ...string) (err error) {
	allNames, err := tasks.definedTasks()
	if err != nil {
		return
	}
	if len(allNames) == 0 {
		fmt.Println("no tasks defined")
		return nil
	}
	if len(taskNames) == 0 {
		taskNames = []string{allNames[0]}
	}
	failedTasks := newFailedTasks()
	go failedTasks.run()
	if err = tasks.execute(taskNames, allNames, &failedTasks); err == nil {
		if len(failedTasks.get()) != 0 {
			err = fmt.Errorf("tasks %v failed", failedTasks)
		}
	}
	return
}

// RunWithFlags adds gbtb to flagSet, parses flags and runs with them. If flagSet nil
// fall back to flag.CommandLine
func (tasks Tasks) RunWithFlags(flagSet *flag.FlagSet, args ...string) error {
	if flagSet == nil {
		flagSet = flag.CommandLine
		if len(args) == 0 {
			args = os.Args[1:]
		}
	}
	FlagsInit(flagSet)
	if err := flagSet.Parse(args); err != nil {
		return err
	}
	return tasks.Do(flagSet.Args()...)
}

// MustRunWithFlags adds gbtb to flagSet, parses flags and runs with them. If flagSet nil
// fall back to flag.CommandLine, if there's an error process exits with status 1
func (tasks Tasks) MustRunWithFlags(flagSet *flag.FlagSet, args ...string) {
	if err := tasks.RunWithFlags(flagSet, args...); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

// Run gbtb with command line arguments
func (tasks Tasks) Run() error {
	return tasks.RunWithFlags(nil)
}

// MustRun gbtb with command line arguments, if there's an error exits process with status 1
func (tasks Tasks) MustRun() {
	tasks.MustRunWithFlags(nil)
}

// FlagsInit adds gbtb flags to flagSet, if flagSet is nil, flags are added
// to flag.CommandLine
func FlagsInit(flagSet *flag.FlagSet) {
	if flagSet == nil {
		flagSet = flag.CommandLine
	}
	flag.IntVar(&Jobs, "jobs", runtime.NumCPU(), "maximum number of jubs that can be run in parallel")
}
