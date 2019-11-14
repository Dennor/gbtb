# About

Go Build Toolbox is a very simple toolbox providing some small utility functions that make
it easy to write golang files dedicated to building.

It's purpose is to eliminate any build tool dependencies in your project *except* for go compiler itself.
Including `make`.
That's why Go Build Toolbox does not have any command line utility, as it's not meant to be ran as one.

Writing a build script in go is quite simple, the only thing really missing is running tasks in parallel and task dependency
management similar to `make`. And that's pretty much the main thing this toolbox aims to provide, along with few convienience functions.

# Example

For instance if we have a simple project that would contain Makefile like:


```Makefile
all: app

go_files := $(shell find . -name '*.go')

app: $(go_files)
	go build -o app main.go
```


Could be accomplished with:
```golang
// +build make

package main

import (
	"github.com/Dennor/gbtb"
)

func main() {
	gbtb.Tasks{
		gbtb.Task{
			Name:         "all",
			Dependencies: gbtb.StaticDependencies{"app"},
		},
		gbtb.Task{
			Name:         "app",
			Job:          gbtb.GoBuild("main.go", "-o", "app"),
			Dependencies: gbtb.GlobFiles("**/*.go"),
		},
	}.MustRun()
}
```

# Non-goals

* Make syntax shorter than `Makefile`. While it would be nice to add as many convienience functions as possible to shorten the build files, that's not the goal of this project. Atleast not a main one. If anyone writes a convienience function and makes a PR with it, I'll be happy to include it, but unless I personally need something, I will not be adding new functionalities.
* Make a tool out of it. I do not want it to become another tool that you need to install (even through go get). Your main function should be enough.
