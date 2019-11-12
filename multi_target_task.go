package gbtb

import (
	"sync"
	"time"
)

// MultiTargetTask is a group targets sharing similar tasks
type MultiTargetTask struct {
	Names        []string
	Job          MultiTargetJob
	ModTime      MultiTargetModTime
	Dependencies MultiTargetDependencies
	lock         sync.Mutex
	tasks        map[string]*Task
}

// GetNames defined for MultiTargetTask
func (m *MultiTargetTask) GetNames() []string {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.Names
}

func (m *MultiTargetTask) init() {
	m.tasks = make(map[string]*Task)
	for _, tg := range m.Names {
		m.tasks[tg] = nil
	}
}

func (m *MultiTargetTask) createTask(name string) {
	task := Task{
		Name: name,
	}
	if m.Job != nil {
		task.Job = func() error {
			return m.Job(name)
		}
	}
	if m.ModTime != nil {
		task.ModTime = func() (time.Time, error) {
			return m.ModTime(name)
		}
	}
	if m.Dependencies != nil {
		task.Dependencies = m.Dependencies.TargetDependencies(name)
	}
	m.tasks[name] = &task
}

// GetTask for a name
func (m *MultiTargetTask) GetTask(name string) *Task {
	m.lock.Lock()
	defer m.lock.Unlock()
	if len(m.tasks) != len(m.Names) {
		m.init()
	}
	if t, ok := m.tasks[name]; ok && t == nil {
		m.createTask(name)
	}
	return m.tasks[name]
}
