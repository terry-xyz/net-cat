package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net-cat/client"
	"net-cat/cmd"
	"net-cat/logger"
	"net-cat/models"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// errCapacityFull is returned by RegisterClient when the server has reached
// the maximum number of active clients. The caller should re-queue the client.
var errCapacityFull = errors.New("server at capacity")

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
	return &Server{
		port:       port,
		clients:    make(map[string]*client.Client),
		allClients: make(map[*client.Client]struct{}),
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

// RegisterClient atomically checks capacity, uniqueness, and adds the client.
// Returns nil on success, errCapacityFull if at max active clients, or a
// generic error if the name is taken or reserved.
func (s *Server) RegisterClient(c *client.Client, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.clients) >= MaxActiveClients {
		return errCapacityFull
	}
	if _, exists := s.clients[name]; exists {
		return errors.New("name taken")
	}
	if s.reservedNames[name] {
		return errors.New("name reserved")
	}
	now := time.Now()
	c.Username = name
	c.JoinTime = now
	c.SetLastActivity(now)
	s.clients[name] = c
	return nil
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

// GetClientsByIP returns all registered clients whose IP matches the given host.
// Pass exclude to skip a specific username (e.g. the command issuer).
func (s *Server) GetClientsByIP(host string, exclude string) []*client.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*client.Client
	for _, c := range s.clients {
		if extractHost(c.IP) == host && c.Username != exclude {
			result = append(result, c)
		}
	}
	return result
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

// ClearHistory removes all in-memory history entries.
// Called at midnight to reset history for the new calendar day.
func (s *Server) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = s.history[:0]
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

// RemoveFromQueueByIP removes all queue entries whose IP matches the given
// address and returns their clients. Position updates are sent to remaining
// queue members. Used by the operator to kick/ban queued users by IP.
func (s *Server) RemoveFromQueueByIP(ip string) []*client.Client {
	host := extractHost(ip)
	s.mu.Lock()
	defer s.mu.Unlock()

	var removed []*client.Client
	var remaining []*QueueEntry
	for _, e := range s.queue {
		if extractHost(e.client.IP) == host {
			removed = append(removed, e.client)
		} else {
			remaining = append(remaining, e)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	s.queue = remaining

	// Send position updates to remaining queue members
	for i, e := range s.queue {
		e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
	}
	return removed
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

// ---------- IP-based moderation ----------

// extractHost extracts the host part from a "host:port" address string.
func extractHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr // bare IP or pipe address
	}
	return host
}

// AddKickCooldown blocks an IP from reconnecting for 5 minutes.
func (s *Server) AddKickCooldown(ip string) {
	host := extractHost(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kickedIPs[host] = time.Now().Add(5 * time.Minute)
}

// AddBanIP blocks an IP for the remainder of the server session.
func (s *Server) AddBanIP(ip string) {
	host := extractHost(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bannedIPs[host] = true
}

// IsIPBlocked checks if an IP is blocked by kick cooldown or ban.
// Returns (blocked, rejection message). Cleans up expired kick cooldowns.
func (s *Server) IsIPBlocked(ip string) (bool, string) {
	host := extractHost(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bannedIPs[host] {
		return true, "You are banned from this server.\n"
	}
	if expiry, ok := s.kickedIPs[host]; ok {
		if time.Now().Before(expiry) {
			return true, "You are temporarily blocked. Try again later.\n"
		}
		delete(s.kickedIPs, host) // expired
	}
	return false, ""
}

// ---------- admin persistence ----------

// LoadAdmins reads admins.json from disk. Missing or corrupt file is handled
// gracefully: the server starts with no saved admins and a console warning.
func (s *Server) LoadAdmins() {
	path := s.adminsFile
	if path == "" {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: could not open admins.json: %v\n", err)
		}
		return
	}
	defer f.Close()

	var names []string
	if err := json.NewDecoder(f).Decode(&names); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: corrupt admins.json, starting with no saved admins: %v\n", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, name := range names {
		s.admins[name] = true
	}
}

// SaveAdmins writes the current admin list to admins.json atomically.
// Writes to a temp file then renames for crash safety.
func (s *Server) SaveAdmins() {
	path := s.adminsFile
	if path == "" {
		return
	}

	s.mu.RLock()
	names := make([]string, 0, len(s.admins))
	for name := range s.admins {
		names = append(names, name)
	}
	s.mu.RUnlock()

	// Sort for deterministic output (simple insertion sort, no sort package)
	for i := 1; i < len(names); i++ {
		key := names[i]
		j := i - 1
		for j >= 0 && names[j] > key {
			names[j+1] = names[j]
			j--
		}
		names[j+1] = key
	}

	data, err := json.MarshalIndent(names, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not marshal admins.json: %v\n", err)
		return
	}

	dir := filepath.Dir(path)
	tmpFile := filepath.Join(dir, ".admins.json.tmp")
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write admins.json: %v\n", err)
		return
	}
	if err := os.Rename(tmpFile, path); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save admins.json: %v\n", err)
	}
}

// IsKnownAdmin checks if a username is in the persisted admin list.
func (s *Server) IsKnownAdmin(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.admins[name]
}

// AddAdmin adds a username to the persisted admin list and saves.
func (s *Server) AddAdmin(name string) {
	s.mu.Lock()
	s.admins[name] = true
	s.mu.Unlock()
	s.SaveAdmins()
}

// RemoveAdmin removes a username from the persisted admin list and saves.
func (s *Server) RemoveAdmin(name string) {
	s.mu.Lock()
	delete(s.admins, name)
	s.mu.Unlock()
	s.SaveAdmins()
}

// RenameAdmin updates the persisted admin list when an admin changes their name.
func (s *Server) RenameAdmin(oldName, newName string) {
	s.mu.Lock()
	if s.admins[oldName] {
		delete(s.admins, oldName)
		s.admins[newName] = true
	}
	s.mu.Unlock()
	s.SaveAdmins()
}

// ---------- heartbeat ----------

// startHeartbeat runs a per-client goroutine that periodically probes the connection
// to detect dead/ghost clients. A null byte (\x00) write probe is used because it is
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

// startMidnightWatcher runs a goroutine that detects day boundaries and resets
// in-memory history at midnight. The logger already handles file switching based
// on message timestamps (via ensureFile), so this only needs to clear the history
// so that clients joining after midnight see only the new day's events.
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

// ---------- operator terminal ----------

// StartOperator reads commands from the given reader (typically os.Stdin)
// and dispatches them with full operator authority. Blocks until the reader
// is exhausted or the server shuts down.
func (s *Server) StartOperator(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1048576)
	for scanner.Scan() {
		if s.IsShuttingDown() {
			return
		}
		line := scanner.Text()
		s.OperatorDispatch(line)
	}
}

