package server

import (
	"bufio"
	"fmt"
	"net-cat/logger"
	"net-cat/models"
	"os"
	"time"
)

// ---------- history ----------

// ClearHistory removes all in-memory history entries across all rooms.
// Called at midnight to reset history for the new calendar day.
func (s *Server) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rooms {
		r.history = r.history[:0]
	}
}

// AddHistory appends a message to the appropriate room's history.
// If msg.Room is empty, uses DefaultRoom.
func (s *Server) AddHistory(msg models.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	roomName := msg.Room
	if roomName == "" {
		roomName = s.DefaultRoom
	}
	r := s.getOrCreateRoom(roomName)
	r.history = append(r.history, msg)
}

// GetHistory returns a combined copy of all room histories for backward compatibility.
func (s *Server) GetHistory() []models.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var all []models.Message
	for _, r := range s.rooms {
		all = append(all, r.history...)
	}
	return all
}

// RecoverHistory loads today's log file and reconstructs the in-memory history.
// Only called on startup so that clients connecting after a restart see prior events.
// Server events are excluded (not user-visible). Corrupt lines are skipped with warnings.
// Messages are routed to the correct room based on the @room tag; old lines without
// a room tag are assigned to the DefaultRoom.
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
		// Route to correct room; old logs without @room go to DefaultRoom
		roomName := msg.Room
		if roomName == "" {
			roomName = s.DefaultRoom
			msg.Room = roomName
		}
		r := s.getOrCreateRoom(roomName)
		r.history = append(r.history, msg)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error reading log file: %v\n", err)
	}

	if corrupt > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %d corrupt line(s) skipped during history recovery\n", corrupt)
	}
}
