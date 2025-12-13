package main

import (
	"os"

	"csftp/pkg/client"
	"csftp/pkg/server"
)

func main() {
	if len(os.Args) < 2 {
		println("usage: csftp [server|client]")
		return
	}

	switch os.Args[1] {
	case "server":
		server.StartServer()
	case "client":
		client.StartClient()
	default:
		println("unknown command")
	}
}
