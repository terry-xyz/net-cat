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

// RoomPrompt is sent during room selection.
const RoomPrompt = "[ENTER ROOM NAME] (Enter for 'general'):"

// handleConnection manages one TCP connection through onboarding, messaging, and cleanup.
func (s *Server) handleConnection(conn net.Conn) {
	enableTCPKeepAlive(conn)

	c := client.NewClient(conn)
	s.TrackClient(c)
	defer s.UntrackClient(c)

	if s.rejectConnection(c) {
		return
	}
	if !s.onboardClient(c) {
		return
	}

	defer s.cleanupConnection(c)

	s.prepareClientSession(c)
	s.serveClientSession(c)
}

func enableTCPKeepAlive(conn net.Conn) {
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(5 * time.Second)
	}
}

func (s *Server) rejectConnection(c *client.Client) bool {
	if blocked, reason := s.IsIPBlocked(c.IP); blocked {
		c.Conn.Write([]byte(reason))
		c.Close()
		return true
	}
	if s.IsShuttingDown() {
		c.Conn.Write([]byte("Server is shutting down. Goodbye!\n"))
		c.Close()
		return true
	}
	return false
}

func (s *Server) onboardClient(c *client.Client) bool {
	c.Send(WelcomeBanner + NamePrompt)
	if !s.registerClientName(c) {
		return false
	}
	s.restoreKnownAdmin(c)
	return s.selectRoomAndJoin(c)
}

func (s *Server) registerClientName(c *client.Client) bool {
	for {
		name, err := c.ReadLine()
		if err != nil {
			s.logOnboardingError(c, err)
			c.Close()
			return false
		}
		if validationMessage, ok := s.validateRequestedName(name); ok {
			c.Send(validationMessage + "\n" + NamePrompt)
			continue
		}
		if err := s.RegisterClient(c, name); err != nil {
			c.Send("Name is already taken.\n" + NamePrompt)
			continue
		}
		return true
	}
}

func (s *Server) logOnboardingError(c *client.Client, err error) {
	if err == io.EOF {
		return
	}
	s.Logger.Log(models.Message{
		Timestamp: time.Now(),
		Type:      models.MsgServerEvent,
		Content:   fmt.Sprintf("Connection error during onboarding from %s", c.IP),
	})
}

func (s *Server) validateRequestedName(name string) (string, bool) {
	if valErr := validateName(name); valErr != nil {
		return valErr.Error(), true
	}
	if s.IsReservedName(name) {
		return "Name '" + name + "' is reserved.", true
	}
	return "", false
}

func (s *Server) restoreKnownAdmin(c *client.Client) {
	if !s.IsKnownAdmin(c.Username) {
		return
	}
	c.SetAdmin(true)
	c.Send("Welcome back, admin!\n")
}

func (s *Server) selectRoomAndJoin(c *client.Client) bool {
	roomName := s.readRoomChoice(c)
	if roomName == "" {
		s.RemoveClient(c.Username)
		c.Close()
		return false
	}
	if !s.checkOrQueueRoom(c, roomName) {
		s.RemoveClient(c.Username)
		c.Close()
		return false
	}

	s.mu.Lock()
	s.JoinRoom(c, roomName)
	s.mu.Unlock()
	return true
}

func (s *Server) cleanupConnection(c *client.Client) {
	username := c.Username
	currentRoom := c.Room
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
		s.recordRoomEvent(currentRoom, leaveMsg)
		s.BroadcastRoom(currentRoom, models.FormatLeave(username)+"\n", username)
	}
	s.admitFromRoomQueue(currentRoom)
	s.deleteRoomIfEmpty(currentRoom)
	c.Close()
}

func (s *Server) prepareClientSession(c *client.Client) {
	for _, msg := range s.GetRoomHistory(c.Room) {
		c.Send(msg.Display() + "\n")
	}
	c.SetEchoMode(true)
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
	s.broadcastClientJoin(c)
	c.SetLastInput(time.Now())
	go s.startHeartbeat(c)
}

func (s *Server) broadcastClientJoin(c *client.Client) {
	joinMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    c.Username,
		Type:      models.MsgJoin,
	}
	s.recordRoomEvent(c.Room, joinMsg)
	s.BroadcastRoom(c.Room, models.FormatJoin(c.Username)+"\n", c.Username)
}

