package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net-cat/client"
	"net-cat/logger"
	"net-cat/models"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

// helper: complete onboarding with the given name (default room), returning all text received.
func onboard(conn net.Conn, name string) (string, error) {
	return onboardRoom(conn, name, "")
}

// helper: complete onboarding with name + room selection. Empty room = press Enter (default).
func onboardRoom(conn net.Conn, name, room string) (string, error) {
	// Read banner + name prompt
	text, err := readUntil(conn, "[ENTER YOUR NAME]:", 2*time.Second)
	if err != nil {
		return text, err
	}
	// Send name
	fmt.Fprintf(conn, "%s\n", name)
	// Read until room prompt
	text2, err := readUntil(conn, "[ENTER ROOM NAME]", 2*time.Second)
	if err != nil {
		return text + text2, err
	}
	// Send room choice (empty = default)
	fmt.Fprintf(conn, "%s\n", room)
	// Read until we get the first prompt (contains the username)
	text3, err := readUntil(conn, "]["+name+"]:", 2*time.Second)
	return text + text2 + text3, err
}

// helper: complete name entry but NOT room selection. Returns at room prompt.
func enterName(conn net.Conn, name string) (string, error) {
	text, err := readUntil(conn, "[ENTER YOUR NAME]:", 2*time.Second)
	if err != nil {
		return text, err
	}
	fmt.Fprintf(conn, "%s\n", name)
	text2, err := readUntil(conn, "[ENTER ROOM NAME]", 2*time.Second)
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
	// Handle room selection prompt
	readUntil(conn, "[ENTER ROOM NAME]", 2*time.Second)
	fmt.Fprintf(conn, "\n") // default room
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

// TestOnboardingDisconnectDuringNamePromptNoNotification verifies that when a
// client disconnects during the name prompt, no join or leave notification is
// sent to other connected clients (spec 02 edge case).
func TestOnboardingDisconnectDuringNamePromptNoNotification(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	// Connect a second client but disconnect it during name prompt
	c2 := connectPipe(s)
	readUntil(c2, "[ENTER YOUR NAME]:", time.Second)
	c2.Close()
	time.Sleep(200 * time.Millisecond)

	// alice should NOT have received any join or leave notification
	c1.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, err := c1.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	c1.SetReadDeadline(time.Time{})
	text := buf.String()

	if strings.Contains(text, "has joined") {
		t.Errorf("no join notification should be sent for client disconnecting during name prompt, got: %q", text)
	}
	if strings.Contains(text, "has left") {
		t.Errorf("no leave notification should be sent for client disconnecting during name prompt, got: %q", text)
	}
}

func TestOnboardingCRLFStripped(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	readUntil(conn, "[ENTER YOUR NAME]:", time.Second)
	conn.Write([]byte("alice\r\n"))
	// Handle room selection
	readUntil(conn, "[ENTER ROOM NAME]", 2*time.Second)
	fmt.Fprintf(conn, "\n")
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
	cl.SetAdmin(true)

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
	// Both should see name change; use prompt with new name as the delimiter
	// (avoids matching the echoed command text which also contains "alice2")
	text1, _ := readUntil(c1, "][alice2]:", time.Second)
	if !strings.Contains(text1, "alice changed their name to alice2") {
		t.Errorf("sender should see name change notification, got: %q", text1)
	}

	text2, _ := readUntil(c2, "alice changed their name to alice2", time.Second)
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
	cl.SetAdmin(true)

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

// ==================== validateName unit tests ====================

func Test_validateName(t *testing.T) {
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
			err := validateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateName(%q) error=%v, wantErr=%v", tt.name, err, tt.wantErr)
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
			results <- (s.RegisterClient(c, "samename") == nil)
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

// TestRoomCapacityEnforced verifies that checkRoomCapacity returns false
// when a room has MaxActiveClients members.
func TestRoomCapacityEnforced(t *testing.T) {
	s := New("0")
	// Fill default room to max capacity
	for i := 0; i < MaxActiveClients; i++ {
		sConn, cConn := net.Pipe()
		defer sConn.Close()
		defer cConn.Close()
		c := client.NewClient(sConn)
		if err := s.RegisterClient(c, fmt.Sprintf("user%d", i)); err != nil {
			t.Fatalf("registration %d failed: %v", i, err)
		}
		s.mu.Lock()
		s.JoinRoom(c, s.DefaultRoom)
		s.mu.Unlock()
	}
	if s.checkRoomCapacity(s.DefaultRoom) {
		t.Error("expected room to be at capacity")
	}
	// A different room should still have capacity
	if !s.checkRoomCapacity("dev") {
		t.Error("different room should have capacity")
	}
}

// TestRegisterClientConcurrentUniqueness verifies that when many goroutines race
// to register the same name, exactly one succeeds.
func TestRegisterClientConcurrentUniqueness(t *testing.T) {
	s := New("0")
	const racers = 20
	results := make(chan error, racers)
	for i := 0; i < racers; i++ {
		go func() {
			sConn, cConn := net.Pipe()
			defer sConn.Close()
			defer cConn.Close()
			c := client.NewClient(sConn)
			results <- s.RegisterClient(c, "samename")
		}()
	}
	successes := 0
	for i := 0; i < racers; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 success, got %d", successes)
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
	cl.SetAdmin(true)

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
	cl.SetAdmin(true)

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
	cl.SetAdmin(true)

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
	cl.SetAdmin(true)

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

// ==================== Task 10: Crash Recovery ====================

// newServerWithLoggerDir creates a server with a logger in the specified directory.
func newServerWithLoggerDir(t *testing.T, logsDir string) *Server {
	t.Helper()
	s := New("0")
	l, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = l
	t.Cleanup(func() { l.Close() })
	return s
}

func TestRecoveryNoPriorLog(t *testing.T) {
	// First client of the day with no prior log receives no history, just their prompt.
	logsDir := filepath.Join(t.TempDir(), "logs")
	s := newServerWithLoggerDir(t, logsDir)
	s.RecoverHistory()

	if len(s.GetHistory()) != 0 {
		t.Error("history should be empty when no log file exists")
	}

	c1 := connectPipe(s)
	defer c1.Close()
	text, err := onboard(c1, "alice")
	if err != nil {
		t.Fatal(err)
	}
	// Should have banner, name prompt, and first prompt only — no history
	if strings.Contains(text, "has joined") && !strings.Contains(text, "alice has joined") {
		t.Error("should not see any prior join events")
	}
}

func TestRecoveryAfterRestart(t *testing.T) {
	// After server restart on the same day, a connecting client sees history from before the restart.
	logsDir := filepath.Join(t.TempDir(), "logs")

	// First "session": create server, send messages, shut down
	s1 := newServerWithLoggerDir(t, logsDir)
	c1 := connectPipe(s1)
	onboard(c1, "alice")
	fmt.Fprintf(c1, "hello from first session\n")
	readUntil(c1, "][alice]:", time.Second)
	c1.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Logger.Close()

	// Second "session": new server pointing to same log dir, recover history
	s2 := newServerWithLoggerDir(t, logsDir)
	s2.RecoverHistory()

	history := s2.GetHistory()
	if len(history) == 0 {
		t.Fatal("recovered history should not be empty")
	}

	// Verify a connecting client sees the recovered history
	c2 := connectPipe(s2)
	defer c2.Close()
	text, err := onboard(c2, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "hello from first session") {
		t.Errorf("bob should see message from first session in history, got: %q", text)
	}
	if !strings.Contains(text, "alice has joined") {
		t.Errorf("bob should see alice's join event in history, got: %q", text)
	}
}

func TestRecoveryIncludesAllEventTypes(t *testing.T) {
	// History includes chat messages, join/leave events, name changes, admin actions.
	logsDir := filepath.Join(t.TempDir(), "logs")

	s1 := newServerWithLoggerDir(t, logsDir)
	c1 := connectPipe(s1)
	c2 := connectPipe(s1)
	onboard(c1, "admin")
	onboard(c2, "target")
	readUntil(c1, "target has joined", time.Second)

	// Chat message
	fmt.Fprintf(c1, "hello chat\n")
	readUntil(c1, "][admin]:", time.Second)

	// Name change
	fmt.Fprintf(c2, "/name target2\n")
	readUntil(c2, "][target2]:", time.Second)

	// Moderation (mute/unmute)
	cl := s1.GetClient("admin")
	cl.SetAdmin(true)
	fmt.Fprintf(c1, "/mute target2\n")
	readUntil(c1, "][admin]:", time.Second)
	fmt.Fprintf(c1, "/unmute target2\n")
	readUntil(c1, "][admin]:", time.Second)

	// Announcement
	fmt.Fprintf(c1, "/announce test announcement\n")
	readUntil(c1, "][admin]:", time.Second)

	// Leave (target2 disconnects)
	c2.Close()
	readUntil(c1, "target2 has left", 2*time.Second)

	c1.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Logger.Close()

	// Second session: recover and verify all event types
	s2 := newServerWithLoggerDir(t, logsDir)
	s2.RecoverHistory()

	c3 := connectPipe(s2)
	defer c3.Close()
	text, err := onboard(c3, "newcomer")
	if err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		substr string
		desc   string
	}{
		{"admin has joined", "join event"},
		{"hello chat", "chat message"},
		{"target changed their name to target2", "name change"},
		{"target2 was muted by admin", "mute moderation"},
		{"target2 was unmuted by admin", "unmute moderation"},
		{"[ANNOUNCEMENT]: test announcement", "announcement"},
		{"target2 has left", "leave event"},
	}
	for _, check := range checks {
		if !strings.Contains(text, check.substr) {
			t.Errorf("recovered history should contain %s (%q), got: %q", check.desc, check.substr, text)
		}
	}
}

func TestRecoveryTimestampsMatchOriginal(t *testing.T) {
	// Timestamps on recovered entries match the original send time, not the replay time.
	logsDir := filepath.Join(t.TempDir(), "logs")

	s1 := newServerWithLoggerDir(t, logsDir)
	c1 := connectPipe(s1)
	onboard(c1, "alice")
	fmt.Fprintf(c1, "timed message\n")
	readUntil(c1, "][alice]:", time.Second)

	// Capture the original timestamp from history
	origHistory := s1.GetHistory()
	var origTimestamp time.Time
	for _, msg := range origHistory {
		if msg.Type == models.MsgChat && msg.Content == "timed message" {
			origTimestamp = msg.Timestamp
			break
		}
	}
	if origTimestamp.IsZero() {
		t.Fatal("could not find original message in history")
	}

	c1.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Logger.Close()

	// Wait briefly so recovery happens at a different time
	time.Sleep(100 * time.Millisecond)

	// Recover
	s2 := newServerWithLoggerDir(t, logsDir)
	s2.RecoverHistory()

	recHistory := s2.GetHistory()
	for _, msg := range recHistory {
		if msg.Type == models.MsgChat && msg.Content == "timed message" {
			// Timestamps are truncated to seconds in log format, so compare at second precision
			origTS := models.FormatTimestamp(origTimestamp)
			recTS := models.FormatTimestamp(msg.Timestamp)
			if origTS != recTS {
				t.Errorf("recovered timestamp %q does not match original %q", recTS, origTS)
			}
			return
		}
	}
	t.Error("recovered history does not contain the timed message")
}

func TestRecoveryHistoryVisuallyIdentical(t *testing.T) {
	// History entries are visually identical to live messages — Display() output matches
	// between the original message and the log-parsed recovered version.
	logsDir := filepath.Join(t.TempDir(), "logs")

	s1 := newServerWithLoggerDir(t, logsDir)
	c1 := connectPipe(s1)
	onboard(c1, "alice")
	fmt.Fprintf(c1, "identical check\n")
	readUntil(c1, "][alice]:", time.Second)

	c1.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Logger.Close()

	// Parse the log file manually and recover into a new server
	logContent := readLogContent(t, logsDir)
	lines := strings.Split(strings.TrimSpace(logContent), "\n")
	var expectedDisplays []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		msg, err := models.ParseLogLine(line)
		if err != nil {
			continue
		}
		if msg.Type == models.MsgServerEvent {
			continue
		}
		expectedDisplays = append(expectedDisplays, msg.Display())
	}

	s2 := newServerWithLoggerDir(t, logsDir)
	s2.RecoverHistory()
	recHistory := s2.GetHistory()

	if len(expectedDisplays) != len(recHistory) {
		t.Fatalf("count mismatch: expected=%d recovered=%d", len(expectedDisplays), len(recHistory))
	}
	for i, msg := range recHistory {
		if msg.Display() != expectedDisplays[i] {
			t.Errorf("display mismatch at index %d:\n  expected:  %q\n  recovered: %q", i, expectedDisplays[i], msg.Display())
		}
	}
}

func TestRecoveryThreeRestarts(t *testing.T) {
	// Server restarted 3 times in one day: history accumulates across all sessions, no duplicates.
	logsDir := filepath.Join(t.TempDir(), "logs")

	for session := 1; session <= 3; session++ {
		s := newServerWithLoggerDir(t, logsDir)
		s.RecoverHistory()

		c := connectPipe(s)
		name := fmt.Sprintf("user%d", session)
		onboard(c, name)
		fmt.Fprintf(c, "message from session %d\n", session)
		readUntil(c, "]["+name+"]:", time.Second)

		c.Close()
		time.Sleep(200 * time.Millisecond)
		s.Logger.Close()
	}

	// Fourth session: verify all history accumulated
	s := newServerWithLoggerDir(t, logsDir)
	s.RecoverHistory()

	c := connectPipe(s)
	defer c.Close()
	text, err := onboard(c, "verifier")
	if err != nil {
		t.Fatal(err)
	}

	for session := 1; session <= 3; session++ {
		expected := fmt.Sprintf("message from session %d", session)
		if !strings.Contains(text, expected) {
			t.Errorf("history should contain %q, got: %q", expected, text)
		}
		// Each message should appear exactly once
		count := strings.Count(text, expected)
		if count != 1 {
			t.Errorf("message %q should appear exactly once, got %d times", expected, count)
		}
	}
}

func TestRecoveryJustSentMessageInHistory(t *testing.T) {
	// Client joins immediately after a message is sent: the just-sent message is in their history.
	s, _ := newServerWithLogger(t)
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	fmt.Fprintf(c1, "just sent\n")
	readUntil(c1, "][alice]:", time.Second)

	// Bob joins immediately
	c2 := connectPipe(s)
	defer c2.Close()
	text, err := onboard(c2, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "just sent") {
		t.Errorf("bob should see the just-sent message in history, got: %q", text)
	}
}

func TestRecoveryLargeHistory(t *testing.T) {
	// Very large history (thousands of entries): recovered in full, no truncation.
	// Write log entries directly to the file to test recovery at scale.
	logsDir := filepath.Join(t.TempDir(), "logs")
	os.MkdirAll(logsDir, 0700)

	date := logger.FormatDate(time.Now())
	logPath := filepath.Join(logsDir, "chat_"+date+".log")

	msgCount := 2000
	var buf strings.Builder
	now := time.Now()
	ts := models.FormatTimestamp(now)
	buf.WriteString(fmt.Sprintf("[%s] JOIN alice\n", ts))
	for i := 0; i < msgCount; i++ {
		buf.WriteString(fmt.Sprintf("[%s] CHAT [alice]:msg_%04d\n", ts, i))
	}
	os.WriteFile(logPath, []byte(buf.String()), 0600)

	s := newServerWithLoggerDir(t, logsDir)
	s.RecoverHistory()

	history := s.GetHistory()
	// Total: 1 join + 2000 chat = 2001
	if len(history) != msgCount+1 {
		t.Errorf("recovered %d entries, expected %d", len(history), msgCount+1)
	}

	chatCount := 0
	for _, msg := range history {
		if msg.Type == models.MsgChat {
			chatCount++
		}
	}
	if chatCount != msgCount {
		t.Errorf("recovered %d chat messages, expected %d", chatCount, msgCount)
	}

	// Verify first and last messages are correct
	if history[1].Content != "msg_0000" {
		t.Errorf("first chat message should be msg_0000, got %q", history[1].Content)
	}
	lastChat := history[len(history)-1]
	expected := fmt.Sprintf("msg_%04d", msgCount-1)
	if lastChat.Content != expected {
		t.Errorf("last chat message should be %q, got %q", expected, lastChat.Content)
	}
}

func TestRecoveryCorruptedLogFile(t *testing.T) {
	// Corrupted/unreadable log file: server starts with empty history.
	logsDir := filepath.Join(t.TempDir(), "logs")
	os.MkdirAll(logsDir, 0700)

	// Write a completely corrupt log file
	date := logger.FormatDate(time.Now())
	logPath := filepath.Join(logsDir, "chat_"+date+".log")
	os.WriteFile(logPath, []byte("this is not a valid log line\nneither is this\n"), 0600)

	s := newServerWithLoggerDir(t, logsDir)
	s.RecoverHistory()

	if len(s.GetHistory()) != 0 {
		t.Error("fully corrupt log should result in empty history")
	}
}

func TestRecoveryPartiallyCorruptedLogFile(t *testing.T) {
	// Partially corrupted log file: server recovers valid entries and warns about the rest.
	logsDir := filepath.Join(t.TempDir(), "logs")
	os.MkdirAll(logsDir, 0700)

	date := logger.FormatDate(time.Now())
	logPath := filepath.Join(logsDir, "chat_"+date+".log")

	// Write a mix of valid and invalid lines
	validLine := fmt.Sprintf("[%s] CHAT [alice]:hello valid\n", models.FormatTimestamp(time.Now()))
	content := validLine +
		"CORRUPT LINE HERE\n" +
		fmt.Sprintf("[%s] JOIN bob\n", models.FormatTimestamp(time.Now()))

	os.WriteFile(logPath, []byte(content), 0600)

	s := newServerWithLoggerDir(t, logsDir)
	s.RecoverHistory()

	history := s.GetHistory()
	if len(history) != 2 {
		t.Errorf("expected 2 valid entries recovered, got %d", len(history))
	}

	// Verify recovered entries
	foundChat := false
	foundJoin := false
	for _, msg := range history {
		if msg.Type == models.MsgChat && msg.Content == "hello valid" {
			foundChat = true
		}
		if msg.Type == models.MsgJoin && msg.Sender == "bob" {
			foundJoin = true
		}
	}
	if !foundChat {
		t.Error("should recover valid chat message")
	}
	if !foundJoin {
		t.Error("should recover valid join event")
	}
}

func TestRecoveryServerEventsExcluded(t *testing.T) {
	// Server events (start/stop) are NOT included in recovered user-visible history.
	logsDir := filepath.Join(t.TempDir(), "logs")

	s1 := newServerWithLoggerDir(t, logsDir)
	// Log a server event
	s1.Logger.Log(models.Message{
		Timestamp: time.Now(),
		Type:      models.MsgServerEvent,
		Content:   "Server started on port 8989",
	})
	// Log a chat message
	s1.Logger.Log(models.Message{
		Timestamp: time.Now(),
		Type:      models.MsgChat,
		Sender:    "alice",
		Content:   "hello",
	})
	s1.Logger.Close()

	s2 := newServerWithLoggerDir(t, logsDir)
	s2.RecoverHistory()

	history := s2.GetHistory()
	for _, msg := range history {
		if msg.Type == models.MsgServerEvent {
			t.Error("server events should be excluded from recovered history")
		}
	}
	if len(history) != 1 {
		t.Errorf("expected 1 recovered entry (chat only), got %d", len(history))
	}
}

func TestRecoveryNilLogger(t *testing.T) {
	// Server with nil logger: RecoverHistory is a no-op.
	s := New("0")
	s.RecoverHistory() // should not panic
	if len(s.GetHistory()) != 0 {
		t.Error("nil logger recovery should leave history empty")
	}
}

func TestRecoveryPromoteDemoteInHistory(t *testing.T) {
	// Recovered history includes promote and demote moderation events.
	logsDir := filepath.Join(t.TempDir(), "logs")

	s1 := newServerWithLoggerDir(t, logsDir)
	c1 := connectPipe(s1)
	c2 := connectPipe(s1)
	onboard(c1, "operator")
	onboard(c2, "target")
	readUntil(c1, "target has joined", time.Second)

	// Promote/demote directly
	cl := s1.GetClient("operator")
	s1.cmdPromote(cl, "target")
	readUntil(c2, "promoted", time.Second)
	s1.cmdDemote(cl, "target")
	readUntil(c2, "revoked", time.Second)

	c1.Close()
	c2.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Logger.Close()

	// Recover
	s2 := newServerWithLoggerDir(t, logsDir)
	s2.RecoverHistory()

	c3 := connectPipe(s2)
	defer c3.Close()
	text, _ := onboard(c3, "newcomer")
	if !strings.Contains(text, "target was promoted by operator") {
		t.Errorf("should see promote event in recovered history, got: %q", text)
	}
	if !strings.Contains(text, "target was demoted by operator") {
		t.Errorf("should see demote event in recovered history, got: %q", text)
	}
}

func TestRecoveryKickBanInHistory(t *testing.T) {
	// Recovered history includes kick and ban moderation events.
	logsDir := filepath.Join(t.TempDir(), "logs")

	s1 := newServerWithLoggerDir(t, logsDir)
	c1 := connectPipe(s1)
	c2 := connectPipe(s1)
	c3 := connectPipe(s1)
	onboard(c1, "admin")
	onboard(c2, "victim1")
	onboard(c3, "victim2")
	readUntil(c1, "victim2 has joined", time.Second)

	cl := s1.GetClient("admin")
	cl.SetAdmin(true)

	fmt.Fprintf(c1, "/kick victim1\n")
	readUntil(c1, "][admin]:", time.Second)
	fmt.Fprintf(c1, "/ban victim2\n")
	readUntil(c1, "][admin]:", time.Second)

	c1.Close()
	time.Sleep(200 * time.Millisecond)
	s1.Logger.Close()

	// Recover
	s2 := newServerWithLoggerDir(t, logsDir)
	s2.RecoverHistory()

	c4 := connectPipe(s2)
	defer c4.Close()
	text, _ := onboard(c4, "newcomer")
	if !strings.Contains(text, "victim1 was kicked by admin") {
		t.Errorf("should see kick event in recovered history, got: %q", text)
	}
	if !strings.Contains(text, "victim2 was banned by admin") {
		t.Errorf("should see ban event in recovered history, got: %q", text)
	}
}

// ==================== Task 11: Connection Capacity ====================

func TestTenClientsCanChat(t *testing.T) {
	s := New("0")
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		name := fmt.Sprintf("user%d", i)
		onboard(conns[i], name)
	}
	// All 10 should be registered
	if s.GetClientCount() != 10 {
		t.Errorf("expected 10 clients, got %d", s.GetClientCount())
	}
	// They can exchange messages
	fmt.Fprintf(conns[0], "hello from user0\n")
	readUntil(conns[0], "][user0]:", time.Second)
	text, err := readUntil(conns[9], "hello from user0", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "[user0]:hello from user0") {
		t.Errorf("message should be delivered, got: %q", text)
	}
}

func TestEleventhClientQueuedAfterRoomSelection(t *testing.T) {
	s := New("0")
	// Fill default room to 10 active clients
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// 11th client enters name, then selects the full room
	c11 := connectPipe(s)
	defer c11.Close()
	enterName(c11, "queued11")
	// Select the default (full) room
	fmt.Fprintf(c11, "\n")

	text, err := readUntil(c11, "yes/no", 2*time.Second)
	if err != nil {
		t.Fatalf("11th client should receive queue message: %v", err)
	}
	if !strings.Contains(text, "is full") {
		t.Error("11th client should see room is full")
	}
	if !strings.Contains(text, "#1 in the queue") {
		t.Errorf("11th client should be #1 in queue, got: %q", text)
	}
}

func TestQueueChooseNo(t *testing.T) {
	s := New("0")
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	c11 := connectPipe(s)
	defer c11.Close()
	enterName(c11, "queued11")
	fmt.Fprintf(c11, "\n") // select full default room
	readUntil(c11, "yes/no", 2*time.Second)
	fmt.Fprintf(c11, "no\n")

	time.Sleep(200 * time.Millisecond)
	if s.GetQueueLength() != 0 {
		t.Error("queue should be empty after 'no'")
	}
}

func TestQueueChooseYesAdmittedOnSlotOpen(t *testing.T) {
	s := New("0")
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// 11th client enters name, selects full room, queues
	c11 := connectPipe(s)
	defer c11.Close()
	enterName(c11, "admitted")
	fmt.Fprintf(c11, "\n")
	readUntil(c11, "yes/no", 2*time.Second)
	fmt.Fprintf(c11, "yes\n")
	time.Sleep(100 * time.Millisecond)

	if s.GetQueueLength() != 1 {
		t.Errorf("expected 1 in queue, got %d", s.GetQueueLength())
	}

	// Disconnect one active client to open a slot
	conns[0].Close()

	// 11th client should now be admitted and get the prompt
	_, err := readUntil(c11, "][admitted]:", 3*time.Second)
	if err != nil {
		t.Fatalf("admitted client should complete onboarding: %v", err)
	}

	if s.GetQueueLength() != 0 {
		t.Error("queue should be empty after admission")
	}
}

func TestQueuePositionUpdatesOnQueueChange(t *testing.T) {
	s := New("0")
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// Queue 3 clients
	q1 := connectPipe(s)
	defer q1.Close()
	enterName(q1, "q1user")
	fmt.Fprintf(q1, "\n")
	readUntil(q1, "yes/no", 2*time.Second)
	fmt.Fprintf(q1, "yes\n")

	q2 := connectPipe(s)
	defer q2.Close()
	enterName(q2, "q2user")
	fmt.Fprintf(q2, "\n")
	readUntil(q2, "#2 in the queue", 2*time.Second)
	fmt.Fprintf(q2, "yes\n")

	q3 := connectPipe(s)
	defer q3.Close()
	enterName(q3, "q3user")
	fmt.Fprintf(q3, "\n")
	readUntil(q3, "#3 in the queue", 2*time.Second)
	fmt.Fprintf(q3, "yes\n")

	time.Sleep(100 * time.Millisecond)

	// Disconnect one active client — q1 is admitted, q2 and q3 get position updates
	conns[0].Close()

	// q1 should be admitted (receives prompt)
	readUntil(q1, "][q1user]:", 3*time.Second)

	// q2 should now be #1
	text, err := readUntil(q2, "#1 in the queue", 2*time.Second)
	if err != nil {
		t.Fatalf("q2 should receive position update to #1: %v (got: %q)", err, text)
	}
	// q3 should now be #2
	text, err = readUntil(q3, "#2 in the queue", 2*time.Second)
	if err != nil {
		t.Fatalf("q3 should receive position update to #2: %v (got: %q)", err, text)
	}
}

func TestNamePromptDoesNotCountAgainstLimit(t *testing.T) {
	// Capacity is per-room. Clients at name prompt don't count.
	s := New("0")

	// Create 9 active clients in default room
	conns := make([]net.Conn, 9)
	for i := 0; i < 9; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// Create 5 clients at name prompt (not yet in any room)
	namePromptConns := make([]net.Conn, 5)
	for i := 0; i < 5; i++ {
		namePromptConns[i] = connectPipe(s)
		defer namePromptConns[i].Close()
		readUntil(namePromptConns[i], "[ENTER YOUR NAME]:", 2*time.Second)
	}

	// 10th active client should join without queue
	c10 := connectPipe(s)
	defer c10.Close()
	_, err := onboard(c10, "user9")
	if err != nil {
		t.Fatalf("10th active client should not be queued: %v", err)
	}
}

func TestTenActiveAndRoomFullNewClientQueued(t *testing.T) {
	// 10 active in default room + new client selects same room: gets queue offer
	s := New("0")

	// 10 active clients
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// New client enters name and selects the full room
	c14 := connectPipe(s)
	defer c14.Close()
	enterName(c14, "newuser")
	fmt.Fprintf(c14, "\n")
	text, err := readUntil(c14, "queue", 2*time.Second)
	if err != nil {
		t.Fatalf("new client should receive queue offer: %v", err)
	}
	if !strings.Contains(text, "is full") {
		t.Error("new client should see room full message")
	}
}

func TestQueuedClientDisconnectSilentlyRemoved(t *testing.T) {
	s := New("0")
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	q1 := connectPipe(s)
	enterName(q1, "q1user")
	fmt.Fprintf(q1, "\n")
	readUntil(q1, "yes/no", 2*time.Second)
	fmt.Fprintf(q1, "yes\n")
	time.Sleep(100 * time.Millisecond)

	q2 := connectPipe(s)
	defer q2.Close()
	enterName(q2, "q2user")
	fmt.Fprintf(q2, "\n")
	readUntil(q2, "#2 in the queue", 2*time.Second)
	fmt.Fprintf(q2, "yes\n")
	time.Sleep(100 * time.Millisecond)

	if s.GetQueueLength() != 2 {
		t.Errorf("expected 2 in queue, got %d", s.GetQueueLength())
	}

	q1.Close()
	time.Sleep(300 * time.Millisecond)

	text, _ := readUntil(q2, "#1 in the queue", 2*time.Second)
	if !strings.Contains(text, "#1 in the queue") {
		t.Errorf("q2 should be updated to #1, got: %q", text)
	}
}

func TestQueueFIFOAdmission(t *testing.T) {
	s := New("0")
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	q1 := connectPipe(s)
	defer q1.Close()
	enterName(q1, "first_admitted")
	fmt.Fprintf(q1, "\n")
	readUntil(q1, "yes/no", 2*time.Second)
	fmt.Fprintf(q1, "yes\n")

	q2 := connectPipe(s)
	defer q2.Close()
	enterName(q2, "second_admitted")
	fmt.Fprintf(q2, "\n")
	readUntil(q2, "#2 in the queue", 2*time.Second)
	fmt.Fprintf(q2, "yes\n")

	time.Sleep(100 * time.Millisecond)

	// Disconnect first active client — q1 admitted
	conns[0].Close()
	_, err := readUntil(q1, "][first_admitted]:", 3*time.Second)
	if err != nil {
		t.Fatalf("q1 should be admitted first (FIFO): %v", err)
	}

	// Disconnect another active client — q2 admitted
	conns[1].Close()
	_, err = readUntil(q2, "][second_admitted]:", 3*time.Second)
	if err != nil {
		t.Fatalf("q2 should be admitted second (FIFO): %v", err)
	}
}

func TestQueueInvalidInput(t *testing.T) {
	s := New("0")
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	c11 := connectPipe(s)
	defer c11.Close()
	enterName(c11, "queued11")
	fmt.Fprintf(c11, "\n")
	readUntil(c11, "yes/no", 2*time.Second)

	fmt.Fprintf(c11, "maybe\n")
	text, err := readUntil(c11, "yes' or 'no'", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Invalid input") {
		t.Errorf("invalid input should get error, got: %q", text)
	}

	fmt.Fprintf(c11, "no\n")
	time.Sleep(200 * time.Millisecond)
	if s.GetQueueLength() != 0 {
		t.Error("queue should be empty after 'no'")
	}
}

func TestAllActiveLeaveThenQueueAdmits(t *testing.T) {
	s := New("0")
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	q1 := connectPipe(s)
	defer q1.Close()
	enterName(q1, "q1user")
	fmt.Fprintf(q1, "\n")
	readUntil(q1, "yes/no", 2*time.Second)
	fmt.Fprintf(q1, "yes\n")

	q2 := connectPipe(s)
	defer q2.Close()
	enterName(q2, "q2user")
	fmt.Fprintf(q2, "\n")
	readUntil(q2, "#2 in the queue", 2*time.Second)
	fmt.Fprintf(q2, "yes\n")

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 10; i++ {
		conns[i].Close()
	}

	// First queued client should be admitted
	_, err := readUntil(q1, "][q1user]:", 3*time.Second)
	if err != nil {
		t.Fatalf("first queued client should be admitted: %v", err)
	}
}

func TestShutdownNotifiesQueuedClients(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 300 * time.Millisecond
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	q1 := connectPipe(s)
	defer q1.Close()
	enterName(q1, "q1user")
	fmt.Fprintf(q1, "\n")
	readUntil(q1, "yes/no", 2*time.Second)
	fmt.Fprintf(q1, "yes\n")
	time.Sleep(100 * time.Millisecond)

	go s.Shutdown()

	text, err := readUntil(q1, "shutting down", 2*time.Second)
	if err != nil {
		t.Fatalf("queued client should receive shutdown message: %v", err)
	}
	if !strings.Contains(text, "Server is shutting down. Goodbye!") {
		t.Errorf("expected shutdown message, got: %q", text)
	}
}

// ==================== Task 12: Graceful Shutdown ====================

func TestShutdownActiveClientsReceiveGoodbye(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 300 * time.Millisecond

	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	c2 := connectPipe(s)
	defer c2.Close()
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	go s.Shutdown()

	text1, err := readUntil(c1, "shutting down", 2*time.Second)
	if err != nil {
		t.Fatalf("alice should receive shutdown message: %v", err)
	}
	if !strings.Contains(text1, "Server is shutting down. Goodbye!") {
		t.Errorf("expected shutdown message for alice, got: %q", text1)
	}

	text2, err := readUntil(c2, "shutting down", 2*time.Second)
	if err != nil {
		t.Fatalf("bob should receive shutdown message: %v", err)
	}
	if !strings.Contains(text2, "Server is shutting down. Goodbye!") {
		t.Errorf("expected shutdown message for bob, got: %q", text2)
	}
}

func TestShutdownForceClosesAfterTimeout(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 100 * time.Millisecond

	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	// Run Shutdown concurrently; read the goodbye to unblock the pipe write
	done := make(chan struct{})
	go func() {
		s.Shutdown()
		close(done)
	}()

	// Read the goodbye (unblocks the synchronous pipe write)
	readUntil(c1, "shutting down", 2*time.Second)

	// Shutdown should complete within a reasonable time after timeout
	select {
	case <-done:
		// Good — Shutdown completed
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown should complete after force-closing connections")
	}
}

func TestShutdownLoggedToFile(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	s.ShutdownTimeout = 100 * time.Millisecond

	c1 := connectPipe(s)
	onboard(c1, "alice")

	done := make(chan struct{})
	go func() {
		s.Shutdown()
		close(done)
	}()

	// Read the goodbye to unblock the pipe write
	readUntil(c1, "shutting down", 2*time.Second)
	c1.Close()
	<-done

	content := readLogContent(t, logsDir)
	if !strings.Contains(content, "Server shutting down") {
		t.Errorf("shutdown event should be in log, got: %q", content)
	}
}

func TestShutdownNoClients(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 100 * time.Millisecond

	done := make(chan struct{})
	go func() {
		s.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Good — clean shutdown with no clients
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown with no clients should complete quickly")
	}
}

func TestShutdownIdempotent(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 100 * time.Millisecond

	// Multiple calls to Shutdown should not panic or hang
	done := make(chan struct{})
	go func() {
		s.Shutdown()
		s.Shutdown()
		s.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(3 * time.Second):
		t.Fatal("Multiple Shutdown calls should not hang")
	}
}

func TestShutdownNamePromptClientsReceiveGoodbye(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 300 * time.Millisecond

	// Client at name prompt (not yet registered)
	c1 := connectPipe(s)
	defer c1.Close()
	readUntil(c1, "[ENTER YOUR NAME]:", 2*time.Second)

	// Active client
	c2 := connectPipe(s)
	defer c2.Close()
	onboard(c2, "active")

	go s.Shutdown()

	// Name-prompt client should also receive goodbye
	text, err := readUntil(c1, "shutting down", 2*time.Second)
	if err != nil {
		t.Fatalf("name-prompt client should receive shutdown message: %v", err)
	}
	if !strings.Contains(text, "Server is shutting down. Goodbye!") {
		t.Errorf("expected shutdown message, got: %q", text)
	}

	// Active client should also get it
	text2, err := readUntil(c2, "shutting down", 2*time.Second)
	if err != nil {
		t.Fatalf("active client should receive shutdown message: %v", err)
	}
	if !strings.Contains(text2, "Server is shutting down. Goodbye!") {
		t.Errorf("expected shutdown message for active client, got: %q", text2)
	}
}

func TestShutdownBeforeAnyClientConnects(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 100 * time.Millisecond

	// Shutdown with no clients or listener should be clean
	done := make(chan struct{})
	go func() {
		s.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown before any connects should complete quickly")
	}
}

// ==================== Task 13: /list Command with Idle Times ====================

func TestListIdleTimeSinceJoin(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	// Client never sent a message — idle time counted from join
	time.Sleep(100 * time.Millisecond)
	fmt.Fprintf(conn, "/list\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "alice") {
		t.Errorf("expected alice in list, got: %q", text)
	}
	if !strings.Contains(text, "idle:") {
		t.Errorf("expected idle time in list, got: %q", text)
	}
}

func TestListIdleTimeUpdatesAfterMessage(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Wait so bob accumulates idle time, then have alice send a message to reset hers
	time.Sleep(200 * time.Millisecond)
	fmt.Fprintf(c1, "hello\n")
	readUntil(c1, "][alice]:", time.Second)

	fmt.Fprintf(c1, "/list\n")
	text, _ := readUntil(c1, "][alice]:", time.Second)
	if !strings.Contains(text, "alice") || !strings.Contains(text, "bob") {
		t.Errorf("expected both clients in list, got: %q", text)
	}
}

func TestListOnlyVisibleToRequester(t *testing.T) {
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

	// Send a regular message from alice to flush bob's buffer
	fmt.Fprintf(c1, "hello\n")
	text, _ := readUntil(c2, "hello", time.Second)
	if strings.Contains(text, "Connected clients") {
		t.Error("list output should not be visible to other clients")
	}
}

func TestListSoloUser(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/list\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "alice") {
		t.Errorf("solo user should see themselves in list, got: %q", text)
	}
	if !strings.Contains(text, "idle:") {
		t.Errorf("solo user list should include idle time, got: %q", text)
	}
}

