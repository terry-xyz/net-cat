package client

import (
	"bufio"
	"io"
	"net"
	"sync"
	"time"
)

const (
	maxLineLength     = 1048576 // 1 MB scanner buffer limit
	msgChanSize       = 4096   // large enough for echo messages of max-length lines
	maxInteractiveBuf = 4096   // max partial input tracked for redraw
)

// Write-message types for the writeLoop channel.
const (
	wmMessage   = iota // broadcast/notification: in echoMode clear+redraw; else raw write
	wmPrompt          // set prompt, clear inputBuf, write data
	wmEcho            // append char to inputBuf, write char
	wmBackspace       // remove last from inputBuf, write \b \b
	wmNewline         // clear inputBuf/prompt, write \r\n
)

// writeMsg is the internal message type for the write channel.
type writeMsg struct {
	data    string
	msgType int
}

// Client represents a single connected user with its own write goroutine.
type Client struct {
	Conn     net.Conn
	Username string
	JoinTime time.Time
	IP       string

	// lastActivity, muted, admin are accessed from multiple goroutines (handler,
	// operator, heartbeat) — protected by mu alongside lastInput et al.
	lastActivity time.Time
	muted        bool
	admin        bool

	msgChan   chan writeMsg
	done      chan struct{}
	closeOnce sync.Once
	scanner   *bufio.Scanner

	// mu protects fields accessed concurrently by multiple goroutines (handler,
	// operator, heartbeat): lastInput, disconnectReason, echoMode, lastActivity,
	// muted, and admin.
	mu               sync.Mutex
	lastInput        time.Time
	disconnectReason string
	echoMode         bool // true after onboarding; enables input continuity

	// The following fields are ONLY accessed by the writeLoop goroutine:
	inputBuf []byte // partial input typed so far (server-side tracking)
	prompt   string // current prompt string for redraw

	// skipLF is ONLY accessed by the handler goroutine (ReadLineInteractive):
	skipLF bool // skip next \n after \r (for \r\n handling)
}

// NewClient wraps a connection and starts the background write goroutine.
func NewClient(conn net.Conn) *Client {
	c := &Client{
		Conn:    conn,
		IP:      conn.RemoteAddr().String(),
		msgChan: make(chan writeMsg, msgChanSize),
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
	c.enqueue(writeMsg{data: msg, msgType: wmMessage})
}

// SendPrompt enqueues a prompt message and tells the writeLoop to update the
// tracked prompt (clears inputBuf). Used after the client submits a line.
func (c *Client) SendPrompt(prompt string) {
	c.enqueue(writeMsg{data: prompt, msgType: wmPrompt})
}

func (c *Client) enqueue(m writeMsg) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.msgChan <- m:
	case <-c.done:
	default:
		// channel full – drop for this client to protect others
	}
}

// ReadLine blocks until a full line is available. Strips \r\n → content only.
// Used during onboarding (before echoMode is enabled).
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

// ResetScanner creates a fresh scanner for the connection.
// Used after queue admission where the scanner may be in an error state
// due to a read deadline used to cancel the monitoring goroutine.
func (c *Client) ResetScanner() {
	scanner := bufio.NewScanner(c.Conn)
	scanner.Buffer(make([]byte, 4096), maxLineLength)
	c.scanner = scanner
}

// SetLastInput records the time the client last sent any data (for heartbeat tracking).
func (c *Client) SetLastInput(t time.Time) {
	c.mu.Lock()
	c.lastInput = t
	c.mu.Unlock()
}

// GetLastInput returns the time the client last sent any data.
func (c *Client) GetLastInput() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastInput
}

// SetDisconnectReason atomically sets the disconnect reason if not already set.
// Used to distinguish voluntary, dropped, kicked, and banned disconnects.
func (c *Client) SetDisconnectReason(reason string) {
	c.mu.Lock()
	if c.disconnectReason == "" {
		c.disconnectReason = reason
	}
	c.mu.Unlock()
}

// GetDisconnectReason returns the current disconnect reason.
func (c *Client) GetDisconnectReason() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disconnectReason
}

// ForceDisconnectReason sets the disconnect reason unconditionally (for moderation).
func (c *Client) ForceDisconnectReason(reason string) {
	c.mu.Lock()
	c.disconnectReason = reason
	c.mu.Unlock()
}

// SetLastActivity records the time the client last sent a chat message.
func (c *Client) SetLastActivity(t time.Time) {
	c.mu.Lock()
	c.lastActivity = t
	c.mu.Unlock()
}

// GetLastActivity returns the time the client last sent a chat message.
func (c *Client) GetLastActivity() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastActivity
}

// SetMuted sets whether the client is muted.
func (c *Client) SetMuted(v bool) {
	c.mu.Lock()
	c.muted = v
	c.mu.Unlock()
}

