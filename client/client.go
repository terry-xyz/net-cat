package client

import (
	"bufio"
	"io"
	"net"
	"sync"
	"time"
)

const (
	maxLineLength = 1048576 // 1 MB scanner buffer limit
	msgChanSize   = 256
)

// Client represents a single connected user with its own write goroutine.
type Client struct {
	Conn             net.Conn
	Username         string
	JoinTime         time.Time
	LastActivity     time.Time
	Muted            bool
	Admin            bool
	IP               string
	DisconnectReason string // set by moderation: "kicked", "banned"; empty for normal

	msgChan   chan string
	done      chan struct{}
	closeOnce sync.Once
	scanner   *bufio.Scanner
}

// NewClient wraps a connection and starts the background write goroutine.
func NewClient(conn net.Conn) *Client {
	c := &Client{
		Conn:    conn,
		IP:      conn.RemoteAddr().String(),
		msgChan: make(chan string, msgChanSize),
		done:    make(chan struct{}),
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), maxLineLength)
	c.scanner = scanner
	go c.writeLoop()
	return c
}

// Send enqueues a message for delivery. Non-blocking: drops if channel is full.
func (c *Client) Send(msg string) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.msgChan <- msg:
	case <-c.done:
	default:
		// channel full – drop for this client to protect others
	}
}

// ReadLine blocks until a full line is available. Strips \r\n → content only.
func (c *Client) ReadLine() (string, error) {
	if c.scanner.Scan() {
		return c.scanner.Text(), nil
	}
	if err := c.scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// Close tears down the connection and stops the write goroutine. Safe to call multiple times.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.Conn.Close()
	})
}

// IsClosed reports whether Close has been called.
func (c *Client) IsClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Done returns a channel that is closed when the client is shutting down.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

func (c *Client) writeLoop() {
	for {
		select {
		case msg := <-c.msgChan:
			if _, err := c.Conn.Write([]byte(msg)); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}
