package main

import (
	"fmt"
	"github.com/terry-xyz/net-cat/logger"
	"github.com/terry-xyz/net-cat/server"
	"os"
	"os/signal"
)

// main wires together logging, signal handling, operator input, and the TCP server process.
func main() {
	port := "8989"
	if len(os.Args) > 2 {
		fmt.Println("[USAGE]: ./TCPChat $port")
		os.Exit(1)
	}
	if len(os.Args) == 2 {
		port = os.Args[1]
	}
	if !isValidPort(port) {
		fmt.Println("[USAGE]: ./TCPChat $port")
		os.Exit(1)
	}

	srv := server.New(port)

	l, err := logger.New("logs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logger: %v\n", err)
	}
	srv.Logger = l

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		srv.Shutdown()
		// Drain subsequent signals so the default handler does not terminate
		// the process before shutdown completes
		for range sigChan {
		}
	}()

	// Start operator terminal (reads commands from stdin)
	go srv.StartOperator(os.Stdin)

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// isValidPort validates a port string using byte-range checks (no strconv).
func isValidPort(s string) bool {
	if len(s) == 0 {
		return false
	}
	port := 0
	for _, b := range []byte(s) {
		if b < '0' || b > '9' {
			return false
		}
		port = port*10 + int(b-'0')
		if port > 65535 {
			return false
		}
	}
	return port >= 1
}
