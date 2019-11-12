package gbtb

// Go is a simple job using go compiler
func Go(subCommand string, args ...string) Job {
	return func() error {
		args = append([]string{subCommand}, args...)
		return RunCommand("go", args...)
	}
}

// GoBuild is a `go build` job
func GoBuild(pkg string, opts ...string) Job {
	return Go("build", opts...)
}

// GoRun is a `go run` job
func GoRun(pkg string, opts ...string) Job {
	return Go("run", opts...)
}
