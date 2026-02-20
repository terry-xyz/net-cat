package server

import (
	"bufio"
	"fmt"
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
	clients         map[string]*client.Client
	allClients      map[*client.Client]struct{} // all connections in any phase (name-prompt, queued, active)
	mu              sync.RWMutex
	history         []models.Message
	reservedNames   map[string]bool
	quit            chan struct{}
	shutdownOnce    sync.Once
	shutdownDone    chan struct{}
	ShutdownTimeout time.Duration // defaults to 5s; override in tests for faster execution
	Logger          *logger.Logger
	queue           []*QueueEntry // protected by mu
}

// New creates a server that will listen on the given port.
func New(port string) *Server {
	return &Server{
		port:       port,
		clients:    make(map[string]*client.Client),
		allClients: make(map[*client.Client]struct{}),
		reservedNames: map[string]bool{
			"Server": true,
		},
		quit:         make(chan struct{}),
		shutdownDone: make(chan struct{}),
	}
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
	s.RecoverHistory()
	s.acceptLoop()
	<-s.shutdownDone
	return nil
}

// Shutdown sends the goodbye message, waits for clients to disconnect, then
// force-closes remaining connections. Idempotent via sync.Once.
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

// ---------- connection tracking ----------

// TrackClient registers a connection in allClients for shutdown notification.
func (s *Server) TrackClient(c *client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allClients[c] = struct{}{}
}

// UntrackClient removes a connection from allClients.
func (s *Server) UntrackClient(c *client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.allClients, c)
}

// ---------- client map ----------

// RegisterClient atomically checks uniqueness and adds the client.
func (s *Server) RegisterClient(c *client.Client, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.clients[name]; exists {
		return false
	}
	if s.reservedNames[name] {
		return false
	}
	now := time.Now()
	c.Username = name
	c.JoinTime = now
	c.LastActivity = now
	s.clients[name] = c
	return true
}

func (s *Server) RemoveClient(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, username)
}

func (s *Server) GetClient(name string) *client.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[name]
}

func (s *Server) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

func (s *Server) GetClientNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.clients))
	for n := range s.clients {
		names = append(names, n)
	}
	return names
}

func (s *Server) IsReservedName(name string) bool {
	return s.reservedNames[name]
}

// RenameClient atomically swaps the key in the client map.
// Returns false if newName is already taken or reserved.
func (s *Server) RenameClient(c *client.Client, oldName, newName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.clients[newName]; exists {
		return false
	}
	if s.reservedNames[newName] {
		return false
	}
	delete(s.clients, oldName)
	c.Username = newName
	s.clients[newName] = c
	return true
}

// ---------- history ----------

func (s *Server) AddHistory(msg models.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, msg)
}

func (s *Server) GetHistory() []models.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Message, len(s.history))
	copy(out, s.history)
	return out
}

// recordEvent adds the message to in-memory history and writes it to the log file.
func (s *Server) recordEvent(msg models.Message) {
	s.AddHistory(msg)
	s.Logger.Log(msg)
}

// ---------- broadcast ----------

// Broadcast sends msg to every connected client except the one named exclude.
func (s *Server) Broadcast(msg string, exclude string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for name, c := range s.clients {
		if name != exclude {
			c.Send(msg)
		}
	}
}

// BroadcastAll sends msg to every connected client.
func (s *Server) BroadcastAll(msg string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		c.Send(msg)
	}
}

// RecoverHistory loads today's log file and reconstructs the in-memory history.
// Only called on startup so that clients connecting after a restart see prior events.
// Server events are excluded (not user-visible). Corrupt lines are skipped with warnings.
func (s *Server) RecoverHistory() {
	if s.Logger == nil {
		return
	}

	date := logger.FormatDate(time.Now())
	path := s.Logger.FilePath(date)
	if path == "" {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		fmt.Fprintf(os.Stderr, "Warning: could not open log file for recovery: %v\n", err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	corrupt := 0
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		msg, err := models.ParseLogLine(line)
		if err != nil {
			corrupt++
			fmt.Fprintf(os.Stderr, "Warning: skipping corrupt log line: %v\n", err)
			continue
		}
		if msg.Type == models.MsgServerEvent {
			continue
		}
		s.AddHistory(msg)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error reading log file: %v\n", err)
	}

	if corrupt > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %d corrupt line(s) skipped during history recovery\n", corrupt)
	}
}

// ---------- queue management ----------

// MaxActiveClients is the maximum number of clients that can be actively chatting.
const MaxActiveClients = 10

// removeFromQueue removes the given entry from the queue and sends position updates.
func (s *Server) removeFromQueue(entry *QueueEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.queue {
		if e == entry {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			break
		}
	}
	// Send position updates to remaining queue members
	for i, e := range s.queue {
		e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
	}
}

// admitFromQueue admits the first valid queued client, if any.
// Called after a registered client departs to fill the opened slot.
// No-op during shutdown to prevent admitting clients into a closing server.
func (s *Server) admitFromQueue() {
	if s.IsShuttingDown() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.queue) > 0 {
		entry := s.queue[0]
		s.queue = s.queue[1:]
		if entry.client.IsClosed() {
			continue
		}
		close(entry.admit)
		// Send position updates to remaining queue members
		for i, e := range s.queue {
			e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
		}
		return
	}
}

// GetQueueLength returns the current number of queued clients.
func (s *Server) GetQueueLength() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.queue)
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
