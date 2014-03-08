package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
	"os/exec"
	"sync"
	"syscall"
)

// Server is a remote command execution server.
//
// Once a server is started (Serve or ServeConn is called), it is no
// longer safe to modify any configuration, though it is safe to read.
type Server struct {
	Commands    []ServerCommand // The commands this server can exectute
	ServerCodec rpc.ServerCodec // Codec to handle conns, or nil for gob

	once      sync.Once
	rpcServer *rpc.Server
}

// ServerCommand is a command that a server can execute.
//
// Name is the name that the client must send as part of a ClientCommand.
// Command is the actual command-line command to execute. Params is a list of
// valid parameter names.
type ServerCommand struct {
	Name    string
	Command string
	Params  []string
}

// Serve accepts connects on the given listener and executes commands
// that are sent to it.
//
// It is the responsibility of the caller to close the listener.
func (s *Server) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		log.Printf("[INFO] Accepted connection: %s", c.RemoteAddr().String())

		go s.ServeConn(c)
	}
}

func (s *Server) ServeConn(rwc io.ReadWriteCloser) error {
	// Initialize if we haven't already
	s.once.Do(s.init)

	// Defer the connection to the RPC server
	if s.ServerCodec == nil {
		s.rpcServer.ServeConn(rwc)
	} else {
		s.rpcServer.ServeCodec(s.ServerCodec)
	}

	return nil
}

func (s *Server) init() {
	receiver := &serverReceiver{server: s}
	s.rpcServer = rpc.NewServer()
	if err := s.rpcServer.RegisterName("Server", receiver); err != nil {
		// This should never happen because we fully control the
		// receiver interface. So if we ever fail to register, it is a bug.
		panic(err)
	}
}

type serverReceiver struct {
	server *Server
}

func (s *serverReceiver) Call(req *Request, reply *Response) error {
	var command *ServerCommand = nil
	for _, needle := range s.server.Commands {
		if needle.Name == req.Command {
			command = &needle
			break
		}
	}

	if command == nil {
		return fmt.Errorf("Command not supported: %s", req.Command)
	}

	log.Printf("[DEBUG] Executing command: %s", command.Command)
	var stdout, stderr bytes.Buffer
	cliCmd := exec.Command("sh", "-c", command.Command)
	cliCmd.Stdout = &stdout
	cliCmd.Stderr = &stderr
	cliCmd.Start()

	exitCode := 0
	if err := cliCmd.Wait(); err != nil {
		bad := true
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
				bad = false
			}
		}

		if bad {
			return errors.New(err.Error())
		}
	}

	*reply = Response{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}

	log.Printf("[DEBUG] Exit code: %d", exitCode)
	log.Printf("[DEBUG] Stdout: %s", stdout.String())
	log.Printf("[DEBUG] Stderr: %s", stderr.String())
	return nil
}
