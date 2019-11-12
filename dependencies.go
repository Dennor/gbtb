package gbtb

import (
	"fmt"

	"github.com/bmatcuk/doublestar"
)

// Dependencies is an interface providing access to task
// dependecies
type Dependencies interface {
	Get() ([]string, error)
}

// StaticDependencies is a list of strings naming dependencies directly
type StaticDependencies []string

func (d StaticDependencies) Get() ([]string, error) {
	return []string(d), nil
}

func (d StaticDependencies) TargetDependencies(string) Dependencies {
	return d
}

// DependencyFunc is a helper type useful for constructing dependencies
// only when tasked is executed
type DependencyFunc func() ([]string, error)

func (d DependencyFunc) Get() ([]string, error) {
	return d()
}

func (d DependencyFunc) TargetDependencies(string) Dependencies {
	return d
}

// GlobFiles returns a glob dependency supporting double start (**) matching
func GlobFiles(pattern string) DependencyFunc {
	return DependencyFunc(func() ([]string, error) {
		return doublestar.Glob(pattern)
	})
}

// DependenciesList is a helper to easly build multiple dependencies for target
type DependenciesList []Dependencies

// Append a new dependency to chain
func (dc DependenciesList) Append(d ...Dependencies) DependenciesList {
	return DependenciesList(append(dc, d...))
}

// Get returns evaluated list of dependencies for a target
func (dc DependenciesList) Get() ([]string, error) {
	var deps []string
	for _, d := range dc {
		dep, err := d.Get()
		if err != nil {
			return nil, err
		}
		deps = append(deps, dep...)
	}
	return deps, nil
}

func (dc DependenciesList) TargetDependencies(string) Dependencies {
	return dc
}

// NewDependenciesList returns a new dependency chain
func NewDependenciesList(d Dependencies, nd ...Dependencies) DependenciesList {
	return DependenciesList{d}.Append(nd...)
}

// MultiTargetDependencies is an interface providing access to MultiTargetTask
// dependecies
type MultiTargetDependencies interface {
	TargetDependencies(string) Dependencies
}

type StringFormatWithName []string

func (m StringFormatWithName) TargetDependencies(name string) Dependencies {
	sd := make(StaticDependencies, 0, len(m))
	for _, s := range m {
		sd = append(sd, fmt.Sprintf(s, name))
	}
	return sd
}

// MultiTargetDependencyFunc is an convienience type implementing MultiTargetDependencies
type MultiTargetDependencyFunc func(string) Dependencies

func (m MultiTargetDependencyFunc) TargetDependencies(name string) Dependencies {
	return m(name)
}

// MultiTargetDependencyList is a static list of multi target dependencies
type MultiTargetDependencyList []MultiTargetDependencies

func (d MultiTargetDependencyList) TargetDependencies(name string) Dependencies {
	dl := make(DependenciesList, 0, len(d))
	for _, dep := range d {
		dl = append(dl, dep.TargetDependencies(name))
	}
	return dl
}
