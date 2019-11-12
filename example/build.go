// +build make

package main

import (
	"flag"
	"log"

	"github.com/Dennor/gbtb"
)

func main() {
	gbtb.FlagsInit(nil)
	flag.Parse()
	tasks := gbtb.Tasks{
		&gbtb.Task{ // Similar to make, a default task that run's all tasks
			Name:         "all",
			Dependencies: gbtb.StaticDependencies{"app", "task-with-output"},
		},
		&gbtb.Task{ // Task that is always out-of-date and prints some output to stdout
			Name: "task-with-output",
			Job: func() error {
				return gbtb.RunCommand("/bin/sh", "-c", "echo 'hello world'")
			},
		},
		&gbtb.Task{ // Task dependant on files matching glob pattern, build
			Name: "app",
			Job: func() error {
				return gbtb.RunCommand("go", "build", "-o", "app", "main.go")
			},
			Dependencies: gbtb.GlobFiles("**/*.go"),
		},
	}
	if err := tasks.Do(flag.Args()...); err != nil {
		log.Fatalf("%v", err)
	}
}
