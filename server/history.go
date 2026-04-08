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

// ClearHistory resets the in-memory server history while leaving the on-disk logs untouched.
func (s *Server) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rooms {
		r.history = r.history[:0]
	}
}

// AddHistory appends a message to the in-memory history snapshot used for reconnect recovery.
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

// RecoverHistory rebuilds the current-day in-memory history from the latest log file when the server starts.
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
		// Old logs may not have a room tag, so default them into the room that existed before multi-room support.
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
