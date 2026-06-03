// Package main provides the rna binary, which runs either as a diagnostic HTTP
// server (the default) or as a load-generating client, selected at startup by
// the RNA_MODE environment variable.
//
// Configuration via environment variables:
//
//	RNA_MODE   "server" (default) or "client"
//	RNA_PORT   listening port (default: 8080) — the server port, or the
//	           client's control-API port, depending on the mode
package main

import (
	"log"
	"os"
)

func main() {
	port := os.Getenv("RNA_PORT")
	if port == "" {
		port = "8080"
	}

	var err error
	switch mode := os.Getenv("RNA_MODE"); mode {
	case "", "server":
		err = runServer(port)
	case "client":
		err = runClient(port)
	default:
		log.Fatalf("unknown RNA_MODE %q (want \"server\" or \"client\")", mode)
	}
	if err != nil {
		log.Fatal(err)
	}
}
