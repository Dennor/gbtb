// +build make

package main

import (
	"github.com/Dennor/gbtb"
)

func main() {
	gbtb.Tasks{
		&gbtb.Task{ // Similar to make, a default task that run's all tasks
			Name:         "all",
			Dependencies: gbtb.StaticDependencies{"app", "task-with-output"},
		},
		&gbtb.Task{ // Task that is always out-of-date and prints some output to stdout
			Name: "task-with-output",
			Job:  gbtb.CommandJob("/bin/sh", "-c", "echo 'hello world'"),
		},
		&gbtb.Task{ // Task dependant on files matching glob pattern, build
			Name:         "app",
			Job:          gbtb.GoBuild("main.go", "-o", "app"),
			Dependencies: gbtb.GlobFiles("**/*.go"),
		},
	}.MustRun()
}
