package server

import (
	"fmt"
	"io"
	"net"
	"net-cat/client"
	"net-cat/cmd"
	"net-cat/models"
	"strings"
	"time"
)

// WelcomeBanner is the exact ASCII art sent on first connect (no trailing prompt).
const WelcomeBanner = "Welcome to TCP-Chat!\n" +
	"         _nnnn_\n" +
	"        dGGGGMMb\n" +
	"       @p~qp~~qMb\n" +
	"       M|@||@) M|\n" +
	"       @,----.JM|\n" +
	"      JS^\\__/  qKL\n" +
	"     dZP        qKRb\n" +
	"    dZP          qKKb\n" +
	"   fZP            SMMb\n" +
	"   HZM            MMMM\n" +
	"   FqM            MMMM\n" +
	" __| \".        |\\dS\"qML\n" +
	" |    `.       | `' \\Zq\n" +
	"_)      \\.___.,|     .'\n" +
	"\\____   )MMMMMP|   .'\n" +
	"     `-'       `--'\n"

// NamePrompt is re-sent after every failed validation attempt.
const NamePrompt = "[ENTER YOUR NAME]:"

// handleConnection manages one TCP connection through onboarding, messaging, and cleanup.
func (s *Server) handleConnection(conn net.Conn) {
	// Enable aggressive TCP keepalive for faster dead peer detection on real connections
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(5 * time.Second)
	}

	c := client.NewClient(conn)
	s.TrackClient(c)
	defer s.UntrackClient(c)

	// Check IP against kick/ban lists BEFORE welcome banner or queue prompt.
	// Write directly to conn (bypassing the async writeLoop) to guarantee delivery
	// before we close the connection.
	if blocked, reason := s.IsIPBlocked(c.IP); blocked {
		conn.Write([]byte(reason))
		c.Close()
		return
	}

	// Reject connections that arrive during shutdown
	if s.IsShuttingDown() {
		conn.Write([]byte("Server is shutting down. Goodbye!\n"))
		c.Close()
		return
	}

	// Check capacity before sending banner
	if !s.checkOrQueue(c) {
		c.Close()
		return
	}

	// Send welcome banner + name prompt
	c.Send(WelcomeBanner + NamePrompt)

	// --- Name validation loop (infinite retries) ---
	registered := false
	for {
		name, err := c.ReadLine()
		if err != nil {
			if err != io.EOF {
				s.Logger.Log(models.Message{
					Timestamp: time.Now(),
					Type:      models.MsgServerEvent,
					Content:   fmt.Sprintf("Connection error during onboarding from %s", c.IP),
				})
			}
			c.Close()
			return
		}

		if valErr := ValidateName(name); valErr != nil {
			c.Send(valErr.Error() + "\n" + NamePrompt)
			continue
		}
		if s.IsReservedName(name) {
			c.Send("Name '" + name + "' is reserved.\n" + NamePrompt)
			continue
		}
		if !s.RegisterClient(c, name) {
			c.Send("Name is already taken.\n" + NamePrompt)
			continue
		}
		registered = true
		break
	}
	if !registered {
		c.Close()
		return
	}

	// Auto-restore admin privileges for known admins
	if s.IsKnownAdmin(c.Username) {
		c.SetAdmin(true)
		c.Send("Welcome back, admin!\n")
	}

	// Cleanup runs on any exit from this point (disconnect, /quit, kick, etc.)
	defer func() {
		username := c.Username
		switch c.GetDisconnectReason() {
		case "kicked", "banned":
			// moderation handler already removed from map + broadcast + logged
		default:
			s.RemoveClient(username)
			reason := c.GetDisconnectReason()
			if reason == "" {
				reason = "voluntary"
			}
			leaveMsg := models.Message{
				Timestamp: time.Now(),
				Sender:    username,
				Type:      models.MsgLeave,
				Extra:     reason,
			}
			s.recordEvent(leaveMsg)
			s.Broadcast(models.FormatLeave(username)+"\n", username)
		}
		s.admitFromQueue()
		c.Close()
	}()

	// Deliver history
	for _, msg := range s.GetHistory() {
		c.Send(msg.Display() + "\n")
	}

	// Enable character-at-a-time echo mode for input continuity
	c.SetEchoMode(true)

	// First prompt (uses SendPrompt so writeLoop tracks the prompt for redraw)
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))

	// Broadcast join
	joinMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    c.Username,
		Type:      models.MsgJoin,
	}
	s.recordEvent(joinMsg)
	s.Broadcast(models.FormatJoin(c.Username)+"\n", c.Username)

	// Initialize heartbeat tracking and start the health check goroutine
	c.SetLastInput(time.Now())
	go s.startHeartbeat(c)

	// --- Message loop (character-at-a-time reading with echo) ---
	for {
		line, err := c.ReadLineInteractive()
		if err != nil {
			c.SetDisconnectReason("drop")
			if err != io.EOF {
				s.Logger.Log(models.Message{
					Timestamp: time.Now(),
					Type:      models.MsgServerEvent,
					Content:   fmt.Sprintf("Connection error for %s: %v", c.Username, err),
				})
			}
			return
		}
		// Any input from the client proves they are alive (heartbeat tracking)
		c.SetLastInput(time.Now())
		cmdName, args, isCmd := cmd.ParseCommand(line)
		if isCmd {
			if s.dispatchCommand(c, cmdName, args) {
				return // /quit
			}
			continue
		}
		s.handleChatMessage(c, line)
	}
}

