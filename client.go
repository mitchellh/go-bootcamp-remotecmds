package main

// Request is the type sent by client to the server to request that a
// command be executed.
type Request struct {
	Command string
	Args    map[string]string
}

// Response is the result from the server with the result of executing
// the requested command.
type Response struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