// OperatorDispatch parses and executes a single operator terminal input line.
func (s *Server) OperatorDispatch(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	cmdName, args, isCmd := cmd.ParseCommand(input)
	if !isCmd {
		s.operatorSend("Commands must start with /. Use /help to see available commands.\n")
		return
	}

	def, exists := cmd.Commands[cmdName]
	if !exists {
		s.operatorSend("Unknown command: /" + cmdName + ". Use /help to see available commands.\n")
		return
	}

	// Operator has full privilege, but some commands are inapplicable
	switch cmdName {
	case "quit":
		s.operatorSend("The /quit command is not applicable to the server operator.\n")
	case "name":
		s.operatorSend("The /name command is not applicable to the server operator.\n")
	case "whisper":
		s.operatorSend("The /whisper command is not applicable to the server operator.\n")
	case "list":
		s.operatorCmdList()
	case "help":
		s.operatorCmdHelp()
	case "kick":
		s.operatorCmdKick(args)
	case "ban":
		s.operatorCmdBan(args)
	case "mute":
		s.operatorCmdMute(args)
	case "unmute":
		s.operatorCmdUnmute(args)
	case "announce":
		s.operatorCmdAnnounce(args)
	case "promote":
		s.operatorCmdPromote(args)
	case "demote":
		s.operatorCmdDemote(args)
	default:
		_ = def
		s.operatorSend("Unknown command: /" + cmdName + ".\n")
	}
}

// operatorSend writes a message to the operator's output (typically stdout).
func (s *Server) operatorSend(msg string) {
	if s.OperatorOutput != nil {
		fmt.Fprint(s.OperatorOutput, msg)
	}
}

// ---------- operator command implementations ----------

