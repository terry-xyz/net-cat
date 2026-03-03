package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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