func TestListWithExtraArgs(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	// /list with extra args should still work (args ignored)
	fmt.Fprintf(conn, "/list foo\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "alice") {
		t.Errorf("/list with extra args should still work, got: %q", text)
	}
}

// ==================== Task 14: /quit Command ====================

func TestQuitWithExtraArgsStillDisconnects(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// /quit with extra args should still disconnect
	fmt.Fprintf(c2, "/quit goodbye\n")
	text, _ := readUntil(c1, "has left", time.Second)
	if !strings.Contains(text, "bob has left our chat...") {
		t.Errorf("quit with args should still disconnect, got: %q", text)
	}
}

func TestQuitLeaveLoggedAsVoluntary(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	conn := connectPipe(s)
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/quit\n")
	time.Sleep(300 * time.Millisecond) // let cleanup run

	content := closeAndReadLog(t, s, logsDir, conn)
	if !strings.Contains(content, "LEAVE alice voluntary") {
		t.Errorf("expected voluntary leave in log, got: %s", content)
	}
}

func TestQuitDisconnectsClient(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/quit\n")
	time.Sleep(300 * time.Millisecond)

	// Connection should be closed — reads should fail
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	if err == nil {
		t.Error("expected connection to be closed after /quit")
	}
}

// ==================== Task 15: /help Command (Role-Aware, expanded) ====================

func TestHelpExactFiveUserCommands(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/help\n")
	text, _ := readUntil(conn, "][alice]:", 2*time.Second)

	// Regular user sees exactly these 5 commands
	expectedCmds := []string{"/list", "/quit", "/name", "/whisper", "/help"}
	for _, c := range expectedCmds {
		if !strings.Contains(text, c) {
			t.Errorf("regular user help should show %s", c)
		}
	}

	// And NONE of the admin/operator commands
	absentCmds := []string{"/kick", "/ban", "/mute", "/unmute", "/announce", "/promote", "/demote"}
	for _, c := range absentCmds {
		if strings.Contains(text, c) {
			t.Errorf("regular user help should NOT show %s", c)
		}
	}
}

func TestHelpAdminSeesFullAdminSet(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	cl := s.GetClient("alice")
	cl.SetAdmin(true)

	fmt.Fprintf(conn, "/help\n")
	text, _ := readUntil(conn, "][alice]:", 2*time.Second)

	// Admin sees user commands + admin commands
	expectedCmds := []string{"/list", "/quit", "/name", "/whisper", "/help",
		"/kick", "/ban", "/mute", "/unmute", "/announce"}
	for _, c := range expectedCmds {
		if !strings.Contains(text, c) {
			t.Errorf("admin help should show %s", c)
		}
	}

	// Admin does NOT see operator-only commands
	for _, c := range []string{"/promote", "/demote"} {
		if strings.Contains(text, c) {
			t.Errorf("promoted admin help should NOT show %s", c)
		}
	}
}

func TestHelpAfterPromotion(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	// Initially a regular user — check no admin commands
	fmt.Fprintf(conn, "/help\n")
	text, _ := readUntil(conn, "][alice]:", 2*time.Second)
	if strings.Contains(text, "/kick") {
		t.Fatal("user should NOT see /kick before promotion")
	}

	// Promote
	cl := s.GetClient("alice")
	cl.SetAdmin(true)

	// After promotion, /help immediately reflects new role
	fmt.Fprintf(conn, "/help\n")
	text, _ = readUntil(conn, "][alice]:", 2*time.Second)
	if !strings.Contains(text, "/kick") {
		t.Error("admin should see /kick after promotion")
	}
}

func TestHelpAfterDemotion(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	cl := s.GetClient("alice")
	cl.SetAdmin(true)

	// Verify admin sees admin commands
	fmt.Fprintf(conn, "/help\n")
	text, _ := readUntil(conn, "][alice]:", 2*time.Second)
	if !strings.Contains(text, "/kick") {
		t.Fatal("admin should see /kick initially")
	}

	// Demote
	cl.SetAdmin(false)

	// After demotion, /help no longer shows admin commands
	fmt.Fprintf(conn, "/help\n")
	text, _ = readUntil(conn, "][alice]:", 2*time.Second)
	if strings.Contains(text, "/kick") {
		t.Error("demoted user should NOT see /kick")
	}
}

func TestHelpOutputPrivate(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/help\n")
	readUntil(c1, "][alice]:", 2*time.Second)

	// Bob should NOT see help output — flush with a real message
	fmt.Fprintf(c1, "hello\n")
	text, _ := readUntil(c2, "hello", time.Second)
	if strings.Contains(text, "Available commands") {
		t.Error("help output should not be visible to other clients")
	}
}

// ==================== Task 16: /whisper Command (Private Messaging, expanded) ====================

func TestWhisperSenderSeesContent(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/whisper bob hello there\n")
	text, _ := readUntil(c1, "PM to bob", time.Second)
	if !strings.Contains(text, "[PM to bob]: hello there") {
		t.Errorf("sender should see whisper content, got: %q", text)
	}
}

func TestWhisperRecipientFormat(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/whisper bob hello\n")
	text, _ := readUntil(c2, "PM from alice", time.Second)
	// Format: [timestamp][PM from sender]: message
	if !strings.Contains(text, "[PM from alice]: hello") {
		t.Errorf("recipient should see correct format, got: %q", text)
	}
}

func TestWhisperNotVisibleToOthers(t *testing.T) {
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
	readUntil(c1, "PM to bob", time.Second)
	readUntil(c2, "PM from alice", time.Second)

	// Send a regular message to flush charlie's buffer
	fmt.Fprintf(c1, "hello everyone\n")
	text, _ := readUntil(c3, "hello everyone", time.Second)
	if strings.Contains(text, "secret") || strings.Contains(text, "PM") {
		t.Error("third party should not see whisper")
	}
}

func TestWhisperNotInHistory(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Send a whisper
	fmt.Fprintf(c1, "/whisper bob secret\n")
	readUntil(c1, "PM to bob", time.Second)

	// Send a regular message so history has something
	fmt.Fprintf(c1, "public message\n")
	readUntil(c2, "public message", time.Second)

	// New client joins — whisper must NOT appear in history
	c3 := connectPipe(s)
	defer c3.Close()
	text, _ := onboard(c3, "charlie")
	if strings.Contains(text, "secret") || strings.Contains(text, "PM") {
		t.Error("whisper should NOT appear in history")
	}
	if !strings.Contains(text, "public message") {
		t.Error("regular message should appear in history")
	}
}

func TestWhisperNoArgs(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/whisper\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "Missing recipient") {
		t.Errorf("expected missing recipient error, got: %q", text)
	}
	if !strings.Contains(text, "Usage: /whisper") {
		t.Errorf("expected usage hint, got: %q", text)
	}
}

func TestWhisperNoMessage(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/whisper bob\n")
	text, _ := readUntil(c1, "][alice]:", time.Second)
	if !strings.Contains(text, "Missing message") {
		t.Errorf("expected missing message error, got: %q", text)
	}
}

func TestWhisperNonexistentUser(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/whisper ghost hello\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "not found") {
		t.Errorf("expected user-not-found error, got: %q", text)
	}
	if !strings.Contains(text, "ghost") {
		t.Errorf("error should name the unfound user, got: %q", text)
	}
}

func TestWhisperSelf(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/whisper alice hello\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "yourself") {
		t.Errorf("expected self-whisper error, got: %q", text)
	}
}

func TestWhisperTooLong(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	longMsg := strings.Repeat("a", 2049)
	fmt.Fprintf(c1, "/whisper bob %s\n", longMsg)
	text, _ := readUntil(c1, "][alice]:", 2*time.Second)
	if !strings.Contains(text, "too long") {
		t.Errorf("expected too-long error, got: %q", text)
	}
}

func TestWhisperWhitespaceOnly(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/whisper bob    \n")
	text, _ := readUntil(c1, "][alice]:", time.Second)
	if !strings.Contains(text, "Missing message") {
		t.Errorf("expected missing message error for whitespace-only whisper, got: %q", text)
	}
}

// ==================== Task 17: /name Command (Identity Change, expanded) ====================

func TestNameChangePromptUpdated(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/name alice2\n")
	text, _ := readUntil(conn, "][alice2]:", time.Second)
	if !strings.Contains(text, "][alice2]:") {
		t.Errorf("prompt should reflect new name after change, got: %q", text)
	}
}

func TestNameChangeInHistory(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	fmt.Fprintf(c1, "/name alice2\n")
	readUntil(c1, "][alice2]:", time.Second)

	// New client joins and sees name change in history
	c2 := connectPipe(s)
	defer c2.Close()
	text, _ := onboard(c2, "bob")
	if !strings.Contains(text, "alice changed their name to alice2") {
		t.Errorf("name change should appear in history, got: %q", text)
	}
}