// IsMuted reports whether the client is muted.
func (c *Client) IsMuted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.muted
}

// SetAdmin sets whether the client has admin privileges.
func (c *Client) SetAdmin(v bool) {
	c.mu.Lock()
	c.admin = v
	c.mu.Unlock()
}

// IsAdmin reports whether the client has admin privileges.
func (c *Client) IsAdmin() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.admin
}

// ---------- writeLoop: single goroutine responsible for all Conn writes ----------

// writeLoop drains the message channel and writes to the connection.
// It is the sole writer to Conn (except heartbeat null-byte probes).
// In echoMode it preserves the client's partial input across incoming messages.
func (c *Client) writeLoop() {
	for {
		select {
		case msg := <-c.msgChan:
			c.mu.Lock()
			echo := c.echoMode
			c.mu.Unlock()

			if !echo {
				// Pre-onboarding: raw write
				if _, err := c.Conn.Write([]byte(msg.data)); err != nil {
					return
				}
				continue
			}

			// echoMode active: handle by message type
			switch msg.msgType {
			case wmMessage:
				c.writeWithContinuity(msg.data)
			case wmPrompt:
				c.prompt = msg.data
				c.inputBuf = c.inputBuf[:0]
				c.Conn.Write([]byte(msg.data))
			case wmEcho:
				if len(msg.data) > 0 && len(c.inputBuf) < maxInteractiveBuf {
					c.inputBuf = append(c.inputBuf, msg.data[0])
				}
				c.Conn.Write([]byte(msg.data))
			case wmBackspace:
				if len(c.inputBuf) > 0 {
					c.inputBuf = c.inputBuf[:len(c.inputBuf)-1]
				}
				c.Conn.Write([]byte("\b \b"))
			case wmNewline:
				c.inputBuf = c.inputBuf[:0]
				c.prompt = ""
				c.Conn.Write([]byte("\r\n"))
			}
		case <-c.done:
			return
		}
	}
}

// writeWithContinuity clears the current prompt+input line, writes the message,
// then redraws the prompt and partial input. All output is batched into a single
// Conn.Write to avoid partial-write blocking on synchronous pipes. Only called
// from writeLoop.
func (c *Client) writeWithContinuity(msg string) {
	hasPrompt := len(c.prompt) > 0
	hasInput := len(c.inputBuf) > 0

	// Pre-calculate total size for a single allocation
	size := len(msg)
	if hasPrompt || hasInput {
		size += 4 // "\r\033[K"
	}
	if hasPrompt {
		size += len(c.prompt)
	}
	if hasInput {
		size += len(c.inputBuf)
	}

	buf := make([]byte, 0, size)
	if hasPrompt || hasInput {
		buf = append(buf, '\r')
		buf = append(buf, "\033[K"...)
	}
	buf = append(buf, msg...)
	if hasPrompt {
		buf = append(buf, c.prompt...)
	}
	if hasInput {
		buf = append(buf, c.inputBuf...)
	}
	c.Conn.Write(buf)
}

// ---------- echo mode & interactive reading ----------

// SetEchoMode enables character-at-a-time input handling with input continuity.
func (c *Client) SetEchoMode(enabled bool) {
	c.mu.Lock()
	c.echoMode = enabled
	c.mu.Unlock()
}

// ReadLineInteractive reads input byte-by-byte, sending echo/backspace/newline
// commands through the writeLoop channel. This avoids direct Conn writes from
// the handler goroutine, preventing deadlocks with synchronous pipes.
// Returns the complete line (without newline) when Enter is pressed.
func (c *Client) ReadLineInteractive() (string, error) {
	buf := make([]byte, 1)
	var line []byte
	for {
		n, err := c.Conn.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}
		b := buf[0]

		// Handle \r\n: after \r we skip the next \n
		if c.skipLF && b == '\n' {
			c.skipLF = false
			continue
		}
		c.skipLF = false

		switch {
		case b == '\r':
			c.skipLF = true
			c.enqueue(writeMsg{msgType: wmNewline})
			return string(line), nil

		case b == '\n':
			c.enqueue(writeMsg{msgType: wmNewline})
			return string(line), nil

		case b == 0x7F || b == 0x08: // Backspace / Delete
			if len(line) > 0 {
				line = line[:len(line)-1]
				c.enqueue(writeMsg{msgType: wmBackspace})
			}

		case b == 0x00: // Null byte from heartbeat probe — ignore
			continue

		case b >= 0x20 && b <= 0x7E: // Printable ASCII
			line = append(line, b)
			c.enqueue(writeMsg{data: string([]byte{b}), msgType: wmEcho})

		default:
			// Non-printable control character — ignore
			continue
		}
	}
}
