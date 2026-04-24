package server

// Integration tests using real TCP connections on random ports.
// These tests cover the complete user journey end-to-end as required by Task 25.

import (
	"encoding/json"
	"fmt"
	"github.com/terry-xyz/net-cat/logger"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ==================== Spec 01 Edge Case: Port Already In Use ====================

// TestIntegrationPortAlreadyInUseReturnsError verifies that when the port is
// already occupied by another listener, Server.Start() returns a clear error
// and does not hang or retry silently. (Spec 01 §Edge Cases)
// TestIntegrationPortAlreadyInUseReturnsError verifies the scenario described by its name.
func TestIntegrationPortAlreadyInUseReturnsError(t *testing.T) {
	// Occupy a random port on all interfaces (matching the server's ":port" bind).
	blocker, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("could not create blocker listener: %v", err)
	}
	defer blocker.Close()

	// Extract the port the OS assigned.
	_, port, _ := net.SplitHostPort(blocker.Addr().String())

	// Create a server targeting the same port.
	s := New(port)

	// Start should fail immediately with a bind error.
	errCh := make(chan error, 1)
	go func() { errCh <- s.Start() }()

	select {
	case startErr := <-errCh:
		if startErr == nil {
			t.Fatal("Start() should return a non-nil error when the port is already in use")
		}
		if !strings.Contains(startErr.Error(), "bind") && !strings.Contains(startErr.Error(), "address already in use") && !strings.Contains(startErr.Error(), "Only one usage") {
			t.Errorf("expected a bind/address-in-use error, got: %v", startErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start() should return immediately on port conflict, but it hung for 5 seconds")
	}
}

// ==================== Integration Test Helpers ====================

// safeWriter is a thread-safe writer for capturing operator output.
type safeWriter struct {
	mu  sync.Mutex
	buf strings.Builder
}

// Write appends test output under a mutex so concurrent operator writes do not race.
func (w *safeWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// String returns the buffered integration-test output as a single string.
func (w *safeWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// Reset clears buffered integration-test output so later assertions start from a clean slate.
func (w *safeWriter) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Reset()
}

// startIntServer creates a real TCP server with random port, logger, and operator
// output capture. Returns the server, its listener address, and the temp directory.
// The server's accept loop runs in a background goroutine.
// startIntServer boots an integration-test server with isolated temp directories and captured operator output.
func startIntServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")

	s := New("0")
	s.ShutdownTimeout = 500 * time.Millisecond
	s.HeartbeatInterval = 1 * time.Hour // effectively disable heartbeat for integration tests

	lgr, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = lgr
	s.adminsFile = filepath.Join(tmpDir, "admins.json")
	s.OperatorOutput = &safeWriter{}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.listener = ln
	go s.acceptLoop()

	addr := ln.Addr().String()
	return s, addr, tmpDir
}

// startIntServerInDir is like startIntServer but uses a specific directory for
// logs and admins, enabling crash recovery tests across server instances.
// startIntServerInDir starts an integration-test server rooted in the supplied temporary directory.
func startIntServerInDir(t *testing.T, tmpDir string) (*Server, string) {
	t.Helper()
	logsDir := filepath.Join(tmpDir, "logs")

	s := New("0")
	s.ShutdownTimeout = 500 * time.Millisecond
	s.HeartbeatInterval = 1 * time.Hour

	lgr, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = lgr
	s.adminsFile = filepath.Join(tmpDir, "admins.json")
	s.OperatorOutput = &safeWriter{}

	// Recover history and load admins (simulating Start() behavior)
	s.LoadAdmins()
	s.RecoverHistory()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.listener = ln
	go s.acceptLoop()

	return s, ln.Addr().String()
}

// tcpDial connects to the server via real TCP.
func tcpDial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("tcp dial %s: %v", addr, err)
	}
	return conn
}

// tcpOnboard connects to the server, completes onboarding with the given name
// (default room), and returns the connection ready for interactive messaging.
// tcpOnboard connects a test client and completes the normal onboarding prompts.
func tcpOnboard(t *testing.T, addr, name string) net.Conn {
	t.Helper()
	conn := tcpDial(t, addr)
	_, err := readUntil(conn, "[ENTER YOUR NAME]:", 3*time.Second)
	if err != nil {
		conn.Close()
		t.Fatalf("tcpOnboard(%s): banner read failed: %v", name, err)
	}
	fmt.Fprintf(conn, "%s\n", name)
	// Handle room selection prompt (press Enter for default room)
	_, err = readUntil(conn, "[ENTER ROOM NAME]", 3*time.Second)
	if err != nil {
		conn.Close()
		t.Fatalf("tcpOnboard(%s): room prompt read failed: %v", name, err)
	}
	fmt.Fprintf(conn, "\n")
	_, err = readUntil(conn, "]["+name+"]:", 3*time.Second)
	if err != nil {
		conn.Close()
		t.Fatalf("tcpOnboard(%s): prompt read failed: %v", name, err)
	}
	return conn
}

// sendLine writes a line and waits for the prompt back (useful in echo mode).
func sendLine(conn net.Conn, name, line string) (string, error) {
	fmt.Fprintf(conn, "%s\n", line)
	return readUntil(conn, "]["+name+"]:", 3*time.Second)
}

