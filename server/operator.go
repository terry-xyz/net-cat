package server

import (
	"bufio"
	"fmt"
	"io"
	"net-cat/cmd"
	"net-cat/models"
	"strings"
	"time"
)

// ---------- operator terminal ----------

// StartOperator reads terminal input and feeds it through the operator command dispatcher until shutdown.
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
	case "switch":
		s.operatorSend("The /switch command is not applicable to the server operator.\n")
	case "create":
		s.operatorSend("The /create command is not applicable to the server operator.\n")
	case "list":
		s.operatorCmdList()
	case "help":
		s.operatorCmdHelp()
	case "rooms":
		s.operatorCmdRooms()
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

// operatorCmdList prints connected and queued users grouped by room for the server operator terminal.
func (s *Server) operatorCmdList() {
	s.mu.RLock()
	type entry struct {
		name string
		idle time.Duration
	}

	// Collect sorted room names
	roomNames := make([]string, 0, len(s.rooms))
	for rn := range s.rooms {
		roomNames = append(roomNames, rn)
	}
	// insertion sort
	for i := 1; i < len(roomNames); i++ {
		key := roomNames[i]
		j := i - 1
		for j >= 0 && roomNames[j] > key {
			roomNames[j+1] = roomNames[j]
			j--
		}
		roomNames[j+1] = key
	}

	type roomData struct {
		name    string
		entries []entry
		queue   []string // IPs of queued users
	}
	var rooms []roomData
	for _, rn := range roomNames {
		r := s.rooms[rn]
		entries := make([]entry, 0, len(r.clients))
		for n, cl := range r.clients {
			entries = append(entries, entry{name: n, idle: time.Since(cl.GetLastActivity()).Truncate(time.Second)})
		}
		// sort entries
		for i := 1; i < len(entries); i++ {
			key := entries[i]
			j := i - 1
			for j >= 0 && entries[j].name > key.name {
				entries[j+1] = entries[j]
				j--
			}
			entries[j+1] = key
		}
		var queueIPs []string
		for _, e := range r.queue {
			queueIPs = append(queueIPs, extractHost(e.client.IP))
		}
		rooms = append(rooms, roomData{name: rn, entries: entries, queue: queueIPs})
	}
	s.mu.RUnlock()

	for _, rd := range rooms {
		s.operatorSend(fmt.Sprintf("Room %s (%d clients):\n", rd.name, len(rd.entries)))
		for _, e := range rd.entries {
			s.operatorSend(fmt.Sprintf("  %s (idle: %s)\n", e.name, e.idle.String()))
		}
		for i, ip := range rd.queue {
			s.operatorSend(fmt.Sprintf("  [queued #%d] %s\n", i+1, ip))
		}
	}
}

// operatorCmdHelp prints the command set available to the server operator.
func (s *Server) operatorCmdHelp() {
	s.operatorSend("Available commands:\n")
	for _, name := range cmd.CommandOrder {
		def := cmd.Commands[name]
		s.operatorSend(fmt.Sprintf("  %-30s %s\n", def.Usage, def.Description))
	}
}

// operatorCmdKick kicks a connected user or same-IP queued clients from the operator terminal.
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
	targetRoom := target.Room
	target.ForceDisconnectReason("kicked")
	s.RemoveClient(args)

	modMsg := models.Message{
		Timestamp: time.Now(),
		Sender:    args,
		Content:   "kicked",
		Type:      models.MsgModeration,
		Extra:     "Server",
	}
	s.recordRoomEvent(targetRoom, modMsg)
	s.BroadcastRoom(targetRoom, models.FormatModeration(args, "kicked", "Server")+"\n", "")
	target.Send("You have been kicked by Server.\n")
	target.Close()
	s.AddKickCooldown(targetIP)
	s.admitFromRoomQueue(targetRoom)
	s.operatorSend(args + " has been kicked.\n")
}