func (s *Server) serveClientSession(c *client.Client) {
	for {
		line, err := c.ReadLineInteractive()
		if err != nil {
			s.handleSessionReadError(c, err)
			return
		}
		c.SetLastInput(time.Now())
		cmdName, args, isCmd := cmd.ParseCommand(line)
		if isCmd {
			if s.dispatchCommand(c, cmdName, args) {
				return
			}
			continue
		}
		s.handleChatMessage(c, line)
	}
}

func (s *Server) handleSessionReadError(c *client.Client, err error) {
	c.SetDisconnectReason("drop")
	if err == io.EOF {
		return
	}
	s.Logger.Log(models.Message{
		Timestamp: time.Now(),
		Type:      models.MsgServerEvent,
		Content:   fmt.Sprintf("Connection error for %s: %v", c.Username, err),
	})
}

// ---------- room selection ----------

// sendRoomSelection lists available rooms with counts and sends the room prompt.
func (s *Server) sendRoomSelection(c *client.Client) {
	roomNames := s.GetRoomNames()
	c.Send("Available rooms:\n")
	for _, rn := range roomNames {
		count := s.GetRoomClientCount(rn)
		c.Send(fmt.Sprintf("  %s (%d/%d users)\n", rn, count, MaxActiveClients))
	}
	c.Send(RoomPrompt)
}

// readRoomChoice prompts until the client picks a valid room or disconnects.
func (s *Server) readRoomChoice(c *client.Client) string {
	s.sendRoomSelection(c)
	for {
		line, err := c.ReadLine()
		if err != nil {
			return ""
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return s.DefaultRoom
		}
		if err := validateRoomName(line); err != nil {
			c.Send(err.Error() + "\n" + RoomPrompt)
			continue
		}
		return line
	}
}

// ---------- capacity check and queue (per-room) ----------

// checkOrQueueRoom either admits the client immediately or offers a per-room wait queue when capacity is exhausted.
func (s *Server) checkOrQueueRoom(c *client.Client, roomName string) bool {
	if s.IsShuttingDown() {
		return false
	}

	if s.checkRoomCapacity(roomName) {
		return true
	}

	// Room is full — offer queue
	// Queue first so the position we show matches the admission order the room will later use.
	s.mu.Lock()
	r := s.getOrCreateRoom(roomName)
	entry := &QueueEntry{
		client: c,
		admit:  make(chan struct{}),
	}
	r.queue = append(r.queue, entry)
	pos := len(r.queue)
	s.mu.Unlock()

	c.Send(fmt.Sprintf("Room '%s' is full. You are #%d in the queue. Would you like to wait? (yes/no)\n", roomName, pos))

	// Read yes/no response
	for {
		line, err := c.ReadLine()
		if err != nil {
			s.removeFromRoomQueue(roomName, entry)
			return false
		}
		line = strings.TrimSpace(line)
		switch line {
		case "yes":
			c.Send("Waiting for a slot to open... (press Ctrl+C to cancel)\n")
			return s.waitForRoomAdmission(c, roomName, entry)
		case "no":
			s.removeFromRoomQueue(roomName, entry)
			return false
		default:
			c.Send("Invalid input. Please type 'yes' or 'no'.\n")
		}
	}
}

// waitForRoomAdmission blocks queued clients until their room slot opens, they disconnect, or shutdown begins.
func (s *Server) waitForRoomAdmission(c *client.Client, roomName string, entry *QueueEntry) bool {
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
			select {
			case <-entry.admit:
				return
			default:
			}
		}
	}()

	select {
	case <-entry.admit:
		// Break the blocking ReadLine in the monitor goroutine so the client can resume normal reads after admission.
		c.Conn.SetReadDeadline(time.Now())
		<-monitorDone
		c.Conn.SetReadDeadline(time.Time{})
		c.ResetScanner()
		return true
	case <-s.quit:
		time.Sleep(100 * time.Millisecond)
		s.removeFromRoomQueue(roomName, entry)
		return false
	case <-readDone:
		s.removeFromRoomQueue(roomName, entry)
		return false
	}
}

// ---------- name validation ----------

// validateName checks format rules (no uniqueness – that is checked during registration).
func validateName(name string) error {
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

// validateRoomName checks room name format (same rules as validateName).
func validateRoomName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("Room name cannot be empty.")
	}
	if len(name) > 32 {
		return fmt.Errorf("Room name too long (max 32 characters).")
	}
	for _, b := range []byte(name) {
		if b < 0x21 || b > 0x7E {
			return fmt.Errorf("Room name must contain only printable characters.")
		}
	}
	return nil
}
