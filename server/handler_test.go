package server

import (
	"bufio"
	"fmt"
	"net"
	"net-cat/client"
	"net-cat/logger"
	"net-cat/models"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helper: connect a pipe-based client to the server handler, return the "client" side.
func connectPipe(s *Server) net.Conn {
	serverConn, clientConn := net.Pipe()
	go s.handleConnection(serverConn)
	return clientConn
}

// helper: read until a specific substring appears or timeout.
func readUntil(conn net.Conn, substr string, timeout time.Duration) (string, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if strings.Contains(buf.String(), substr) {
				conn.SetReadDeadline(time.Time{})
				return buf.String(), nil
			}
		}
		if err != nil {
			return buf.String(), err
		}
	}
}

// helper: complete onboarding with the given name, returning all text received.
func onboard(conn net.Conn, name string) (string, error) {
	// Read banner + name prompt
	text, err := readUntil(conn, "[ENTER YOUR NAME]:", 2*time.Second)
	if err != nil {
		return text, err
	}
	// Send name
	fmt.Fprintf(conn, "%s\n", name)
	// Read until we get the first prompt (contains the username)
	text2, err := readUntil(conn, "]["+name+"]:", 2*time.Second)
	return text + text2, err
}

// ==================== Task 2: Server Accepts Connections ====================

func TestServerAcceptsTCPConnection(t *testing.T) {
	s := New("0") // port 0 = random
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	s.listener = ln
	s.quit = make(chan struct{})
	go s.acceptLoop()
	defer func() {
		close(s.quit)
		ln.Close()
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("could not connect: %v", err)
	}
	defer conn.Close()

	// Should receive welcome banner
	_, err = readUntil(conn, "Welcome to TCP-Chat!", time.Second)
	if err != nil {
		t.Errorf("did not receive banner: %v", err)
	}
}

func TestMultipleClientsConcurrent(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	_, err1 := readUntil(c1, "[ENTER YOUR NAME]:", time.Second)
	_, err2 := readUntil(c2, "[ENTER YOUR NAME]:", time.Second)
	if err1 != nil || err2 != nil {
		t.Errorf("both clients should receive banner: err1=%v err2=%v", err1, err2)
	}
}

// ==================== Task 5: Client Onboarding ====================

func TestOnboardingBanner(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	text, err := readUntil(conn, "[ENTER YOUR NAME]:", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Welcome to TCP-Chat!") {
		t.Error("banner missing 'Welcome to TCP-Chat!'")
	}
	if !strings.Contains(text, "_nnnn_") {
		t.Error("banner missing penguin ASCII art")
	}
	if !strings.Contains(text, "[ENTER YOUR NAME]:") {
		t.Error("missing name prompt")
	}
}

func TestOnboardingEmptyName(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	fmt.Fprintf(conn, "\n") // empty name
	text, err := readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "cannot be empty") {
		t.Errorf("expected empty-name error, got: %q", text)
	}
	// Banner should NOT be re-sent
	if strings.Contains(text, "Welcome to TCP-Chat!") {
		t.Error("banner was re-sent on retry")
	}
}

func TestOnboardingWhitespaceOnlyName(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	fmt.Fprintf(conn, "   \n")
	text, _ := readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	if !strings.Contains(text, "cannot be empty") {
		t.Errorf("whitespace-only should be rejected as empty")
	}
}

func TestOnboardingNameWithSpaces(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	fmt.Fprintf(conn, "John Doe\n")
	text, _ := readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	if !strings.Contains(text, "spaces") {
		t.Errorf("expected no-spaces error, got: %q", text)
	}
}

func TestOnboardingNameTooLong(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	longName := strings.Repeat("a", 33)
	fmt.Fprintf(conn, "%s\n", longName)
	text, _ := readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	if !strings.Contains(text, "32") {
		t.Errorf("expected max-length error, got: %q", text)
	}
}

func TestOnboardingNameTaken(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	c2 := connectPipe(s)
	defer c2.Close()
	readUntil(c2, "[ENTER YOUR NAME]:", time.Second)
	fmt.Fprintf(c2, "alice\n")
	text, _ := readUntil(c2, "[ENTER YOUR NAME]:", time.Second)
	if !strings.Contains(text, "taken") {
		t.Errorf("expected name-taken error, got: %q", text)
	}
}