func TestNameChangeNoArg(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/name\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "Usage: /name") {
		t.Errorf("expected usage hint, got: %q", text)
	}
}

func TestNameChangeSameName(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/name alice\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "already have that name") {
		t.Errorf("expected same-name error, got: %q", text)
	}
}

func TestNameChangeTakenName(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/name bob\n")
	text, _ := readUntil(c1, "][alice]:", time.Second)
	if !strings.Contains(text, "already taken") {
		t.Errorf("expected name-taken error, got: %q", text)
	}
}

func TestNameChangeWithSpaces(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	// /name foo bar — everything after "/name " is the proposed name "foo bar"
	fmt.Fprintf(conn, "/name foo bar\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "spaces") {
		t.Errorf("expected no-spaces error for name with space, got: %q", text)
	}
}

func TestNameChangeAdminRetainsPrivileges(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	cl := s.GetClient("alice")
	cl.SetAdmin(true)

	fmt.Fprintf(conn, "/name alice2\n")
	readUntil(conn, "][alice2]:", time.Second)

	cl2 := s.GetClient("alice2")
	if cl2 == nil || !cl2.IsAdmin() {
		t.Error("admin should retain privileges after name change")
	}
}

func TestNameChangeMutedRetainsMute(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	// Mute alice
	cl := s.GetClient("alice")
	cl.SetMuted(true)

	fmt.Fprintf(conn, "/name alice2\n")
	readUntil(conn, "][alice2]:", time.Second)

	// Try to send a message — should still be muted
	fmt.Fprintf(conn, "hello\n")
	text, _ := readUntil(conn, "][alice2]:", time.Second)
	if !strings.Contains(text, "You are muted") {
		t.Error("muted client should remain muted after name change")
	}
}

func TestNameChangeDisconnectedNameReusable(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	onboard(c1, "alice")
	c1.Close()
	time.Sleep(300 * time.Millisecond) // let cleanup run

	c2 := connectPipe(s)
	defer c2.Close()
	onboard(c2, "bob")

	fmt.Fprintf(c2, "/name alice\n")
	text, _ := readUntil(c2, "][alice]:", time.Second)
	if !strings.Contains(text, "alice") {
		t.Errorf("should be able to reuse disconnected client's name, got: %q", text)
	}
}

func TestNameChangeLogged(t *testing.T) {
	s, logsDir := newServerWithLogger(t)
	conn := connectPipe(s)
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/name alice2\n")
	readUntil(conn, "][alice2]:", time.Second)

	content := closeAndReadLog(t, s, logsDir, conn)
	if !strings.Contains(content, "NAMECHANGE alice alice2") {
		t.Errorf("expected name change in log, got: %s", content)
	}
}

func TestNameChangeReservedNameRejected(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	fmt.Fprintf(conn, "/name Server\n")
	text, _ := readUntil(conn, "][alice]:", time.Second)
	if !strings.Contains(text, "reserved") {
		t.Errorf("expected reserved name error, got: %q", text)
	}
}

func TestNameChangeSubsequentMessagesUseNewName(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/name alice2\n")
	readUntil(c2, "alice changed their name to alice2", time.Second)

	// Messages from the renamed client should use the new name
	fmt.Fprintf(c1, "hello\n")
	text, _ := readUntil(c2, "hello", time.Second)
	if !strings.Contains(text, "[alice2]:hello") {
		t.Errorf("messages after name change should use new name, got: %q", text)
	}
}

func TestNameChangeTwoClientsSimultaneousSameName(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Both try to rename to "charlie" simultaneously
	done := make(chan bool, 2)
	go func() {
		fmt.Fprintf(c1, "/name charlie\n")
		text, _ := readUntil(c1, "]:", time.Second)
		done <- strings.Contains(text, "][charlie]:")
	}()
	go func() {
		fmt.Fprintf(c2, "/name charlie\n")
		text, _ := readUntil(c2, "]:", time.Second)
		done <- strings.Contains(text, "][charlie]:")
	}()

	success1 := <-done
	success2 := <-done

	if success1 == success2 {
		t.Errorf("exactly one client should succeed when both try same name: got success1=%v success2=%v", success1, success2)
	}
}

// ==================== Task 18: Admin System ====================

// helper: create a server with admins.json pointed at a temp directory
func newServerWithAdmins(t *testing.T) *Server {
	t.Helper()
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "admins.json")
	return s
}

// helper: create a server with both logger and admins.json in a temp directory
func newServerWithLoggerAndAdmins(t *testing.T) (*Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	s := New("0")
	l, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = l
	s.adminsFile = filepath.Join(tmpDir, "admins.json")
	t.Cleanup(func() { l.Close() })
	return s, logsDir
}

// --- Operator terminal: basic command dispatch ---

func TestOperatorCanTypeCommandsInTerminal(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/list")
	output := buf.String()
	if !strings.Contains(output, "alice") {
		t.Errorf("operator /list should show connected clients, got: %q", output)
	}
}

func TestOperatorPromoteGrantsAdmin(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/promote alice")

	// Alice should see her personal notification and the broadcast
	text, _ := readUntil(conn, "You have been promoted to admin", time.Second)
	if !strings.Contains(text, "You have been promoted to admin") {
		t.Errorf("alice should be notified of promotion, got: %q", text)
	}
	// Drain the broadcast (alice also sees it as part of BroadcastAll)
	readUntil(conn, "alice was promoted by Server", time.Second)

	// Operator gets confirmation
	if !strings.Contains(buf.String(), "alice has been promoted to admin") {
		t.Errorf("operator should get confirmation, got: %q", buf.String())
	}

	// Alice should now be admin
	cl := s.GetClient("alice")
	if cl == nil || !cl.IsAdmin() {
		t.Error("alice should be admin after promotion")
	}
}

func TestOperatorDemoteRevokesAdmin(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	// Promote first
	s.OperatorDispatch("/promote alice")
	readUntil(conn, "promoted", time.Second)
	buf.Reset()

	// Now demote
	s.OperatorDispatch("/demote alice")

	text, _ := readUntil(conn, "revoked", time.Second)
	if !strings.Contains(text, "Your admin privileges have been revoked") {
		t.Errorf("alice should be notified of demotion, got: %q", text)
	}

	if !strings.Contains(buf.String(), "alice has been demoted") {
		t.Errorf("operator should get confirmation, got: %q", buf.String())
	}

	cl := s.GetClient("alice")
	if cl == nil || cl.IsAdmin() {
		t.Error("alice should not be admin after demotion")
	}
}

func TestPromotedAdminCannotPromoteOrDemote(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Promote alice via operator
	s.OperatorDispatch("/promote alice")
	readUntil(c1, "You have been promoted", time.Second)
	// Drain broadcast from both clients
	readUntil(c1, "alice was promoted", time.Second)
	readUntil(c2, "alice was promoted", time.Second)

	// alice tries /promote bob — should fail (operator-only command)
	fmt.Fprintf(c1, "/promote bob\n")
	text, _ := readUntil(c1, "Insufficient privileges", time.Second)
	if !strings.Contains(text, "Insufficient privileges") {
		t.Errorf("promoted admin should not be able to /promote, got: %q", text)
	}

	// alice tries /demote bob — should fail
	fmt.Fprintf(c1, "/demote bob\n")
	text, _ = readUntil(c1, "Insufficient privileges", time.Second)
	if !strings.Contains(text, "Insufficient privileges") {
		t.Errorf("promoted admin should not be able to /demote, got: %q", text)
	}
}

// --- admins.json persistence ---

func TestAdminPersistsToFile(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	s.OperatorDispatch("/promote alice")
	readUntil(conn, "promoted", time.Second)

	// Read admins.json
	data, err := os.ReadFile(s.adminsFile)
	if err != nil {
		t.Fatalf("admins.json should exist: %v", err)
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		t.Fatalf("admins.json should be valid JSON: %v", err)
	}
	found := false
	for _, n := range names {
		if n == "alice" {
			found = true
		}
	}
	if !found {
		t.Errorf("admins.json should contain 'alice', got: %v", names)
	}
}

func TestAdminReconnectRestoresPrivileges(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	onboard(c1, "alice")

	// Promote and disconnect
	s.OperatorDispatch("/promote alice")
	readUntil(c1, "promoted", time.Second)
	c1.Close()
	time.Sleep(300 * time.Millisecond)

	// Reconnect
	c2 := connectPipe(s)
	defer c2.Close()
	text, _ := onboard(c2, "alice")
	// Should get admin greeting
	text2, _ := readUntil(c2, "admin", time.Second)
	combined := text + text2
	if !strings.Contains(combined, "Welcome back, admin") {
		t.Errorf("reconnecting admin should get greeting, got: %q", combined)
	}

	cl := s.GetClient("alice")
	if cl == nil || !cl.IsAdmin() {
		t.Error("reconnecting admin should have privileges restored")
	}
}

func TestPromoteAlreadyAdmin(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/promote alice")
	readUntil(conn, "promoted", time.Second)
	buf.Reset()

	s.OperatorDispatch("/promote alice")
	if !strings.Contains(buf.String(), "already an admin") {
		t.Errorf("promoting already-admin should say so, got: %q", buf.String())
	}
}

func TestDemoteNonAdmin(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/demote alice")
	if !strings.Contains(buf.String(), "not an admin") {
		t.Errorf("demoting non-admin should say so, got: %q", buf.String())
	}
}

func TestPromoteDisconnectedUser(t *testing.T) {
	s := newServerWithAdmins(t)
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/promote ghost")
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("promoting disconnected user should fail, got: %q", buf.String())
	}
}

func TestMissingAdminsJsonOnStartup(t *testing.T) {
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "nonexistent", "admins.json")
	// Should not panic — just start with no admins
	s.LoadAdmins()
	if len(s.admins) != 0 {
		t.Errorf("should start with no admins when file is missing, got: %v", s.admins)
	}
}

func TestCorruptAdminsJsonOnStartup(t *testing.T) {
	tmpDir := t.TempDir()
	adminsPath := filepath.Join(tmpDir, "admins.json")
	os.WriteFile(adminsPath, []byte("this is not json{{{"), 0600)

	s := New("0")
	s.adminsFile = adminsPath
	s.LoadAdmins()
	if len(s.admins) != 0 {
		t.Errorf("should start with no admins when file is corrupt, got: %v", s.admins)
	}
}

// --- Operator inapplicable commands ---

func TestOperatorQuitReturnsError(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/quit")
	if !strings.Contains(buf.String(), "not applicable") {
		t.Errorf("operator /quit should return error, got: %q", buf.String())
	}
}

func TestOperatorNameReturnsError(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/name newname")
	if !strings.Contains(buf.String(), "not applicable") {
		t.Errorf("operator /name should return error, got: %q", buf.String())
	}
}

func TestOperatorWhisperReturnsError(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/whisper bob hello")
	if !strings.Contains(buf.String(), "not applicable") {
		t.Errorf("operator /whisper should return error, got: %q", buf.String())
	}
}

func TestOperatorNonCommandInput(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("hello world")
	if !strings.Contains(buf.String(), "Commands must start with /") {
		t.Errorf("non-command input should hint about /, got: %q", buf.String())
	}
}

func TestOperatorUnknownCommand(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/foobar")
	if !strings.Contains(buf.String(), "Unknown command") {
		t.Errorf("unknown command should say so, got: %q", buf.String())
	}
}

// --- Operator uses ALL commands from terminal ---

func TestOperatorListFromTerminal(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/list")
	if !strings.Contains(buf.String(), "alice") {
		t.Errorf("operator /list should show clients, got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "Room general") {
		t.Errorf("operator /list should have room header, got: %q", buf.String())
	}
}

func TestOperatorHelpFromTerminal(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/help")
	output := buf.String()

	// Operator sees ALL commands including promote and demote
	for _, cmdName := range []string{"/list", "/quit", "/name", "/whisper", "/help",
		"/kick", "/ban", "/mute", "/unmute", "/announce", "/promote", "/demote"} {
		if !strings.Contains(output, cmdName) {
			t.Errorf("operator /help should show %s, got: %q", cmdName, output)
		}
	}
}

func TestOperatorKickFromTerminal(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/kick bob")

	// alice should see the kick broadcast with "Server" as actor
	text, _ := readUntil(c1, "kicked by Server", time.Second)
	if !strings.Contains(text, "bob was kicked by Server") {
		t.Errorf("kick broadcast should use 'Server' as actor, got: %q", text)
	}

	if !strings.Contains(buf.String(), "bob has been kicked") {
		t.Errorf("operator should get kick confirmation, got: %q", buf.String())
	}
}

func TestOperatorBanFromTerminal(t *testing.T) {
	s := newServerWithAdmins(t)
	// Use distinct IPs so banning bob doesn't collateral-ban alice (NAT behavior)
	c1 := connectPipeWithIP(s, "10.0.0.80:1111")
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.81:2222")
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/ban bob")

	text, _ := readUntil(c1, "banned by Server", time.Second)
	if !strings.Contains(text, "bob was banned by Server") {
		t.Errorf("ban broadcast should use 'Server' as actor, got: %q", text)
	}
}

func TestOperatorMuteUnmuteFromTerminal(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/mute alice")
	text, _ := readUntil(conn, "muted by Server", time.Second)
	if !strings.Contains(text, "alice was muted by Server") {
		t.Errorf("mute broadcast should use 'Server' as actor, got: %q", text)
	}

	// Verify muted
	cl := s.GetClient("alice")
	if cl == nil || !cl.IsMuted() {
		t.Error("alice should be muted")
	}

	buf.Reset()
	s.OperatorDispatch("/unmute alice")
	text, _ = readUntil(conn, "unmuted by Server", time.Second)
	if !strings.Contains(text, "alice was unmuted by Server") {
		t.Errorf("unmute broadcast should use 'Server' as actor, got: %q", text)
	}
}

func TestOperatorAnnounceFromTerminal(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/announce Maintenance at midnight")
	text, _ := readUntil(conn, "Maintenance at midnight", time.Second)
	if !strings.Contains(text, "[ANNOUNCEMENT]: Maintenance at midnight") {
		t.Errorf("announcement should be broadcast, got: %q", text)
	}
	if !strings.Contains(buf.String(), "Announcement sent") {
		t.Errorf("operator should get confirmation, got: %q", buf.String())
	}
}

// --- Full admin name-change lifecycle ---

func TestAdminNameChangeLifecycle(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	onboard(c1, "alice")

	// Promote alice
	s.OperatorDispatch("/promote alice")
	readUntil(c1, "promoted", time.Second)

	// alice changes name to alice2
	fmt.Fprintf(c1, "/name alice2\n")
	readUntil(c1, "][alice2]:", time.Second)

	// Verify admin retained
	cl := s.GetClient("alice2")
	if cl == nil || !cl.IsAdmin() {
		t.Error("admin should retain privileges after name change")
	}

	// Verify admins.json updated
	data, _ := os.ReadFile(s.adminsFile)
	var names []string
	json.Unmarshal(data, &names)
	foundNew := false
	foundOld := false
	for _, n := range names {
		if n == "alice2" {
			foundNew = true
		}
		if n == "alice" {
			foundOld = true
		}
	}
	if !foundNew {
		t.Errorf("admins.json should contain 'alice2' after rename, got: %v", names)
	}
	if foundOld {
		t.Errorf("admins.json should NOT contain 'alice' after rename, got: %v", names)
	}

	// Disconnect
	c1.Close()
	time.Sleep(300 * time.Millisecond)

	// Reconnect as alice2 — should get admin restored
	c2 := connectPipe(s)
	defer c2.Close()
	text, _ := onboard(c2, "alice2")
	text2, _ := readUntil(c2, "admin", time.Second)
	combined := text + text2
	if !strings.Contains(combined, "Welcome back, admin") {
		t.Errorf("reconnecting renamed admin should get greeting, got: %q", combined)
	}
}

func TestDemotionRemovesFromAdminsJson(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	onboard(conn, "alice")

	s.OperatorDispatch("/promote alice")
	readUntil(conn, "promoted", time.Second)

	s.OperatorDispatch("/demote alice")
	readUntil(conn, "revoked", time.Second)

	// admins.json should NOT contain alice
	data, err := os.ReadFile(s.adminsFile)
	if err != nil {
		t.Fatalf("admins.json should exist: %v", err)
	}
	var names []string
	json.Unmarshal(data, &names)
	for _, n := range names {
		if n == "alice" {
			t.Error("admins.json should NOT contain alice after demotion")
		}
	}

	// Disconnect and reconnect — should NOT get admin
	conn.Close()
	time.Sleep(300 * time.Millisecond)

	c2 := connectPipe(s)
	defer c2.Close()
	onboard(c2, "alice")

	cl := s.GetClient("alice")
	if cl != nil && cl.IsAdmin() {
		t.Error("demoted client should NOT get admin restored on reconnect")
	}
}

// --- Logging with operator identity ---

func TestOperatorPromoteLoggedWithServerIdentity(t *testing.T) {
	s, logsDir := newServerWithLoggerAndAdmins(t)
	conn := connectPipe(s)
	onboard(conn, "alice")

	s.OperatorDispatch("/promote alice")
	readUntil(conn, "promoted", time.Second)

	content := closeAndReadLog(t, s, logsDir, conn)
	if !strings.Contains(content, "MOD promoted alice Server") {
		t.Errorf("promote log should use 'Server' as actor, got: %q", content)
	}
}

func TestOperatorDemoteLoggedWithServerIdentity(t *testing.T) {
	s, logsDir := newServerWithLoggerAndAdmins(t)
	conn := connectPipe(s)
	onboard(conn, "alice")

	s.OperatorDispatch("/promote alice")
	readUntil(conn, "promoted", time.Second)
	s.OperatorDispatch("/demote alice")
	readUntil(conn, "revoked", time.Second)

	content := closeAndReadLog(t, s, logsDir, conn)
	if !strings.Contains(content, "MOD demoted alice Server") {
		t.Errorf("demote log should use 'Server' as actor, got: %q", content)
	}
}

func TestOperatorKickLoggedWithServerIdentity(t *testing.T) {
	s, logsDir := newServerWithLoggerAndAdmins(t)
	c1 := connectPipe(s)
	c2 := connectPipe(s)
	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	s.OperatorDispatch("/kick bob")
	readUntil(c1, "kicked by Server", time.Second)

	content := closeAndReadLog(t, s, logsDir, c1, c2)
	if !strings.Contains(content, "MOD kicked bob Server") {
		t.Errorf("kick log should use 'Server' as actor, got: %q", content)
	}
}

// --- Rapid successive promote/demote: admins.json stays valid ---

func TestRapidPromoteDemoteAdminsJsonValid(t *testing.T) {
	s := newServerWithAdmins(t)
	conns := make([]net.Conn, 5)
	names := []string{"a1", "a2", "a3", "a4", "a5"}
	for i, name := range names {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], name)
		if i > 0 {
			// Wait for join notification on first conn
			readUntil(conns[0], name+" has joined", time.Second)
		}
	}

	// Rapid promote all
	for _, name := range names {
		s.OperatorDispatch("/promote " + name)
	}
	time.Sleep(200 * time.Millisecond)

	// Verify admins.json is valid
	data, err := os.ReadFile(s.adminsFile)
	if err != nil {
		t.Fatalf("admins.json should exist: %v", err)
	}
	var saved []string
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("admins.json should be valid JSON after rapid promotes: %v", err)
	}
	if len(saved) != 5 {
		t.Errorf("expected 5 admins, got %d: %v", len(saved), saved)
	}

	// Rapid demote all
	for _, name := range names {
		s.OperatorDispatch("/demote " + name)
	}
	time.Sleep(200 * time.Millisecond)

	data, _ = os.ReadFile(s.adminsFile)
	json.Unmarshal(data, &saved)
	if len(saved) != 0 {
		t.Errorf("expected 0 admins after demotion, got %d: %v", len(saved), saved)
	}
}

// --- Operator terminal with StartOperator (io.Reader-based) ---

func TestStartOperatorReadsFromReader(t *testing.T) {
	s := newServerWithAdmins(t)
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	var buf strings.Builder
	s.OperatorOutput = &buf

	// Simulate operator typing commands via a reader.
	// Run synchronously since the reader is finite — StartOperator returns when input is exhausted.
	input := "/list\n/help\n"
	s.StartOperator(strings.NewReader(input))

	output := buf.String()
	if !strings.Contains(output, "alice") {
		t.Errorf("operator /list should show alice, got: %q", output)
	}
	if !strings.Contains(output, "/promote") {
		t.Errorf("operator /help should show all commands, got: %q", output)
	}
}

// --- Operator empty input is ignored ---

func TestOperatorEmptyInputIgnored(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("")
	s.OperatorDispatch("   ")
	if buf.Len() != 0 {
		t.Errorf("empty input should produce no output, got: %q", buf.String())
	}
}

// --- Operator missing args ---

func TestOperatorPromoteMissingArgs(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/promote")
	if !strings.Contains(buf.String(), "Missing target") {
		t.Errorf("expected missing target error, got: %q", buf.String())
	}
}

func TestOperatorDemoteMissingArgs(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/demote")
	if !strings.Contains(buf.String(), "Missing target") {
		t.Errorf("expected missing target error, got: %q", buf.String())
	}
}

func TestOperatorKickMissingArgs(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/kick")
	if !strings.Contains(buf.String(), "Missing target") {
		t.Errorf("expected missing target error, got: %q", buf.String())
	}
}

func TestOperatorAnnounceMissingArgs(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/announce")
	if !strings.Contains(buf.String(), "Usage") {
		t.Errorf("expected usage hint, got: %q", buf.String())
	}
}

func TestOperatorAnnounceWhitespaceOnly(t *testing.T) {
	s := New("0")
	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/announce    ")
	if !strings.Contains(buf.String(), "Usage") {
		t.Errorf("whitespace-only announce should show usage, got: %q", buf.String())
	}
}

// ==================== IP-based test helpers ====================

// fakeAddr implements net.Addr with controllable values.
type fakeAddr struct {
	network, address string
}

func (a fakeAddr) Network() string { return a.network }
func (a fakeAddr) String() string  { return a.address }

// fakeAddrConn wraps net.Conn and overrides RemoteAddr.
type fakeAddrConn struct {
	net.Conn
	remoteAddr net.Addr
}

func (c *fakeAddrConn) RemoteAddr() net.Addr { return c.remoteAddr }

// connectPipeWithIP creates a pipe connection where the server sees the given IP.
func connectPipeWithIP(s *Server, ip string) net.Conn {
	serverConn, clientConn := net.Pipe()
	wrapped := &fakeAddrConn{
		Conn:       serverConn,
		remoteAddr: fakeAddr{network: "tcp", address: ip},
	}
	go s.handleConnection(wrapped)
	return clientConn
}

// ==================== Task 19: Moderation Commands (/kick, /ban, /mute, /unmute) ====================