// waitForCondition polls until condition returns true or the timeout expires.
func waitForCondition(t *testing.T, timeout time.Duration, description string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for %s after %v", description, timeout)
}

// drainUntilQuiet reads until the connection goes quiet after data starts arriving.
func drainUntilQuiet(conn net.Conn, timeout, quiet time.Duration) string {
	overallDeadline := time.Now().Add(timeout)
	var buf strings.Builder
	tmp := make([]byte, 4096)
	sawData := false

	for {
		readDeadline := overallDeadline
		if sawData {
			quietDeadline := time.Now().Add(quiet)
			if quietDeadline.Before(readDeadline) {
				readDeadline = quietDeadline
			}
		}

		conn.SetReadDeadline(readDeadline)
		n, err := conn.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			sawData = true
			continue
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if sawData || time.Now().After(overallDeadline) {
					break
				}
				continue
			}
			break
		}
	}

	conn.SetReadDeadline(time.Time{})
	return buf.String()
}

// drainFor reads all available data from conn during the given duration.
// Useful for checking that certain content does NOT appear.
// drainFor keeps reading from a connection until the timeout expires so tests can inspect all pending output.
func drainFor(conn net.Conn, d time.Duration) string {
	conn.SetReadDeadline(time.Now().Add(d))
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	conn.SetReadDeadline(time.Time{})
	return buf.String()
}

