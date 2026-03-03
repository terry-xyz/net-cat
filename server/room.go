package server

import (
	"fmt"
	"net-cat/client"
	"net-cat/models"
)

// Room holds per-room state: members, history, and waiting queue.
// All access is protected by s.mu — no per-room mutex.
type Room struct {
	Name    string
	clients map[string]*client.Client
	history []models.Message
	queue   []*QueueEntry
}

func newRoom(name string) *Room {
	return &Room{
		Name:    name,
		clients: make(map[string]*client.Client),
	}
}

// ---------- room management (all require s.mu held) ----------

// getOrCreateRoom returns the room, creating it if needed. Must hold s.mu write lock.
func (s *Server) getOrCreateRoom(name string) *Room {
	r, ok := s.rooms[name]
	if !ok {
		r = newRoom(name)
		s.rooms[name] = r
	}
	return r
}

// deleteRoomIfEmpty removes a room from the map if it has no clients and no queue.
// Never deletes the DefaultRoom.
func (s *Server) deleteRoomIfEmpty(name string) {
	if name == s.DefaultRoom {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rooms[name]
	if !ok {
		return
	}
	if len(r.clients) == 0 && len(r.queue) == 0 {
		delete(s.rooms, name)
	}
}

// JoinRoom moves a client from their current room (if any) into the target room.
// Must hold s.mu write lock.
func (s *Server) JoinRoom(c *client.Client, roomName string) {
	// Remove from old room if any
	if c.Room != "" {
		if oldRoom, ok := s.rooms[c.Room]; ok {
			delete(oldRoom.clients, c.Username)
		}
	}
	r := s.getOrCreateRoom(roomName)
	r.clients[c.Username] = c
	c.Room = roomName
}

// ---------- room-scoped broadcast ----------

// BroadcastRoom sends msg to every client in the room except exclude.
func (s *Server) BroadcastRoom(roomName, msg string, exclude string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rooms[roomName]
	if !ok {
		return
	}
	for name, c := range r.clients {
		if name != exclude {
			c.Send(msg)
		}
	}
}

// BroadcastRoomAll sends msg to every client in the room.
func (s *Server) BroadcastRoomAll(roomName, msg string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rooms[roomName]
	if !ok {
		return
	}
	for _, c := range r.clients {
		c.Send(msg)
	}
}

// BroadcastAllRooms sends msg to every connected client across all rooms.
func (s *Server) BroadcastAllRooms(msg string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		c.Send(msg)
	}
}

// ---------- room history ----------

// GetRoomHistory returns a copy of the room's history.
func (s *Server) GetRoomHistory(roomName string) []models.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rooms[roomName]
	if !ok {
		return nil
	}
	out := make([]models.Message, len(r.history))
	copy(out, r.history)
	return out
}

// AddRoomHistory appends a message to the room's history.
func (s *Server) AddRoomHistory(roomName string, msg models.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.getOrCreateRoom(roomName)
	r.history = append(r.history, msg)
}

// recordRoomEvent sets the Room field on the message, adds to room history, and logs.
func (s *Server) recordRoomEvent(roomName string, msg models.Message) {
	msg.Room = roomName
	s.AddRoomHistory(roomName, msg)
	s.Logger.Log(msg)
}

// ---------- room queries ----------

// GetRoomNames returns a sorted list of room names.
func (s *Server) GetRoomNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.rooms))
	for name := range s.rooms {
		names = append(names, name)
	}
	// insertion sort
	for i := 1; i < len(names); i++ {
		key := names[i]
		j := i - 1
		for j >= 0 && names[j] > key {
			names[j+1] = names[j]
			j--
		}
		names[j+1] = key
	}
	return names
}

// GetRoomClientCount returns the number of clients in a room.
func (s *Server) GetRoomClientCount(roomName string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rooms[roomName]
	if !ok {
		return 0
	}
	return len(r.clients)
}

// GetRoomClientNames returns a sorted list of client names in a room.
func (s *Server) GetRoomClientNames(roomName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rooms[roomName]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	// insertion sort
	for i := 1; i < len(names); i++ {
		key := names[i]
		j := i - 1
		for j >= 0 && names[j] > key {
			names[j+1] = names[j]
			j--
		}
		names[j+1] = key
	}
	return names
}

// checkRoomCapacity returns true if the room has space for another client.
func (s *Server) checkRoomCapacity(roomName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rooms[roomName]
	if !ok {
		return true // room doesn't exist yet, will be created with 0 members
	}
	return len(r.clients) < MaxActiveClients
}

// ---------- room-scoped queue management ----------

// admitFromRoomQueue admits the first valid queued client in a specific room.
// No-op during shutdown.
func (s *Server) admitFromRoomQueue(roomName string) {
	if s.IsShuttingDown() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rooms[roomName]
	if !ok {
		return
	}
	for len(r.queue) > 0 {
		entry := r.queue[0]
		r.queue = r.queue[1:]
		if entry.client.IsClosed() {
			continue
		}
		close(entry.admit)
		for i, e := range r.queue {
			e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
		}
		return
	}
}

// removeFromRoomQueue removes the given entry from a room's queue and sends position updates.
func (s *Server) removeFromRoomQueue(roomName string, entry *QueueEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rooms[roomName]
	if !ok {
		return
	}
	for i, e := range r.queue {
		if e == entry {
			r.queue = append(r.queue[:i], r.queue[i+1:]...)
			break
		}
	}
	for i, e := range r.queue {
		e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
	}
}

// RemoveFromQueueByIP removes all queue entries across all rooms whose IP matches
// and returns their clients.
func (s *Server) RemoveFromAllRoomQueuesByIP(ip string) []*client.Client {
	host := extractHost(ip)
	s.mu.Lock()
	defer s.mu.Unlock()

	var removed []*client.Client
	for _, r := range s.rooms {
		var remaining []*QueueEntry
		for _, e := range r.queue {
			if extractHost(e.client.IP) == host {
				removed = append(removed, e.client)
			} else {
				remaining = append(remaining, e)
			}
		}
		if len(removed) > 0 {
			r.queue = remaining
			for i, e := range r.queue {
				e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
			}
		}
	}
	return removed
}