func TestOnboardingControlCharsRejected(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	fmt.Fprintf(conn, "bad\x01name\n")
	text, _ := readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	if !strings.Contains(text, "printable") {
		t.Errorf("expected printable-chars error, got: %q", text)
	}
}

func TestOnboardingValidSpecialChars(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	_, err := onboard(conn, "user-name_1")
	if err != nil {
		t.Errorf("user-name_1 should be accepted: %v", err)
	}
}

func TestOnboardingReservedNameServer(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	fmt.Fprintf(conn, "Server\n")
	text, _ := readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	if !strings.Contains(text, "reserved") {
		t.Errorf("expected reserved-name error, got: %q", text)
	}
}

func TestOnboardingNoRetryLimit(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	// Fail 10 times, succeed on 11th
	for i := 0; i < 10; i++ {
		fmt.Fprintf(conn, "\n") // empty
		readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	}
	fmt.Fprintf(conn, "finalname\n")
	_, err := readUntil(conn, "][finalname]:", 2*time.Second)
	if err != nil {
		t.Errorf("should succeed after 10 failures: %v", err)
	}
}

func TestOnboardingDisconnectDuringNamePrompt(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	conn.Close() // disconnect without completing onboarding
	time.Sleep(100 * time.Millisecond)

	// No clients should be registered
	if s.GetClientCount() != 0 {
		t.Error("disconnecting during name prompt should not register a client")
	}
}

func TestOnboardingCRLFStripped(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	conn.Write([]byte("alice\r\n"))
	_, err := readUntil(conn, "][alice]:", 2*time.Second)
	if err != nil {
		t.Errorf("name with \\r\\n should be accepted: %v", err)
	}
}

func TestOnboardingHistoryDelivered(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	// Alice sends a message
	fmt.Fprintf(c1, "hello everyone\n")
	readUntil(c1, "][alice]:", time.Second) // read prompt back

	// Bob joins and should see history
	c2 := connectPipe(s)
	defer c2.Close()
	text, _ := onboard(c2, "bob")
	if !strings.Contains(text, "hello everyone") {
		t.Errorf("bob should see alice's message in history, got: %q", text)
	}
}

// ==================== Task 6: Message Broadcast ====================

func TestMessageBroadcastFormat(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")

	// Drain bob's join notification from alice
	readUntil(c1, "bob has joined", time.Second)

	// Alice sends a message
	fmt.Fprintf(c1, "hello bob\n")
	// Alice gets prompt back
	readUntil(c1, "][alice]:", time.Second)

	// Bob receives the message
	text, err := readUntil(c2, "hello bob", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "[alice]:hello bob") {
		t.Errorf("bob should see [timestamp][alice]:hello bob, got: %q", text)
	}
}

func TestSenderDoesNotReceiveOwnMessage(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "alice")
	fmt.Fprintf(c1, "hello\n")

	// Read whatever alice gets back (should be just a prompt)
	text, _ := readUntil(c1, "][alice]:", time.Second)
	// The received text should NOT contain the broadcast format of own message
	// It should only contain the fresh prompt
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "[alice]:hello") {
			t.Error("sender should NOT receive their own message echoed back")
		}
	}
}

func TestEmptyMessageSilentlyDiscarded(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Alice sends empty message
	fmt.Fprintf(c1, "\n")
	// Alice should get a fresh prompt
	_, err := readUntil(c1, "][alice]:", time.Second)
	if err != nil {
		t.Error("empty message should still return prompt")
	}

	// Bob should NOT receive anything from Alice's empty message.
	// Send a real message to verify bob is still responsive.
	fmt.Fprintf(c1, "real message\n")
	text, _ := readUntil(c2, "real message", 2*time.Second)
	// Count occurrences of alice's messages
	count := strings.Count(text, "[alice]:")
	if count != 1 {
		t.Errorf("bob should see 1 message from alice, got %d in: %q", count, text)
	}
}

func TestWhitespaceOnlyMessageDiscarded(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "alice")
	fmt.Fprintf(c1, "   \n")
	_, err := readUntil(c1, "][alice]:", time.Second)
	if err != nil {
		t.Error("whitespace message should return prompt")
	}
}

func TestMessageExactly2048Accepted(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	msg := strings.Repeat("A", 2048)
	fmt.Fprintf(c1, "%s\n", msg)
	_, err := readUntil(c2, msg[:50], 2*time.Second)
	if err != nil {
		t.Error("message of exactly 2048 chars should be accepted")
	}
}

