package server

import (
	"fmt"
	"io"
	"net"
	"net-cat/client"
	"net-cat/logger"
	"net-cat/models"
	"os"
	"sync"
	"time"
)

// QueueEntry represents a client waiting for a slot to open.
type QueueEntry struct {
	client *client.Client
	admit  chan struct{} // closed when the client is admitted
}

// Server manages the TCP listener, connected clients, and chat history.
type Server struct {
	port            string
	listener        net.Listener
	clients         map[string]*client.Client   // global username→client for cross-room lookups
	allClients      map[*client.Client]struct{} // all connections in any phase (name-prompt, queued, active)
	mu              sync.RWMutex
	rooms           map[string]*Room // room name → Room (history+queue are per-room)
	DefaultRoom     string           // default room name ("general")
	reservedNames   map[string]bool
	quit            chan struct{}
	shutdownOnce    sync.Once
	shutdownDone    chan struct{}
	ShutdownTimeout time.Duration // defaults to 5s; override in tests for faster execution
	Logger          *logger.Logger
	startTime       time.Time

	// IP-based moderation (protected by mu)
	kickedIPs map[string]time.Time // host IP -> cooldown expiry
	bannedIPs map[string]bool      // host IP -> banned for server session

	// Admin persistence
	admins     map[string]bool // known admin usernames, protected by mu
	adminsFile string          // path to admins.json

	// Operator terminal output (defaults to os.Stdout)
	OperatorOutput io.Writer

	// Heartbeat configuration (zero values use defaults: 10s interval, 5s timeout)
	HeartbeatInterval time.Duration // how often to check idle clients (default 10s)
	HeartbeatTimeout  time.Duration // write probe deadline (default 5s)
}

// New creates a server that will listen on the given port.
func New(port string) *Server {
	s := &Server{
		port:        port,
		clients:     make(map[string]*client.Client),
		allClients:  make(map[*client.Client]struct{}),
		rooms:       make(map[string]*Room),
		DefaultRoom: "general",
		reservedNames: map[string]bool{
			"Server": true,
		},
		quit:           make(chan struct{}),
		shutdownDone:   make(chan struct{}),
		kickedIPs:      make(map[string]time.Time),
		bannedIPs:      make(map[string]bool),
		admins:         make(map[string]bool),
		adminsFile:     "admins.json",
		OperatorOutput: os.Stdout,
	}
	s.startTime = time.Now()
	// Ensure the default room always exists
	s.rooms[s.DefaultRoom] = newRoom(s.DefaultRoom)
	return s
}

// Start opens the TCP listener and blocks in the accept loop until shutdown.
func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", ":"+s.port)
	if err != nil {
		return err
	}
	fmt.Printf("Listening on the port :%s\n", s.port)
	s.Logger.Log(models.Message{
		Timestamp: time.Now(),
		Type:      models.MsgServerEvent,
		Content:   "Server started on port " + s.port,
	})
	s.LoadAdmins()
	s.RecoverHistory()
	go s.startMidnightWatcher()
	s.acceptLoop()
	<-s.shutdownDone
	return nil
}

// Shutdown stops new accepts, notifies tracked clients, and closes remaining connections after a grace period.
func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.quit)

		// Stop accepting new connections
		if s.listener != nil {
			s.listener.Close()
		}

		// Send goodbye to ALL tracked connections (active, queued, and name-prompt)
		s.mu.RLock()
		for c := range s.allClients {
			c.Send("Server is shutting down. Goodbye!\n")
		}
		s.mu.RUnlock()

		// Wait up to ShutdownTimeout for clients to disconnect voluntarily
		timeout := s.ShutdownTimeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			s.mu.RLock()
			remaining := len(s.allClients)
			s.mu.RUnlock()
			if remaining == 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		// Force-close any remaining connections
		s.mu.RLock()
		for c := range s.allClients {
			c.Close()
		}
		s.mu.RUnlock()

		// Wait for handler goroutine cleanup (leave logging, untracking)
		time.Sleep(200 * time.Millisecond)

		// Log shutdown synchronously — guaranteed before process exit
		s.Logger.Log(models.Message{
			Timestamp: time.Now(),
			Type:      models.MsgServerEvent,
			Content:   "Server shutting down",
		})
		s.Logger.Close()

		close(s.shutdownDone)
	})
}