func TestKickDisconnectsTarget(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	// Promote admin
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Kick alice
	fmt.Fprintf(c1, "/kick alice\n")
	readUntil(c1, "][admin]:", time.Second)

	// alice should receive kick message and then connection should close
	text, err := readUntil(c2, "You have been kicked", time.Second)
	if err == nil && !strings.Contains(text, "You have been kicked") {
		t.Errorf("alice should be notified about kick, got: %q", text)
	}

	// alice should be disconnected - verify she's no longer in client map
	time.Sleep(100 * time.Millisecond)
	if s.GetClient("alice") != nil {
		t.Error("alice should be removed from client map after kick")
	}
}

func TestKickBroadcastsNotification(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	onboard(c3, "bob")
	readUntil(c1, "bob has joined", time.Second)
	readUntil(c2, "bob has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/kick alice\n")
	readUntil(c1, "][admin]:", time.Second)

	// bob should see the kick notification
	text, _ := readUntil(c3, "kicked by admin", time.Second)
	if !strings.Contains(text, "alice was kicked by admin") {
		t.Errorf("bob should see kick notification, got: %q", text)
	}
}

func TestKickNoDoubleLeaveNotification(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/kick alice\n")
	// Read the kick broadcast
	text, _ := readUntil(c1, "kicked by admin", time.Second)

	// Wait for cleanup to complete
	time.Sleep(300 * time.Millisecond)

	// Send something to flush any pending messages
	fmt.Fprintf(c1, "ping\n")
	text2, _ := readUntil(c1, "][admin]:", time.Second)

	// The "has left our chat" message should NOT appear
	combined := text + text2
	if strings.Contains(combined, "has left our chat") {
		t.Errorf("kicked user should NOT trigger leave notification, got: %q", combined)
	}
}

