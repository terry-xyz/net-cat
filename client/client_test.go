package client

import (
	"net"
	"strings"
	"testing"
	"time"
)

// TestSendDeliversToConnection verifies the scenario described by its name.
func TestSendDeliversToConnection(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	c := NewClient(server)
	defer c.Close()

	c.Send("hello\n")
	buf := make([]byte, 64)
	client.SetReadDeadline(time.Now().Add(time.Second))
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(buf[:n]) != "hello\n" {
		t.Errorf("got %q, want %q", string(buf[:n]), "hello\n")
	}
}

// TestWriteLoopExitsOnClose verifies the scenario described by its name.
func TestWriteLoopExitsOnClose(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	c := NewClient(server)

	c.Close()
	// After close, Send should not panic
	c.Send("should not panic")
	// Give write goroutine time to exit
	time.Sleep(50 * time.Millisecond)
	if !c.IsClosed() {
		t.Error("expected client to be closed")
	}
}

// TestReadLineStripsCarriageReturn verifies the scenario described by its name.
func TestReadLineStripsCarriageReturn(t *testing.T) {
	server, clientConn := net.Pipe()
	defer server.Close()
	c := NewClient(server)
	defer c.Close()

	// Simulate Windows netcat sending \r\n
	go func() {
		clientConn.Write([]byte("hello\r\n"))
	}()

	line, err := c.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine error: %v", err)
	}
	if line != "hello" {
		t.Errorf("got %q, want %q (\\r should be stripped)", line, "hello")
	}
}

// TestReadLineExitsOnConnectionClose verifies the scenario described by its name.
func TestReadLineExitsOnConnectionClose(t *testing.T) {
	server, clientConn := net.Pipe()
	c := NewClient(server)
	defer c.Close()

	go func() {
		time.Sleep(20 * time.Millisecond)
		clientConn.Close()
	}()

	_, err := c.ReadLine()
	if err == nil {
		t.Error("expected error when connection closed")
	}
}

// TestBufferedChannelDoesNotBlockBroadcaster verifies the scenario described by its name.
func TestBufferedChannelDoesNotBlockBroadcaster(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	c := NewClient(server)
	defer c.Close()

	// Don't read from the client side — fill the channel
	// This should complete without blocking indefinitely
	done := make(chan bool, 1)
	go func() {
		for i := 0; i < msgChanSize+10; i++ {
			c.Send("msg\n")
		}
		done <- true
	}()

	select {
	case <-done:
		// good — Send didn't block
	case <-time.After(2 * time.Second):
		t.Fatal("Send blocked when channel was full")
	}
}

// TestReadLineLongLineWithoutNewline verifies the scenario described by its name.
func TestReadLineLongLineWithoutNewline(t *testing.T) {
	server, clientConn := net.Pipe()
	c := NewClient(server)
	defer c.Close()

	// Send a line that exceeds maxLineLength without a newline
	bigData := strings.Repeat("A", maxLineLength+100)
	go func() {
		clientConn.Write([]byte(bigData))
		clientConn.Close()
	}()

	_, err := c.ReadLine()
	if err == nil {
		t.Error("expected error for extremely long line without newline")
	}
}

// TestSendAfterCloseDoesNotPanic verifies the scenario described by its name.
func TestSendAfterCloseDoesNotPanic(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	c := NewClient(server)
	c.Close()

	// Multiple sends after close should not panic
	for i := 0; i < 100; i++ {
		c.Send("test\n")
	}
}

// TestDoubleCloseDoesNotPanic verifies the scenario described by its name.
func TestDoubleCloseDoesNotPanic(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	c := NewClient(server)
	c.Close()
	c.Close() // second close should not panic
}
