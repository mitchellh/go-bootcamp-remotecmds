package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	var addr string
	var modeServer bool

	flag.StringVar(&addr, "addr", "127.0.0.1:0", "Address to connect or listen")
	flag.BoolVar(&modeServer, "server", false, "Run in server mode")
	flag.Parse()
	args := flag.Args()

	if modeServer {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Server mode requires config path as arg\n")
			return 1
		}

		commands, err := parseCommands(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 1
		}

		for _, command := range commands {
			log.Printf("[INFO] Registered command: %s", command.Name)
		}

		l, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 1
		}
		log.Printf("[INFO] Listening on: %s", l.Addr().String())

		server := &Server{Commands: commands}
		server.Serve(l)
		return 0
	}

	// Client mode
	c, err := rpc.Dial("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 1
	}
	defer c.Close()

	req := Request{Command: args[0]}
	var response Response
	if err := c.Call("Server.Call", &req, &response); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 1
	}

	fmt.Fprint(os.Stdout, response.Stdout)
	fmt.Fprint(os.Stderr, response.Stderr)
	return response.ExitCode
}

func parseCommands(path string) ([]ServerCommand, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var result []ServerCommand
	dec := json.NewDecoder(f)
	if err := dec.Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}
