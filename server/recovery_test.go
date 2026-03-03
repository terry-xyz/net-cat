package server

// Crash Recovery End-to-End Tests (Task 27)
// Verifies log-based history recovery across multiple server lifecycles within
// a single calendar day. Uses real TCP connections on random ports — same
// infrastructure as integration_test.go.

import (
	"fmt"
	"net-cat/logger"
	"net-cat/models"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ==================== Test 1: Basic Recovery with 3 Clients ====================
// Start server, 3 clients send messages, stop; restart server, new client sees
// all prior messages in history.

func TestRecoveryE2EBasicThreeClients(t *testing.T) {
	tmpDir := t.TempDir()

	// --- First server lifecycle ---
	s1, addr1 := startIntServerInDir(t, tmpDir)

	alice := tcpOnboard(t, addr1, "alice")
	sendLine(alice, "alice", "hello from alice")

	bob := tcpOnboard(t, addr1, "bob")
	readUntil(alice, "bob has joined", 2*time.Second)
	sendLine(bob, "bob", "hello from bob")
	readUntil(alice, "hello from bob", 2*time.Second)

	carol := tcpOnboard(t, addr1, "carol")
	readUntil(alice, "carol has joined", 2*time.Second)
	readUntil(bob, "carol has joined", 2*time.Second)
	sendLine(carol, "carol", "hello from carol")
	readUntil(alice, "hello from carol", 2*time.Second)
	readUntil(bob, "hello from carol", 2*time.Second)

	alice.Close()
	bob.Close()
	carol.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Shutdown()

	// --- Second server lifecycle (same directory) ---
	s2, addr2 := startIntServerInDir(t, tmpDir)
	defer s2.Shutdown()

	dave := tcpDial(t, addr2)
	defer dave.Close()
	readUntil(dave, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(dave, "dave\n")
	readUntil(dave, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(dave, "\n")
	historyText, err := readUntil(dave, "][dave]:", 5*time.Second)
	if err != nil {
		t.Fatalf("dave onboarding failed: %v", err)
	}

	stripped := stripAnsi(historyText)
	if !strings.Contains(stripped, "hello from alice") {
		t.Error("history should contain alice's message")
	}
	if !strings.Contains(stripped, "hello from bob") {
		t.Error("history should contain bob's message")
	}
	if !strings.Contains(stripped, "hello from carol") {
		t.Error("history should contain carol's message")
	}
}

// ==================== Test 2: Three Restarts - History Accumulates ====================
// 3 restarts in one day: history accumulates across all sessions, no duplicates.

func TestRecoveryE2EThreeRestarts(t *testing.T) {
	tmpDir := t.TempDir()

	// Session 1
	s1, addr1 := startIntServerInDir(t, tmpDir)
	c1 := tcpOnboard(t, addr1, "user1")
	sendLine(c1, "user1", "msg-session-1")
	c1.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Shutdown()

	// Session 2
	s2, addr2 := startIntServerInDir(t, tmpDir)
	c2 := tcpOnboard(t, addr2, "user2")
	sendLine(c2, "user2", "msg-session-2")
	c2.Close()
	time.Sleep(200 * time.Millisecond)
	s2.Shutdown()

	// Session 3
	s3, addr3 := startIntServerInDir(t, tmpDir)
	c3 := tcpOnboard(t, addr3, "user3")
	sendLine(c3, "user3", "msg-session-3")
	c3.Close()
	time.Sleep(200 * time.Millisecond)
	s3.Shutdown()

	// Session 4 — verify all messages accumulated with no duplicates
	s4, addr4 := startIntServerInDir(t, tmpDir)
	defer s4.Shutdown()

	verifier := tcpDial(t, addr4)
	defer verifier.Close()
	readUntil(verifier, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(verifier, "verifier\n")
	readUntil(verifier, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(verifier, "\n")
	historyText, err := readUntil(verifier, "][verifier]:", 5*time.Second)
	if err != nil {
		t.Fatalf("verifier onboarding failed: %v", err)
	}

	stripped := stripAnsi(historyText)

	// All 3 session messages must be present
	for _, msg := range []string{"msg-session-1", "msg-session-2", "msg-session-3"} {
		if !strings.Contains(stripped, msg) {
			t.Errorf("history should contain %q", msg)
		}
		if strings.Count(stripped, msg) != 1 {
			t.Errorf("%q should appear exactly once, appeared %d times",
				msg, strings.Count(stripped, msg))
		}
	}
}

// ==================== Test 3: Fully Corrupted Log File ====================
// Server starts with empty history, no crash.

func TestRecoveryE2ECorruptedLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logsDir, 0700)

	// Write a fully corrupted log file for today
	today := logger.FormatDate(time.Now())
	logFile := filepath.Join(logsDir, "chat_"+today+".log")
	corrupt := "this is garbage\nnot a valid log line\n\x00\xff\xfe\nmore junk\n"
	os.WriteFile(logFile, []byte(corrupt), 0600)

	// Start server — must not crash
	s, addr := startIntServerInDir(t, tmpDir)
	defer s.Shutdown()

	// History should be empty
	history := s.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected empty history with corrupted log file, got %d entries", len(history))
	}

	// Server should still function normally
	client := tcpOnboard(t, addr, "testuser")
	defer client.Close()
	sendLine(client, "testuser", "works after corruption")

	// Verify the message was recorded
	history = s.GetHistory()
	found := false
	for _, msg := range history {
		if msg.Content == "works after corruption" {
			found = true
		}
	}
	if !found {
		t.Error("server should function normally after recovering from corrupted log")
	}
}

// ==================== Test 4: Partially Corrupted Log File ====================
// Mix of valid and invalid lines: server recovers valid entries, skips corrupt ones.

func TestRecoveryE2EPartiallyCorruptedLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logsDir, 0700)

	// Build a log file with interleaved valid and corrupt lines
	now := time.Now()
	today := logger.FormatDate(now)
	logFile := filepath.Join(logsDir, "chat_"+today+".log")

	ts := models.FormatTimestamp(now.Add(-1 * time.Hour))
	validLine1 := fmt.Sprintf("[%s] CHAT [alice]:hello world", ts)
	validLine2 := fmt.Sprintf("[%s] JOIN bob", ts)
	validLine3 := fmt.Sprintf("[%s] CHAT [bob]:hi alice", ts)

	content := strings.Join([]string{
		validLine1,
		"CORRUPT LINE HERE",
		validLine2,
		"another bad line",
		validLine3,
		"garbage at end",
	}, "\n") + "\n"

	os.WriteFile(logFile, []byte(content), 0600)

	// Start server
	s, addr := startIntServerInDir(t, tmpDir)
	defer s.Shutdown()

	// Should have recovered 3 valid entries
	history := s.GetHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 recovered history entries, got %d", len(history))
	}

	// Verify via client history delivery
	client := tcpDial(t, addr)
	defer client.Close()
	readUntil(client, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(client, "checker\n")
	readUntil(client, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(client, "\n")
	historyText, err := readUntil(client, "][checker]:", 5*time.Second)
	if err != nil {
		t.Fatalf("checker onboarding failed: %v", err)
	}

	stripped := stripAnsi(historyText)
	if !strings.Contains(stripped, "hello world") {
		t.Error("history should contain alice's recovered message")
	}
	if !strings.Contains(stripped, "bob has joined") {
		t.Error("history should contain bob's recovered join event")
	}
	if !strings.Contains(stripped, "hi alice") {
		t.Error("history should contain bob's recovered message")
	}
}

// ==================== Test 5: Previous Day Log Only ====================
// Log file from a previous day exists but no current-day log: server starts
// with empty history, prior-day file untouched.

func TestRecoveryE2EPreviousDayLogOnly(t *testing.T) {
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	os.MkdirAll(logsDir, 0700)

	// Create a log file for yesterday
	yesterday := time.Now().AddDate(0, 0, -1)
	yesterdayDate := logger.FormatDate(yesterday)
	yesterdayFile := filepath.Join(logsDir, "chat_"+yesterdayDate+".log")
	yesterdayContent := fmt.Sprintf("[%s] CHAT [alice]:yesterday message\n[%s] JOIN bob\n",
		models.FormatTimestamp(yesterday),
		models.FormatTimestamp(yesterday))
	os.WriteFile(yesterdayFile, []byte(yesterdayContent), 0600)

	// Start server
	s, addr := startIntServerInDir(t, tmpDir)
	defer s.Shutdown()

	// History should be empty (no current-day log)
	history := s.GetHistory()
	if len(history) != 0 {
		t.Errorf("expected empty history with only previous-day log, got %d entries", len(history))
	}

	// Yesterday's file must be untouched
	data, err := os.ReadFile(yesterdayFile)
	if err != nil {
		t.Fatalf("yesterday's file should still exist: %v", err)
	}
	if string(data) != yesterdayContent {
		t.Error("yesterday's log file content should be unchanged")
	}

	// Server should function normally with a new day
	client := tcpOnboard(t, addr, "testuser")
	defer client.Close()
	sendLine(client, "testuser", "today message")

	// Today's log file should now exist
	todayDate := logger.FormatDate(time.Now())
	todayFile := filepath.Join(logsDir, "chat_"+todayDate+".log")
	if _, err := os.Stat(todayFile); os.IsNotExist(err) {
		t.Error("today's log file should exist after first activity")
	}
}

// ==================== Test 6: History Includes Join/Leave and Admin Actions ====================
// Recovered history contains join/leave events, kicks, and announcements.

func TestRecoveryE2EJoinLeaveAndAdminActions(t *testing.T) {
	tmpDir := t.TempDir()

	// First server: generate diverse events
	s1, addr1 := startIntServerInDir(t, tmpDir)

	alice := tcpOnboard(t, addr1, "alice")
	bob := tcpOnboard(t, addr1, "bob")
	readUntil(alice, "bob has joined", 2*time.Second)

	// Chat message
	sendLine(alice, "alice", "hello bob")
	readUntil(bob, "hello bob", 2*time.Second)

	// Promote alice and kick bob (admin actions)
	s1.OperatorDispatch("/promote alice")
	readUntil(alice, "promoted", 2*time.Second)

	fmt.Fprintf(alice, "/kick bob\n")
	readUntil(alice, "][alice]:", 2*time.Second)
	time.Sleep(200 * time.Millisecond)
	bob.Close()

	// Clear kick cooldown for future connections from same IP
	s1.mu.Lock()
	delete(s1.kickedIPs, "127.0.0.1")
	s1.mu.Unlock()

	// Carol joins and leaves voluntarily
	carol := tcpOnboard(t, addr1, "carol")
	readUntil(alice, "carol has joined", 2*time.Second)
	fmt.Fprintf(carol, "/quit\n")
	readUntil(alice, "carol has left", 2*time.Second)
	carol.Close()

	// Operator announcement
	s1.OperatorDispatch("/announce test announcement")
	readUntil(alice, "test announcement", 2*time.Second)

	alice.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Shutdown()

	// Second server: verify full recovery
	s2, addr2 := startIntServerInDir(t, tmpDir)
	defer s2.Shutdown()

	verifier := tcpDial(t, addr2)
	defer verifier.Close()
	readUntil(verifier, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(verifier, "verifier\n")
	readUntil(verifier, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(verifier, "\n")
	historyText, err := readUntil(verifier, "][verifier]:", 5*time.Second)
	if err != nil {
		t.Fatalf("verifier onboarding failed: %v", err)
	}

	stripped := stripAnsi(historyText)

	checks := map[string]string{
		"alice has joined":          "alice's join event",
		"bob has joined":            "bob's join event",
		"hello bob":                 "alice's chat message",
		"alice was promoted":        "promotion event",
		"bob was kicked by alice":   "kick event",
		"carol has joined":          "carol's join event",
		"carol has left":            "carol's leave event",
		"test announcement":         "announcement",
	}
	for expected, desc := range checks {
		if !strings.Contains(stripped, expected) {
			t.Errorf("recovered history should contain %s (%q)", desc, expected)
		}
	}
}

// ==================== Test 7: Recovered Timestamps Match Originals ====================
// Recovered history entries are visually identical to live entries.

func TestRecoveryE2ETimestampsMatchOriginals(t *testing.T) {
	tmpDir := t.TempDir()

	// First server: capture live history
	s1, addr1 := startIntServerInDir(t, tmpDir)

	alice := tcpOnboard(t, addr1, "alice")
	sendLine(alice, "alice", "timestamp check")
	alice.Close()
	time.Sleep(200 * time.Millisecond)

	originalHistory := s1.GetHistory()
	s1.Shutdown()

	// Second server: compare recovered history
	s2, _ := startIntServerInDir(t, tmpDir)
	defer s2.Shutdown()

	recoveredHistory := s2.GetHistory()

	// Filter out server events (they are not recovered)
	var origFiltered, recFiltered []models.Message
	for _, m := range originalHistory {
		if m.Type != models.MsgServerEvent {
			origFiltered = append(origFiltered, m)
		}
	}
	for _, m := range recoveredHistory {
		if m.Type != models.MsgServerEvent {
			recFiltered = append(recFiltered, m)
		}
	}

	if len(origFiltered) != len(recFiltered) {
		t.Fatalf("recovered history length (%d) != original (%d)",
			len(recFiltered), len(origFiltered))
	}

	for i := range origFiltered {
		origDisplay := origFiltered[i].Display()
		recDisplay := recFiltered[i].Display()
		if origDisplay != recDisplay {
			t.Errorf("entry %d: recovered %q != original %q", i, recDisplay, origDisplay)
		}
		// Verify second-level timestamp accuracy
		origTS := models.FormatTimestamp(origFiltered[i].Timestamp)
		recTS := models.FormatTimestamp(recFiltered[i].Timestamp)
		if origTS != recTS {
			t.Errorf("entry %d: timestamp %q != original %q", i, recTS, origTS)
		}
	}
}

// ==================== Test 8: Admin Persistence Across Restart ====================
// Admin status from admins.json is restored; returning admin gets privileges back
// and can perform admin actions immediately.

func TestRecoveryE2EAdminRestoredAfterRestart(t *testing.T) {
	tmpDir := t.TempDir()

	// First server: promote alice
	s1, addr1 := startIntServerInDir(t, tmpDir)
	alice := tcpOnboard(t, addr1, "alice")
	bob := tcpOnboard(t, addr1, "bob")
	readUntil(alice, "bob has joined", 2*time.Second)

	s1.OperatorDispatch("/promote alice")
	readUntil(alice, "promoted", 2*time.Second)

	alice.Close()
	bob.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Shutdown()

	// Verify admins.json persisted
	adminsFile := filepath.Join(tmpDir, "admins.json")
	data, err := os.ReadFile(adminsFile)
	if err != nil {
		t.Fatalf("admins.json should exist: %v", err)
	}
	if !strings.Contains(string(data), "alice") {
		t.Fatalf("admins.json should contain 'alice', got: %s", string(data))
	}

	// Second server: alice reconnects and should auto-restore admin
	s2, addr2 := startIntServerInDir(t, tmpDir)
	defer s2.Shutdown()

	alice2 := tcpDial(t, addr2)
	defer alice2.Close()
	readUntil(alice2, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(alice2, "alice\n")
	// Admin greeting comes before room selection
	text, _ := readUntil(alice2, "[ENTER ROOM NAME]", 5*time.Second)
	stripped := stripAnsi(text)

	// Admin welcome-back greeting
	if !strings.Contains(stripped, "Welcome back") || !strings.Contains(stripped, "admin") {
		t.Errorf("alice should be greeted as returning admin, got: %q", stripped)
	}
	fmt.Fprintf(alice2, "\n")
	readUntil(alice2, "][alice]:", 5*time.Second)

	// Verify alice can use admin commands (kick)
	target := tcpOnboard(t, addr2, "target")
	defer target.Close()
	readUntil(alice2, "target has joined", 2*time.Second)

	fmt.Fprintf(alice2, "/kick target\n")
	kickText, _ := readUntil(alice2, "][alice]:", 2*time.Second)
	if strings.Contains(stripAnsi(kickText), "Insufficient") {
		t.Error("restored admin alice should be able to kick")
	}

	time.Sleep(200 * time.Millisecond)
	if s2.GetClient("target") != nil {
		t.Error("target should be removed after kick by restored admin")
	}
}