func TestMessage2049Rejected(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "alice")
	msg := strings.Repeat("A", 2049)
	fmt.Fprintf(c1, "%s\n", msg)
	text, _ := readUntil(c1, "too long", 2*time.Second)
	if !strings.Contains(text, "Message too long") {
		t.Errorf("expected 'Message too long' error, got: %q", text)
	}
}

func TestCommandInputNeverBroadcast(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/list\n")
	readUntil(c1, "][alice]:", time.Second)

	// Send a real message to check bob only sees that
	fmt.Fprintf(c1, "normal msg\n")
	text, _ := readUntil(c2, "normal msg", 2*time.Second)
	if strings.Contains(text, "/list") {
		t.Error("/list should not be broadcast to other clients")
	}
}

// ==================== Task 7: Command Routing ====================

func TestUnknownCommandError(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/foobar\n")
	text, _ := readUntil(conn, "/help", time.Second)
	if !strings.Contains(text, "Unknown command") {
		t.Error("unrecognized command should return error")
	}
	if !strings.Contains(text, "/help") {
		t.Error("error should suggest /help")
	}
}

func TestLoneSlashUnrecognized(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/\n")
	text, _ := readUntil(conn, "Unknown command", time.Second)
	if !strings.Contains(text, "Unknown command") {
		t.Error("lone / should be treated as unrecognized command")
	}
}

func TestWrongCaseNotRecognized(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/LIST\n")
	text, _ := readUntil(conn, "Unknown command", time.Second)
	if !strings.Contains(text, "Unknown command") {
		t.Error("/LIST should be unrecognized (case-sensitive)")
	}
}

func TestCommandOutputPrivate(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/list\n")
	readUntil(c1, "Connected clients", time.Second)

	// Bob should NOT see the /list output
	fmt.Fprintf(c1, "marker message\n")
	text, _ := readUntil(c2, "marker message", 2*time.Second)
	if strings.Contains(text, "Connected clients") {
		t.Error("command output should be private to the issuer")
	}
}

func TestNonAdminKickInsufficientPrivileges(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/kick bob\n")
	text, _ := readUntil(c1, "Insufficient", time.Second)
	if !strings.Contains(text, "Insufficient privileges") {
		t.Errorf("expected insufficient privileges, got: %q", text)
	}
}

func TestNonAdminPromoteInsufficientPrivileges(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/promote bob\n")
	text, _ := readUntil(conn, "Insufficient", time.Second)
	if !strings.Contains(text, "Insufficient privileges") {
		t.Errorf("expected insufficient privileges for /promote")
	}
}

func TestMissingArgsReturnsUsage(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	// Set alice as admin so she can test admin commands
	cl := s.GetClient("alice")
	cl.Admin = true

	tests := []struct {
		command string
		expect  string
	}{
		{"/name\n", "Usage: /name"},
		{"/whisper\n", "Usage: /whisper"},
		{"/kick\n", "Usage: /kick"},
		{"/ban\n", "Usage: /ban"},
		{"/mute\n", "Usage: /mute"},
		{"/unmute\n", "Usage: /unmute"},
		{"/announce\n", "Usage: /announce"},
	}
	for _, tt := range tests {
		fmt.Fprint(conn, tt.command)
		text, err := readUntil(conn, "Usage:", time.Second)
		if err != nil || !strings.Contains(text, tt.expect) {
			t.Errorf("command %q should return usage hint containing %q, got: %q", tt.command, tt.expect, text)
		}
	}
}

// ==================== Task 8: Join/Leave Notifications ====================

func TestJoinNotification(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	c2 := connectPipe(s)
	defer c2.Close()
	onboard(c2, "bob")

	// Alice should see bob's join
	text, err := readUntil(c1, "bob has joined", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "bob has joined our chat...") {
		t.Errorf("expected join notification, got: %q", text)
	}
}

func TestJoinerDoesNotSeeOwnJoin(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	text, _ := onboard(c1, "alice")

	if strings.Contains(text, "alice has joined") {
		t.Error("alice should NOT see her own join notification")
	}
}

func TestLeaveNotificationOnDisconnect(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	c2 := connectPipe(s)
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	c2.Close() // bob disconnects
	text, err := readUntil(c1, "bob has left", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "bob has left our chat...") {
		t.Errorf("expected leave notification, got: %q", text)
	}
}