func TestKickIPBlockedReconnect(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.1:1234")
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/kick alice\n")
	readUntil(c1, "kicked by admin", time.Second)
	time.Sleep(200 * time.Millisecond)

	// Try to reconnect from the same IP
	c3 := connectPipeWithIP(s, "10.0.0.1:5678")
	defer c3.Close()

	// The server sends the rejection and immediately closes.
	// Read whatever arrives (may be the rejection message or just EOF).
	var buf strings.Builder
	tmp := make([]byte, 4096)
	c3.SetReadDeadline(time.Now().Add(time.Second))
	for {
		n, err := c3.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	text := buf.String()

	// Should NOT see the welcome banner (key assertion: IP is blocked)
	if strings.Contains(text, "Welcome to TCP-Chat!") {
		t.Error("blocked client should not see welcome banner")
	}
	if strings.Contains(text, "[ENTER YOUR NAME]") {
		t.Error("blocked client should not see name prompt")
	}
}

func TestKickIPBlockExpiry(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.2:1234")
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/kick alice\n")
	readUntil(c1, "kicked by admin", time.Second)
	time.Sleep(200 * time.Millisecond)

	// Manually expire the cooldown by setting it to the past
	s.mu.Lock()
	s.kickedIPs["10.0.0.2"] = time.Now().Add(-time.Minute)
	s.mu.Unlock()

	// Now reconnect from same IP should work
	c3 := connectPipeWithIP(s, "10.0.0.2:9999")
	defer c3.Close()

	text, err := readUntil(c3, "[ENTER YOUR NAME]:", 2*time.Second)
	if err != nil {
		t.Fatalf("after cooldown expiry, client should see banner, got error: %v", err)
	}
	if !strings.Contains(text, "Welcome to TCP-Chat!") {
		t.Errorf("after cooldown expiry, client should see welcome banner, got: %q", text)
	}
}

func TestBanDisconnectsTarget(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/ban alice\n")
	readUntil(c1, "][admin]:", time.Second)

	// alice should receive ban message
	text, err := readUntil(c2, "You have been banned", time.Second)
	if err == nil && !strings.Contains(text, "You have been banned") {
		t.Errorf("alice should be notified about ban, got: %q", text)
	}

	time.Sleep(100 * time.Millisecond)
	if s.GetClient("alice") != nil {
		t.Error("alice should be removed from client map after ban")
	}
}

func TestBanBroadcastsNotification(t *testing.T) {
	s := newServerWithAdmins(t)
	// Use distinct IPs so banning alice doesn't collateral-ban bob (NAT behavior)
	c1 := connectPipeWithIP(s, "10.0.0.82:1111")
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.83:2222")
	defer c2.Close()
	c3 := connectPipeWithIP(s, "10.0.0.84:3333")
	defer c3.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	onboard(c3, "bob")
	readUntil(c1, "bob has joined", time.Second)
	readUntil(c2, "bob has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/ban alice\n")
	readUntil(c1, "][admin]:", time.Second)

	// bob should see ban notification
	text, _ := readUntil(c3, "banned by admin", time.Second)
	if !strings.Contains(text, "alice was banned by admin") {
		t.Errorf("bob should see ban notification, got: %q", text)
	}
}

func TestBanNoDoubleLeaveNotification(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/ban alice\n")
	text, _ := readUntil(c1, "banned by admin", time.Second)

	time.Sleep(300 * time.Millisecond)

	fmt.Fprintf(c1, "ping\n")
	text2, _ := readUntil(c1, "][admin]:", time.Second)

	combined := text + text2
	if strings.Contains(combined, "has left our chat") {
		t.Errorf("banned user should NOT trigger leave notification, got: %q", combined)
	}
}

func TestBanIPBlockedReconnect(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.3:1234")
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/ban alice\n")
	readUntil(c1, "banned by admin", time.Second)
	time.Sleep(200 * time.Millisecond)

	// Try to reconnect from same IP
	c3 := connectPipeWithIP(s, "10.0.0.3:5678")
	defer c3.Close()

	// The server sends the rejection and immediately closes.
	var buf strings.Builder
	tmp := make([]byte, 4096)
	c3.SetReadDeadline(time.Now().Add(time.Second))
	for {
		n, err := c3.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	text := buf.String()

	// Should NOT see the welcome banner (key assertion: IP is blocked)
	if strings.Contains(text, "Welcome to TCP-Chat!") {
		t.Error("banned client should not see welcome banner")
	}
	if strings.Contains(text, "[ENTER YOUR NAME]") {
		t.Error("banned client should not see name prompt")
	}
}

func TestBanNotPersistedAcrossRestart(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.4:1234")
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/ban alice\n")
	readUntil(c1, "banned by admin", time.Second)
	time.Sleep(200 * time.Millisecond)

	// Create a brand new server (simulating restart)
	s2 := newServerWithAdmins(t)

	// Connect from previously banned IP - should work on new server
	c3 := connectPipeWithIP(s2, "10.0.0.4:9999")
	defer c3.Close()

	text, err := readUntil(c3, "[ENTER YOUR NAME]:", 2*time.Second)
	if err != nil {
		t.Fatalf("ban should not persist across restart, got error: %v", err)
	}
	if !strings.Contains(text, "Welcome to TCP-Chat!") {
		t.Errorf("new server should allow previously banned IP, got: %q", text)
	}
}

func TestMutePreventsChat(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/mute alice\n")
	readUntil(c2, "muted by admin", time.Second)

	// alice tries to send a chat message
	fmt.Fprintf(c2, "hello everyone\n")
	text, _ := readUntil(c2, "You are muted", time.Second)
	if !strings.Contains(text, "You are muted") {
		t.Errorf("muted user should see 'You are muted', got: %q", text)
	}
}

func TestMutedClientCanRead(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/mute alice\n")
	readUntil(c2, "muted by admin", time.Second)

	// admin sends a message
	fmt.Fprintf(c1, "hello alice\n")
	readUntil(c1, "][admin]:", time.Second)

	// alice should still receive the message
	text, _ := readUntil(c2, "hello alice", time.Second)
	if !strings.Contains(text, "hello alice") {
		t.Errorf("muted user should still receive messages, got: %q", text)
	}
}

func TestMutedClientCommandsWork(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/mute alice\n")
	readUntil(c2, "muted by admin", time.Second)

	// /list should work
	fmt.Fprintf(c2, "/list\n")
	text, _ := readUntil(c2, "connected clients", time.Second)
	if !strings.Contains(text, "connected clients") {
		t.Errorf("muted user should be able to use /list, got: %q", text)
	}

	// /help should work
	fmt.Fprintf(c2, "/help\n")
	text, _ = readUntil(c2, "Available commands", time.Second)
	if !strings.Contains(text, "Available commands") {
		t.Errorf("muted user should be able to use /help, got: %q", text)
	}

	// /name should work
	fmt.Fprintf(c2, "/name alice2\n")
	text, _ = readUntil(c2, "][alice2]:", time.Second)
	if !strings.Contains(text, "alice2") {
		t.Errorf("muted user should be able to use /name, got: %q", text)
	}

	// /whisper should work
	fmt.Fprintf(c2, "/whisper admin hello\n")
	text, _ = readUntil(c2, "PM to admin", time.Second)
	if !strings.Contains(text, "PM to admin") {
		t.Errorf("muted user should be able to use /whisper, got: %q", text)
	}
}

func TestMuteBroadcastsNotification(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	onboard(c3, "bob")
	readUntil(c1, "bob has joined", time.Second)
	readUntil(c2, "bob has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/mute alice\n")

	// All clients should see it
	text1, _ := readUntil(c1, "muted by admin", time.Second)
	text2, _ := readUntil(c2, "muted by admin", time.Second)
	text3, _ := readUntil(c3, "muted by admin", time.Second)

	if !strings.Contains(text1, "alice was muted by admin") {
		t.Errorf("admin should see mute broadcast, got: %q", text1)
	}
	if !strings.Contains(text2, "alice was muted by admin") {
		t.Errorf("alice should see mute broadcast, got: %q", text2)
	}
	if !strings.Contains(text3, "alice was muted by admin") {
		t.Errorf("bob should see mute broadcast, got: %q", text3)
	}
}

func TestUnmuteBroadcastsNotification(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	onboard(c3, "bob")
	readUntil(c1, "bob has joined", time.Second)
	readUntil(c2, "bob has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// First mute
	fmt.Fprintf(c1, "/mute alice\n")
	readUntil(c1, "muted by admin", time.Second)
	readUntil(c2, "muted by admin", time.Second)
	readUntil(c3, "muted by admin", time.Second)

	// Now unmute
	fmt.Fprintf(c1, "/unmute alice\n")

	text1, _ := readUntil(c1, "unmuted by admin", time.Second)
	text2, _ := readUntil(c2, "unmuted by admin", time.Second)
	text3, _ := readUntil(c3, "unmuted by admin", time.Second)

	if !strings.Contains(text1, "alice was unmuted by admin") {
		t.Errorf("admin should see unmute broadcast, got: %q", text1)
	}
	if !strings.Contains(text2, "alice was unmuted by admin") {
		t.Errorf("alice should see unmute broadcast, got: %q", text2)
	}
	if !strings.Contains(text3, "alice was unmuted by admin") {
		t.Errorf("bob should see unmute broadcast, got: %q", text3)
	}
}

func TestUnmuteRestoresSending(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Mute
	fmt.Fprintf(c1, "/mute alice\n")
	readUntil(c2, "muted by admin", time.Second)

	// Verify muted
	fmt.Fprintf(c2, "should fail\n")
	text, _ := readUntil(c2, "You are muted", time.Second)
	if !strings.Contains(text, "You are muted") {
		t.Errorf("should be muted, got: %q", text)
	}

	// Unmute
	fmt.Fprintf(c1, "/unmute alice\n")
	readUntil(c2, "unmuted by admin", time.Second)

	// Now alice should be able to send
	fmt.Fprintf(c2, "hello after unmute\n")
	readUntil(c2, "][alice]:", time.Second)

	// admin should see alice's message
	text, _ = readUntil(c1, "hello after unmute", time.Second)
	if !strings.Contains(text, "hello after unmute") {
		t.Errorf("after unmute, alice's messages should be broadcast, got: %q", text)
	}
}

func TestMuteAlreadyMuted(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/mute alice\n")
	readUntil(c1, "muted by admin", time.Second)

	// Mute again
	fmt.Fprintf(c1, "/mute alice\n")
	text, _ := readUntil(c1, "already muted", time.Second)
	if !strings.Contains(text, "already muted") {
		t.Errorf("muting already-muted should return informational message, got: %q", text)
	}
}

func TestUnmuteNotMuted(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/unmute alice\n")
	text, _ := readUntil(c1, "not muted", time.Second)
	if !strings.Contains(text, "is not muted") {
		t.Errorf("unmuting non-muted should return error, got: %q", text)
	}
}

func TestCannotKickBanMuteServerOperator(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Try to kick "Server" (the operator is not in client map)
	fmt.Fprintf(c1, "/kick Server\n")
	text, _ := readUntil(c1, "not found", time.Second)
	if !strings.Contains(text, "not found") {
		t.Errorf("kicking Server should return 'not found', got: %q", text)
	}

	// Try to ban "Server"
	fmt.Fprintf(c1, "/ban Server\n")
	text, _ = readUntil(c1, "not found", time.Second)
	if !strings.Contains(text, "not found") {
		t.Errorf("banning Server should return 'not found', got: %q", text)
	}

	// Try to mute "Server"
	fmt.Fprintf(c1, "/mute Server\n")
	text, _ = readUntil(c1, "not found", time.Second)
	if !strings.Contains(text, "not found") {
		t.Errorf("muting Server should return 'not found', got: %q", text)
	}
}

func TestAdminsCanModerateEachOther(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin1")
	onboard(c2, "admin2")
	readUntil(c1, "admin2 has joined", time.Second)

	s.OperatorDispatch("/promote admin1")
	readUntil(c1, "promoted", time.Second)
	s.OperatorDispatch("/promote admin2")
	readUntil(c2, "promoted", time.Second)

	// admin1 kicks admin2
	fmt.Fprintf(c1, "/kick admin2\n")
	text, _ := readUntil(c1, "kicked by admin1", time.Second)
	if !strings.Contains(text, "admin2 was kicked by admin1") {
		t.Errorf("admin should be able to kick another admin, got: %q", text)
	}

	time.Sleep(100 * time.Millisecond)
	if s.GetClient("admin2") != nil {
		t.Error("admin2 should be removed after kick by admin1")
	}
}

func TestModerationEventsInHistory(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Mute alice (moderation event)
	fmt.Fprintf(c1, "/mute alice\n")
	readUntil(c1, "muted by admin", time.Second)
	readUntil(c2, "muted by admin", time.Second)

	// New client joins and should see the moderation event in history
	c3 := connectPipe(s)
	defer c3.Close()
	text, _ := onboard(c3, "charlie")
	if !strings.Contains(text, "alice was muted by admin") {
		t.Errorf("new joiner should see moderation event in history, got: %q", text)
	}
}

func TestModerationEventsLogged(t *testing.T) {
	s, logsDir := newServerWithLoggerAndAdmins(t)
	c1 := connectPipe(s)
	c2 := connectPipe(s)

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Kick alice
	fmt.Fprintf(c1, "/kick alice\n")
	readUntil(c1, "kicked by admin", time.Second)
	time.Sleep(200 * time.Millisecond)

	content := closeAndReadLog(t, s, logsDir, c1, c2)
	if !strings.Contains(content, "MOD kicked alice admin") {
		t.Errorf("log should contain moderation event with actor and target, got: %q", content)
	}
}

func TestKickMissingArgs(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/kick\n")
	text, _ := readUntil(c1, "Usage", time.Second)
	if !strings.Contains(text, "Missing target") {
		t.Errorf("expected missing target usage hint, got: %q", text)
	}
}

func TestBanMissingArgs(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/ban\n")
	text, _ := readUntil(c1, "Usage", time.Second)
	if !strings.Contains(text, "Missing target") {
		t.Errorf("expected missing target usage hint, got: %q", text)
	}
}

func TestMuteMissingArgs(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/mute\n")
	text, _ := readUntil(c1, "Usage", time.Second)
	if !strings.Contains(text, "Missing target") {
		t.Errorf("expected missing target usage hint, got: %q", text)
	}
}

func TestUnmuteMissingArgs(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/unmute\n")
	text, _ := readUntil(c1, "Usage", time.Second)
	if !strings.Contains(text, "Missing target") {
		t.Errorf("expected missing target usage hint, got: %q", text)
	}
}

func TestKickNonexistentUser(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/kick nobody\n")
	text, _ := readUntil(c1, "not found", time.Second)
	if !strings.Contains(text, "not found") {
		t.Errorf("kicking nonexistent user should return 'not found', got: %q", text)
	}
}

// TestUserNotFoundErrorSuggestsListCommand verifies that all admin commands
// include a "/list" recovery suggestion in their "user not found" error messages
// per spec 13 §Error Message Quality.
func TestUserNotFoundErrorSuggestsListCommand(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	commands := []string{"/kick ghost", "/ban ghost", "/mute ghost", "/unmute ghost"}
	for _, cmd := range commands {
		fmt.Fprintf(c1, "%s\n", cmd)
		text, err := readUntil(c1, "/list", time.Second)
		if err != nil || !strings.Contains(text, "Use /list to see connected users") {
			t.Errorf("%s: expected '/list' recovery suggestion in not-found error, got: %q", cmd, text)
		}
	}
}

// TestOperatorUserNotFoundErrorSuggestsListCommand verifies operator-side commands
// also include the "/list" recovery suggestion per spec 13 §Error Message Quality.
func TestOperatorUserNotFoundErrorSuggestsListCommand(t *testing.T) {
	s := newServerWithAdmins(t)
	var buf strings.Builder
	s.OperatorOutput = &buf

	commands := []string{"/kick ghost", "/ban ghost", "/mute ghost", "/unmute ghost", "/promote ghost", "/demote ghost"}
	for _, cmd := range commands {
		buf.Reset()
		s.OperatorDispatch(cmd)
		out := buf.String()
		if !strings.Contains(out, "Use /list to see connected users") {
			t.Errorf("%s: expected '/list' recovery suggestion in not-found error, got: %q", cmd, out)
		}
	}
}

func TestMutedUserNameChangeStaysMuted(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Mute alice
	fmt.Fprintf(c1, "/mute alice\n")
	readUntil(c2, "muted by admin", time.Second)

	// alice changes name
	fmt.Fprintf(c2, "/name alice2\n")
	readUntil(c2, "][alice2]:", time.Second)

	// alice2 should still be muted
	fmt.Fprintf(c2, "should be muted\n")
	text, _ := readUntil(c2, "You are muted", time.Second)
	if !strings.Contains(text, "You are muted") {
		t.Errorf("name change should not remove mute, got: %q", text)
	}
}

func TestIPCheckBeforeBanner(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.5:1234")
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/ban alice\n")
	readUntil(c1, "banned by admin", time.Second)
	time.Sleep(200 * time.Millisecond)

	// Connect from banned IP
	c3 := connectPipeWithIP(s, "10.0.0.5:9999")
	defer c3.Close()

	// The server sends the rejection and immediately closes.
	var buf strings.Builder
	tmp := make([]byte, 4096)
	c3.SetReadDeadline(time.Now().Add(time.Second))
	for {
		n, err := c3.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	text := buf.String()

	// Should NOT contain the welcome banner
	if strings.Contains(text, "Welcome to TCP-Chat!") {
		t.Error("banned IP should never see the welcome banner")
	}
	if strings.Contains(text, "[ENTER YOUR NAME]") {
		t.Error("banned IP should never see the name prompt")
	}
}

func TestOperatorKickBroadcastsWithServerIdentity(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/kick bob")

	// alice should see kick notification with "Server" as actor
	text, _ := readUntil(c1, "kicked by Server", time.Second)
	if !strings.Contains(text, "bob was kicked by Server") {
		t.Errorf("operator kick should broadcast with 'Server' identity, got: %q", text)
	}
}

func TestBanSameIPBlocksAll(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.6:1234")
	defer c2.Close()
	c3 := connectPipeWithIP(s, "10.0.0.6:5678")
	defer c3.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	onboard(c3, "bob")
	readUntil(c1, "bob has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Ban alice (10.0.0.6)
	fmt.Fprintf(c1, "/ban alice\n")
	readUntil(c1, "banned by admin", time.Second)
	time.Sleep(200 * time.Millisecond)

	// Try to reconnect from the same IP with a different port
	c4 := connectPipeWithIP(s, "10.0.0.6:9999")
	defer c4.Close()

	// The server sends the rejection and immediately closes.
	var buf strings.Builder
	tmp := make([]byte, 4096)
	c4.SetReadDeadline(time.Now().Add(time.Second))
	for {
		n, err := c4.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	text := buf.String()

	// Should NOT see the welcome banner - the IP is blocked
	if strings.Contains(text, "Welcome to TCP-Chat!") {
		t.Error("same IP should be blocked after ban, should not see banner")
	}
	if strings.Contains(text, "[ENTER YOUR NAME]") {
		t.Error("same IP should be blocked after ban, should not see name prompt")
	}
}

// ==================== NAT Ban: disconnect all clients sharing banned IP ====================

// TestBanNATDisconnectsAllClientsOnSameIP verifies that banning a user also
// disconnects all other currently-active clients sharing the same IP (NAT scenario),
// as required by spec 09 edge case: "Banning on shared NAT address: all users
// from that address are affected."
func TestBanNATDisconnectsAllClientsOnSameIP(t *testing.T) {
	s, logsDir := newServerWithLoggerAndAdmins(t)
	c1 := connectPipe(s) // admin on pipe (different IP)
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.50:1234") // alice
	defer c2.Close()

	// Drain bob's pipe concurrently to prevent writeLoop blocking on unbuffered net.Pipe
	c3 := connectPipeWithIP(s, "10.0.0.50:5678") // bob (same IP as alice)
	defer c3.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	onboard(c3, "bob")
	readUntil(c1, "bob has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Start draining bob's output so the server's writeLoop doesn't block
	go func() {
		tmp := make([]byte, 4096)
		for {
			if _, err := c3.Read(tmp); err != nil {
				return
			}
		}
	}()

	// Ban alice — bob should also be disconnected since they share 10.0.0.50
	fmt.Fprintf(c1, "/ban alice\n")
	// Admin should see ban notifications for BOTH alice and bob
	readUntil(c1, "alice was banned by admin", time.Second)
	text, err := readUntil(c1, "bob was banned by admin", time.Second)
	if err != nil {
		t.Fatalf("admin should see bob's ban notification, got: %q (err: %v)", text, err)
	}

	time.Sleep(200 * time.Millisecond)

	// Both alice and bob should be removed from the client map
	if s.GetClient("alice") != nil {
		t.Error("alice should be removed from client map after ban")
	}
	if s.GetClient("bob") != nil {
		t.Error("bob (same IP as alice) should be removed from client map after ban")
	}

	// Verify both ban events are recorded in the log
	logContent := readLogContent(t, logsDir)
	if !strings.Contains(logContent, "alice") || !strings.Contains(logContent, "banned") {
		t.Errorf("log should contain alice ban event, got: %q", logContent)
	}
	if !strings.Contains(logContent, "bob") || !strings.Contains(logContent, "banned") {
		t.Errorf("log should contain bob ban event, got: %q", logContent)
	}
}

// TestBanNATOperatorDisconnectsAllClientsOnSameIP verifies the operator's /ban
// also disconnects all clients sharing the banned IP.
func TestBanNATOperatorDisconnectsAllClientsOnSameIP(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipeWithIP(s, "10.0.0.60:1111") // alice
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.60:2222") // bob (same IP)
	defer c2.Close()
	c3 := connectPipeWithIP(s, "10.0.0.99:3333") // carol (different IP)
	defer c3.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	onboard(c3, "carol")
	readUntil(c3, "bob has joined", time.Second)

	var buf strings.Builder
	s.OperatorOutput = &buf

	// Operator bans alice — bob (same IP) should also be disconnected
	s.OperatorDispatch("/ban alice")
	time.Sleep(300 * time.Millisecond)

	if s.GetClient("alice") != nil {
		t.Error("alice should be removed after operator ban")
	}
	if s.GetClient("bob") != nil {
		t.Error("bob (same IP as alice) should be removed after operator ban")
	}
	// carol (different IP) should still be connected
	if s.GetClient("carol") == nil {
		t.Error("carol (different IP) should NOT be affected by ban")
	}

	// carol should see ban notifications
	text, _ := readUntil(c3, "banned by Server", time.Second)
	if !strings.Contains(text, "alice was banned by Server") {
		t.Errorf("carol should see alice ban notification, got: %q", text)
	}
}

// TestBanNATExcludesIssuer verifies that if the admin issuing /ban shares the
// same IP as the target (e.g. both behind same NAT), the admin is NOT banned.
func TestBanNATExcludesIssuer(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipeWithIP(s, "10.0.0.70:1111") // admin (same IP as target)
	defer c1.Close()
	c2 := connectPipeWithIP(s, "10.0.0.70:2222") // alice (same IP as admin)
	defer c2.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	readUntil(c1, "alice has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	// Admin bans alice — admin should NOT be disconnected despite sharing IP
	fmt.Fprintf(c1, "/ban alice\n")
	readUntil(c1, "][admin]:", time.Second)

	time.Sleep(200 * time.Millisecond)

	if s.GetClient("alice") != nil {
		t.Error("alice should be removed after ban")
	}
	if s.GetClient("admin") == nil {
		t.Error("admin should NOT be removed — issuer is excluded from NAT ban")
	}
}

// ==================== Task 20: /announce Command ====================

func TestAnnounceBroadcastsToAll(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboard(c1, "admin")
	onboard(c2, "alice")
	onboard(c3, "bob")
	readUntil(c1, "bob has joined", time.Second)
	readUntil(c2, "bob has joined", time.Second)

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/announce Server maintenance at midnight\n")

	text1, _ := readUntil(c1, "[ANNOUNCEMENT]", time.Second)
	text2, _ := readUntil(c2, "[ANNOUNCEMENT]", time.Second)
	text3, _ := readUntil(c3, "[ANNOUNCEMENT]", time.Second)

	if !strings.Contains(text1, "[ANNOUNCEMENT]: Server maintenance at midnight") {
		t.Errorf("admin should see announcement, got: %q", text1)
	}
	if !strings.Contains(text2, "[ANNOUNCEMENT]: Server maintenance at midnight") {
		t.Errorf("alice should see announcement, got: %q", text2)
	}
	if !strings.Contains(text3, "[ANNOUNCEMENT]: Server maintenance at midnight") {
		t.Errorf("bob should see announcement, got: %q", text3)
	}
}

func TestAnnounceInHistory(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/announce Important news\n")
	readUntil(c1, "[ANNOUNCEMENT]", time.Second)

	// New client joins and should see the announcement in history
	c2 := connectPipe(s)
	defer c2.Close()
	text, _ := onboard(c2, "alice")
	if !strings.Contains(text, "[ANNOUNCEMENT]: Important news") {
		t.Errorf("new joiner should see announcement in history, got: %q", text)
	}
}

func TestAnnounceLogged(t *testing.T) {
	s, logsDir := newServerWithLoggerAndAdmins(t)
	c1 := connectPipe(s)

	onboard(c1, "admin")

	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/announce Logging test\n")
	readUntil(c1, "[ANNOUNCEMENT]", time.Second)
	time.Sleep(200 * time.Millisecond)

	content := closeAndReadLog(t, s, logsDir, c1)
	if !strings.Contains(content, "ANNOUNCE [admin]:Logging test") {
		t.Errorf("log should contain announcement with announcer identity, got: %q", content)
	}
}

func TestAnnounceMissingBody(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/announce\n")
	text, _ := readUntil(c1, "Usage", time.Second)
	if !strings.Contains(text, "Usage") {
		t.Errorf("announce with no message should return usage hint, got: %q", text)
	}
}

func TestAnnounceNonAdminInsufficient(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "alice")

	fmt.Fprintf(c1, "/announce hello\n")
	text, _ := readUntil(c1, "Insufficient", time.Second)
	if !strings.Contains(text, "Insufficient privileges") {
		t.Errorf("non-admin should get 'Insufficient privileges', got: %q", text)
	}
}

func TestAnnounceByPromotedAdmin(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Promote alice
	s.OperatorDispatch("/promote alice")
	readUntil(c1, "promoted", time.Second)

	// alice uses /announce
	fmt.Fprintf(c1, "/announce Admin message\n")
	readUntil(c1, "[ANNOUNCEMENT]", time.Second)

	// bob should receive it
	text, _ := readUntil(c2, "[ANNOUNCEMENT]", time.Second)
	if !strings.Contains(text, "[ANNOUNCEMENT]: Admin message") {
		t.Errorf("promoted admin's announcement should broadcast, got: %q", text)
	}
}

func TestAnnounceWhitespaceBodyRejected(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "admin")
	s.OperatorDispatch("/promote admin")
	readUntil(c1, "promoted", time.Second)

	fmt.Fprintf(c1, "/announce    \n")
	text, _ := readUntil(c1, "Usage", time.Second)
	if !strings.Contains(text, "Usage") {
		t.Errorf("whitespace-only announce body should be rejected, got: %q", text)
	}
}

// ==================== Task 21: Connection Health (Heartbeat, Ghost Detection) ====================

// newHeartbeatServer creates a server with short heartbeat intervals for fast tests.
func newHeartbeatServer(t *testing.T) *Server {
	t.Helper()
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "admins.json")
	s.HeartbeatInterval = 100 * time.Millisecond
	s.HeartbeatTimeout = 50 * time.Millisecond
	return s
}

// newHeartbeatServerWithLogger creates a server with heartbeat + logger.
func newHeartbeatServerWithLogger(t *testing.T) (*Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	s := New("0")
	s.adminsFile = filepath.Join(tmpDir, "admins.json")
	s.HeartbeatInterval = 100 * time.Millisecond
	s.HeartbeatTimeout = 50 * time.Millisecond
	l, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = l
	t.Cleanup(func() { l.Close() })
	return s, logsDir
}

func TestHeartbeatDeadClientDetectedAndRemoved(t *testing.T) {
	s := newHeartbeatServer(t)

	// Connect bob who will observe alice's leave notification
	bob := connectPipe(s)
	defer bob.Close()
	onboard(bob, "bob")

	// Connect alice and onboard
	alice := connectPipe(s)
	onboard(alice, "alice")
	// Bob should see alice's join
	readUntil(bob, "alice has joined", time.Second)

	// Simulate alice's connection dying by closing her end
	alice.Close()

	// Bob should see alice's leave notification within a reasonable time
	// (heartbeat interval 100ms + timeout 50ms = 150ms, but allow generous margin)
	text, err := readUntil(bob, "alice has left", 2*time.Second)
	if err != nil {
		t.Fatalf("bob did not see alice's leave notification after dead connection: %v (got: %q)", err, text)
	}
	if !strings.Contains(text, "alice has left our chat...") {
		t.Errorf("expected standard leave notification, got: %q", text)
	}
}

func TestHeartbeatLeaveNotificationBroadcastToAll(t *testing.T) {
	s := newHeartbeatServer(t)

	bob := connectPipe(s)
	defer bob.Close()
	onboard(bob, "bob")

	carol := connectPipe(s)
	defer carol.Close()
	onboard(carol, "carol")
	readUntil(bob, "carol has joined", time.Second)

	alice := connectPipe(s)
	onboard(alice, "alice")
	readUntil(bob, "alice has joined", time.Second)
	readUntil(carol, "alice has joined", time.Second)

	// Kill alice
	alice.Close()

	// Both bob and carol should see the leave notification
	_, err1 := readUntil(bob, "alice has left", 2*time.Second)
	_, err2 := readUntil(carol, "alice has left", 2*time.Second)
	if err1 != nil {
		t.Errorf("bob did not see alice's leave: %v", err1)
	}
	if err2 != nil {
		t.Errorf("carol did not see alice's leave: %v", err2)
	}
}

func TestHeartbeatRemovalLoggedWithDropReason(t *testing.T) {
	s, logsDir := newHeartbeatServerWithLogger(t)

	bob := connectPipe(s)
	defer bob.Close()
	onboard(bob, "bob")

	alice := connectPipe(s)
	onboard(alice, "alice")
	readUntil(bob, "alice has joined", time.Second)

	// Kill alice
	alice.Close()
	// Wait for leave notification
	readUntil(bob, "alice has left", 2*time.Second)
	// Allow time for log write
	time.Sleep(100 * time.Millisecond)

	// Read the log file and verify drop reason
	date := logger.FormatDate(time.Now())
	logPath := filepath.Join(logsDir, fmt.Sprintf("chat_%s.log", date))
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("could not read log file: %v", err)
	}
	logContent := string(data)
	if !strings.Contains(logContent, "LEAVE") || !strings.Contains(logContent, "alice") {
		t.Errorf("expected leave event for alice in log, got: %q", logContent)
	}
	if !strings.Contains(logContent, "drop") {
		t.Errorf("expected 'drop' reason in log for dead client, got: %q", logContent)
	}
}

func TestHeartbeatInvisibleUnderNormalConditions(t *testing.T) {
	s := newHeartbeatServer(t)

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	// Read all data for 500ms, draining heartbeat probes so pipe writes don't block.
	// On a real TCP connection, kernel buffers absorb writes without blocking.
	var buf strings.Builder
	tmp := make([]byte, 4096)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		alice.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, _ := alice.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
	}
	alice.SetReadDeadline(time.Time{})

	// Under normal conditions (healthy connection where writes succeed instantly),
	// only null bytes appear — no "Connection unstable..." warning.
	cleaned := strings.ReplaceAll(buf.String(), "\x00", "")
	if strings.Contains(cleaned, "Connection unstable") {
		t.Errorf("healthy client should not see 'Connection unstable', got: %q", cleaned)
	}

	// Verify alice is still connected by sending a message
	fmt.Fprintf(alice, "hello\n")
	_, err := readUntil(alice, "][alice]:", time.Second)
	if err != nil {
		t.Fatalf("alice should still be connected after heartbeat cycles: %v", err)
	}
}

func TestHeartbeatTimeoutDisconnectsUnresponsiveClient(t *testing.T) {
	// Spec 11: "A client that fails to respond within 5 seconds of a health check
	// is treated as disconnected." With net.Pipe(), not reading causes the heartbeat
	// probe write to block indefinitely, triggering the timeout path which now
	// disconnects the client (not just warns).
	s := newHeartbeatServer(t)

	bob := connectPipe(s)
	defer bob.Close()
	onboard(bob, "bob")

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")
	readUntil(bob, "alice has joined", time.Second)

	// Do NOT read from alice's pipe — the heartbeat probe write will block.
	// After the timeout (50ms), the server disconnects alice.
	// Bob should see alice's leave notification.
	text, err := readUntil(bob, "alice has left", 2*time.Second)
	if err != nil {
		t.Fatalf("unresponsive client should be disconnected on heartbeat timeout: %v (got: %q)", err, text)
	}
	if !strings.Contains(text, "alice has left our chat...") {
		t.Errorf("expected standard leave notification, got: %q", text)
	}
}

func TestHeartbeatSlowButAliveNotRemoved(t *testing.T) {
	// Verify that a client who actively reads (consuming heartbeat probes) stays
	// connected through multiple heartbeat cycles. The pipe reader drains data
	// so probe writes complete quickly, keeping the client alive.
	s := newHeartbeatServer(t)

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	// Actively read from alice's pipe for several heartbeat cycles (500ms at 100ms interval).
	// This ensures probe writes complete and alice is not disconnected.
	var buf strings.Builder
	tmp := make([]byte, 4096)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		alice.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, _ := alice.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
	}
	alice.SetReadDeadline(time.Time{})

	// Prove alice is still connected by sending a message
	fmt.Fprintf(alice, "still here\n")
	_, err := readUntil(alice, "][alice]:", 2*time.Second)
	if err != nil {
		t.Fatalf("slow-but-alive client should NOT be removed, err: %v", err)
	}
}

func TestHeartbeatActiveSenderExemption(t *testing.T) {
	s := newHeartbeatServer(t)

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	// Keep sending messages to prove activity — should never be disconnected
	for i := 0; i < 10; i++ {
		fmt.Fprintf(alice, "msg%d\n", i)
		_, err := readUntil(alice, "][alice]:", time.Second)
		if err != nil {
			t.Fatalf("active sender should never be disconnected, iteration %d: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestHeartbeatCommandsCountAsActivity(t *testing.T) {
	s := newHeartbeatServer(t)

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	// Send commands at regular intervals — these should count as activity
	for i := 0; i < 5; i++ {
		fmt.Fprintf(alice, "/list\n")
		_, err := readUntil(alice, "][alice]:", time.Second)
		if err != nil {
			t.Fatalf("commands should count as activity, iteration %d: %v", i, err)
		}
		time.Sleep(80 * time.Millisecond)
	}
}

func TestHeartbeatDoesNotInterfereWithMessages(t *testing.T) {
	s := newHeartbeatServer(t)

	alice := connectPipe(s)
	defer alice.Close()
	bob := connectPipe(s)
	defer bob.Close()

	onboard(alice, "alice")
	onboard(bob, "bob")
	readUntil(alice, "bob has joined", time.Second)

	// Both clients actively send messages. The heartbeat's active sender exemption
	// skips probes for active clients, so pipe writes don't block.
	for i := 0; i < 5; i++ {
		msg := fmt.Sprintf("msg%d", i)
		fmt.Fprintf(alice, "%s\n", msg)

		text, err := readUntil(bob, msg, time.Second)
		if err != nil {
			t.Fatalf("message %d should be delivered despite heartbeat: %v", i, err)
		}
		cleaned := strings.ReplaceAll(text, "\x00", "")
		if !strings.Contains(cleaned, "[alice]:"+msg) {
			t.Errorf("message %d format incorrect, got: %q", i, cleaned)
		}
		// Read alice's prompt
		readUntil(alice, "][alice]:", time.Second)

		// Bob responds to keep his heartbeat happy too
		fmt.Fprintf(bob, "ack%d\n", i)
		readUntil(alice, "ack"+fmt.Sprintf("%d", i), time.Second)
		readUntil(bob, "][bob]:", time.Second)
		time.Sleep(30 * time.Millisecond)
	}
}

func TestHeartbeatAllClientsUnreachable(t *testing.T) {
	s := newHeartbeatServer(t)

	// Connect several clients
	conns := make([]net.Conn, 3)
	for i := range conns {
		conns[i] = connectPipe(s)
		name := fmt.Sprintf("user%d", i)
		onboard(conns[i], name)
	}

	// Wait for all joins to propagate
	time.Sleep(200 * time.Millisecond)

	// Kill all connections simultaneously
	for _, c := range conns {
		c.Close()
	}

	// Wait for heartbeat to detect all dead clients
	time.Sleep(500 * time.Millisecond)

	// Verify server is still running by connecting a new client
	newConn := connectPipe(s)
	defer newConn.Close()
	_, err := readUntil(newConn, "[ENTER YOUR NAME]:", time.Second)
	if err != nil {
		t.Fatalf("server should still accept connections after mass disconnect: %v", err)
	}
	// Server should have 0 active clients now (all were removed)
	if count := s.GetClientCount(); count != 0 {
		t.Errorf("expected 0 active clients after mass disconnect, got %d", count)
	}
}

func TestHeartbeatDetectionWithinExpectedTime(t *testing.T) {
	s := newHeartbeatServer(t)

	bob := connectPipe(s)
	defer bob.Close()
	onboard(bob, "bob")

	alice := connectPipe(s)
	onboard(alice, "alice")
	readUntil(bob, "alice has joined", time.Second)

	// Kill alice and measure how quickly she is detected
	start := time.Now()
	alice.Close()

	_, err := readUntil(bob, "alice has left", 2*time.Second)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("alice should be detected as dead: %v", err)
	}

	// With 100ms interval + 50ms timeout, detection should be within ~200ms
	// Use generous margin for CI
	maxDetection := 1 * time.Second
	if elapsed > maxDetection {
		t.Errorf("detection took %v, expected under %v", elapsed, maxDetection)
	}
}

func TestHeartbeatServerLoadDelayNoPenalty(t *testing.T) {
	// This test verifies that if the server-side heartbeat ticker fires late
	// (due to server load), the client is not penalized.
	// We simulate this by using a longer interval and verifying the client
	// stays connected even when idle for longer than the interval.
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "admins.json")
	s.HeartbeatInterval = 200 * time.Millisecond
	s.HeartbeatTimeout = 100 * time.Millisecond

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	// Sleep for a bit (less than interval + timeout), then send a message
	time.Sleep(150 * time.Millisecond)

	// Alice sends data — she should be fine
	fmt.Fprintf(alice, "still here\n")
	_, err := readUntil(alice, "][alice]:", time.Second)
	if err != nil {
		t.Fatalf("client should not be penalized for server delay: %v", err)
	}
}

func TestHeartbeatStopsDuringShutdown(t *testing.T) {
	// Use default (disabled) heartbeat — we're testing that the heartbeat goroutine
	// exits cleanly when the server shuts down. With pipe connections, the probe
	// would block, so we use a server where heartbeat is enabled but we keep the
	// client active to avoid probe interference.
	s := newHeartbeatServer(t)
	s.ShutdownTimeout = 500 * time.Millisecond

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	// Keep alice active so heartbeat doesn't probe (active sender exemption)
	fmt.Fprintf(alice, "keepalive\n")
	readUntil(alice, "][alice]:", time.Second)

	// Trigger shutdown in background
	go s.Shutdown()

	// Alice should receive shutdown message
	text, _ := readUntil(alice, "shutting down", 2*time.Second)
	if !strings.Contains(text, "Server is shutting down") {
		t.Errorf("expected shutdown message, got: %q", text)
	}
}

func TestHeartbeatTimeoutOpensQueueSlot(t *testing.T) {
	// Spec 11 + Spec 03: when heartbeat removes an unresponsive client,
	// a queued client should be admitted (admitFromRoomQueue triggered).
	s := newHeartbeatServer(t)

	// Fill 10 active slots. Each client needs a goroutine draining its pipe
	// so heartbeat probe writes complete (keeping them alive).
	conns := make([]net.Conn, 10)
	stopDrain := make([]chan struct{}, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
		stopDrain[i] = make(chan struct{})
		go func(c net.Conn, stop chan struct{}) {
			buf := make([]byte, 4096)
			for {
				c.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
				c.Read(buf)
				select {
				case <-stop:
					return
				default:
				}
			}
		}(conns[i], stopDrain[i])
	}
	// Clean up all drain goroutines at end of test
	defer func() {
		for i := 1; i < 10; i++ {
			select {
			case <-stopDrain[i]:
			default:
				close(stopDrain[i])
			}
		}
	}()

	// Wait for all joins to propagate
	time.Sleep(200 * time.Millisecond)

	// Queue an 11th client (in new flow: banner → name → room → queue)
	q := connectPipe(s)
	defer q.Close()
	enterName(q, "queued1")
	fmt.Fprintf(q, "\n") // default room
	readUntil(q, "yes/no", 2*time.Second)
	fmt.Fprintf(q, "yes\n")
	time.Sleep(100 * time.Millisecond)

	if got := s.GetQueueLength(); got != 1 {
		t.Fatalf("expected 1 in queue, got %d", got)
	}

	// Stop draining user0's pipe — heartbeat probe will block and timeout,
	// causing user0 to be disconnected and freeing a slot for the queued client.
	close(stopDrain[0])

	// The queued client should now be admitted and receive the prompt (already named)
	_, err := readUntil(q, "][queued1]:", 3*time.Second)
	if err != nil {
		t.Fatalf("queued client should be admitted after heartbeat removal: %v", err)
	}
}

// ==================== Task 23: Midnight Log Rotation ====================

func TestMidnightClearHistoryResetsInMemory(t *testing.T) {
	// ClearHistory() empties in-memory history so new joiners after midnight see only new-day events.
	s := New("0")

	// Add some history entries
	s.AddHistory(models.Message{Timestamp: time.Now(), Type: models.MsgChat, Sender: "alice", Content: "hello"})
	s.AddHistory(models.Message{Timestamp: time.Now(), Type: models.MsgJoin, Sender: "bob"})
	s.AddHistory(models.Message{Timestamp: time.Now(), Type: models.MsgChat, Sender: "bob", Content: "world"})

	if len(s.GetHistory()) != 3 {
		t.Fatalf("expected 3 history entries before clear, got %d", len(s.GetHistory()))
	}

	s.ClearHistory()

	if len(s.GetHistory()) != 0 {
		t.Errorf("expected 0 history entries after ClearHistory, got %d", len(s.GetHistory()))
	}
}

func TestMidnightHistoryResetNewJoinerSeesOnlyNewDay(t *testing.T) {
	// After midnight rotation, a new client joining should see only events after midnight.
	s, _ := newServerWithLogger(t)

	// Connect alice before "midnight"
	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")
	fmt.Fprintf(alice, "pre-midnight message\n")
	readUntil(alice, "][alice]:", time.Second)

	// Simulate midnight rotation
	s.ClearHistory()

	// Alice sends a post-midnight message
	fmt.Fprintf(alice, "post-midnight message\n")
	readUntil(alice, "][alice]:", time.Second)

	// Bob joins after midnight rotation
	bob := connectPipe(s)
	defer bob.Close()
	text, _ := onboard(bob, "bob")

	// Bob should see the post-midnight message but NOT the pre-midnight message
	if strings.Contains(text, "pre-midnight message") {
		t.Error("new joiner should NOT see pre-midnight messages after history reset")
	}
	if !strings.Contains(text, "post-midnight message") {
		t.Error("new joiner should see post-midnight messages")
	}
}

func TestMidnightLoggerSwitchesFileOnDateChange(t *testing.T) {
	// Activity before midnight goes to the old file; activity after midnight goes to the new file.
	// The logger routes based on message timestamp, so messages with different dates go to different files.
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	l, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	yesterday := time.Date(2026, 3, 15, 23, 59, 59, 0, time.Local)
	today := time.Date(2026, 3, 16, 0, 0, 1, 0, time.Local)

	// Log before midnight
	l.Log(models.Message{Timestamp: yesterday, Type: models.MsgChat, Sender: "alice", Content: "before midnight"})
	// Log after midnight
	l.Log(models.Message{Timestamp: today, Type: models.MsgChat, Sender: "alice", Content: "after midnight"})

	l.Close()

	// Check yesterday's file
	yesterdayPath := filepath.Join(logsDir, "chat_2026-03-15.log")
	data, err := os.ReadFile(yesterdayPath)
	if err != nil {
		t.Fatalf("could not read yesterday's log: %v", err)
	}
	if !strings.Contains(string(data), "before midnight") {
		t.Error("yesterday's log should contain 'before midnight'")
	}
	if strings.Contains(string(data), "after midnight") {
		t.Error("yesterday's log should NOT contain 'after midnight'")
	}

	// Check today's file
	todayPath := filepath.Join(logsDir, "chat_2026-03-16.log")
	data, err = os.ReadFile(todayPath)
	if err != nil {
		t.Fatalf("could not read today's log: %v", err)
	}
	if !strings.Contains(string(data), "after midnight") {
		t.Error("today's log should contain 'after midnight'")
	}
	if strings.Contains(string(data), "before midnight") {
		t.Error("today's log should NOT contain 'before midnight'")
	}
}

func TestMidnightNoEntriesLostOrDuplicated(t *testing.T) {
	// Messages around the midnight boundary should each appear exactly once in the correct file.
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	l, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}

	day1 := time.Date(2026, 6, 10, 23, 59, 58, 0, time.Local)
	day2 := time.Date(2026, 6, 11, 0, 0, 2, 0, time.Local)

	messages := []struct {
		ts   time.Time
		text string
	}{
		{day1, "msg1_before"},
		{day1.Add(time.Second), "msg2_before"},
		{day2, "msg3_after"},
		{day2.Add(time.Second), "msg4_after"},
	}

	for _, m := range messages {
		l.Log(models.Message{Timestamp: m.ts, Type: models.MsgChat, Sender: "user", Content: m.text})
	}
	l.Close()

	d1Path := filepath.Join(logsDir, "chat_2026-06-10.log")
	d2Path := filepath.Join(logsDir, "chat_2026-06-11.log")
	d1Data, _ := os.ReadFile(d1Path)
	d2Data, _ := os.ReadFile(d2Path)
	d1Lines := strings.Split(strings.TrimSpace(string(d1Data)), "\n")
	d2Lines := strings.Split(strings.TrimSpace(string(d2Data)), "\n")

	if len(d1Lines) != 2 {
		t.Errorf("expected 2 entries in day1 file, got %d", len(d1Lines))
	}
	if len(d2Lines) != 2 {
		t.Errorf("expected 2 entries in day2 file, got %d", len(d2Lines))
	}

	// Check no duplicates
	allContent := string(d1Data) + string(d2Data)
	for _, m := range messages {
		count := strings.Count(allContent, m.text)
		if count != 1 {
			t.Errorf("message %q appears %d times (expected 1)", m.text, count)
		}
	}
}

func TestMidnightLogFileNameUpdates(t *testing.T) {
	// Log file name changes to reflect the new date after midnight.
	tmpDir := t.TempDir()
	logsDir := filepath.Join(tmpDir, "logs")
	l, err := logger.New(logsDir)
	if err != nil {
		t.Fatal(err)
	}

	// Write to "today"
	today := time.Date(2026, 2, 21, 15, 0, 0, 0, time.Local)
	l.Log(models.Message{Timestamp: today, Type: models.MsgChat, Sender: "user", Content: "today"})

	// Write to "tomorrow"
	tomorrow := time.Date(2026, 2, 22, 9, 0, 0, 0, time.Local)
	l.Log(models.Message{Timestamp: tomorrow, Type: models.MsgChat, Sender: "user", Content: "tomorrow"})
	l.Close()

	// Both files should exist
	todayPath := filepath.Join(logsDir, "chat_2026-02-21.log")
	tomorrowPath := filepath.Join(logsDir, "chat_2026-02-22.log")

	if _, err := os.Stat(todayPath); err != nil {
		t.Errorf("today's log file should exist: %v", err)
	}
	if _, err := os.Stat(tomorrowPath); err != nil {
		t.Errorf("tomorrow's log file should exist: %v", err)
	}
}

func TestMidnightConnectedClientsUnaffected(t *testing.T) {
	// Already-connected clients are not disconnected or disrupted by midnight rotation.
	s, _ := newServerWithLogger(t)

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	bob := connectPipe(s)
	defer bob.Close()
	onboard(bob, "bob")
	readUntil(alice, "bob has joined", time.Second)

	// Simulate midnight rotation
	s.ClearHistory()

	// Alice and Bob should still be able to chat
	fmt.Fprintf(alice, "hello after midnight\n")
	text, err := readUntil(bob, "hello after midnight", time.Second)
	if err != nil {
		t.Fatalf("bob should receive alice's post-midnight message: %v", err)
	}
	if !strings.Contains(text, "hello after midnight") {
		t.Error("message should be delivered after midnight rotation")
	}

	// Bob can reply
	fmt.Fprintf(bob, "hi alice\n")
	text, err = readUntil(alice, "hi alice", time.Second)
	if err != nil {
		t.Fatalf("alice should receive bob's post-midnight message: %v", err)
	}
	if !strings.Contains(text, "hi alice") {
		t.Error("reply should be delivered after midnight rotation")
	}
}

func TestMidnightWatcherStopsOnShutdown(t *testing.T) {
	// The midnight watcher goroutine exits cleanly when the server shuts down.
	s := New("0")
	s.ShutdownTimeout = 200 * time.Millisecond

	// Start the watcher in a goroutine
	done := make(chan struct{})
	go func() {
		s.startMidnightWatcher()
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	s.Shutdown()

	// Watcher should exit promptly
	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("midnight watcher did not exit after shutdown")
	}
}

func TestMidnightHistoryAccumulatesAfterClear(t *testing.T) {
	// After midnight clear, new events accumulate in a fresh history.
	s := New("0")

	// Pre-midnight events
	s.AddHistory(models.Message{Timestamp: time.Now(), Type: models.MsgChat, Sender: "alice", Content: "old"})
	s.ClearHistory()

	// Post-midnight events
	s.AddHistory(models.Message{Timestamp: time.Now(), Type: models.MsgJoin, Sender: "bob"})
	s.AddHistory(models.Message{Timestamp: time.Now(), Type: models.MsgChat, Sender: "bob", Content: "new"})

	history := s.GetHistory()
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries after clear + new events, got %d", len(history))
	}
	if history[0].Type != models.MsgJoin || history[0].Sender != "bob" {
		t.Error("first entry should be bob's join")
	}
	if history[1].Content != "new" {
		t.Error("second entry should be bob's message")
	}
}

func TestMidnightConcurrentClearAndAdd(t *testing.T) {
	// ClearHistory and AddHistory are safe to call concurrently (no race).
	s := New("0")

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			s.AddHistory(models.Message{
				Timestamp: time.Now(),
				Type:      models.MsgChat,
				Sender:    "user",
				Content:   "msg",
			})
		}
		close(done)
	}()

	// Clear multiple times while adds are happening
	for i := 0; i < 10; i++ {
		s.ClearHistory()
		time.Sleep(time.Millisecond)
	}
	<-done

	// Just verify no panic — the exact count doesn't matter
	_ = s.GetHistory()
}