// ---------- capacity check and queue ----------

// checkOrQueue returns true if the client can proceed to onboarding.
// If the server is at capacity, offers a queue position and blocks until
// admitted, declined, or the server shuts down.
func (s *Server) checkOrQueue(c *client.Client) bool {
	if s.IsShuttingDown() {
		return false
	}

	s.mu.RLock()
	activeCount := len(s.clients)
	s.mu.RUnlock()

	if activeCount < MaxActiveClients {
		return true
	}

	// Server is full — offer queue
	s.mu.Lock()
	entry := &QueueEntry{
		client: c,
		admit:  make(chan struct{}),
	}
	s.queue = append(s.queue, entry)
	pos := len(s.queue)
	s.mu.Unlock()

	c.Send(fmt.Sprintf("Chat is full. You are #%d in the queue. Would you like to wait? (yes/no)\n", pos))

	// Read yes/no response
	for {
		line, err := c.ReadLine()
		if err != nil {
			s.removeFromQueue(entry)
			return false
		}
		line = strings.TrimSpace(line)
		switch line {
		case "yes":
			return s.waitForAdmission(c, entry)
		case "no":
			s.removeFromQueue(entry)
			return false
		default:
			c.Send("Invalid input. Please type 'yes' or 'no'.\n")
		}
	}
}

// waitForAdmission blocks until the client is admitted from the queue, disconnects,
// or the server shuts down. Returns true if admitted.
func (s *Server) waitForAdmission(c *client.Client, entry *QueueEntry) bool {
	// Start a goroutine that reads from the connection to detect disconnection.
	// Any input while queued is ignored; errors signal disconnection.
	readDone := make(chan error, 1)
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		for {
			_, err := c.ReadLine()
			if err != nil {
				select {
				case readDone <- err:
				default:
				}
				return
			}
			// Check if admitted (entry.admit closed)
			select {
			case <-entry.admit:
				return
			default:
				// Ignore input while queued
			}
		}
	}()

	select {
	case <-entry.admit:
		// Admitted — stop the monitor goroutine by setting a deadline
		c.Conn.SetReadDeadline(time.Now())
		<-monitorDone // wait for goroutine to exit
		c.Conn.SetReadDeadline(time.Time{})
		c.ResetScanner()
		return true
	case <-s.quit:
		// Server shutting down — Shutdown() already sent the goodbye message;
		// give the write goroutine time to flush it before we return and
		// handleConnection closes the connection.
		time.Sleep(100 * time.Millisecond)
		s.removeFromQueue(entry)
		return false
	case <-readDone:
		// Client disconnected while waiting
		s.removeFromQueue(entry)
		return false
	}
}

// ---------- name validation ----------

// ValidateName checks format rules (no uniqueness – that is checked during registration).
func ValidateName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("Name cannot be empty.")
	}
	allWhitespace := true
	for _, b := range []byte(name) {
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			allWhitespace = false
			break
		}
	}
	if allWhitespace {
		return fmt.Errorf("Name cannot be empty.")
	}
	for _, b := range []byte(name) {
		if b == ' ' {
			return fmt.Errorf("Name cannot contain spaces.")
		}
	}
	if len(name) > 32 {
		return fmt.Errorf("Name too long (max 32 characters).")
	}
	for _, b := range []byte(name) {
		if b < 0x21 || b > 0x7E {
			return fmt.Errorf("Name must contain only printable characters.")
		}
	}
	return nil
}