func TestQuitCommandTriggersLeave(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	c2 := connectPipe(s)
	defer c2.Close()
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c2, "/quit\n")
	text, err := readUntil(c1, "bob has left", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "bob has left our chat...") {
		t.Errorf("expected leave on /quit, got: %q", text)
	}
}

func TestJoinLeaveEventsInHistory(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	c2 := connectPipe(s)
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)
	c2.Close()
	readUntil(c1, "bob has left", 2*time.Second)

	// Charlie joins and should see join+leave in history
	c3 := connectPipe(s)
	defer c3.Close()
	text, _ := onboard(c3, "charlie")
	if !strings.Contains(text, "bob has joined our chat...") {
		t.Error("history should contain bob's join")
	}
	if !strings.Contains(text, "bob has left our chat...") {
		t.Error("history should contain bob's leave")
	}
}

func TestOtherClientsNotDisconnectedOnLeave(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	c2 := connectPipe(s)
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	c2.Close() // bob leaves
	readUntil(c1, "bob has left", 2*time.Second)

	// Alice should still be able to send messages
	fmt.Fprintf(c1, "still here\n")
	_, err := readUntil(c1, "][alice]:", time.Second)
	if err != nil {
		t.Error("alice should still be connected after bob leaves")
	}
}

// ==================== /name command (basic) ====================

func TestNameChangeBasic(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/name alice2\n")
	// Both should see name change
	text1, _ := readUntil(c1, "alice2", time.Second)
	if !strings.Contains(text1, "alice changed their name to alice2") {
		t.Errorf("sender should see name change notification, got: %q", text1)
	}

	text2, _ := readUntil(c2, "alice2", time.Second)
	if !strings.Contains(text2, "alice changed their name to alice2") {
		t.Errorf("other client should see name change, got: %q", text2)
	}
}

// ==================== /whisper command (basic) ====================

func TestWhisperBasic(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	onboard(c3, "charlie")
	readUntil(c1, "charlie has joined", time.Second)

	fmt.Fprintf(c1, "/whisper bob secret\n")
	// Alice sees send confirmation
	text1, _ := readUntil(c1, "PM to bob", time.Second)
	if !strings.Contains(text1, "[PM to bob]: secret") {
		t.Errorf("sender should see whisper confirmation, got: %q", text1)
	}
	// Bob sees the whisper
	text2, _ := readUntil(c2, "PM from alice", time.Second)
	if !strings.Contains(text2, "[PM from alice]: secret") {
		t.Errorf("recipient should see whisper, got: %q", text2)
	}
}

// ==================== /list command ====================

func TestListShowsConnectedClients(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/list\n")
	text, _ := readUntil(c1, "][alice]:", time.Second)
	if !strings.Contains(text, "alice") || !strings.Contains(text, "bob") {
		t.Errorf("/list should show all clients, got: %q", text)
	}
	if !strings.Contains(text, "idle:") {
		t.Errorf("/list should show idle times, got: %q", text)
	}
}

// ==================== /help command (role-aware) ====================

func TestHelpRegularUser(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/help\n")
	text, _ := readUntil(conn, "][alice]:", 2*time.Second)

	userCmds := []string{"/list", "/quit", "/name", "/whisper", "/help"}
	for _, cmd := range userCmds {
		if !strings.Contains(text, cmd) {
			t.Errorf("regular user help should show %s", cmd)
		}
	}
	adminCmds := []string{"/kick", "/ban", "/mute", "/unmute", "/announce"}
	for _, cmd := range adminCmds {
		if strings.Contains(text, cmd) {
			t.Errorf("regular user help should NOT show %s", cmd)
		}
	}
}

func TestHelpAdmin(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	cl := s.GetClient("alice")
	cl.Admin = true

	fmt.Fprintf(conn, "/help\n")
	text, _ := readUntil(conn, "][alice]:", 2*time.Second)

	for _, cmd := range []string{"/list", "/kick", "/announce"} {
		if !strings.Contains(text, cmd) {
			t.Errorf("admin help should show %s", cmd)
		}
	}
	for _, cmd := range []string{"/promote", "/demote"} {
		if strings.Contains(text, cmd) {
			t.Errorf("promoted admin help should NOT show %s", cmd)
		}
	}
}