// acceptLoop accepts new TCP connections until shutdown closes the listener.
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				continue
			}
		}
		go s.handleConnection(conn)
	}
}

// ---------- queue management ----------

// MaxActiveClients is the maximum number of clients that can be actively chatting per room.
const MaxActiveClients = 10

// GetQueueLength returns the total number of queued clients across all rooms.
func (s *Server) GetQueueLength() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, r := range s.rooms {
		total += len(r.queue)
	}
	return total
}

// RemoveFromQueueByIP removes queued clients with the same IP across every room.
func (s *Server) RemoveFromQueueByIP(ip string) []*client.Client {
	return s.RemoveFromAllRoomQueuesByIP(ip)
}

// IsShuttingDown reports whether the server is in the shutdown process.
func (s *Server) IsShuttingDown() bool {
	select {
	case <-s.quit:
		return true
	default:
		return false
	}
}

// ---------- heartbeat ----------

// A null byte (\x00) write probe is used because it is
// invisible to most terminal emulators (including netcat).
//
// The probe runs in a separate goroutine to avoid calling SetWriteDeadline, which
// would interfere with the client's writeLoop. Instead, a timer detects whether the
// probe completes in time:
//   - If the write returns an error (io.ErrClosedPipe, ECONNRESET, etc.): dead client.
//   - If the write doesn't complete within the timeout: slow/unstable — warn but keep alive.
//   - If the write completes quickly: healthy connection.
//
// For real TCP connections, TCP keepalive (enabled in handleConnection) provides an
// additional layer of dead peer detection at the OS level.
// startHeartbeat probes idle clients so dead connections are dropped without waiting for user traffic.
func (s *Server) startHeartbeat(c *client.Client) {
	interval := s.HeartbeatInterval
	if interval == 0 {
		interval = 10 * time.Second
	}
	timeout := s.HeartbeatTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.IsClosed() {
				return
			}
			// Active sender exemption: client recently sent data, skip probe
			if time.Since(c.GetLastInput()) < interval {
				continue
			}
			// Write probe in a goroutine to avoid blocking and to avoid
			// calling SetWriteDeadline (which would interfere with writeLoop).
			probeResult := make(chan error, 1)
			start := time.Now()
			go func() {
				_, err := c.Conn.Write([]byte{0})
				probeResult <- err
			}()

			select {
			case err := <-probeResult:
				elapsed := time.Since(start)
				if err != nil {
					// Non-timeout write error — connection is truly broken
					c.SetDisconnectReason("drop")
					c.Close()
					return
				}
				// Write succeeded; warn if slow
				if elapsed > timeout/2 {
					c.Send("Connection unstable...\n")
				}
			case <-time.After(timeout):
				// Write probe timed out — client is unresponsive.
				// Per spec 11: "A client that fails to respond within 5 seconds
				// of a health check is treated as disconnected." Disconnect now;
				// the deferred cleanup in handleConnection handles leave broadcast,
				// logging, and queue admission.
				c.SetDisconnectReason("drop")
				c.Close()
				return
			case <-c.Done():
				return
			case <-s.quit:
				return
			}
		case <-c.Done():
			return
		case <-s.quit:
			return
		}
	}
}

// ---------- midnight log rotation ----------

// The logger already handles file switching based on message timestamps, so only the in-memory replay buffer resets here.
// startMidnightWatcher clears in-memory history at each day boundary so new joins only see the current day.
func (s *Server) startMidnightWatcher() {
	for {
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		duration := nextMidnight.Sub(now)

		timer := time.NewTimer(duration)
		select {
		case <-timer.C:
			s.ClearHistory()
		case <-s.quit:
			timer.Stop()
			return
		}
	}
}