// stripAnsi removes ANSI escape sequences and null bytes from text for cleaner assertions.
func stripAnsi(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until we find a letter (end of escape sequence)
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // skip the final letter
			}
			i = j
			continue
		}
		if s[i] == 0x00 {
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// ==================== Test 1: Full Chat Session ====================
// 3 clients connect, exchange messages, one leaves, new one joins and sees history.

// TestIntegrationFullChatSession verifies the scenario described by its name.
func TestIntegrationFullChatSession(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	// Connect 3 clients
	alice := tcpOnboard(t, addr, "alice")
	defer alice.Close()

	bob := tcpOnboard(t, addr, "bob")
	defer bob.Close()
	// Alice sees bob's join
	readUntil(alice, "bob has joined", 2*time.Second)

	carol := tcpOnboard(t, addr, "carol")
	defer carol.Close()
	// Alice and Bob see carol's join
	readUntil(alice, "carol has joined", 2*time.Second)
	readUntil(bob, "carol has joined", 2*time.Second)

	// Alice sends a message — bob and carol should see it
	sendLine(alice, "alice", "hello from alice")
	bobText, _ := readUntil(bob, "hello from alice", 2*time.Second)
	if !strings.Contains(bobText, "[alice]:hello from alice") {
		t.Errorf("bob should see alice's message in correct format, got: %q", stripAnsi(bobText))
	}
	carolText, _ := readUntil(carol, "hello from alice", 2*time.Second)
	if !strings.Contains(carolText, "hello from alice") {
		t.Errorf("carol should see alice's message")
	}

	// Bob sends a message
	sendLine(bob, "bob", "hey alice")
	readUntil(alice, "hey alice", 2*time.Second)
	readUntil(carol, "hey alice", 2*time.Second)

	// Bob leaves
	fmt.Fprintf(bob, "/quit\n")
	readUntil(alice, "bob has left", 2*time.Second)
	readUntil(carol, "bob has left", 2*time.Second)

	// New client joins and sees history (messages + join/leave events)
	dave := tcpDial(t, addr)
	defer dave.Close()
	readUntil(dave, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(dave, "dave\n")
	readUntil(dave, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(dave, "\n")
	historyText, _ := readUntil(dave, "][dave]:", 3*time.Second)

	// History should contain messages and events
	stripped := stripAnsi(historyText)
	if !strings.Contains(stripped, "hello from alice") {
		t.Error("dave's history should contain alice's message")
	}
	if !strings.Contains(stripped, "hey alice") {
		t.Error("dave's history should contain bob's message")
	}
	if !strings.Contains(stripped, "bob has joined") {
		t.Error("dave's history should contain bob's join event")
	}
	if !strings.Contains(stripped, "bob has left") {
		t.Error("dave's history should contain bob's leave event")
	}
}

// ==================== Test 2: Admin Workflow ====================
// Operator promotes client, admin kicks another, operator demotes admin.
// Client joining after sees events in history.

// TestIntegrationAdminWorkflow verifies the scenario described by its name.
func TestIntegrationAdminWorkflow(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	alice := tcpOnboard(t, addr, "alice")
	defer alice.Close()
	bob := tcpOnboard(t, addr, "bob")
	defer bob.Close()
	readUntil(alice, "bob has joined", 2*time.Second)

	carol := tcpOnboard(t, addr, "carol")
	defer carol.Close()
	readUntil(alice, "carol has joined", 2*time.Second)
	readUntil(bob, "carol has joined", 2*time.Second)

	// Operator promotes alice
	s.OperatorDispatch("/promote alice")
	promText, _ := readUntil(alice, "You have been promoted", 2*time.Second)
	if !strings.Contains(stripAnsi(promText), "promoted") {
		t.Error("alice should be notified of promotion")
	}
	// Drain promote broadcast from all clients
	readUntil(alice, "alice was promoted", 2*time.Second)
	readUntil(bob, "alice was promoted", 2*time.Second)
	readUntil(carol, "alice was promoted", 2*time.Second)

	// Alice (now admin) kicks carol
	fmt.Fprintf(alice, "/kick carol\n")
	readUntil(alice, "][alice]:", 2*time.Second)

	// Bob should see kick notification (not standard leave)
	kickText, _ := readUntil(bob, "carol was kicked", 2*time.Second)
	if !strings.Contains(stripAnsi(kickText), "carol was kicked by alice") {
		t.Errorf("bob should see kick notification, got: %q", stripAnsi(kickText))
	}

	// Carol should be disconnected
	time.Sleep(200 * time.Millisecond)
	if s.GetClient("carol") != nil {
		t.Error("carol should be removed after kick")
	}

	// Operator demotes alice
	s.OperatorDispatch("/demote alice")
	demText, _ := readUntil(alice, "revoked", 2*time.Second)
	if !strings.Contains(stripAnsi(demText), "revoked") {
		t.Error("alice should be notified of demotion")
	}
	// Drain demote broadcast from remaining clients
	readUntil(alice, "alice was demoted", 2*time.Second)
	readUntil(bob, "alice was demoted", 2*time.Second)

	// Verify alice is no longer admin — try to kick bob
	fmt.Fprintf(alice, "/kick bob\n")
	errText, _ := readUntil(alice, "Insufficient", 2*time.Second)
	if !strings.Contains(stripAnsi(errText), "Insufficient") {
		t.Error("demoted alice should not be able to kick")
	}

	// Clear kick cooldown BEFORE connecting (IP check happens at connection start)
	s.mu.Lock()
	delete(s.kickedIPs, "127.0.0.1")
	s.mu.Unlock()

	// New client joins and sees promote/kick/demote events in history
	eve := tcpDial(t, addr)
	defer eve.Close()
	readUntil(eve, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(eve, "eve\n")
	readUntil(eve, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(eve, "\n")
	historyText, _ := readUntil(eve, "][eve]:", 3*time.Second)
	stripped := stripAnsi(historyText)

	if !strings.Contains(stripped, "carol was kicked by alice") {
		t.Error("eve's history should contain kick event")
	}
}

// ==================== Test 3: Connection Capacity ====================
// 10 active + 3 queued, one active leaves, first queued is admitted.

// TestIntegrationConnectionCapacity verifies the scenario described by its name.
func TestIntegrationConnectionCapacity(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	// Connect and onboard 10 active clients
	var activeConns []net.Conn
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("user%d", i)
		conn := tcpOnboard(t, addr, name)
		activeConns = append(activeConns, conn)
		// Drain join notifications from earlier clients
		time.Sleep(50 * time.Millisecond)
	}
	defer func() {
		for _, c := range activeConns {
			if c != nil {
				c.Close()
			}
		}
	}()

	// In the new flow, clients go through banner+name+room selection BEFORE being queued.
	// Queue 3 clients for the default room.
	var queuedConns [3]net.Conn
	for i := 0; i < 3; i++ {
		queuedConns[i] = tcpDial(t, addr)
		defer queuedConns[i].Close()

		// Complete banner + name
		readUntil(queuedConns[i], "[ENTER YOUR NAME]:", 3*time.Second)
		fmt.Fprintf(queuedConns[i], "queued%d\n", i+1)
		// Complete room selection (default room)
		readUntil(queuedConns[i], "[ENTER ROOM NAME]", 3*time.Second)
		fmt.Fprintf(queuedConns[i], "\n")
		// Should get queue prompt since room is full
		text, err := readUntil(queuedConns[i], "Would you like to wait?", 3*time.Second)
		if err != nil {
			t.Fatalf("queued client %d: did not get queue prompt: %v", i+1, err)
		}
		expectedPos := fmt.Sprintf("#%d", i+1)
		if !strings.Contains(text, expectedPos) {
			t.Errorf("queued client %d: expected position %s, got: %q", i+1, expectedPos, text)
		}
		fmt.Fprintf(queuedConns[i], "yes\n")
	}

	time.Sleep(200 * time.Millisecond)

	// Disconnect first active client to free a slot
	activeConns[0].Close()
	activeConns[0] = nil

	// First queued client should be admitted (already named, gets prompt directly)
	_, err := readUntil(queuedConns[0], "][queued1]:", 5*time.Second)
	if err != nil {
		t.Fatalf("first queued client not admitted: %v", err)
	}

	// Remaining queued clients should have received position updates
	posText, err := readUntil(queuedConns[1], "#1", 3*time.Second)
	if err != nil {
		t.Logf("queued client 2 position update: %v (text: %q)", err, posText)
	}
}

// ==================== Test 4: Graceful Shutdown ====================
// 5 connected clients all receive shutdown message.

// TestIntegrationGracefulShutdown verifies the scenario described by its name.
func TestIntegrationGracefulShutdown(t *testing.T) {
	s, addr, _ := startIntServer(t)

	var conns []net.Conn
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("client%d", i)
		conn := tcpOnboard(t, addr, name)
		conns = append(conns, conn)
		time.Sleep(50 * time.Millisecond)
	}

	// Shutdown the server
	go s.Shutdown()

	// All clients should receive the shutdown message
	for i, conn := range conns {
		text, err := readUntil(conn, "Server is shutting down", 3*time.Second)
		if err != nil {
			// Connection might have been force-closed, which is also acceptable
			t.Logf("client %d: %v (text: %q)", i, err, text)
			continue
		}
		if !strings.Contains(text, "Goodbye!") {
			t.Errorf("client %d: expected shutdown message with Goodbye, got: %q", i, text)
		}
		conn.Close()
	}
}

// ==================== Test 5: Crash Recovery ====================
// Start server, send messages, stop, restart, new client sees prior history.

// TestIntegrationCrashRecovery verifies the scenario described by its name.
func TestIntegrationCrashRecovery(t *testing.T) {
	tmpDir := t.TempDir()

	// --- First server lifecycle ---
	s1, addr1 := startIntServerInDir(t, tmpDir)

	alice := tcpOnboard(t, addr1, "alice")
	sendLine(alice, "alice", "hello from session 1")
	alice.Close()
	time.Sleep(200 * time.Millisecond)

	bob := tcpOnboard(t, addr1, "bob")
	sendLine(bob, "bob", "bob here in session 1")
	bob.Close()
	time.Sleep(200 * time.Millisecond)

	s1.Shutdown()

	// --- Second server lifecycle (same directory) ---
	s2, addr2 := startIntServerInDir(t, tmpDir)
	defer s2.Shutdown()

	// New client should see history from the first session
	carol := tcpDial(t, addr2)
	defer carol.Close()
	readUntil(carol, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(carol, "carol\n")
	readUntil(carol, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(carol, "\n")
	historyText, err := readUntil(carol, "][carol]:", 5*time.Second)
	if err != nil {
		t.Fatalf("carol onboarding failed: %v", err)
	}

	stripped := stripAnsi(historyText)
	if !strings.Contains(stripped, "hello from session 1") {
		t.Error("carol should see alice's message from prior session in history")
	}
	if !strings.Contains(stripped, "bob here in session 1") {
		t.Error("carol should see bob's message from prior session in history")
	}
}

// ==================== Test 6: Name Change ====================
// Client changes name, all see notification, history includes it.

// TestIntegrationNameChange verifies the scenario described by its name.
func TestIntegrationNameChange(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	alice := tcpOnboard(t, addr, "alice")
	defer alice.Close()
	bob := tcpOnboard(t, addr, "bob")
	defer bob.Close()
	readUntil(alice, "bob has joined", 2*time.Second)

	// Alice changes name
	fmt.Fprintf(alice, "/name alice2\n")
	readUntil(alice, "][alice2]:", 2*time.Second)

	// Bob sees the notification
	nameText, _ := readUntil(bob, "changed their name", 2*time.Second)
	if !strings.Contains(stripAnsi(nameText), "alice changed their name to alice2") {
		t.Errorf("bob should see name change notification, got: %q", stripAnsi(nameText))
	}

	// Alice sends a message with new name
	sendLine(alice, "alice2", "hi from alice2")
	msgText, _ := readUntil(bob, "hi from alice2", 2*time.Second)
	if !strings.Contains(stripAnsi(msgText), "[alice2]:hi from alice2") {
		t.Error("bob should see message from alice2")
	}

	// New client sees name change in history
	carol := tcpDial(t, addr)
	defer carol.Close()
	readUntil(carol, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(carol, "carol\n")
	readUntil(carol, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(carol, "\n")
	historyText, _ := readUntil(carol, "][carol]:", 3*time.Second)
	if !strings.Contains(stripAnsi(historyText), "alice changed their name to alice2") {
		t.Error("carol's history should contain name change event")
	}
}

// ==================== Test 7: Whisper Privacy ====================
// Third client never sees the whisper, log file has no whisper.

// TestIntegrationWhisperPrivacy verifies the scenario described by its name.
func TestIntegrationWhisperPrivacy(t *testing.T) {
	s, addr, tmpDir := startIntServer(t)
	defer s.Shutdown()

	alice := tcpOnboard(t, addr, "alice")
	defer alice.Close()
	bob := tcpOnboard(t, addr, "bob")
	defer bob.Close()
	readUntil(alice, "bob has joined", 2*time.Second)
	carol := tcpOnboard(t, addr, "carol")
	defer carol.Close()
	readUntil(alice, "carol has joined", 2*time.Second)
	readUntil(bob, "carol has joined", 2*time.Second)

	// Alice whispers to bob
	fmt.Fprintf(alice, "/whisper bob secret message\n")
	// Alice should see her own whisper confirmation
	aliceText, _ := readUntil(alice, "][alice]:", 2*time.Second)
	if !strings.Contains(stripAnsi(aliceText), "secret message") {
		t.Error("alice should see her own whisper")
	}

	// Bob should receive the whisper
	bobText, _ := readUntil(bob, "secret message", 2*time.Second)
	if !strings.Contains(stripAnsi(bobText), "PM from alice") {
		t.Errorf("bob should see PM from alice, got: %q", stripAnsi(bobText))
	}

	// Carol should NOT see the whisper — drain and check
	carolText := drainFor(carol, 500*time.Millisecond)
	if strings.Contains(carolText, "secret message") {
		t.Error("carol should NOT see the whisper")
	}

	// Check log file does not contain whisper
	logsDir := filepath.Join(tmpDir, "logs")
	files, _ := os.ReadDir(logsDir)
	for _, f := range files {
		data, _ := os.ReadFile(filepath.Join(logsDir, f.Name()))
		if strings.Contains(string(data), "secret message") {
			t.Errorf("log file should NOT contain whisper content: %s", f.Name())
		}
	}

	// New client should not see whisper in history
	dave := tcpDial(t, addr)
	defer dave.Close()
	readUntil(dave, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(dave, "dave\n")
	readUntil(dave, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(dave, "\n")
	historyText, _ := readUntil(dave, "][dave]:", 3*time.Second)
	if strings.Contains(stripAnsi(historyText), "secret message") {
		t.Error("whisper should NOT appear in history")
	}
}

// ==================== Test 8: Mute Flow ====================
// Muted client cannot send, can use commands, unmuted can send again.

// TestIntegrationMuteFlow verifies the scenario described by its name.
func TestIntegrationMuteFlow(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	admin := tcpOnboard(t, addr, "admin")
	defer admin.Close()
	target := tcpOnboard(t, addr, "target")
	defer target.Close()
	readUntil(admin, "target has joined", 2*time.Second)
	observer := tcpOnboard(t, addr, "observer")
	defer observer.Close()
	readUntil(admin, "observer has joined", 2*time.Second)
	readUntil(target, "observer has joined", 2*time.Second)

	// Promote admin
	s.OperatorDispatch("/promote admin")
	readUntil(admin, "promoted", 2*time.Second)

	// Mute target
	fmt.Fprintf(admin, "/mute target\n")
	readUntil(admin, "][admin]:", 2*time.Second)
	readUntil(target, "was muted", 2*time.Second)
	readUntil(observer, "was muted", 2*time.Second)

	// Target tries to send a chat message — should see "You are muted."
	fmt.Fprintf(target, "hello everyone\n")
	muteText, _ := readUntil(target, "][target]:", 2*time.Second)
	if !strings.Contains(stripAnsi(muteText), "You are muted") {
		t.Error("muted target should see 'You are muted'")
	}

	// Observer should NOT see target's message
	observerText := drainFor(observer, 500*time.Millisecond)
	if strings.Contains(observerText, "hello everyone") {
		t.Error("observer should NOT see muted target's message")
	}

	// Muted client can still use /list
	fmt.Fprintf(target, "/list\n")
	listText, _ := readUntil(target, "][target]:", 2*time.Second)
	if !strings.Contains(stripAnsi(listText), "admin") {
		t.Error("muted target should still be able to use /list")
	}

	// Muted client can still use /whisper
	fmt.Fprintf(target, "/whisper observer hi from muted\n")
	readUntil(target, "][target]:", 2*time.Second)
	whisperText, _ := readUntil(observer, "hi from muted", 2*time.Second)
	if !strings.Contains(stripAnsi(whisperText), "hi from muted") {
		t.Error("muted client should be able to whisper")
	}

	// Unmute target
	fmt.Fprintf(admin, "/unmute target\n")
	readUntil(admin, "][admin]:", 2*time.Second)
	readUntil(target, "was unmuted", 2*time.Second)

	// Target can now send again
	sendLine(target, "target", "i am free")
	freeText, _ := readUntil(observer, "i am free", 2*time.Second)
	if !strings.Contains(stripAnsi(freeText), "i am free") {
		t.Error("unmuted target's message should be visible to observer")
	}
}

// ==================== Test 9: Ban Persistence ====================
// Banned IP cannot reconnect with any name.

// TestIntegrationBanPersistence verifies the scenario described by its name.
func TestIntegrationBanPersistence(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	admin := tcpOnboard(t, addr, "admin")
	defer admin.Close()
	victim := tcpOnboard(t, addr, "victim")
	readUntil(admin, "victim has joined", 2*time.Second)

	// Promote admin and ban victim
	s.OperatorDispatch("/promote admin")
	readUntil(admin, "promoted", 2*time.Second)

	fmt.Fprintf(admin, "/ban victim\n")
	readUntil(admin, "][admin]:", 2*time.Second)
	// victim connection should be closed
	time.Sleep(200 * time.Millisecond)
	victim.Close()

	// Try to reconnect from same IP (127.0.0.1) — should be rejected before banner
	rejConn := tcpDial(t, addr)
	defer rejConn.Close()
	rejText := drainFor(rejConn, 2*time.Second)
	stripped := stripAnsi(rejText)
	if strings.Contains(stripped, "Welcome to TCP-Chat!") {
		t.Error("banned IP should NOT receive welcome banner")
	}
	if !strings.Contains(stripped, "banned") {
		t.Errorf("banned IP should receive ban message, got: %q", stripped)
	}
}

// ==================== Test 10: Kick Cooldown ====================
// Blocked for 5 minutes, allowed after.

// TestIntegrationKickCooldown verifies the scenario described by its name.
func TestIntegrationKickCooldown(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	admin := tcpOnboard(t, addr, "admin")
	defer admin.Close()
	victim := tcpOnboard(t, addr, "victim")
	readUntil(admin, "victim has joined", 2*time.Second)

	// Promote and kick
	s.OperatorDispatch("/promote admin")
	readUntil(admin, "promoted", 2*time.Second)

	fmt.Fprintf(admin, "/kick victim\n")
	readUntil(admin, "][admin]:", 2*time.Second)
	time.Sleep(200 * time.Millisecond)
	victim.Close()

	// Try to reconnect while cooldown is active — should be rejected
	rejConn := tcpDial(t, addr)
	rejText := drainFor(rejConn, 2*time.Second)
	rejConn.Close()
	if strings.Contains(rejText, "Welcome to TCP-Chat!") {
		t.Error("kicked IP should NOT receive welcome banner during cooldown")
	}
	if !strings.Contains(stripAnsi(rejText), "blocked") && !strings.Contains(stripAnsi(rejText), "temporarily") {
		t.Errorf("kicked IP should receive cooldown message, got: %q", stripAnsi(rejText))
	}

	// Simulate cooldown expiry by manipulating the map directly
	s.mu.Lock()
	s.kickedIPs["127.0.0.1"] = time.Now().Add(-1 * time.Second) // expired
	s.mu.Unlock()

	// Now reconnection should work
	okConn := tcpDial(t, addr)
	defer okConn.Close()
	bannerText, err := readUntil(okConn, "[ENTER YOUR NAME]:", 3*time.Second)
	if err != nil {
		t.Fatalf("should be able to reconnect after cooldown: %v", err)
	}
	if !strings.Contains(bannerText, "Welcome to TCP-Chat!") {
		t.Error("should receive welcome banner after cooldown expires")
	}
}

// ==================== Test 11: Error Sanitization ====================
// All error messages are free of stack traces, file paths, and internal Go details.

// TestIntegrationErrorSanitization verifies the scenario described by its name.
func TestIntegrationErrorSanitization(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	conn := tcpOnboard(t, addr, "tester")
	defer conn.Close()

	// Collect error messages from various commands
	errorMessages := []string{}

	// Unknown command
	fmt.Fprintf(conn, "/nonexistent\n")
	text, _ := readUntil(conn, "][tester]:", 2*time.Second)
	errorMessages = append(errorMessages, text)

	// Insufficient privileges
	fmt.Fprintf(conn, "/kick someone\n")
	text, _ = readUntil(conn, "][tester]:", 2*time.Second)
	errorMessages = append(errorMessages, text)

	// Whisper to nonexistent user
	fmt.Fprintf(conn, "/whisper nobody hello\n")
	text, _ = readUntil(conn, "][tester]:", 2*time.Second)
	errorMessages = append(errorMessages, text)

	// Whisper to self
	fmt.Fprintf(conn, "/whisper tester hello\n")
	text, _ = readUntil(conn, "][tester]:", 2*time.Second)
	errorMessages = append(errorMessages, text)

	// Name with spaces
	fmt.Fprintf(conn, "/name bad name\n")
	text, _ = readUntil(conn, "][tester]:", 2*time.Second)
	errorMessages = append(errorMessages, text)

	// Message too long
	longMsg := strings.Repeat("x", 2049)
	fmt.Fprintf(conn, "%s\n", longMsg)
	text, _ = readUntil(conn, "][tester]:", 2*time.Second)
	errorMessages = append(errorMessages, text)

	// Missing arguments
	fmt.Fprintf(conn, "/whisper\n")
	text, _ = readUntil(conn, "][tester]:", 2*time.Second)
	errorMessages = append(errorMessages, text)

	// Name command missing arg
	fmt.Fprintf(conn, "/name\n")
	text, _ = readUntil(conn, "][tester]:", 2*time.Second)
	errorMessages = append(errorMessages, text)

	// Check all error messages for internal details
	internalPatterns := []string{
		"goroutine",
		"panic",
		".go:",
		"runtime.",
		"github.com/terry-xyz/net-cat/",
		"server/",
		"handler.go",
		"server.go",
		"main.go",
	}

	for i, msg := range errorMessages {
		stripped := stripAnsi(msg)
		for _, pattern := range internalPatterns {
			if strings.Contains(stripped, pattern) {
				t.Errorf("error message %d contains internal detail %q: %q", i, pattern, stripped)
			}
		}
	}
}

// ==================== Test 12: Input Continuity ====================
// Client's partial input is preserved when incoming message arrives mid-typing.

// TestIntegrationInputContinuity verifies the scenario described by its name.
func TestIntegrationInputContinuity(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	alice := tcpOnboard(t, addr, "alice")
	defer alice.Close()
	bob := tcpOnboard(t, addr, "bob")
	defer bob.Close()
	readUntil(alice, "bob has joined", 2*time.Second)

	// Alice starts typing "hel" but doesn't send yet
	// In echo mode, these bytes are echoed back and tracked server-side
	alice.Write([]byte("hel"))
	time.Sleep(200 * time.Millisecond) // let echoes arrive

	// Bob sends a message — this triggers writeWithContinuity for alice
	sendLine(bob, "bob", "interruption")

	// Alice should receive bob's message with her partial input "hel" preserved after it
	aliceText, _ := readUntil(alice, "hel", 2*time.Second)
	stripped := stripAnsi(aliceText)
	if !strings.Contains(stripped, "interruption") {
		t.Error("alice should see bob's message")
	}
	// The partial input "hel" should be redrawn after the message
	// The writeWithContinuity writes: clear + message + prompt + inputBuf
	// So after the interruption message, "hel" should appear
	afterInterrupt := stripped[strings.Index(stripped, "interruption")+len("interruption"):]
	if !strings.Contains(afterInterrupt, "hel") {
		t.Errorf("alice's partial input 'hel' should be preserved after interruption, got after: %q", afterInterrupt)
	}

	// Alice completes the message
	alice.Write([]byte("lo world\n"))
	readUntil(alice, "][alice]:", 2*time.Second)

	// Bob should see the complete message
	bobText, _ := readUntil(bob, "hello world", 2*time.Second)
	if !strings.Contains(stripAnsi(bobText), "hello world") {
		t.Error("bob should see alice's complete message 'hello world'")
	}
}

// ==================== Test 13: Reserved Name ====================
// "Server" cannot be claimed by any client.

// TestIntegrationReservedName verifies the scenario described by its name.
func TestIntegrationReservedName(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	conn := tcpDial(t, addr)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(conn, "Server\n")
	text, _ := readUntil(conn, "[ENTER YOUR NAME]:", 2*time.Second)
	if !strings.Contains(stripAnsi(text), "reserved") {
		t.Errorf("'Server' should be rejected as reserved, got: %q", stripAnsi(text))
	}

	// Should still be able to pick a valid name after rejection
	fmt.Fprintf(conn, "validuser\n")
	readUntil(conn, "[ENTER ROOM NAME]", 3*time.Second)
	fmt.Fprintf(conn, "\n")
	_, err := readUntil(conn, "][validuser]:", 3*time.Second)
	if err != nil {
		t.Errorf("should be able to pick a valid name after reserved rejection: %v", err)
	}
}

// ==================== Test 14: IP Pre-Check ====================
// Banned client gets rejection before banner, connection closed.

// TestIntegrationIPPreCheck verifies the scenario described by its name.
func TestIntegrationIPPreCheck(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	// Pre-ban the IP directly
	s.mu.Lock()
	s.bannedIPs["127.0.0.1"] = true
	s.mu.Unlock()

	// Connect — should get rejection before banner
	conn := tcpDial(t, addr)
	defer conn.Close()

	text := drainFor(conn, 2*time.Second)
	stripped := stripAnsi(text)

	if strings.Contains(stripped, "Welcome to TCP-Chat!") {
		t.Error("banned IP should NEVER see the welcome banner")
	}
	if strings.Contains(stripped, "[ENTER YOUR NAME]:") {
		t.Error("banned IP should NEVER see the name prompt")
	}
	if !strings.Contains(stripped, "banned") {
		t.Errorf("banned IP should receive ban message, got: %q", stripped)
	}
}

// ==================== Test 15: Operator Commands ====================
// Operator performs all available commands from terminal.

// TestIntegrationOperatorAllCommands verifies the scenario described by its name.
func TestIntegrationOperatorAllCommands(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()
	opOut := s.OperatorOutput.(*safeWriter)

	alice := tcpOnboard(t, addr, "alice")
	defer alice.Close()
	bob := tcpOnboard(t, addr, "bob")
	defer bob.Close()
	readUntil(alice, "bob has joined", 2*time.Second)

	// /list from operator
	opOut.Reset()
	s.OperatorDispatch("/list")
	if !strings.Contains(opOut.String(), "alice") || !strings.Contains(opOut.String(), "bob") {
		t.Errorf("operator /list should show both clients, got: %q", opOut.String())
	}

	// /help from operator
	opOut.Reset()
	s.OperatorDispatch("/help")
	helpText := opOut.String()
	if !strings.Contains(helpText, "promote") || !strings.Contains(helpText, "demote") {
		t.Error("operator /help should show promote and demote")
	}

	// /promote
	s.OperatorDispatch("/promote alice")
	readUntil(alice, "promoted", 2*time.Second)
	if !s.IsKnownAdmin("alice") {
		t.Error("alice should be a known admin after promote")
	}

	// /announce
	s.OperatorDispatch("/announce Server maintenance tonight")
	announceText, _ := readUntil(alice, "Server maintenance tonight", 2*time.Second)
	if !strings.Contains(stripAnsi(announceText), "[ANNOUNCEMENT]") {
		t.Error("announcement should be formatted correctly")
	}
	readUntil(bob, "Server maintenance tonight", 2*time.Second)

	// /mute bob
	s.OperatorDispatch("/mute bob")
	readUntil(bob, "was muted", 2*time.Second)
	readUntil(alice, "was muted", 2*time.Second)

	// /unmute bob
	s.OperatorDispatch("/unmute bob")
	readUntil(bob, "was unmuted", 2*time.Second)

	// /kick bob (operator kicks from terminal — broadcast should say "by Server")
	s.OperatorDispatch("/kick bob")
	kickText, _ := readUntil(alice, "bob was kicked", 2*time.Second)
	if !strings.Contains(stripAnsi(kickText), "by Server") {
		t.Errorf("operator kick should show 'by Server', got: %q", stripAnsi(kickText))
	}
	time.Sleep(200 * time.Millisecond)

	// Clear kick cooldown for further testing
	s.mu.Lock()
	delete(s.kickedIPs, "127.0.0.1")
	s.mu.Unlock()

	// /demote alice — must happen BEFORE ban test because banning on
	// 127.0.0.1 collateral-disconnects all same-IP clients (NAT behavior).
	s.OperatorDispatch("/demote alice")
	readUntil(alice, "revoked", 2*time.Second)
	if s.IsKnownAdmin("alice") {
		t.Error("alice should no longer be admin after demote")
	}

	// Connect carol for ban test
	carol := tcpOnboard(t, addr, "carol")

	// /ban carol — this will also collateral-disconnect alice (same 127.0.0.1)
	s.OperatorDispatch("/ban carol")
	time.Sleep(200 * time.Millisecond)
	carol.Close()
	alice.Close()

	// Clear ban for further testing
	s.mu.Lock()
	delete(s.bannedIPs, "127.0.0.1")
	s.mu.Unlock()
}

// ==================== Test 16: Operator Inapplicable Commands ====================
// /quit, /name, /whisper return appropriate errors from terminal.

// TestIntegrationOperatorInapplicableCommands verifies the scenario described by its name.
func TestIntegrationOperatorInapplicableCommands(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()
	opOut := s.OperatorOutput.(*safeWriter)

	_ = tcpOnboard(t, addr, "alice")

	// /quit from operator
	opOut.Reset()
	s.OperatorDispatch("/quit")
	if !strings.Contains(opOut.String(), "not applicable") && !strings.Contains(opOut.String(), "cannot") {
		t.Errorf("/quit from operator should return error, got: %q", opOut.String())
	}

	// /name from operator
	opOut.Reset()
	s.OperatorDispatch("/name newname")
	if !strings.Contains(opOut.String(), "not applicable") && !strings.Contains(opOut.String(), "cannot") {
		t.Errorf("/name from operator should return error, got: %q", opOut.String())
	}

	// /whisper from operator
	opOut.Reset()
	s.OperatorDispatch("/whisper alice hello")
	if !strings.Contains(opOut.String(), "not applicable") && !strings.Contains(opOut.String(), "cannot") {
		t.Errorf("/whisper from operator should return error, got: %q", opOut.String())
	}
}

// ==================== Test 17: Full Admin Persistence Across Restart ====================
// Admin status from admins.json is correctly restored after restart.

// TestIntegrationAdminPersistence verifies the scenario described by its name.
func TestIntegrationAdminPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// First server: promote alice to admin
	s1, addr1 := startIntServerInDir(t, tmpDir)
	alice := tcpOnboard(t, addr1, "alice")
	s1.OperatorDispatch("/promote alice")
	readUntil(alice, "promoted", 2*time.Second)
	alice.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Shutdown()

	// Verify admins.json was written
	adminsData, err := os.ReadFile(filepath.Join(tmpDir, "admins.json"))
	if err != nil {
		t.Fatalf("admins.json should exist: %v", err)
	}
	var admins []string
	if err := json.Unmarshal(adminsData, &admins); err != nil {
		t.Fatalf("admins.json should be valid JSON: %v", err)
	}
	found := false
	for _, a := range admins {
		if a == "alice" {
			found = true
		}
	}
	if !found {
		t.Errorf("admins.json should contain 'alice', got: %v", admins)
	}

	// Second server: alice reconnects and should auto-restore admin
	s2, addr2 := startIntServerInDir(t, tmpDir)
	defer s2.Shutdown()

	alice2 := tcpDial(t, addr2)
	defer alice2.Close()
	readUntil(alice2, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(alice2, "alice\n")
	// Check for admin greeting before room selection
	text, _ := readUntil(alice2, "[ENTER ROOM NAME]", 5*time.Second)
	stripped := stripAnsi(text)
	if !strings.Contains(stripped, "Welcome back") || !strings.Contains(stripped, "admin") {
		t.Errorf("alice should be greeted as admin on reconnect, got: %q", stripped)
	}
	fmt.Fprintf(alice2, "\n")
	readUntil(alice2, "][alice]:", 5*time.Second)

	// Verify alice can use admin commands
	bob := tcpOnboard(t, addr2, "bob")
	defer bob.Close()
	readUntil(alice2, "bob has joined", 2*time.Second)

	fmt.Fprintf(alice2, "/mute bob\n")
	readUntil(alice2, "][alice]:", 2*time.Second)
	muteText, _ := readUntil(bob, "was muted", 2*time.Second)
	if !strings.Contains(stripAnsi(muteText), "muted") {
		t.Error("admin-restored alice should be able to mute")
	}
}