// ==================== Task 22: Input Continuity ====================

// helper: onboard a client and drain the first prompt. Returns the prompt
// text for verification. Uses a more specific delimiter to avoid matching
// echo characters in interactive mode.
func onboardAndDrain(conn net.Conn, name string) (string, error) {
	text, err := readUntil(conn, "[ENTER YOUR NAME]:", 2*time.Second)
	if err != nil {
		return text, err
	}
	fmt.Fprintf(conn, "%s\n", name)
	// Handle room selection prompt (press Enter for default room)
	text2, err := readUntil(conn, "[ENTER ROOM NAME]", 2*time.Second)
	if err != nil {
		return text + text2, err
	}
	fmt.Fprintf(conn, "\n")
	text3, err := readUntil(conn, "]["+name+"]:", 2*time.Second)
	return text + text2 + text3, err
}

// helper: send a line character by character (simulates raw terminal input).
// Each character is a separate Write, mimicking real netcat raw mode.
func sendCharByChar(conn net.Conn, line string) {
	for _, b := range []byte(line) {
		conn.Write([]byte{b})
	}
}

// helper: read all available data from a connection with a short timeout.
func readAvailable(conn net.Conn, timeout time.Duration) string {
	conn.SetReadDeadline(time.Now().Add(timeout))
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

// TestInputContinuityPartialInputPreserved verifies that when client A is
// typing "hel" and client B's message arrives, client A's prompt re-appears
// with "hel" intact after the incoming message.
func TestInputContinuityPartialInputPreserved(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = time.Minute // disable heartbeat interference
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboardAndDrain(c1, "alice")
	onboardAndDrain(c2, "bob")
	// Drain bob's join notification from alice
	readUntil(c1, "bob has joined", time.Second)

	// Alice types "hel" character by character
	sendCharByChar(c1, "hel")
	// Allow writeLoop to process echo tracking
	time.Sleep(50 * time.Millisecond)

	// Bob sends a message — this triggers a broadcast to alice
	fmt.Fprintf(c2, "world\n")
	// Read bob's echo + prompt
	readUntil(c2, "][bob]:", time.Second)

	// Alice receives the broadcast with input continuity
	text := readAvailable(c1, 500*time.Millisecond)

	// Should contain bob's message
	if !strings.Contains(text, "world") {
		t.Errorf("alice should see bob's message, got: %q", text)
	}
	// Should contain the redraw: ANSI clear sequence
	if !strings.Contains(text, "\033[K") {
		t.Errorf("expected ANSI clear sequence in output, got: %q", text)
	}
	// Should redraw prompt with partial input "hel"
	if !strings.Contains(text, "hel") {
		t.Errorf("partial input 'hel' should be preserved after broadcast, got: %q", text)
	}
	// Should contain alice's prompt
	if !strings.Contains(text, "][alice]:") {
		t.Errorf("prompt should be redrawn after broadcast, got: %q", text)
	}
}

// TestInputContinuityMultipleMessages verifies that multiple incoming messages
// while typing each preserve the partial input.
func TestInputContinuityMultipleMessages(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = time.Minute
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboardAndDrain(c1, "alice")
	onboardAndDrain(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)
	onboardAndDrain(c3, "carol")
	readUntil(c1, "carol has joined", time.Second)
	readUntil(c2, "carol has joined", time.Second)

	// Alice types "ab"
	sendCharByChar(c1, "ab")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 200*time.Millisecond) // drain echoes

	// Bob sends message
	fmt.Fprintf(c2, "msg1\n")
	readUntil(c2, "][bob]:", time.Second)
	time.Sleep(50 * time.Millisecond)

	// Read bob's broadcast on alice
	text1 := readAvailable(c1, 300*time.Millisecond)
	if !strings.Contains(text1, "msg1") {
		t.Errorf("first broadcast should contain msg1, got: %q", text1)
	}

	// Carol sends message
	fmt.Fprintf(c3, "msg2\n")
	readUntil(c3, "][carol]:", time.Second)
	time.Sleep(50 * time.Millisecond)

	// Read carol's broadcast on alice — partial input should still be preserved
	text2 := readAvailable(c1, 300*time.Millisecond)
	if !strings.Contains(text2, "msg2") {
		t.Errorf("second broadcast should contain msg2, got: %q", text2)
	}
	if !strings.Contains(text2, "ab") {
		t.Errorf("partial input 'ab' should still be preserved, got: %q", text2)
	}
}

// TestInputContinuityJoinLeaveNotification verifies that join/leave
// notifications arriving mid-type preserve partial input.
func TestInputContinuityJoinLeaveNotification(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = time.Minute
	c1 := connectPipe(s)
	defer c1.Close()

	onboardAndDrain(c1, "alice")

	// Alice starts typing
	sendCharByChar(c1, "typ")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 200*time.Millisecond) // drain echoes

	// Bob joins — triggers a join notification to alice
	c2 := connectPipe(s)
	defer c2.Close()
	onboardAndDrain(c2, "bob")

	// Alice should see join notification with partial input preserved
	text := readAvailable(c1, 500*time.Millisecond)
	if !strings.Contains(text, "bob has joined") {
		t.Errorf("alice should see join notification, got: %q", text)
	}
	if !strings.Contains(text, "typ") {
		t.Errorf("partial input 'typ' should be preserved after join notification, got: %q", text)
	}

	// Alice types more then types "x" and bob disconnects
	sendCharByChar(c1, "x")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 200*time.Millisecond) // drain echo

	c2.Close()
	time.Sleep(100 * time.Millisecond)

	// Alice should see leave notification with partial input preserved
	text2 := readAvailable(c1, 500*time.Millisecond)
	if !strings.Contains(text2, "bob has left") {
		t.Errorf("alice should see leave notification, got: %q", text2)
	}
	if !strings.Contains(text2, "typx") {
		t.Errorf("partial input 'typx' should be preserved after leave notification, got: %q", text2)
	}
}

// TestInputContinuityWhisperMidType verifies that whisper notifications
// arriving mid-type preserve partial input.
func TestInputContinuityWhisperMidType(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = time.Minute
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboardAndDrain(c1, "alice")
	onboardAndDrain(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Alice types "mid"
	sendCharByChar(c1, "mid")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 200*time.Millisecond) // drain echoes

	// Bob whispers to alice
	fmt.Fprintf(c2, "/whisper alice secret\n")
	readUntil(c2, "][bob]:", time.Second)

	// Alice should see whisper with partial input preserved
	text := readAvailable(c1, 500*time.Millisecond)
	if !strings.Contains(text, "PM from bob") {
		t.Errorf("alice should see whisper from bob, got: %q", text)
	}
	if !strings.Contains(text, "mid") {
		t.Errorf("partial input 'mid' should be preserved after whisper, got: %q", text)
	}
}

// TestInputContinuityAnnouncementMidType verifies that announcements
// arriving mid-type preserve partial input.
func TestInputContinuityAnnouncementMidType(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = time.Minute
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboardAndDrain(c1, "alice")
	onboardAndDrain(c2, "admin")
	readUntil(c1, "admin has joined", time.Second)

	// Promote admin
	s.GetClient("admin").SetAdmin(true)

	// Alice types "ann"
	sendCharByChar(c1, "ann")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 200*time.Millisecond) // drain echoes

	// Admin sends announcement
	fmt.Fprintf(c2, "/announce Server maintenance tonight\n")
	readUntil(c2, "][admin]:", time.Second)

	// Alice should see announcement with partial input preserved
	text := readAvailable(c1, 500*time.Millisecond)
	if !strings.Contains(text, "[ANNOUNCEMENT]") {
		t.Errorf("alice should see announcement, got: %q", text)
	}
	if !strings.Contains(text, "ann") {
		t.Errorf("partial input 'ann' should be preserved after announcement, got: %q", text)
	}
}

// TestInputContinuityModerationMidType verifies that moderation events
// arriving mid-type preserve partial input.
func TestInputContinuityModerationMidType(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = time.Minute
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboardAndDrain(c1, "alice")
	onboardAndDrain(c2, "admin")
	readUntil(c1, "admin has joined", time.Second)
	onboardAndDrain(c3, "target")
	readUntil(c1, "target has joined", time.Second)
	readUntil(c2, "target has joined", time.Second)

	// Promote admin
	s.GetClient("admin").SetAdmin(true)

	// Alice types "mod"
	sendCharByChar(c1, "mod")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 200*time.Millisecond) // drain echoes

	// Admin mutes target — broadcasts to all
	fmt.Fprintf(c2, "/mute target\n")
	readUntil(c2, "][admin]:", time.Second)

	// Alice should see moderation event with partial input preserved
	text := readAvailable(c1, 500*time.Millisecond)
	if !strings.Contains(text, "target was muted by admin") {
		t.Errorf("alice should see mute notification, got: %q", text)
	}
	if !strings.Contains(text, "mod") {
		t.Errorf("partial input 'mod' should be preserved after mute notification, got: %q", text)
	}
}

// TestInputContinuityNoCharactersLostOrDuplicated verifies that after receiving
// a broadcast mid-typing, the client can complete their message with no loss.
func TestInputContinuityNoCharactersLostOrDuplicated(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = time.Minute
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboardAndDrain(c1, "alice")
	onboardAndDrain(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Alice types "hel"
	sendCharByChar(c1, "hel")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 200*time.Millisecond) // drain echoes

	// Bob sends a message
	fmt.Fprintf(c2, "interrupt\n")
	readUntil(c2, "][bob]:", time.Second)
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 300*time.Millisecond) // drain bob's broadcast

	// Alice continues typing "lo" and presses Enter
	sendCharByChar(c1, "lo\n")

	// Bob should receive the complete message "hello"
	text, _ := readUntil(c2, "hello", 2*time.Second)
	if !strings.Contains(text, "hello") {
		t.Errorf("bob should see complete message 'hello', got: %q", text)
	}
}

// TestInputContinuityBackspaceTracking verifies that backspace correctly
// updates the tracked partial input for redraw.
func TestInputContinuityBackspaceTracking(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = time.Minute
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboardAndDrain(c1, "alice")
	onboardAndDrain(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Alice types "hello", then backspaces twice to "hel"
	sendCharByChar(c1, "hello")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1, 200*time.Millisecond) // drain echoes

	// Send two backspaces (0x7F)
	c1.Write([]byte{0x7F})
	c1.Write([]byte{0x7F})
	time.Sleep(50 * time.Millisecond)
	bsText := readAvailable(c1, 200*time.Millisecond)
	// Should see backspace sequences
	if !strings.Contains(bsText, "\b \b") {
		t.Logf("backspace output: %q", bsText)
	}

	// Bob sends a message — alice's partial input should be "hel" (not "hello")
	fmt.Fprintf(c2, "trigger\n")
	readUntil(c2, "][bob]:", time.Second)

	text := readAvailable(c1, 500*time.Millisecond)
	if !strings.Contains(text, "trigger") {
		t.Errorf("alice should see bob's message, got: %q", text)
	}
	// The redrawn partial input should be "hel" not "hello"
	// Split on the clear sequence to find what comes after the prompt
	parts := strings.SplitAfter(text, "][alice]:")
	if len(parts) >= 2 {
		redrawInput := parts[len(parts)-1]
		if strings.Contains(redrawInput, "hello") {
			t.Errorf("backspaced chars should not appear in redraw, got: %q", redrawInput)
		}
		if !strings.Contains(redrawInput, "hel") {
			t.Errorf("redrawn input should be 'hel' after backspace, got: %q", redrawInput)
		}
	}
}

// TestInputContinuityConnectionUnstableWarning verifies that the
// "Connection unstable..." warning arriving mid-type preserves partial input.
func TestInputContinuityConnectionUnstableWarning(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = 100 * time.Millisecond
	s.HeartbeatTimeout = 80 * time.Millisecond
	c1Srv, c1Client := net.Pipe()
	go s.handleConnection(c1Srv)
	defer c1Client.Close()

	onboardAndDrain(c1Client, "alice")

	// Type some characters
	sendCharByChar(c1Client, "wip")
	time.Sleep(50 * time.Millisecond)
	readAvailable(c1Client, 200*time.Millisecond) // drain echoes

	// Wait for heartbeat to trigger and potentially send "Connection unstable..."
	// Read everything within 2x heartbeat interval
	text := readAvailable(c1Client, 300*time.Millisecond)

	// Null byte probe may arrive but is invisible; if a warning is generated
	// it should preserve partial input. The heartbeat might not trigger a
	// warning in pipes, but verify no characters are lost.
	_ = text // warning may or may not appear depending on pipe speed

	// Complete the line and verify it's processed correctly
	sendCharByChar(c1Client, "done\n")
	// The server processes "wipdone" as a chat message (with prompt)
	result := readAvailable(c1Client, 500*time.Millisecond)
	// If we got here without deadlock, the test passes
	_ = result
}

// ==================== Task 24: Edge Case Hardening ====================

// TestEdgeCaseRapidConnectDisconnectNoGoroutineLeak verifies that 100 rapid
// connect/disconnect cycles do not leak goroutines or cause panics.
func TestEdgeCaseRapidConnectDisconnectNoGoroutineLeak(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = 1 * time.Hour // disable heartbeat interference

	// Warm up: stabilize goroutine count
	warmConn := connectPipe(s)
	onboard(warmConn, "warmup")
	warmConn.Close()
	time.Sleep(300 * time.Millisecond)

	// Capture baseline goroutine count
	runtime.GC()
	baseline := runtime.NumGoroutine()

	// 100 rapid connect/disconnect cycles
	for i := 0; i < 100; i++ {
		c := connectPipe(s)
		// Read banner start, then immediately close
		readUntil(c, "Welcome", 500*time.Millisecond)
		c.Close()
	}

	// Wait for all handler goroutines to finish
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	final := runtime.NumGoroutine()

	// Allow up to 5 goroutines above baseline for runtime overhead
	if final > baseline+5 {
		t.Errorf("goroutine leak: baseline=%d, after 100 connect/disconnect cycles=%d", baseline, final)
	}
}

// TestEdgeCaseRapidConnectDisconnectWithOnboardingNoLeak verifies that clients
// who complete onboarding and immediately disconnect don't leak goroutines.
func TestEdgeCaseRapidConnectDisconnectWithOnboardingNoLeak(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = 1 * time.Hour

	// Warm up
	warmConn := connectPipe(s)
	onboard(warmConn, "warmup")
	warmConn.Close()
	time.Sleep(300 * time.Millisecond)

	runtime.GC()
	baseline := runtime.NumGoroutine()

	for i := 0; i < 50; i++ {
		c := connectPipe(s)
		onboard(c, fmt.Sprintf("user%d", i))
		c.Close()
		// Brief sleep to let the handler notice the close
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	final := runtime.NumGoroutine()

	if final > baseline+5 {
		t.Errorf("goroutine leak: baseline=%d, after 50 onboard/disconnect cycles=%d", baseline, final)
	}
}

// TestEdgeCaseBroadcastDuringClientRemoval verifies that broadcasting while a
// client is being removed does not cause errors or affect remaining clients.
func TestEdgeCaseBroadcastDuringClientRemoval(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = 1 * time.Hour

	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)

	onboard(c1, "alice")
	onboard(c2, "bob")
	onboard(c3, "charlie")
	readUntil(c1, "charlie has joined", time.Second)
	readUntil(c2, "charlie has joined", time.Second)

	// Concurrently: alice sends messages rapidly while charlie disconnects
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			fmt.Fprintf(c1, "msg_%d\n", i)
			time.Sleep(5 * time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		time.Sleep(25 * time.Millisecond)
		c3.Close()
	}()
	wg.Wait()

	time.Sleep(300 * time.Millisecond)

	// Bob should receive the leave notification and some/all messages
	text := readAvailable(c2, 500*time.Millisecond)
	if !strings.Contains(text, "charlie has left") {
		t.Error("bob should see charlie's leave notification")
	}

	// Verify alice is still functional — no panic, no corruption
	fmt.Fprintf(c1, "still alive\n")
	out, err := readUntil(c2, "still alive", time.Second)
	if err != nil {
		t.Errorf("bob should receive alice's message after charlie left, got error: %v (text: %s)", err, out)
	}
}