func (s *Server) operatorCmdList() {
	s.mu.RLock()
	type entry struct {
		name string
		idle time.Duration
	}
	entries := make([]entry, 0, len(s.clients))
	for n, cl := range s.clients {
		entries = append(entries, entry{name: n, idle: time.Since(cl.GetLastActivity()).Truncate(time.Second)})
	}

	// Capture queued user IPs while holding the lock
	type queueInfo struct {
		pos int
		ip  string
	}
	queueEntries := make([]queueInfo, 0, len(s.queue))
	for i, e := range s.queue {
		queueEntries = append(queueEntries, queueInfo{pos: i + 1, ip: extractHost(e.client.IP)})
	}
	s.mu.RUnlock()

	for i := 1; i < len(entries); i++ {
		key := entries[i]
		j := i - 1
		for j >= 0 && entries[j].name > key.name {
			entries[j+1] = entries[j]
			j--
		}
		entries[j+1] = key
	}

	s.operatorSend("Connected clients:\n")
	for _, e := range entries {
		s.operatorSend(fmt.Sprintf("  %s (idle: %s)\n", e.name, e.idle.String()))
	}

	if len(queueEntries) > 0 {
		s.operatorSend("Queued clients:\n")
		for _, q := range queueEntries {
			s.operatorSend(fmt.Sprintf("  #%d %s\n", q.pos, q.ip))
		}
	}
}

func (s *Server) operatorCmdHelp() {
	s.operatorSend("Available commands:\n")
	for _, name := range cmd.CommandOrder {
		def := cmd.Commands[name]
		s.operatorSend(fmt.Sprintf("  %-30s %s\n", def.Usage, def.Description))
	}
}

func (s *Server) operatorCmdKick(args string) {
	if args == "" {
		s.operatorSend("Missing target. Usage: /kick <name>\n")
		return
	}
	target := s.GetClient(args)
	if target == nil {
		// Fallback: treat args as IP and search queued users (operator can see IPs via /list)
		removed := s.RemoveFromQueueByIP(args)
		if len(removed) == 0 {
			s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
			return
		}
		for _, c := range removed {
			c.Send("You have been kicked by Server.\n")
			c.Close()
		}
		s.AddKickCooldown(args)
		s.operatorSend(fmt.Sprintf("Queued user(s) from IP %s have been kicked.\n", extractHost(args)))
		return
	}

	targetIP := target.IP
	target.ForceDisconnectReason("kicked")
	s.RemoveClient(args)

	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "kicked",
		Type:      models.MsgModeration,
		Extra:     "Server",
	}
	s.recordEvent(modMsg)
	s.Broadcast(models.FormatModeration(args, "kicked", "Server")+"\n", "")
	target.Send("You have been kicked by Server.\n")
	target.Close()
	s.AddKickCooldown(targetIP)
	s.admitFromQueue()
	s.operatorSend(args + " has been kicked.\n")
}

func (s *Server) operatorCmdBan(args string) {
	if args == "" {
		s.operatorSend("Missing target. Usage: /ban <name>\n")
		return
	}
	target := s.GetClient(args)
	if target == nil {
		// Fallback: treat args as IP and search queued users (operator can see IPs via /list)
		removed := s.RemoveFromQueueByIP(args)
		if len(removed) == 0 {
			s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
			return
		}
		for _, c := range removed {
			c.Send("You have been banned by Server.\n")
			c.Close()
		}
		s.AddBanIP(args)
		s.operatorSend(fmt.Sprintf("Queued user(s) from IP %s have been banned.\n", extractHost(args)))
		return
	}

	targetIP := target.IP
	bannedHost := extractHost(targetIP)
	target.ForceDisconnectReason("banned")
	s.RemoveClient(args)

	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "banned",
		Type:      models.MsgModeration,
		Extra:     "Server",
	}
	s.recordEvent(modMsg)
	s.Broadcast(models.FormatModeration(args, "banned", "Server")+"\n", "")
	target.Send("You have been banned by Server.\n")
	target.Close()
	s.AddBanIP(targetIP)

	// Disconnect all other active clients sharing the banned IP (NAT scenario).
	// Operator is on terminal, not a TCP client, so exclude nobody ("").
	slotsOpened := 1
	collateral := s.GetClientsByIP(bannedHost, "")
	for _, cc := range collateral {
		cc.ForceDisconnectReason("banned")
		ccName := cc.Username
		s.RemoveClient(ccName)
		collateralMsg := models.Message{
			Timestamp: time.Now(),
			Sender:    ccName,
			Content:   "banned",
			Type:      models.MsgModeration,
			Extra:     "Server",
		}
		s.recordEvent(collateralMsg)
		s.Broadcast(models.FormatModeration(ccName, "banned", "Server")+"\n", "")
		cc.Send("You have been banned by Server.\n")
		cc.Close()
		slotsOpened++
	}

	// Remove queued users from the banned IP
	queuedRemoved := s.RemoveFromQueueByIP(targetIP)
	for _, qc := range queuedRemoved {
		qc.Send("You have been banned by Server.\n")
		qc.Close()
	}

	for i := 0; i < slotsOpened; i++ {
		s.admitFromQueue()
	}
	s.operatorSend(args + " has been banned.\n")
}

