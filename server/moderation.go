package server

import (
	"net"
	"time"
)

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

// IsIPBlocked reports whether an IP is currently banned or still inside the kick cooldown window.
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
		delete(s.kickedIPs, host) // Expired cooldowns must be cleared or clients stay blocked longer than intended.
	}
	return false, ""
}