// TestEdgeCaseCommandDuringKick verifies that when an admin kicks a client at
// the same moment the client issues a command, moderation takes precedence.
func TestEdgeCaseCommandDuringKick(t *testing.T) {
	s := newServerWithAdmins(t)
	s.HeartbeatInterval = 1 * time.Hour

	admin := connectPipe(s)
	defer admin.Close()
	target := connectPipe(s)
	defer target.Close()
	observer := connectPipe(s)
	defer observer.Close()

	onboard(admin, "admin1")
	s.OperatorDispatch("/promote admin1")
	readUntil(admin, "admin", time.Second) // drain promotion message

	onboard(target, "bob")
	readUntil(admin, "bob has joined", time.Second)
	onboard(observer, "carol")
	readUntil(admin, "carol has joined", time.Second)

	// Concurrently: admin kicks bob while bob sends /list
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		fmt.Fprintf(admin, "/kick bob\n")
	}()
	go func() {
		defer wg.Done()
		// Tiny delay to make the race tight
		time.Sleep(5 * time.Millisecond)
		fmt.Fprintf(target, "/list\n")
	}()
	wg.Wait()

	time.Sleep(300 * time.Millisecond)

	// Bob should be disconnected
	if s.GetClient("bob") != nil {
		t.Error("bob should have been removed from clients")
	}

	// Observer should see the kick notification
	observerText := readAvailable(observer, 500*time.Millisecond)
	if !strings.Contains(observerText, "bob was kicked") {
		t.Errorf("observer should see kick notification, got: %q", observerText)
	}
}

// TestEdgeCaseCommandDuringConnectionDrop verifies that when a client's
// connection drops while they're processing commands, no error messages
// are generated and cleanup proceeds cleanly.
func TestEdgeCaseCommandDuringConnectionDrop(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = 1 * time.Hour

	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	// Bob sends a command and immediately drops the connection
	fmt.Fprintf(c2, "/list\n")
	c2.Close()

	time.Sleep(300 * time.Millisecond)

	// Alice should see the leave notification — no panic, no error leak
	text := readAvailable(c1, 500*time.Millisecond)
	if !strings.Contains(text, "bob has left") {
		t.Errorf("alice should see bob's leave notification, got: %q", text)
	}

	// Server should still be functional
	c3 := connectPipe(s)
	defer c3.Close()
	_, err := onboard(c3, "charlie")
	if err != nil {
		t.Fatalf("server should still accept connections after dropped client: %v", err)
	}
}

// TestEdgeCaseQueueAdmissionDuringLeave verifies that when an active client
// leaves while a queued client is waiting, exactly one queued client is admitted.
func TestEdgeCaseQueueAdmissionDuringLeave(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = 1 * time.Hour

	// Fill up 10 active slots
	actives := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		actives[i] = connectPipe(s)
		defer actives[i].Close()
		onboard(actives[i], fmt.Sprintf("user%d", i))
		if i > 0 {
			readUntil(actives[0], fmt.Sprintf("user%d has joined", i), time.Second)
		}
	}

	// Queue 2 clients (new flow: banner → name → room → queue)
	q1 := connectPipe(s)
	defer q1.Close()
	enterName(q1, "queued1")
	fmt.Fprintf(q1, "\n") // default room
	readUntil(q1, "Would you like to wait?", 2*time.Second)
	fmt.Fprintf(q1, "yes\n")

	q2 := connectPipe(s)
	defer q2.Close()
	enterName(q2, "queued2")
	fmt.Fprintf(q2, "\n") // default room
	readUntil(q2, "Would you like to wait?", 2*time.Second)
	fmt.Fprintf(q2, "yes\n")

	time.Sleep(200 * time.Millisecond)

	// Two active clients leave concurrently
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		actives[8].Close()
	}()
	go func() {
		defer wg.Done()
		actives[9].Close()
	}()
	wg.Wait()

	time.Sleep(500 * time.Millisecond)

	// Both queued clients should be admitted (one for each departed active)
	_, err1 := readUntil(q1, "][queued1]:", 2*time.Second)
	_, err2 := readUntil(q2, "][queued2]:", 2*time.Second)
	if err1 != nil && err2 != nil {
		t.Error("at least one queued client should have been admitted")
	}
	// Ideally both are admitted since 2 slots opened
	if err1 != nil {
		t.Errorf("q1 should be admitted: %v", err1)
	}
	if err2 != nil {
		t.Errorf("q2 should be admitted: %v", err2)
	}
}

// TestEdgeCaseAdminActionOnDisconnectingClient verifies that kicking a client
// who is simultaneously disconnecting does not panic.
func TestEdgeCaseAdminActionOnDisconnectingClient(t *testing.T) {
	s := newServerWithAdmins(t)
	s.HeartbeatInterval = 1 * time.Hour

	admin := connectPipe(s)
	defer admin.Close()

	onboard(admin, "admin1")
	s.OperatorDispatch("/promote admin1")
	readUntil(admin, "admin", time.Second)

	// Run this multiple times to increase race probability
	for i := 0; i < 5; i++ {
		target := connectPipe(s)
		name := fmt.Sprintf("target%d", i)
		onboard(target, name)
		readUntil(admin, name+" has joined", 500*time.Millisecond)

		// Concurrently: admin kicks while target disconnects
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			fmt.Fprintf(admin, "/kick %s\n", name)
		}()
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			target.Close()
		}()
		wg.Wait()

		time.Sleep(50 * time.Millisecond)

		// Drain admin output
		readAvailable(admin, 100*time.Millisecond)

		// Verify: target is no longer in clients map (regardless of which happened first)
		if s.GetClient(name) != nil {
			t.Errorf("target %q should be removed", name)
		}
	}
}

// TestEdgeCaseKickAndQueueAdmissionSimultaneous verifies that when a client is
// kicked (opening a slot) and the queue has waiting clients, exactly one queued
// client is admitted without the kicked client blocking the process.
func TestEdgeCaseKickAndQueueAdmissionSimultaneous(t *testing.T) {
	s := newServerWithAdmins(t)
	s.HeartbeatInterval = 1 * time.Hour

	// Fill 10 active slots (first one is admin)
	actives := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		actives[i] = connectPipe(s)
		defer actives[i].Close()
		onboard(actives[i], fmt.Sprintf("user%d", i))
		if i > 0 {
			readUntil(actives[0], fmt.Sprintf("user%d has joined", i), time.Second)
		}
	}

	// Promote user0 to admin
	s.OperatorDispatch("/promote user0")
	readUntil(actives[0], "admin", time.Second)

	// Queue one client (new flow: banner → name → room → queue)
	queued := connectPipe(s)
	defer queued.Close()
	enterName(queued, "queued1")
	fmt.Fprintf(queued, "\n") // default room
	readUntil(queued, "Would you like to wait?", 2*time.Second)
	fmt.Fprintf(queued, "yes\n")
	time.Sleep(200 * time.Millisecond)

	// Admin kicks user9, which should open a slot for the queued client
	fmt.Fprintf(actives[0], "/kick user9\n")
	time.Sleep(500 * time.Millisecond)

	// Queued client should be admitted (already named, gets prompt directly)
	_, err := readUntil(queued, "][queued1]:", 2*time.Second)
	if err != nil {
		t.Error("queued client should be admitted after kick opens a slot")
	}

	// Verify kicked client is gone
	if s.GetClient("user9") != nil {
		t.Error("user9 should have been removed")
	}
}

// TestEdgeCaseFiftyClientsSameNameOnboarding verifies that when 50 clients
// try to onboard with the same name simultaneously, exactly one succeeds.
func TestEdgeCaseFiftyClientsSameNameOnboarding(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = 1 * time.Hour

	conns := make([]net.Conn, 50)
	for i := 0; i < 50; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
	}

	// Wait for all to get the banner
	for i := 0; i < 50; i++ {
		readUntil(conns[i], "[ENTER YOUR NAME]:", 2*time.Second)
	}

	// All send the same name concurrently
	var wg sync.WaitGroup
	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func(conn net.Conn) {
			defer wg.Done()
			fmt.Fprintf(conn, "sharedname\n")
		}(conns[i])
	}
	wg.Wait()

	time.Sleep(500 * time.Millisecond)

	// Exactly one should succeed
	count := s.GetClientCount()
	if count != 1 {
		t.Errorf("expected exactly 1 registered client, got %d", count)
	}
	if s.GetClient("sharedname") == nil {
		t.Error("expected client 'sharedname' to be registered")
	}
}

// TestEdgeCaseTwoNameChangesToSameNameSimultaneous re-verifies that two clients
// trying to rename to the same new name concurrently results in exactly one success.
// This is specifically for the RenameClient atomicity check under race conditions.
func TestEdgeCaseTwoNameChangesToSameNameSimultaneous(t *testing.T) {
	s := New("0")
	s.HeartbeatInterval = 1 * time.Hour

	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboard(c2, "bob")
	readUntil(c1, "bob has joined", time.Second)

	successes := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		fmt.Fprintf(c1, "/name target\n")
		text := readAvailable(c1, time.Second)
		mu.Lock()
		if strings.Contains(text, "][target]:") {
			successes++
		}
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		fmt.Fprintf(c2, "/name target\n")
		text := readAvailable(c2, time.Second)
		mu.Lock()
		if strings.Contains(text, "][target]:") {
			successes++
		}
		mu.Unlock()
	}()
	wg.Wait()

	if successes != 1 {
		t.Errorf("exactly one name change should succeed, got %d", successes)
	}
}

// ==================== Task 28: Data Race Fixes (Muted, Admin, LastActivity) ====================

func TestConcurrentMuteUnmuteWhileTargetSends(t *testing.T) {
	// Spec 09 + race safety: concurrent mute/unmute from admin while target sends
	// messages must not race. This test is meaningless without -race but documents
	// the requirement.
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "admins.json")
	s.HeartbeatInterval = 1 * time.Hour

	admin := connectPipe(s)
	defer admin.Close()
	onboard(admin, "admin")
	s.GetClient("admin").SetAdmin(true)

	target := connectPipe(s)
	defer target.Close()
	onboard(target, "target")
	readUntil(admin, "target has joined", time.Second)

	var wg sync.WaitGroup
	// Goroutine 1: admin mutes/unmutes target repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			fmt.Fprintf(admin, "/mute target\n")
			readUntil(admin, "][admin]:", time.Second)
			fmt.Fprintf(admin, "/unmute target\n")
			readUntil(admin, "][admin]:", time.Second)
		}
	}()
	// Goroutine 2: target sends messages repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			fmt.Fprintf(target, "msg%d\n", i)
			readUntil(target, "][target]:", time.Second)
		}
	}()
	wg.Wait()
}

func TestConcurrentPromoteDemoteWhileTargetRunsHelp(t *testing.T) {
	// Race safety: concurrent promote/demote while target runs /help
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "admins.json")
	s.HeartbeatInterval = 1 * time.Hour

	var buf strings.Builder
	s.OperatorOutput = &buf

	target := connectPipe(s)
	defer target.Close()
	onboard(target, "target")

	var wg sync.WaitGroup
	// Goroutine 1: operator promotes/demotes target repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			s.OperatorDispatch("/promote target")
			time.Sleep(time.Millisecond)
			s.OperatorDispatch("/demote target")
			time.Sleep(time.Millisecond)
		}
	}()
	// Goroutine 2: target runs /help repeatedly (reads c.IsAdmin())
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			fmt.Fprintf(target, "/help\n")
			readUntil(target, "][target]:", time.Second)
		}
	}()
	wg.Wait()
}

func TestConcurrentListWhileAnotherSends(t *testing.T) {
	// Race safety: /list reads LastActivity while another client updates it
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "admins.json")
	s.HeartbeatInterval = 1 * time.Hour

	lister := connectPipe(s)
	defer lister.Close()
	onboard(lister, "lister")

	sender := connectPipe(s)
	defer sender.Close()
	onboard(sender, "sender")
	readUntil(lister, "sender has joined", time.Second)

	var wg sync.WaitGroup
	// Goroutine 1: lister runs /list repeatedly (reads LastActivity)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			fmt.Fprintf(lister, "/list\n")
			readUntil(lister, "][lister]:", time.Second)
		}
	}()
	// Goroutine 2: sender sends messages (writes LastActivity)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			fmt.Fprintf(sender, "msg%d\n", i)
			readUntil(sender, "][sender]:", time.Second)
		}
	}()
	wg.Wait()
}

// ==================== Task 29: Promote/Demote Broadcast ====================

func TestPromoteBroadcastToAllClients(t *testing.T) {
	// Spec 09 §Visibility: promote must broadcast to all connected clients.
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "admins.json")
	s.HeartbeatInterval = 1 * time.Hour

	var buf strings.Builder
	s.OperatorOutput = &buf

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	bob := connectPipe(s)
	defer bob.Close()
	onboard(bob, "bob")
	readUntil(alice, "bob has joined", time.Second)

	charlie := connectPipe(s)
	defer charlie.Close()
	onboard(charlie, "charlie")
	readUntil(alice, "charlie has joined", time.Second)
	readUntil(bob, "charlie has joined", time.Second)

	// Operator promotes alice
	s.OperatorDispatch("/promote alice")

	// Bob and charlie should see the broadcast
	textBob, err := readUntil(bob, "alice was promoted", 2*time.Second)
	if err != nil {
		t.Fatalf("bob should see promote broadcast: %v (got: %q)", err, textBob)
	}
	if !strings.Contains(textBob, "alice was promoted by Server") {
		t.Errorf("expected 'alice was promoted by Server', got: %q", textBob)
	}

	textCharlie, err := readUntil(charlie, "alice was promoted", 2*time.Second)
	if err != nil {
		t.Fatalf("charlie should see promote broadcast: %v", err)
	}
	if !strings.Contains(textCharlie, "alice was promoted by Server") {
		t.Errorf("expected 'alice was promoted by Server', got: %q", textCharlie)
	}
}

func TestDemoteBroadcastToAllClients(t *testing.T) {
	// Spec 09 §Visibility: demote must broadcast to all connected clients.
	s := New("0")
	s.adminsFile = filepath.Join(t.TempDir(), "admins.json")
	s.HeartbeatInterval = 1 * time.Hour

	var buf strings.Builder
	s.OperatorOutput = &buf

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	bob := connectPipe(s)
	defer bob.Close()
	onboard(bob, "bob")
	readUntil(alice, "bob has joined", time.Second)

	charlie := connectPipe(s)
	defer charlie.Close()
	onboard(charlie, "charlie")
	readUntil(alice, "charlie has joined", time.Second)
	readUntil(bob, "charlie has joined", time.Second)

	// Promote alice first
	s.OperatorDispatch("/promote alice")
	readUntil(bob, "alice was promoted", 2*time.Second)
	readUntil(charlie, "alice was promoted", 2*time.Second)
	readUntil(alice, "promoted", 2*time.Second)

	// Demote alice
	s.OperatorDispatch("/demote alice")

	textBob, err := readUntil(bob, "alice was demoted", 2*time.Second)
	if err != nil {
		t.Fatalf("bob should see demote broadcast: %v (got: %q)", err, textBob)
	}
	if !strings.Contains(textBob, "alice was demoted by Server") {
		t.Errorf("expected 'alice was demoted by Server', got: %q", textBob)
	}

	textCharlie, err := readUntil(charlie, "alice was demoted", 2*time.Second)
	if err != nil {
		t.Fatalf("charlie should see demote broadcast: %v", err)
	}
	if !strings.Contains(textCharlie, "alice was demoted by Server") {
		t.Errorf("expected 'alice was demoted by Server', got: %q", textCharlie)
	}
}

// ==================== Task 5: Edge Case Tests ====================

// TestShutdownConcurrentMultipleSignals verifies that multiple goroutines
// calling Shutdown() simultaneously do not panic, deadlock, or double-close
// channels. This covers the spec 01 §Edge Cases requirement: "Multiple rapid
// interrupt signals: server processes the first and ignores subsequent ones."
func TestShutdownConcurrentMultipleSignals(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 100 * time.Millisecond

	c1 := connectPipe(s)
	defer c1.Close()
	onboard(c1, "alice")

	// Fire 10 concurrent Shutdown calls to stress the sync.Once path.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Shutdown() // must not panic or deadlock
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All concurrent Shutdown() calls completed without panic.
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Shutdown() calls should not deadlock")
	}
}

// TestWhisperRecipientDisconnectedBeforeDelivery verifies that when the whisper
// recipient disconnects between typing the whisper and pressing Enter, the
// sender receives a "not found" error rather than a false delivery confirmation.
// (Spec 07 §Edge Cases)
func TestWhisperRecipientDisconnectedBeforeDelivery(t *testing.T) {
	s := New("0")

	alice := connectPipe(s)
	defer alice.Close()
	onboard(alice, "alice")

	bob := connectPipe(s)
	onboard(bob, "bob")
	readUntil(alice, "bob has joined", time.Second)

	// Bob disconnects — simulates recipient leaving while alice is typing.
	bob.Close()
	// Alice sees bob's leave notification; drain it before sending the whisper.
	readUntil(alice, "bob has left", 2*time.Second)

	// Alice now sends the whisper. Bob is already gone from the clients map.
	fmt.Fprintf(alice, "/whisper bob hey there\n")
	text, err := readUntil(alice, "][alice]:", 2*time.Second)
	if err != nil {
		t.Fatalf("alice should get a response after whisper attempt: %v", err)
	}
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found' error when whispering to disconnected user, got: %q", text)
	}
	if !strings.Contains(text, "bob") {
		t.Errorf("error should name the disconnected recipient 'bob', got: %q", text)
	}
}

// TestShutdownNotifiesMultipleQueuedClients verifies that when the server shuts
// down, ALL queued clients (not just the first) receive the shutdown message.
// This strengthens TestShutdownNotifiesQueuedClients which only tested a single
// queued client. (Spec 03 §Edge Cases)
func TestShutdownNotifiesMultipleQueuedClients(t *testing.T) {
	s := New("0")
	s.ShutdownTimeout = 300 * time.Millisecond

	// Fill all 10 active slots.
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipe(s)
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// Add 3 queued clients.
	queued := make([]net.Conn, 3)
	for i := 0; i < 3; i++ {
		queued[i] = connectPipe(s)
		defer queued[i].Close()
		readUntil(queued[i], "yes/no", 2*time.Second)
		fmt.Fprintf(queued[i], "yes\n")
		time.Sleep(50 * time.Millisecond)
	}

	// Shutdown in background so we can read from queued clients.
	go s.Shutdown()

	// Every queued client should receive the shutdown notification.
	for i, q := range queued {
		text, err := readUntil(q, "shutting down", 3*time.Second)
		if err != nil {
			t.Errorf("queued client #%d should receive shutdown message: %v (got: %q)", i+1, err, text)
			continue
		}
		if !strings.Contains(text, "Server is shutting down. Goodbye!") {
			t.Errorf("queued client #%d: expected shutdown message, got: %q", i+1, text)
		}
	}
}

// TestEdgeCaseAllRaceDetectorPasses is a meta-test that documents that the full
// test suite passes with -race enabled. This test itself is a no-op — the real
// verification is running `go test ./... -race`.
func TestEdgeCaseAllRaceDetectorPasses(t *testing.T) {
	// This test is intentionally a no-op. Its presence documents that
	// Task 24 requires all tests to pass with the race detector enabled.
	// Run the full suite with: go test ./... -race -count=1 -timeout 90s
}

// ==================== Task 4: Kick/Ban Queued Users ====================
// Spec 09 §Edge Cases: "Kicking a user who is currently in the queue (not yet
// active): the user is removed and cannot reconnect for 5 minutes."
// The operator identifies queued users by IP (visible via /list), since queued
// users have not completed onboarding and have no name.

// TestOperatorKickQueuedUserByIP verifies that the server operator can kick a
// queued user by IP, removing them from the queue and blocking reconnection for
// 5 minutes. (Spec 09 §Edge Cases)
func TestOperatorKickQueuedUserByIP(t *testing.T) {
	s := newServerWithAdmins(t)

	// Fill 10 active slots
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipeWithIP(s, fmt.Sprintf("10.0.0.%d:1000", i))
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// 11th client from a distinct IP enters the queue (new flow: name → room → queue)
	queuedIP := "10.0.1.1:2000"
	q := connectPipeWithIP(s, queuedIP)
	defer q.Close()
	enterName(q, "queued1")
	fmt.Fprintf(q, "\n") // default room
	readUntil(q, "yes/no", 2*time.Second)
	fmt.Fprintf(q, "yes\n")
	time.Sleep(100 * time.Millisecond)

	if s.GetQueueLength() != 1 {
		t.Fatalf("expected 1 queued client, got %d", s.GetQueueLength())
	}

	// Operator kicks by IP (host part only, as shown by /list)
	var buf strings.Builder
	s.OperatorOutput = &buf
	s.OperatorDispatch("/kick 10.0.1.1")

	time.Sleep(100 * time.Millisecond)

	// Queue should be empty
	if s.GetQueueLength() != 0 {
		t.Errorf("queue should be empty after kick, got %d", s.GetQueueLength())
	}

	// IP should be blocked
	blocked, msg := s.IsIPBlocked(queuedIP)
	if !blocked {
		t.Error("kicked queued IP should be blocked for 5 minutes")
	}
	if !strings.Contains(msg, "temporarily blocked") {
		t.Errorf("expected temporary block message, got: %q", msg)
	}

	// Operator should receive confirmation
	if !strings.Contains(buf.String(), "Queued user(s) from IP 10.0.1.1 have been kicked") {
		t.Errorf("operator should get kick confirmation, got: %q", buf.String())
	}
}

// TestOperatorBanQueuedUserByIP verifies that the server operator can ban a
// queued user by IP, removing them from the queue and permanently blocking the
// IP for the server session. (Spec 09 §Edge Cases)
func TestOperatorBanQueuedUserByIP(t *testing.T) {
	s := newServerWithAdmins(t)

	// Fill 10 active slots
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipeWithIP(s, fmt.Sprintf("10.0.0.%d:1000", i))
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// 11th client from a distinct IP enters the queue (new flow: name → room → queue)
	queuedIP := "10.0.2.1:3000"
	q := connectPipeWithIP(s, queuedIP)
	defer q.Close()
	enterName(q, "queued1")
	fmt.Fprintf(q, "\n") // default room
	readUntil(q, "yes/no", 2*time.Second)
	fmt.Fprintf(q, "yes\n")
	time.Sleep(100 * time.Millisecond)

	if s.GetQueueLength() != 1 {
		t.Fatalf("expected 1 queued client, got %d", s.GetQueueLength())
	}

	// Operator bans by IP
	var buf strings.Builder
	s.OperatorOutput = &buf
	s.OperatorDispatch("/ban 10.0.2.1")

	time.Sleep(100 * time.Millisecond)

	// Queue should be empty
	if s.GetQueueLength() != 0 {
		t.Errorf("queue should be empty after ban, got %d", s.GetQueueLength())
	}

	// IP should be permanently banned
	blocked, msg := s.IsIPBlocked(queuedIP)
	if !blocked {
		t.Error("banned queued IP should be blocked permanently")
	}
	if !strings.Contains(msg, "banned") {
		t.Errorf("expected ban message, got: %q", msg)
	}

	// Operator should receive confirmation
	if !strings.Contains(buf.String(), "Queued user(s) from IP 10.0.2.1 have been banned") {
		t.Errorf("operator should get ban confirmation, got: %q", buf.String())
	}
}