// ---------- chat messages ----------

func (s *Server) handleChatMessage(c *client.Client, line string) {
	if len(strings.TrimSpace(line)) == 0 {
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if len(line) > 2048 {
		c.Send("Message too long (max 2048 characters).\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if c.IsMuted() {
		c.Send("You are muted.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	now := time.Now()
	msg := models.Message{
		Timestamp: now,
		Sender:    c.Username,
		Content:   line,
		Type:      models.MsgChat,
	}
	s.recordEvent(msg)
	s.Broadcast(msg.Display()+"\n", c.Username)
	c.SetLastActivity(now)
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- command dispatch ----------

// dispatchCommand routes a parsed command. Returns true when the caller should exit (/quit).
func (s *Server) dispatchCommand(c *client.Client, cmdName, args string) bool {
	def, exists := cmd.Commands[cmdName]
	if !exists {
		c.Send("Unknown command: /" + cmdName + ". Use /help to see available commands.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return false
	}
	clientPriv := cmd.GetPrivilegeLevel(c.IsAdmin(), false)
	if def.MinPriv > clientPriv {
		c.Send("Insufficient privileges.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return false
	}
	switch cmdName {
	case "quit":
		return true
	case "list":
		s.cmdList(c)
	case "help":
		s.cmdHelp(c)
	case "name":
		s.cmdName(c, args)
	case "whisper":
		s.cmdWhisper(c, args)
	case "kick":
		s.cmdKick(c, args)
	case "ban":
		s.cmdBan(c, args)
	case "mute":
		s.cmdMute(c, args)
	case "unmute":
		s.cmdUnmute(c, args)
	case "announce":
		s.cmdAnnounce(c, args)
	case "promote":
		s.cmdPromote(c, args)
	case "demote":
		s.cmdDemote(c, args)
	}
	return false
}

// ---------- /list ----------

func (s *Server) cmdList(c *client.Client) {
	s.mu.RLock()
	type entry struct {
		name string
		idle time.Duration
	}
	entries := make([]entry, 0, len(s.clients))
	for n, cl := range s.clients {
		entries = append(entries, entry{name: n, idle: time.Since(cl.GetLastActivity()).Truncate(time.Second)})
	}
	s.mu.RUnlock()

	// simple insertion sort (no sort package allowed)
	for i := 1; i < len(entries); i++ {
		key := entries[i]
		j := i - 1
		for j >= 0 && entries[j].name > key.name {
			entries[j+1] = entries[j]
			j--
		}
		entries[j+1] = key
	}

	c.Send("Connected clients:\n")
	for _, e := range entries {
		c.Send(fmt.Sprintf("  %s (idle: %s)\n", e.name, e.idle.String()))
	}
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /help (role-aware) ----------

func (s *Server) cmdHelp(c *client.Client) {
	priv := cmd.GetPrivilegeLevel(c.IsAdmin(), false)
	c.Send("Available commands:\n")
	for _, name := range cmd.CommandOrder {
		def := cmd.Commands[name]
		if def.MinPriv <= priv {
			c.Send(fmt.Sprintf("  %-30s %s\n", def.Usage, def.Description))
		}
	}
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /name ----------

func (s *Server) cmdName(c *client.Client, args string) {
	if args == "" {
		c.Send("Usage: /name <newname>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	newName := args
	if err := ValidateName(newName); err != nil {
		c.Send(err.Error() + "\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if s.IsReservedName(newName) {
		c.Send("Name '" + newName + "' is reserved.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if newName == c.Username {
		c.Send("You already have that name.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	oldName := c.Username
	if !s.RenameClient(c, oldName, newName) {
		c.Send("Name is already taken.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	// Update admins.json if this client is an admin
	if c.IsAdmin() {
		s.RenameAdmin(oldName, newName)
	}

	nameMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    newName,
		Type:      models.MsgNameChange,
		Extra:     oldName,
	}
	s.recordEvent(nameMsg)
	s.BroadcastAll(models.FormatNameChange(oldName, newName) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /whisper ----------

func (s *Server) cmdWhisper(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing recipient. Usage: /whisper <name> <message>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	idx := strings.IndexByte(args, ' ')
	if idx < 0 {
		c.Send("Missing message. Usage: /whisper <name> <message>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	recipient := args[:idx]
	message := strings.TrimSpace(args[idx+1:])
	if len(strings.TrimSpace(message)) == 0 {
		c.Send("Missing message. Usage: /whisper <name> <message>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if len(message) > 2048 {
		c.Send("Message too long (max 2048 characters).\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if recipient == c.Username {
		c.Send("Cannot whisper to yourself.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	target := s.GetClient(recipient)
	if target == nil {
		c.Send("User '" + recipient + "' not found. Use /list to see connected users.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	now := time.Now()
	target.Send(models.FormatWhisperReceive(now, c.Username, message) + "\n")
	c.Send(models.FormatWhisperSend(now, recipient, message) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /kick ----------

func (s *Server) cmdKick(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /kick <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
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
		Extra:     c.Username,
	}
	s.recordEvent(modMsg)
	s.Broadcast(models.FormatModeration(args, "kicked", c.Username)+"\n", "")
	target.Send("You have been kicked by " + c.Username + ".\n")
	target.Close()
	s.AddKickCooldown(targetIP)
	s.admitFromQueue()
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /ban ----------

func (s *Server) cmdBan(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /ban <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
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
		Extra:     c.Username,
	}
	s.recordEvent(modMsg)
	s.Broadcast(models.FormatModeration(args, "banned", c.Username)+"\n", "")
	target.Send("You have been banned by " + c.Username + ".\n")
	target.Close()
	s.AddBanIP(targetIP)

	// Disconnect all other active clients sharing the banned IP (NAT scenario).
	// Exclude the command issuer in case they share the same IP.
	slotsOpened := 1
	collateral := s.GetClientsByIP(bannedHost, c.Username)
	for _, cc := range collateral {
		cc.ForceDisconnectReason("banned")
		ccName := cc.Username
		s.RemoveClient(ccName)
		collateralMsg := models.Message{
			Timestamp: time.Now(),
			Sender:    ccName,
			Content:   "banned",
			Type:      models.MsgModeration,
			Extra:     c.Username,
		}
		s.recordEvent(collateralMsg)
		s.Broadcast(models.FormatModeration(ccName, "banned", c.Username)+"\n", "")
		cc.Send("You have been banned by " + c.Username + ".\n")
		cc.Close()
		slotsOpened++
	}

	// Remove queued users from the banned IP
	queuedRemoved := s.RemoveFromQueueByIP(targetIP)
	for _, qc := range queuedRemoved {
		qc.Send("You have been banned by " + c.Username + ".\n")
		qc.Close()
	}

	for i := 0; i < slotsOpened; i++ {
		s.admitFromQueue()
	}
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /mute ----------

func (s *Server) cmdMute(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /mute <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if target.IsMuted() {
		c.Send(args + " is already muted.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	target.SetMuted(true)
	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "muted",
		Type:      models.MsgModeration,
		Extra:     c.Username,
	}
	s.recordEvent(modMsg)
	s.BroadcastAll(models.FormatModeration(args, "muted", c.Username) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /unmute ----------

func (s *Server) cmdUnmute(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /unmute <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if !target.IsMuted() {
		c.Send(args + " is not muted.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	target.SetMuted(false)
	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "unmuted",
		Type:      models.MsgModeration,
		Extra:     c.Username,
	}
	s.recordEvent(modMsg)
	s.BroadcastAll(models.FormatModeration(args, "unmuted", c.Username) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /announce ----------

func (s *Server) cmdAnnounce(c *client.Client, args string) {
	if len(strings.TrimSpace(args)) == 0 {
		c.Send("Usage: /announce <message>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	announceMsg := models.Message{
		Timestamp: time.Now(),
		Content:   args,
		Type:      models.MsgAnnouncement,
		Extra:     c.Username,
	}
	s.recordEvent(announceMsg)
	s.BroadcastAll(models.FormatAnnouncement(args) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /promote ----------

func (s *Server) cmdPromote(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /promote <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if target.IsAdmin() {
		c.Send(args + " is already an admin.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
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
		Extra:     c.Username,
	}
	s.recordEvent(modMsg)
	s.BroadcastAll(models.FormatModeration(args, "promoted", c.Username) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /demote ----------

func (s *Server) cmdDemote(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /demote <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if !target.IsAdmin() {
		c.Send(args + " is not an admin.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
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
		Extra:     c.Username,
	}
	s.recordEvent(modMsg)
	s.BroadcastAll(models.FormatModeration(args, "demoted", c.Username) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}
