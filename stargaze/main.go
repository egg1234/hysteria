package main

import (
	"fmt"
	"os"
)

const helpText = `Usage:
  %s [command] [flags]

Commands:
  server - run the server
  client - run the client

Use "[command] -h" for available flags for a command.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Printf(helpText, os.Args[0])
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		server()
	case "client":
		client()
	default:
		fmt.Printf(helpText, os.Args[0])
		os.Exit(1)
	}
}