func (s *Server) operatorCmdMute(args string) {
	if args == "" {
		s.operatorSend("Missing target. Usage: /mute <name>\n")
		return
	}
	target := s.GetClient(args)
	if target == nil {
		s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
		return
	}
	if target.IsMuted() {
		s.operatorSend(args + " is already muted.\n")
		return
	}

	target.SetMuted(true)
	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "muted",
		Type:      models.MsgModeration,
		Extra:     "Server",
	}
	s.recordEvent(modMsg)
	s.BroadcastAll(models.FormatModeration(args, "muted", "Server") + "\n")
	s.operatorSend(args + " has been muted.\n")
}

func (s *Server) operatorCmdUnmute(args string) {
	if args == "" {
		s.operatorSend("Missing target. Usage: /unmute <name>\n")
		return
	}
	target := s.GetClient(args)
	if target == nil {
		s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
		return
	}
	if !target.IsMuted() {
		s.operatorSend(args + " is not muted.\n")
		return
	}

	target.SetMuted(false)
	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "unmuted",
		Type:      models.MsgModeration,
		Extra:     "Server",
	}
	s.recordEvent(modMsg)
	s.BroadcastAll(models.FormatModeration(args, "unmuted", "Server") + "\n")
	s.operatorSend(args + " has been unmuted.\n")
}

func (s *Server) operatorCmdAnnounce(args string) {
	if len(strings.TrimSpace(args)) == 0 {
		s.operatorSend("Usage: /announce <message>\n")
		return
	}
	announceMsg := models.Message{
		Timestamp: time.Now(),
		Content:   args,
		Type:      models.MsgAnnouncement,
		Extra:     "Server",
	}
	s.recordEvent(announceMsg)
	s.BroadcastAll(models.FormatAnnouncement(args) + "\n")
	s.operatorSend("Announcement sent.\n")
}

func (s *Server) operatorCmdPromote(args string) {
	if args == "" {
		s.operatorSend("Missing target. Usage: /promote <name>\n")
		return
	}
	target := s.GetClient(args)
	if target == nil {
		s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
		return
	}
	if target.IsAdmin() {
		s.operatorSend(args + " is already an admin.\n")
		return
	}
	target.SetAdmin(true)
	s.AddAdmin(args)
	target.Send("You have been promoted to admin.\n")

	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "promoted",
		Type:      models.MsgModeration,
		Extra:     "Server",
	}
	s.recordEvent(modMsg)
	s.BroadcastAll(models.FormatModeration(args, "promoted", "Server") + "\n")
	s.operatorSend(args + " has been promoted to admin.\n")
}

func (s *Server) operatorCmdDemote(args string) {
	if args == "" {
		s.operatorSend("Missing target. Usage: /demote <name>\n")
		return
	}
	target := s.GetClient(args)
	if target == nil {
		s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
		return
	}
	if !target.IsAdmin() {
		s.operatorSend(args + " is not an admin.\n")
		return
	}
	target.SetAdmin(false)
	s.RemoveAdmin(args)
	target.Send("Your admin privileges have been revoked.\n")

	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "demoted",
		Type:      models.MsgModeration,
		Extra:     "Server",
	}
	s.recordEvent(modMsg)
	s.BroadcastAll(models.FormatModeration(args, "demoted", "Server") + "\n")
	s.operatorSend(args + " has been demoted.\n")
}
