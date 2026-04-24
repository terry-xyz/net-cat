package server

import (
	"errors"
	"github.com/terry-xyz/net-cat/client"
	"time"
)

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

// RegisterClient atomically checks uniqueness and adds the client to the global map.
// Room assignment happens separately via JoinRoom. Returns nil on success or a
// RegisterClient reserves a username globally so rooms, whispers, and moderation can all resolve the same client.
func (s *Server) RegisterClient(c *client.Client, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// RemoveClient removes a user from the global and room-local indexes without closing the underlying connection.
func (s *Server) RemoveClient(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[username]
	if ok {
		if c.Room != "" {
			if r, rOk := s.rooms[c.Room]; rOk {
				delete(r.clients, username)
			}
		}
		delete(s.clients, username)
	}
}

// GetClient returns the active client for the given username, or nil if the user is not connected.
func (s *Server) GetClient(name string) *client.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[name]
}

// GetClientCount returns the number of actively registered users across all rooms.
func (s *Server) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// GetClientNames returns all active usernames in sorted order for stable command output.
func (s *Server) GetClientNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.clients))
	for n := range s.clients {
		names = append(names, n)
	}
	return names
}

// IsReservedName reports whether a username is blocked from client registration.
func (s *Server) IsReservedName(name string) bool {
	return s.reservedNames[name]
}

// RenameClient atomically swaps the key in the client and room maps.
// RenameClient updates the global and room-local name indexes without disconnecting the client.
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
	// Update room's client map
	if c.Room != "" {
		if r, ok := s.rooms[c.Room]; ok {
			delete(r.clients, oldName)
			r.clients[newName] = c
		}
	}
	return true
}

// GetClientsByIP returns all registered clients whose IP matches the given host.
// Pass exclude to skip a specific username (e.g. the command issuer).
// GetClientsByIP returns connected clients whose host matches the supplied IP, excluding one username when needed.
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
