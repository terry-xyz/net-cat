package server

import (
	"fmt"
	"net-cat/client"
	"net-cat/cmd"
	"net-cat/models"
	"strings"
	"time"
)

// ---------- chat messages ----------

// handleChatMessage validates a chat line, records it in room history, and broadcasts it to the room.
func (s *Server) handleChatMessage(c *client.Client, line string) {
	if len(strings.TrimSpace(line)) == 0 {
		// Empty input still gets a fresh prompt so the client stays in sync after pressing Enter.
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
	s.recordRoomEvent(c.Room, msg)
	s.BroadcastRoom(c.Room, msg.Display()+"\n", c.Username)
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
	case "rooms":
		s.cmdRooms(c)
	case "switch":
		s.cmdSwitch(c, args)
	case "create":
		s.cmdCreate(c, args)
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

// ---------- /list (room-scoped) ----------

// cmdList shows the requesting client who is present in their current room and how long each user has been idle.
func (s *Server) cmdList(c *client.Client) {
	roomName := c.Room
	s.mu.RLock()
	type entry struct {
		name string
		idle time.Duration
	}
	r, ok := s.rooms[roomName]
	if !ok {
		s.mu.RUnlock()
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	entries := make([]entry, 0, len(r.clients))
	for n, cl := range r.clients {
		entries = append(entries, entry{name: n, idle: time.Since(cl.GetLastActivity()).Truncate(time.Second)})
	}
	s.mu.RUnlock()

	// Keep output deterministic without pulling in another dependency for a tiny room-local list.
	for i := 1; i < len(entries); i++ {
		key := entries[i]
		j := i - 1
		for j >= 0 && entries[j].name > key.name {
			entries[j+1] = entries[j]
			j--
		}
		entries[j+1] = key
	}

	c.Send(fmt.Sprintf("Room %s — connected clients:\n", roomName))
	for _, e := range entries {
		c.Send(fmt.Sprintf("  %s (idle: %s)\n", e.name, e.idle.String()))
	}
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /rooms ----------

// cmdRooms lists all rooms with occupancy counts and marks the caller's current room.
func (s *Server) cmdRooms(c *client.Client) {
	roomNames := s.GetRoomNames()
	c.Send("Available rooms:\n")
	for _, rn := range roomNames {
		count := s.GetRoomClientCount(rn)
		marker := ""
		if rn == c.Room {
			marker = " (current)"
		}
		c.Send(fmt.Sprintf("  %s (%d/%d users)%s\n", rn, count, MaxActiveClients, marker))
	}
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /switch ----------

// cmdSwitch moves a client to another room when the target room name is valid and has capacity.
func (s *Server) cmdSwitch(c *client.Client, args string) {
	if args == "" {
		c.Send("Usage: /switch <room>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	targetRoom := args
	if err := validateRoomName(targetRoom); err != nil {
		c.Send(err.Error() + "\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if targetRoom == c.Room {
		c.Send("You are already in room '" + targetRoom + "'.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if !s.checkRoomCapacity(targetRoom) {
		c.Send("Room '" + targetRoom + "' is full.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	s.switchClientRoom(c, targetRoom)
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /create ----------

// cmdCreate creates a new room name on demand and switches the caller into it.
func (s *Server) cmdCreate(c *client.Client, args string) {
	if args == "" {
		c.Send("Usage: /create <room>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	roomName := args
	if err := validateRoomName(roomName); err != nil {
		c.Send(err.Error() + "\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	// Check if room already exists
	s.mu.RLock()
	_, exists := s.rooms[roomName]
	s.mu.RUnlock()
	if exists {
		c.Send("Room '" + roomName + "' already exists. Use /switch " + roomName + " to join it.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	s.switchClientRoom(c, roomName)
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// switchClientRoom moves a client between rooms while preserving leave and join history for both sides.
func (s *Server) switchClientRoom(c *client.Client, newRoom string) {
	oldRoom := c.Room

	// Broadcast leave to old room
	leaveMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    c.Username,
		Type:      models.MsgLeave,
		Extra:     "switched",
	}
	s.recordRoomEvent(oldRoom, leaveMsg)
	s.BroadcastRoom(oldRoom, models.FormatLeave(c.Username)+"\n", c.Username)

	// Move to new room
	s.mu.Lock()
	s.JoinRoom(c, newRoom)
	s.mu.Unlock()

	// Admit from old room's queue and clean up
	s.admitFromRoomQueue(oldRoom)
	s.deleteRoomIfEmpty(oldRoom)

	// Deliver new room's history
	for _, msg := range s.GetRoomHistory(newRoom) {
		c.Send(msg.Display() + "\n")
	}

	// Broadcast join to new room
	joinMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    c.Username,
		Type:      models.MsgJoin,
	}
	s.recordRoomEvent(newRoom, joinMsg)
	s.BroadcastRoom(newRoom, models.FormatJoin(c.Username)+"\n", c.Username)

	c.Send("Switched to room '" + newRoom + "'.\n")
}

// ---------- /help (role-aware) ----------

// cmdHelp prints the commands the caller is allowed to use at their current privilege level.
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

// cmdName validates a requested username change, updates indexes, and broadcasts the rename.
func (s *Server) cmdName(c *client.Client, args string) {
	if args == "" {
		c.Send("Usage: /name <newname>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	newName := args
	if err := validateName(newName); err != nil {
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

	// Persist the rename too, or the returning admin would lose auto-restore on their next reconnect.
	if c.IsAdmin() {
		s.RenameAdmin(oldName, newName)
	}

	nameMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    newName,
		Type:      models.MsgNameChange,
		Extra:     oldName,
	}
	s.recordRoomEvent(c.Room, nameMsg)
	s.BroadcastRoom(c.Room, models.FormatNameChange(oldName, newName)+"\n", "")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /whisper (cross-room) ----------

// cmdWhisper sends a private message directly to another connected user without entering room history.
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

// ---------- /kick (same-room only) ----------

// cmdKick removes a same-room user, broadcasts the moderation event, and starts their reconnect cooldown.
func (s *Server) cmdKick(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /kick <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	if target.Room != c.Room {
		c.Send("User '" + args + "' is not in your room.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	targetIP := target.IP
	targetRoom := target.Room
	target.ForceDisconnectReason("kicked")
	s.RemoveClient(args)

	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "kicked",
		Type:      models.MsgModeration,
		Extra:     c.Username,
	}
	s.recordRoomEvent(targetRoom, modMsg)
	s.BroadcastRoom(targetRoom, models.FormatModeration(args, "kicked", c.Username)+"\n", "")
	target.Send("You have been kicked by " + c.Username + ".\n")
	target.Close()
	s.AddKickCooldown(targetIP)
	s.admitFromRoomQueue(targetRoom)
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /ban (global) ----------

// cmdBan disconnects the target plus same-IP clients, removes queued peers on that IP, and blocks future reconnects.
func (s *Server) cmdBan(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /ban <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}

	targetIP := target.IP
	targetRoom := target.Room
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
	s.recordRoomEvent(targetRoom, modMsg)
	s.BroadcastRoom(targetRoom, models.FormatModeration(args, "banned", c.Username)+"\n", "")
	target.Send("You have been banned by " + c.Username + ".\n")
	target.Close()
	s.AddBanIP(targetIP)

	// Track one opened slot per disconnected user so each affected room admits the right number of queued clients.
	roomsOpened := map[string]int{targetRoom: 1}
	collateral := s.GetClientsByIP(bannedHost, c.Username)
	for _, cc := range collateral {
		cc.ForceDisconnectReason("banned")
		ccName := cc.Username
		ccRoom := cc.Room
		s.RemoveClient(ccName)
		collateralMsg := models.Message{
			Timestamp: time.Now(),
			Sender:    ccName,
			Content:   "banned",
			Type:      models.MsgModeration,
			Extra:     c.Username,
		}
		s.recordRoomEvent(ccRoom, collateralMsg)
		s.BroadcastRoom(ccRoom, models.FormatModeration(ccName, "banned", c.Username)+"\n", "")
		cc.Send("You have been banned by " + c.Username + ".\n")
		cc.Close()
		roomsOpened[ccRoom]++
	}

	queuedRemoved := s.RemoveFromQueueByIP(targetIP)
	for _, qc := range queuedRemoved {
		qc.Send("You have been banned by " + c.Username + ".\n")
		qc.Close()
	}

	for rn, count := range roomsOpened {
		for i := 0; i < count; i++ {
			s.admitFromRoomQueue(rn)
		}
	}
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /mute (global, broadcast to all rooms) ----------

// cmdMute marks a user as unable to chat and broadcasts the moderation event to every room.
func (s *Server) cmdMute(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /mute <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
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
	s.recordRoomEvent(target.Room, modMsg)
	s.BroadcastAllRooms(models.FormatModeration(args, "muted", c.Username) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /unmute ----------

// cmdUnmute restores a muted user's ability to send chat messages.
func (s *Server) cmdUnmute(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /unmute <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
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
	s.recordRoomEvent(target.Room, modMsg)
	s.BroadcastAllRooms(models.FormatModeration(args, "unmuted", c.Username) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /announce (server-wide) ----------

// cmdAnnounce records an announcement in every room history and broadcasts it server-wide.
func (s *Server) cmdAnnounce(c *client.Client, args string) {
	if len(strings.TrimSpace(args)) == 0 {
		c.Send("Usage: /announce <message>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	// Record in every room so late joiners see the announcement no matter where they connect.
	for _, rn := range s.GetRoomNames() {
		announceMsg := models.Message{
			Timestamp: time.Now(),
			Content:   args,
			Type:      models.MsgAnnouncement,
			Extra:     c.Username,
		}
		s.recordRoomEvent(rn, announceMsg)
	}
	s.BroadcastAllRooms(models.FormatAnnouncement(args) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /promote ----------

// cmdPromote grants admin privileges to a connected user and persists that status for future reconnects.
func (s *Server) cmdPromote(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /promote <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
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
	s.recordRoomEvent(target.Room, modMsg)
	s.BroadcastAllRooms(models.FormatModeration(args, "promoted", c.Username) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}

// ---------- /demote ----------

// cmdDemote revokes a connected admin's privileges and removes them from persistent admin storage.
func (s *Server) cmdDemote(c *client.Client, args string) {
	if args == "" {
		c.Send("Missing target. Usage: /demote <name>\n")
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
		return
	}
	target := s.GetClient(args)
	if target == nil {
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
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
	s.recordRoomEvent(target.Room, modMsg)
	s.BroadcastAllRooms(models.FormatModeration(args, "demoted", c.Username) + "\n")
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
}