// ==================== ValidateName unit tests ====================

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		label   string
	}{
		{"alice", false, "valid simple"},
		{"user-name_1", false, "valid special chars"},
		{"A", false, "single char"},
		{strings.Repeat("a", 32), false, "exactly 32 chars"},

		{"", true, "empty"},
		{"   ", true, "whitespace only"},
		{"\t", true, "tab only"},
		{"John Doe", true, "contains space"},
		{strings.Repeat("a", 33), true, "too long"},
		{"bad\x01name", true, "control char"},
		{"\x7f", true, "DEL char"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			err := ValidateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error=%v, wantErr=%v", tt.name, err, tt.wantErr)
			}
		})
	}
}

// ==================== Simultaneous name race ====================

func TestSimultaneousSameNameOneSucceeds(t *testing.T) {
	s := New("0")

	// Connect both clients and read banners sequentially
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	readUntil(c1, "[ENTER YOUR NAME]:", time.Second)
	readUntil(c2, "[ENTER YOUR NAME]:", time.Second)

	// Send names concurrently via goroutines
	done := make(chan struct{}, 2)
	go func() { c1.Write([]byte("samename\n")); done <- struct{}{} }()
	go func() { c2.Write([]byte("samename\n")); done <- struct{}{} }()
	<-done
	<-done

	// Wait for handlers to finish processing
	time.Sleep(200 * time.Millisecond)

	// Exactly one should be registered
	if s.GetClientCount() != 1 {
		t.Errorf("expected exactly 1 registered client, got %d", s.GetClientCount())
	}
	if s.GetClient("samename") == nil {
		t.Error("expected client 'samename' to be registered")
	}
}

func TestRegisterClientConcurrent(t *testing.T) {
	s := New("0")
	results := make(chan bool, 50)

	for i := 0; i < 50; i++ {
		go func() {
			sConn, _ := net.Pipe()
			c := client.NewClient(sConn)
			defer c.Close()
			results <- s.RegisterClient(c, "samename")
		}()
	}

	successes := 0
	for i := 0; i < 50; i++ {
		if <-results {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 success out of 50 concurrent registrations, got %d", successes)
	}
}

// ==================== Server continues after disconnect ====================

func TestServerContinuesAfterClientDisconnect(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	onboard(c1, "first")
	c1.Close()
	time.Sleep(100 * time.Millisecond)

	c2 := connectPipe(s)
	defer c2.Close()
	_, err := onboard(c2, "second")
	if err != nil {
		t.Errorf("server should accept new clients after one disconnects: %v", err)
	}
}

// ==================== Rapid-fire message ordering ====================

func TestRapidFireMessageOrder(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Alice sends 20 messages rapidly
	for i := 0; i < 20; i++ {
		fmt.Fprintf(c1, "msg_%d\n", i)
	}

	// Bob should receive all in order
	reader := bufio.NewReader(c2)
	received := 0
	c2.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		target := fmt.Sprintf("msg_%d", received)
		if strings.Contains(line, target) {
			received++
			if received == 20 {
				break
			}
		}
	}
	if received < 20 {
		t.Errorf("expected 20 messages delivered in order, got %d", received)
	}
}

// ==================== Logging helpers ====================

func newServerWithLogger(t *testing.T) (*Server, string) {
	t.Helper()
	logsDir := filepath.Join(t.TempDir(), "logs")
	s := New("0")
	l, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = l
	t.Cleanup(func() { l.Close() })
	return s, logsDir
}

func readLogContent(t *testing.T, logsDir string) string {
	t.Helper()
	date := logger.FormatDate(time.Now())
	path := filepath.Join(logsDir, "chat_"+date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("could not read log file: %v", err)
	}
	return string(data)
}

// ==================== Task 9: Activity Logging ====================

// closeAndReadLog closes connections, waits for handlers to finish, closes the logger, and reads the log.
func closeAndReadLog(t *testing.T, s *Server, logsDir string, conns ...net.Conn) string {
	t.Helper()
	for _, c := range conns {
		c.Close()
	}
	time.Sleep(200 * time.Millisecond) // wait for handler goroutines to finish
	s.Logger.Close()
	return readLogContent(t, logsDir)
}

