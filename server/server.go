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

// Server manages the TCP listener, connected clients, and chat history.
type Server struct {
	port          string
	listener      net.Listener
	clients       map[string]*client.Client
	mu            sync.RWMutex
	history       []models.Message
	reservedNames map[string]bool
	quit          chan struct{}
	shutdownOnce  sync.Once
	Logger        *logger.Logger
}

// New creates a server that will listen on the given port.
func New(port string) *Server {
	return &Server{
		port:    port,
		clients: make(map[string]*client.Client),
		reservedNames: map[string]bool{
			"Server": true,
		},
		quit: make(chan struct{}),
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
	return nil
}

// Shutdown sends the goodbye message and closes the listener to stop accepting.
func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.quit)
		s.mu.RLock()
		for _, c := range s.clients {
			c.Send("Server is shutting down. Goodbye!\n")
		}
		s.mu.RUnlock()
		// Brief pause so write goroutines can flush the goodbye
		time.Sleep(50 * time.Millisecond)
		// Log shutdown synchronously before process exit
		s.Logger.Log(models.Message{
			Timestamp: time.Now(),
			Type:      models.MsgServerEvent,
			Content:   "Server shutting down",
		})
		s.Logger.Close()
		if s.listener != nil {
			s.listener.Close()
		}
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

// IsShuttingDown reports whether the server is in the shutdown process.
func (s *Server) IsShuttingDown() bool {
	select {
	case <-s.quit:
		return true
	default:
		return false
	}
}