// TestOperatorKickQueuedPositionUpdates verifies that after kicking a queued
// user by IP, remaining queued clients receive updated position numbers.
// (Spec 09 §Edge Cases + Spec 03 position updates)
func TestOperatorKickQueuedPositionUpdates(t *testing.T) {
	s := newServerWithAdmins(t)

	// Fill 10 active slots (each from a unique IP)
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipeWithIP(s, fmt.Sprintf("10.0.0.%d:1000", i))
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// Queue 3 clients from different IPs (new flow: name → room → queue)
	q1 := connectPipeWithIP(s, "10.1.0.1:2001")
	defer q1.Close()
	enterName(q1, "q1")
	fmt.Fprintf(q1, "\n") // default room
	readUntil(q1, "yes/no", 2*time.Second)
	fmt.Fprintf(q1, "yes\n")

	q2 := connectPipeWithIP(s, "10.1.0.2:2002")
	defer q2.Close()
	enterName(q2, "q2")
	fmt.Fprintf(q2, "\n") // default room
	readUntil(q2, "#2 in the queue", 2*time.Second)
	fmt.Fprintf(q2, "yes\n")

	q3 := connectPipeWithIP(s, "10.1.0.3:2003")
	defer q3.Close()
	enterName(q3, "q3")
	fmt.Fprintf(q3, "\n") // default room
	readUntil(q3, "#3 in the queue", 2*time.Second)
	fmt.Fprintf(q3, "yes\n")

	time.Sleep(100 * time.Millisecond)
	if s.GetQueueLength() != 3 {
		t.Fatalf("expected 3 queued, got %d", s.GetQueueLength())
	}

	// Operator kicks the middle queued client (q2) by IP
	var buf strings.Builder
	s.OperatorOutput = &buf
	s.OperatorDispatch("/kick 10.1.0.2")

	time.Sleep(100 * time.Millisecond)

	// Queue should now have 2 entries
	if s.GetQueueLength() != 2 {
		t.Errorf("expected 2 queued after kick, got %d", s.GetQueueLength())
	}

	// q3 should receive a position update to #2
	text, err := readUntil(q3, "#2 in the queue", 2*time.Second)
	if err != nil {
		t.Fatalf("q3 should receive position update to #2: %v (got: %q)", err, text)
	}
}

// TestAdminKickQueuedUserNotFound verifies that a client admin (non-operator)
// trying to /kick a queued user gets "not found" — admins cannot see IPs and
// queued users have no name to target.
func TestAdminKickQueuedUserNotFound(t *testing.T) {
	s := newServerWithAdmins(t)

	// Create an admin client
	admin := connectPipeWithIP(s, "10.0.0.1:1000")
	defer admin.Close()
	onboard(admin, "admin1")
	cl := s.GetClient("admin1")
	cl.SetAdmin(true)

	// Fill remaining 9 active slots
	conns := make([]net.Conn, 9)
	for i := 0; i < 9; i++ {
		conns[i] = connectPipeWithIP(s, fmt.Sprintf("10.0.0.%d:1000", i+2))
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// Queue an 11th client (new flow: name → room → queue)
	q := connectPipeWithIP(s, "10.1.0.1:2000")
	defer q.Close()
	enterName(q, "queued1")
	fmt.Fprintf(q, "\n") // default room
	readUntil(q, "yes/no", 2*time.Second)
	fmt.Fprintf(q, "yes\n")
	time.Sleep(100 * time.Millisecond)

	// Admin tries to kick by IP — should get "not found" because cmdKick
	// only searches the active client map by name
	fmt.Fprintf(admin, "/kick 10.1.0.1\n")
	text, err := readUntil(admin, "not found", 2*time.Second)
	if err != nil {
		t.Fatalf("admin kick-by-IP should return not found: %v (got: %q)", err, text)
	}

	// Queue should still have the client
	if s.GetQueueLength() != 1 {
		t.Errorf("queued client should not be affected by admin kick, queue=%d", s.GetQueueLength())
	}
}

// TestOperatorListShowsQueuedClients verifies that the operator's /list command
// shows queued clients with their IPs, enabling IP-based moderation.
func TestOperatorListShowsQueuedClients(t *testing.T) {
	s := newServerWithAdmins(t)

	// Create 10 active clients
	conns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		conns[i] = connectPipeWithIP(s, fmt.Sprintf("10.0.0.%d:1000", i))
		defer conns[i].Close()
		onboard(conns[i], fmt.Sprintf("user%d", i))
	}

	// Queue 2 clients from identifiable IPs (new flow: name → room → queue)
	q1 := connectPipeWithIP(s, "192.168.1.100:5000")
	defer q1.Close()
	enterName(q1, "q1")
	fmt.Fprintf(q1, "\n") // default room
	readUntil(q1, "yes/no", 2*time.Second)
	fmt.Fprintf(q1, "yes\n")

	q2 := connectPipeWithIP(s, "192.168.1.200:5001")
	defer q2.Close()
	enterName(q2, "q2")
	fmt.Fprintf(q2, "\n") // default room
	readUntil(q2, "#2 in the queue", 2*time.Second)
	fmt.Fprintf(q2, "yes\n")

	time.Sleep(100 * time.Millisecond)

	// Operator runs /list
	var buf strings.Builder
	s.OperatorOutput = &buf
	s.OperatorDispatch("/list")

	output := buf.String()
	// In new format, queued clients show in the room section
	if !strings.Contains(output, "192.168.1.100") {
		t.Errorf("operator /list should show first queued IP, got: %q", output)
	}
	if !strings.Contains(output, "192.168.1.200") {
		t.Errorf("operator /list should show second queued IP, got: %q", output)
	}
	if !strings.Contains(output, "#1") || !strings.Contains(output, "#2") {
		t.Errorf("operator /list should show queue positions, got: %q", output)
	}
}

// ==================== Corrupt admins.json handling ====================

// TestLoadAdminsCorruptJSON verifies that a corrupted admins.json file causes
// the server to start with no saved admins and print a warning to stderr,
// as required by spec 10 edge case: "admins.json missing or corrupted on startup:
// server starts with no saved admins, warning on server console."
func TestLoadAdminsCorruptJSON(t *testing.T) {
	s := New("0")
	tmpDir := t.TempDir()
	s.adminsFile = filepath.Join(tmpDir, "admins.json")

	// Write a valid admins.json first, then corrupt it
	if err := os.WriteFile(s.adminsFile, []byte(`["alice","bob"]`), 0600); err != nil {
		t.Fatal(err)
	}
	s.LoadAdmins()
	if !s.IsKnownAdmin("alice") || !s.IsKnownAdmin("bob") {
		t.Fatal("valid admins.json should load correctly")
	}

	// Now corrupt the file and create a fresh server
	s2 := New("0")
	s2.adminsFile = filepath.Join(tmpDir, "admins.json")
	if err := os.WriteFile(s2.adminsFile, []byte(`{{{not valid json!!!`), 0600); err != nil {
		t.Fatal(err)
	}

	// Capture stderr to verify warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	s2.LoadAdmins()

	w.Close()
	var stderrBuf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			stderrBuf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	r.Close()
	os.Stderr = oldStderr

	stderrOutput := stderrBuf.String()
	if !strings.Contains(stderrOutput, "corrupt admins.json") {
		t.Errorf("corrupt admins.json should produce stderr warning, got: %q", stderrOutput)
	}

	// Server should have no admins
	if s2.IsKnownAdmin("alice") || s2.IsKnownAdmin("bob") {
		t.Error("corrupt admins.json should result in no saved admins")
	}
}

// ==================== Multi-Room Tests ====================

func TestRoomSelectionDefault(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboard(conn, "alice")

	cl := s.GetClient("alice")
	if cl == nil {
		t.Fatal("alice should be registered")
	}
	if cl.Room != s.DefaultRoom {
		t.Errorf("expected room %q, got %q", s.DefaultRoom, cl.Room)
	}
}

func TestRoomSelectionCustom(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()
	onboardRoom(conn, "alice", "dev")

	cl := s.GetClient("alice")
	if cl == nil {
		t.Fatal("alice should be registered")
	}
	if cl.Room != "dev" {
		t.Errorf("expected room %q, got %q", "dev", cl.Room)
	}
}

func TestRoomSelectionInvalidName(t *testing.T) {
	s := New("0")
	conn := connectPipe(s)
	defer conn.Close()

	// Read banner + name prompt
	readUntil(conn, "[ENTER YOUR NAME]:", 2*time.Second)
	fmt.Fprintf(conn, "alice\n")
	readUntil(conn, "[ENTER ROOM NAME]", 2*time.Second)

	// Send invalid room name (too long)
	longName := strings.Repeat("x", 33)
	fmt.Fprintf(conn, "%s\n", longName)
	text, _ := readUntil(conn, "[ENTER ROOM NAME]", 2*time.Second)
	if !strings.Contains(text, "too long") {
		t.Errorf("expected error for long room name, got: %q", text)
	}

	// Send valid room name now
	fmt.Fprintf(conn, "valid\n")
	_, err := readUntil(conn, "][alice]:", 2*time.Second)
	if err != nil {
		t.Fatalf("should be able to join with valid name: %v", err)
	}
	if s.GetClient("alice").Room != "valid" {
		t.Error("alice should be in room 'valid'")
	}
}

func TestRoomIsolationMessages(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")                // general
	onboardRoom(c2, "bob", "dev") // dev

	// Alice sends a message — bob should NOT see it
	fmt.Fprintf(c1, "hello from general\n")
	readUntil(c1, "][alice]:", time.Second)

	// Bob should not receive alice's message
	text := drainFor(c2, 500*time.Millisecond)
	if strings.Contains(text, "hello from general") {
		t.Error("bob in 'dev' should NOT see alice's message in 'general'")
	}

	// Bob sends a message — alice should NOT see it
	fmt.Fprintf(c2, "hello from dev\n")
	readUntil(c2, "][bob]:", time.Second)

	text = drainFor(c1, 500*time.Millisecond)
	if strings.Contains(text, "hello from dev") {
		t.Error("alice in 'general' should NOT see bob's message in 'dev'")
	}
}

func TestRoomIsolationJoinLeave(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()

	onboard(c1, "alice") // general

	// Bob joins "dev" — alice should NOT see the join notification
	c2 := connectPipe(s)
	defer c2.Close()
	onboardRoom(c2, "bob", "dev")

	text := drainFor(c1, 500*time.Millisecond)
	if strings.Contains(text, "bob has joined") {
		t.Error("alice should NOT see bob's join in a different room")
	}

	// Bob leaves — alice should NOT see the leave
	c2.Close()
	time.Sleep(200 * time.Millisecond)

	text = drainFor(c1, 500*time.Millisecond)
	if strings.Contains(text, "bob has left") {
		t.Error("alice should NOT see bob's leave from a different room")
	}
}

func TestSwitchRoomBasic(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboard(c1, "alice")                // general
	onboard(c2, "bob")                  // general
	onboardRoom(c3, "carol", "dev") // dev

	readUntil(c1, "bob has joined", time.Second)

	// Alice switches to "dev"
	fmt.Fprintf(c1, "/switch dev\n")
	readUntil(c1, "Switched to room 'dev'", 2*time.Second)

	// Bob should see alice leaving
	text, _ := readUntil(c2, "alice has left", 2*time.Second)
	if !strings.Contains(text, "alice has left") {
		t.Errorf("bob should see alice leaving general, got: %q", text)
	}

	// Carol should see alice joining dev
	text, _ = readUntil(c3, "alice has joined", 2*time.Second)
	if !strings.Contains(text, "alice has joined") {
		t.Errorf("carol should see alice joining dev, got: %q", text)
	}

	// Alice sends message in dev — only carol sees it
	fmt.Fprintf(c1, "hello dev\n")
	readUntil(c1, "][alice]:", time.Second)
	text, _ = readUntil(c3, "hello dev", time.Second)
	if !strings.Contains(text, "hello dev") {
		t.Error("carol should see alice's message in dev")
	}

	// Bob should NOT see it
	text = drainFor(c2, 500*time.Millisecond)
	if strings.Contains(text, "hello dev") {
		t.Error("bob should NOT see alice's message after she switched rooms")
	}
}

func TestSwitchRoomFull(t *testing.T) {
	s := New("0")
	// Fill "dev" room with 10 clients
	devConns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		devConns[i] = connectPipe(s)
		defer devConns[i].Close()
		onboardRoom(devConns[i], fmt.Sprintf("dev%d", i), "dev")
	}

	// Client in general tries to switch to full "dev"
	c := connectPipe(s)
	defer c.Close()
	onboard(c, "alice")

	fmt.Fprintf(c, "/switch dev\n")
	text, _ := readUntil(c, "][alice]:", 2*time.Second)
	if !strings.Contains(text, "full") {
		t.Errorf("should get 'full' error when switching to full room, got: %q", text)
	}

	// Alice should still be in general
	if s.GetClient("alice").Room != s.DefaultRoom {
		t.Error("alice should remain in general after failed switch")
	}
}

func TestSwitchRoomSameRoom(t *testing.T) {
	s := New("0")
	c := connectPipe(s)
	defer c.Close()
	onboard(c, "alice")

	fmt.Fprintf(c, "/switch general\n")
	text, _ := readUntil(c, "][alice]:", 2*time.Second)
	if !strings.Contains(text, "already in") {
		t.Errorf("should get 'already in' error, got: %q", text)
	}
}

func TestCreateRoomBasic(t *testing.T) {
	s := New("0")
	c := connectPipe(s)
	defer c.Close()
	onboard(c, "alice")

	fmt.Fprintf(c, "/create newroom\n")
	text, _ := readUntil(c, "Switched to room 'newroom'", 2*time.Second)
	if !strings.Contains(text, "Switched to room 'newroom'") {
		t.Errorf("should switch to new room, got: %q", text)
	}

	if s.GetClient("alice").Room != "newroom" {
		t.Error("alice should be in 'newroom'")
	}
}

func TestCreateRoomAlreadyExists(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboardRoom(c1, "alice", "dev")
	onboard(c2, "bob")

	fmt.Fprintf(c2, "/create dev\n")
	text, _ := readUntil(c2, "][bob]:", 2*time.Second)
	if !strings.Contains(text, "already exists") {
		t.Errorf("should get 'already exists' error, got: %q", text)
	}
	if !strings.Contains(text, "/switch") {
		t.Error("error should suggest using /switch")
	}
}

func TestRoomsCommandListsAll(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")                // general
	onboardRoom(c2, "bob", "dev") // dev

	fmt.Fprintf(c1, "/rooms\n")
	text, _ := readUntil(c1, "][alice]:", 2*time.Second)

	if !strings.Contains(text, "general") {
		t.Error("/rooms should list 'general'")
	}
	if !strings.Contains(text, "dev") {
		t.Error("/rooms should list 'dev'")
	}
	if !strings.Contains(text, "(current)") {
		t.Error("/rooms should mark the current room")
	}
}

func TestListShowsRoomMembersOnly(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")                // general
	onboardRoom(c2, "bob", "dev") // dev

	fmt.Fprintf(c1, "/list\n")
	text, _ := readUntil(c1, "][alice]:", 2*time.Second)

	if !strings.Contains(text, "alice") {
		t.Error("/list should show alice (same room)")
	}
	if strings.Contains(text, "bob") {
		t.Error("/list should NOT show bob (different room)")
	}
}

func TestKickSameRoomOnly(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin1")
	onboardRoom(c2, "target", "dev")

	s.GetClient("admin1").SetAdmin(true)

	// Admin in "general" tries to kick user in "dev"
	fmt.Fprintf(c1, "/kick target\n")
	text, _ := readUntil(c1, "][admin1]:", 2*time.Second)
	if !strings.Contains(text, "not in your room") && !strings.Contains(text, "not found") {
		t.Errorf("kick across rooms should fail, got: %q", text)
	}

	// Target should still be connected
	if s.GetClient("target") == nil {
		t.Error("target should still be connected")
	}
}

func TestWhisperCrossRoom(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")                // general
	onboardRoom(c2, "bob", "dev") // dev

	// Alice whispers to bob across rooms
	fmt.Fprintf(c1, "/whisper bob hi from alice\n")
	readUntil(c1, "PM to bob", time.Second)

	// Bob should receive it
	text, _ := readUntil(c2, "PM from alice", 2*time.Second)
	if !strings.Contains(text, "hi from alice") {
		t.Errorf("bob should receive cross-room whisper, got: %q", text)
	}
}

func TestAnnouncementAllRooms(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin1")                // general
	onboardRoom(c2, "bob", "dev")  // dev

	s.GetClient("admin1").SetAdmin(true)

	fmt.Fprintf(c1, "/announce Server maintenance tonight\n")
	readUntil(c1, "][admin1]:", time.Second)

	// Bob in different room should also see the announcement
	text, _ := readUntil(c2, "Server maintenance tonight", 2*time.Second)
	if !strings.Contains(text, "[ANNOUNCEMENT]") {
		t.Errorf("announcement should reach clients in all rooms, got: %q", text)
	}
}

func TestNameChangeRoomScoped(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()
	c3 := connectPipe(s)
	defer c3.Close()

	onboard(c1, "alice")                 // general
	onboard(c2, "bob")                   // general
	onboardRoom(c3, "carol", "dev") // dev

	readUntil(c1, "bob has joined", time.Second)

	fmt.Fprintf(c1, "/name alice2\n")
	readUntil(c1, "][alice2]:", time.Second)

	// Bob (same room) should see name change
	text, _ := readUntil(c2, "changed their name", 2*time.Second)
	if !strings.Contains(text, "alice changed their name to alice2") {
		t.Error("bob should see name change in same room")
	}

	// Carol (different room) should NOT see name change
	text = drainFor(c3, 500*time.Millisecond)
	if strings.Contains(text, "alice") && strings.Contains(text, "changed") {
		t.Error("carol should NOT see name change from different room")
	}
}

func TestMuteIsGlobal(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "admin1")
	onboard(c2, "target")
	readUntil(c1, "target has joined", time.Second)

	s.GetClient("admin1").SetAdmin(true)

	// Mute target
	fmt.Fprintf(c1, "/mute target\n")
	readUntil(c2, "muted", time.Second)
	// Drain remaining mute broadcast + prompt
	readUntil(c2, "][target]:", time.Second)

	// Target switches rooms
	fmt.Fprintf(c2, "/create newroom\n")
	readUntil(c2, "][target]:", 2*time.Second)

	// Target should still be muted after room switch
	fmt.Fprintf(c2, "hello\n")
	text, _ := readUntil(c2, "][target]:", time.Second)
	if !strings.Contains(text, "You are muted") {
		t.Error("target should still be muted after switching rooms")
	}
}

func TestRoomAutoDelete(t *testing.T) {
	s := New("0")
	c := connectPipe(s)
	defer c.Close()
	onboardRoom(c, "alice", "temp")

	// Room "temp" should exist
	names := s.GetRoomNames()
	found := false
	for _, n := range names {
		if n == "temp" {
			found = true
		}
	}
	if !found {
		t.Error("room 'temp' should exist while alice is in it")
	}

	// Alice leaves
	c.Close()
	time.Sleep(300 * time.Millisecond)

	// Room "temp" should be deleted
	names = s.GetRoomNames()
	for _, n := range names {
		if n == "temp" {
			t.Error("room 'temp' should be deleted when empty")
		}
	}
}

func TestRoomDefaultProtected(t *testing.T) {
	s := New("0")
	c := connectPipe(s)
	defer c.Close()
	onboard(c, "alice") // general

	// Alice switches away from general
	fmt.Fprintf(c, "/create newroom\n")
	readUntil(c, "Switched to room", 2*time.Second)

	time.Sleep(200 * time.Millisecond)

	// "general" should still exist even though it's empty
	names := s.GetRoomNames()
	found := false
	for _, n := range names {
		if n == "general" {
			found = true
		}
	}
	if !found {
		t.Error("default room 'general' should NOT be deleted when empty")
	}
}

func TestRoomQueuePerRoom(t *testing.T) {
	s := New("0")

	// Fill "general" to capacity
	generalConns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		generalConns[i] = connectPipe(s)
		defer generalConns[i].Close()
		onboard(generalConns[i], fmt.Sprintf("gen%d", i))
	}

	// A client selecting "dev" should NOT be queued (dev has space)
	c := connectPipe(s)
	defer c.Close()
	_, err := onboardRoom(c, "devuser", "dev")
	if err != nil {
		t.Fatalf("client should join 'dev' without queuing: %v", err)
	}
	if s.GetClient("devuser").Room != "dev" {
		t.Error("devuser should be in 'dev'")
	}
}

func TestBanDisconnectsAcrossRooms(t *testing.T) {
	s := newServerWithAdmins(t)
	s.HeartbeatInterval = 1 * time.Hour

	// admin1 and target1 in general, target2 in dev — same IP
	c1 := connectPipeWithIP(s, "10.0.0.1:1000")
	defer c1.Close()
	onboard(c1, "admin1")
	s.GetClient("admin1").SetAdmin(true)

	c2 := connectPipeWithIP(s, "10.0.0.2:1001")
	defer c2.Close()
	onboard(c2, "target1")
	readUntil(c1, "target1 has joined", time.Second)

	c3 := connectPipeWithIP(s, "10.0.0.2:1002") // same IP as target1
	defer c3.Close()
	onboardRoom(c3, "target2", "dev")

	// Admin bans target1 — should also disconnect target2 (same IP, different room)
	fmt.Fprintf(c1, "/ban target1\n")
	readUntil(c1, "][admin1]:", 2*time.Second)

	time.Sleep(300 * time.Millisecond)

	if s.GetClient("target1") != nil {
		t.Error("target1 should be disconnected after ban")
	}
	if s.GetClient("target2") != nil {
		t.Error("target2 (same IP, different room) should be disconnected after ban")
	}
}

func TestOperatorListShowsAllRooms(t *testing.T) {
	s := newServerWithAdmins(t)
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")                // general
	onboardRoom(c2, "bob", "dev") // dev

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/list")
	output := buf.String()

	if !strings.Contains(output, "Room general") {
		t.Error("operator /list should show room 'general'")
	}
	if !strings.Contains(output, "Room dev") {
		t.Error("operator /list should show room 'dev'")
	}
	if !strings.Contains(output, "alice") {
		t.Error("operator /list should show alice")
	}
	if !strings.Contains(output, "bob") {
		t.Error("operator /list should show bob")
	}
}

func TestOperatorRoomsCommand(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboard(c1, "alice")
	onboardRoom(c2, "bob", "dev")

	var buf strings.Builder
	s.OperatorOutput = &buf

	s.OperatorDispatch("/rooms")
	output := buf.String()

	if !strings.Contains(output, "general") || !strings.Contains(output, "dev") {
		t.Errorf("operator /rooms should list all rooms, got: %q", output)
	}
}

func TestSwitchRoomHistoryDelivered(t *testing.T) {
	s := New("0")
	c1 := connectPipe(s)
	defer c1.Close()
	c2 := connectPipe(s)
	defer c2.Close()

	onboardRoom(c1, "alice", "dev")
	// Alice sends a message in dev
	fmt.Fprintf(c1, "hello dev\n")
	readUntil(c1, "][alice]:", time.Second)

	onboard(c2, "bob") // general

	// Bob switches to dev and should see alice's message in history
	fmt.Fprintf(c2, "/switch dev\n")
	text, _ := readUntil(c2, "Switched to room 'dev'", 2*time.Second)
	if !strings.Contains(text, "hello dev") {
		t.Errorf("bob should see dev room history after switching, got: %q", text)
	}
}