func TestLoggingChatMessages(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)
	c2 := connectPipe(s)

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "hello from alice\n")
	readUntil(c1, "][alice]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1, c2)
	if !strings.Contains(content, "CHAT [alice]:hello from alice") {
		t.Errorf("log should contain chat message, got: %q", content)
	}
}

func TestLoggingJoinEvent(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)

	onboard(c1, "alice")

	content := closeAndReadLog(t, s, logsDir, c1)
	if !strings.Contains(content, "JOIN alice") {
		t.Errorf("log should contain join event, got: %q", content)
	}
}

func TestLoggingLeaveEventVoluntary(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)
	onboard(c1, "alice")

	c2 := connectPipe(s)
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Bob uses /quit (voluntary leave)
	fmt.Fprintf(c2, "/quit\n")
	readUntil(c1, "bob has left", 2*time.Second)

	content := closeAndReadLog(t, s, logsDir, c1)
	if !strings.Contains(content, "LEAVE bob voluntary") {
		t.Errorf("log should contain voluntary leave, got: %q", content)
	}
}

func TestLoggingLeaveEventDrop(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)
	onboard(c1, "alice")

	c2 := connectPipe(s)
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Bob's connection drops (no /quit)
	c2.Close()
	readUntil(c1, "bob has left", 2*time.Second)

	content := closeAndReadLog(t, s, logsDir, c1)
	if !strings.Contains(content, "LEAVE bob drop") {
		t.Errorf("log should contain drop leave, got: %q", content)
	}
}

func TestLoggingModerationKick(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)
	c2 := connectPipe(s)

	onboard(c1, "admin")
	onboard(c2, "target")
	readUntil(c1, "target has joined", time.Second)

	cl := s.GetClient("admin")
	cl.Admin = true

	fmt.Fprintf(c1, "/kick target\n")
	readUntil(c1, "][admin]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1, c2)
	if !strings.Contains(content, "MOD kicked target admin") {
		t.Errorf("log should contain kick moderation event, got: %q", content)
	}
}

func TestLoggingModerationBan(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)
	c2 := connectPipe(s)

	onboard(c1, "admin")
	onboard(c2, "target")
	readUntil(c1, "target has joined", time.Second)

	cl := s.GetClient("admin")
	cl.Admin = true

	fmt.Fprintf(c1, "/ban target\n")
	readUntil(c1, "][admin]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1, c2)
	if !strings.Contains(content, "MOD banned target admin") {
		t.Errorf("log should contain ban moderation event, got: %q", content)
	}
}

func TestLoggingModerationMuteUnmute(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)
	c2 := connectPipe(s)

	onboard(c1, "admin")
	onboard(c2, "target")
	readUntil(c1, "target has joined", time.Second)

	cl := s.GetClient("admin")
	cl.Admin = true

	fmt.Fprintf(c1, "/mute target\n")
	readUntil(c1, "][admin]:", time.Second)
	fmt.Fprintf(c1, "/unmute target\n")
	readUntil(c1, "][admin]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1, c2)
	if !strings.Contains(content, "MOD muted target admin") {
		t.Errorf("log should contain mute event, got: %q", content)
	}
	if !strings.Contains(content, "MOD unmuted target admin") {
		t.Errorf("log should contain unmute event, got: %q", content)
	}
}

func TestLoggingPromoteDemote(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)
	c2 := connectPipe(s)

	onboard(c1, "operator")
	onboard(c2, "target")
	readUntil(c1, "target has joined", time.Second)

	// Call promote/demote directly since they require operator privilege (Task 18)
	cl := s.GetClient("operator")
	s.cmdPromote(cl, "target")
	readUntil(c2, "promoted", time.Second)
	s.cmdDemote(cl, "target")
	readUntil(c2, "revoked", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1, c2)
	if !strings.Contains(content, "MOD promoted target operator") {
		t.Errorf("log should contain promote event, got: %q", content)
	}
	if !strings.Contains(content, "MOD demoted target operator") {
		t.Errorf("log should contain demote event, got: %q", content)
	}
}

func TestLoggingAnnouncement(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)

	onboard(c1, "admin")
	cl := s.GetClient("admin")
	cl.Admin = true

	fmt.Fprintf(c1, "/announce Server maintenance at midnight\n")
	readUntil(c1, "][admin]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1)
	if !strings.Contains(content, "ANNOUNCE [admin]:Server maintenance at midnight") {
		t.Errorf("log should contain announcement, got: %q", content)
	}
}

