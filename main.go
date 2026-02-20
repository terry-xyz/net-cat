package main

import (
	"fmt"
	"net-cat/server"
	"os"
	"os/signal"
)

func main() {
	port := "8989"
	if len(os.Args) > 2 {
		fmt.Println("[USAGE]: ./TCPChat $port")
		os.Exit(1)
	}
	if len(os.Args) == 2 {
		port = os.Args[1]
	}
	if !IsValidPort(port) {
		fmt.Println("[USAGE]: ./TCPChat $port")
		os.Exit(1)
	}

	srv := server.New(port)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		srv.Shutdown()
	}()

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// IsValidPort validates a port string using byte-range checks (no strconv).
func IsValidPort(s string) bool {
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