// operatorCmdBan bans a user plus same-IP connections and queued clients from the operator terminal.
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
	targetRoom := target.Room
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
	s.recordRoomEvent(targetRoom, modMsg)
	s.BroadcastRoom(targetRoom, models.FormatModeration(args, "banned", "Server")+"\n", "")
	target.Send("You have been banned by Server.\n")
	target.Close()
	s.AddBanIP(targetIP)

	// Disconnect all other active clients sharing the banned IP (NAT scenario).
	// Operator is on terminal, not a TCP client, so exclude nobody ("").
	// Count one opened slot per disconnected user so each room admits the correct number of queued clients.
	roomsOpened := map[string]int{targetRoom: 1}
	collateral := s.GetClientsByIP(bannedHost, "")
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
			Extra:     "Server",
		}
		s.recordRoomEvent(ccRoom, collateralMsg)
		s.BroadcastRoom(ccRoom, models.FormatModeration(ccName, "banned", "Server")+"\n", "")
		cc.Send("You have been banned by Server.\n")
		cc.Close()
		roomsOpened[ccRoom]++
	}

	// Remove queued users from the banned IP
	queuedRemoved := s.RemoveFromQueueByIP(targetIP)
	for _, qc := range queuedRemoved {
		qc.Send("You have been banned by Server.\n")
		qc.Close()
	}

	for rn, count := range roomsOpened {
		for i := 0; i < count; i++ {
			s.admitFromRoomQueue(rn)
		}
	}
	s.operatorSend(args + " has been banned.\n")
}

// operatorCmdMute marks a connected user as muted from the operator terminal.
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
	s.recordRoomEvent(target.Room, modMsg)
	s.BroadcastAllRooms(models.FormatModeration(args, "muted", "Server") + "\n")
	s.operatorSend(args + " has been muted.\n")
}

// operatorCmdUnmute removes the muted flag from a connected user from the operator terminal.
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
	s.recordRoomEvent(target.Room, modMsg)
	s.BroadcastAllRooms(models.FormatModeration(args, "unmuted", "Server") + "\n")
	s.operatorSend(args + " has been unmuted.\n")
}

// operatorCmdAnnounce broadcasts a server-wide announcement from the operator terminal.
func (s *Server) operatorCmdAnnounce(args string) {
	if len(strings.TrimSpace(args)) == 0 {
		s.operatorSend("Usage: /announce <message>\n")
		return
	}
	// Record in every room so reconnecting users still see the operator announcement in history.
	for _, rn := range s.GetRoomNames() {
		announceMsg := models.Message{
			Timestamp: time.Now(),
			Content:   args,
			Type:      models.MsgAnnouncement,
			Extra:     "Server",
		}
		s.recordRoomEvent(rn, announceMsg)
	}
	s.BroadcastAllRooms(models.FormatAnnouncement(args) + "\n")
	s.operatorSend("Announcement sent.\n")
}

// operatorCmdPromote grants admin privileges from the operator terminal and persists the change.
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
	s.recordRoomEvent(target.Room, modMsg)
	s.BroadcastAllRooms(models.FormatModeration(args, "promoted", "Server") + "\n")
	s.operatorSend(args + " has been promoted to admin.\n")
}

// operatorCmdDemote removes admin privileges from the operator terminal and persists the change.
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
	s.recordRoomEvent(target.Room, modMsg)
	s.BroadcastAllRooms(models.FormatModeration(args, "demoted", "Server") + "\n")
	s.operatorSend(args + " has been demoted.\n")
}

// operatorCmdRooms lists room occupancy and queued clients for the operator terminal.
func (s *Server) operatorCmdRooms() {
	roomNames := s.GetRoomNames()
	s.operatorSend("Available rooms:\n")
	for _, rn := range roomNames {
		count := s.GetRoomClientCount(rn)
		s.operatorSend(fmt.Sprintf("  %s (%d clients)\n", rn, count))
	}
}