func TestLoggingNameChange(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)

	onboard(c1, "oldname")
	fmt.Fprintf(c1, "/name newname\n")
	readUntil(c1, "][newname]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1)
	if !strings.Contains(content, "NAMECHANGE oldname newname") {
		t.Errorf("log should contain name change, got: %q", content)
	}
}

func TestLoggingWhisperNotInLog(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)
	c2 := connectPipe(s)

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/whisper bob secret message\n")
	readUntil(c1, "PM to bob", time.Second)

	// Send a regular message to ensure the log has been written
	fmt.Fprintf(c1, "regular message\n")
	readUntil(c1, "][alice]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1, c2)
	if strings.Contains(content, "secret message") {
		t.Errorf("whisper should NOT be in log, got: %q", content)
	}
	if strings.Contains(content, "PM") {
		t.Errorf("no PM-related content should be in log, got: %q", content)
	}
	if !strings.Contains(content, "regular message") {
		t.Errorf("regular message should be in log, got: %q", content)
	}
}

func TestLoggingConsoleMinimal(t *testing.T) {
	// Verify the log file contains events (console output cannot be
	// easily captured here, but the code does not print chat to console).
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)

	onboard(c1, "alice")
	fmt.Fprintf(c1, "test message\n")
	readUntil(c1, "][alice]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1)
	if !strings.Contains(content, "CHAT [alice]:test message") {
		t.Error("chat message should be in log file")
	}
	if !strings.Contains(content, "JOIN alice") {
		t.Error("join event should be in log file")
	}
}

func TestLoggingSameDayAppend(t *testing.T) {
	logsDir := filepath.Join(t.TempDir(), "logs")

	// First "session"
	s1 := New("0")
	l1, _ := logger.New(logsDir)
	s1.Logger = l1

	c1 := connectPipe(s1)
	onboard(c1, "alice")
	fmt.Fprintf(c1, "first session\n")
	readUntil(c1, "][alice]:", time.Second)
	c1.Close()
	time.Sleep(200 * time.Millisecond)
	l1.Close()

	// Second "session" (simulated restart)
	s2 := New("0")
	l2, _ := logger.New(logsDir)
	s2.Logger = l2

	c2 := connectPipe(s2)
	onboard(c2, "bob")
	fmt.Fprintf(c2, "second session\n")
	readUntil(c2, "][bob]:", time.Second)
	c2.Close()
	time.Sleep(200 * time.Millisecond)
	l2.Close()

	content := readLogContent(t, logsDir)
	if !strings.Contains(content, "first session") {
		t.Error("first session messages should be present")
	}
	if !strings.Contains(content, "second session") {
		t.Error("second session messages should be appended")
	}
}

func TestLoggingConcurrentMessages(t *testing.T) {
	s, logsDir := newServerWithLogger(t)

	// Create 10 clients
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		name := fmt.Sprintf("user%d", i)
		onboard(conns[i], name)
	}
	// Wait for all join notifications to propagate
	time.Sleep(200 * time.Millisecond)

	// Each client sends a message
	for i := 0; i < 10; i++ {
		fmt.Fprintf(conns[i], "msg from user%d\n", i)
	}
	// Wait for all messages to be processed
	time.Sleep(500 * time.Millisecond)

	content := closeAndReadLog(t, s, logsDir, conns...)
	for i := 0; i < 10; i++ {
		expected := fmt.Sprintf("msg from user%d", i)
		count := strings.Count(content, expected)
		if count != 1 {
			t.Errorf("expected message %q to appear exactly once in log, got %d times", expected, count)
		}
	}
}

func TestLoggingDiskErrorContinues(t *testing.T) {
	// Server with a nil logger should still work (chat functions normally)
	s := New("0")
	// Logger is nil by default

	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "hello bob\n")
	text, err := readUntil(c2, "hello bob", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "[alice]:hello bob") {
		t.Error("chat should still work with nil logger")
	}
}

func TestLoggingEventsSelfContained(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	c1 := connectPipe(s)

	onboard(c1, "alice")
	fmt.Fprintf(c1, "hello world\n")
	readUntil(c1, "][alice]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		_, err := models.ParseLogLine(line)
		if err != nil {
			t.Errorf("log line not parseable: %v (line: %q)", err, line)
		}
	}
}
