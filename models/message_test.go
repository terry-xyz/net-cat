package models

import (
	"strings"
	"testing"
	"time"
)

var refTime = time.Date(2026, 2, 20, 15, 48, 41, 0, time.Local)

func TestFormatTimestamp(t *testing.T) {
	got := FormatTimestamp(refTime)
	want := "2026-02-20 15:48:41"
	if got != want {
		t.Errorf("FormatTimestamp = %q, want %q", got, want)
	}
}

func TestFormatTimestampZeroPadding(t *testing.T) {
	// month=1, day=2, hour=3, min=4, sec=5 → all zero-padded
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	got := FormatTimestamp(ts)
	want := "2026-01-02 03:04:05"
	if got != want {
		t.Errorf("zero-padding: got %q, want %q", got, want)
	}
}

func TestFormatChat(t *testing.T) {
	got := FormatChat(refTime, "alice", "hello")
	want := "[2026-02-20 15:48:41][alice]:hello"
	if got != want {
		t.Errorf("FormatChat = %q, want %q", got, want)
	}
}

func TestFormatPrompt(t *testing.T) {
	got := FormatPrompt(refTime, "alice")
	want := "[2026-02-20 15:48:41][alice]:"
	if got != want {
		t.Errorf("FormatPrompt = %q, want %q", got, want)
	}
	// no trailing content
	if strings.HasSuffix(got, " ") || strings.HasSuffix(got, "\n") {
		t.Error("prompt should have no trailing whitespace or newline")
	}
}

func TestFormatJoin(t *testing.T) {
	got := FormatJoin("alice")
	want := "alice has joined our chat..."
	if got != want {
		t.Errorf("FormatJoin = %q, want %q", got, want)
	}
}

func TestFormatLeave(t *testing.T) {
	got := FormatLeave("alice")
	want := "alice has left our chat..."
	if got != want {
		t.Errorf("FormatLeave = %q, want %q", got, want)
	}
}

func TestFormatNameChange(t *testing.T) {
	got := FormatNameChange("OldName", "NewName")
	want := "OldName changed their name to NewName"
	if got != want {
		t.Errorf("FormatNameChange = %q, want %q", got, want)
	}
}

func TestFormatAnnouncement(t *testing.T) {
	got := FormatAnnouncement("Server maintenance at midnight")
	want := "[ANNOUNCEMENT]: Server maintenance at midnight"
	if got != want {
		t.Errorf("FormatAnnouncement = %q, want %q", got, want)
	}
}

func TestFormatModeration(t *testing.T) {
	tests := []struct {
		target, action, admin, want string
	}{
		{"alice", "kicked", "bob", "alice was kicked by bob"},
		{"alice", "banned", "Server", "alice was banned by Server"},
		{"bob", "muted", "alice", "bob was muted by alice"},
		{"bob", "unmuted", "alice", "bob was unmuted by alice"},
	}
	for _, tt := range tests {
		got := FormatModeration(tt.target, tt.action, tt.admin)
		if got != tt.want {
			t.Errorf("FormatModeration(%s,%s,%s) = %q, want %q", tt.target, tt.action, tt.admin, got, tt.want)
		}
	}
}

func TestFormatWhisper(t *testing.T) {
	recv := FormatWhisperReceive(refTime, "alice", "secret")
	if recv != "[2026-02-20 15:48:41][PM from alice]: secret" {
		t.Errorf("whisper receive = %q", recv)
	}
	send := FormatWhisperSend(refTime, "bob", "secret")
	if send != "[2026-02-20 15:48:41][PM to bob]: secret" {
		t.Errorf("whisper send = %q", send)
	}
}

func TestMessageDisplay(t *testing.T) {
	tests := []struct {
		msg  Message
		want string
	}{
		{Message{Timestamp: refTime, Type: MsgChat, Sender: "alice", Content: "hello"}, "[2026-02-20 15:48:41][alice]:hello"},
		{Message{Type: MsgJoin, Sender: "alice"}, "alice has joined our chat..."},
		{Message{Type: MsgLeave, Sender: "alice"}, "alice has left our chat..."},
		{Message{Type: MsgNameChange, Sender: "NewName", Extra: "OldName"}, "OldName changed their name to NewName"},
		{Message{Type: MsgAnnouncement, Content: "test"}, "[ANNOUNCEMENT]: test"},
		{Message{Type: MsgModeration, Sender: "alice", Content: "kicked", Extra: "bob"}, "alice was kicked by bob"},
	}
	for _, tt := range tests {
		got := tt.msg.Display()
		if got != tt.want {
			t.Errorf("Display() = %q, want %q", got, tt.want)
		}
	}
}

// ---------- log format round-trip ----------

func TestLogFormatRoundTrip(t *testing.T) {
	messages := []Message{
		{Timestamp: refTime, Type: MsgChat, Sender: "alice", Content: "hello world", Room: "general"},
		{Timestamp: refTime, Type: MsgJoin, Sender: "bob", Room: "general"},
		{Timestamp: refTime, Type: MsgLeave, Sender: "bob", Extra: "voluntary", Room: "dev"},
		{Timestamp: refTime, Type: MsgLeave, Sender: "bob", Extra: "kicked", Room: "general"},
		{Timestamp: refTime, Type: MsgNameChange, Sender: "NewName", Extra: "OldName", Room: "general"},
		{Timestamp: refTime, Type: MsgAnnouncement, Content: "maintenance soon", Extra: "Server", Room: "general"},
		{Timestamp: refTime, Type: MsgModeration, Sender: "alice", Content: "kicked", Extra: "bob", Room: "dev"},
		{Timestamp: refTime, Type: MsgModeration, Sender: "alice", Content: "muted", Extra: "Server", Room: "general"},
		{Timestamp: refTime, Type: MsgServerEvent, Content: "started"},
	}
	for _, orig := range messages {
		line := orig.FormatLogLine()
		parsed, err := ParseLogLine(line)
		if err != nil {
			t.Errorf("ParseLogLine(%q) error: %v", line, err)
			continue
		}
		if parsed.Type != orig.Type {
			t.Errorf("type mismatch: got %v, want %v (line: %s)", parsed.Type, orig.Type, line)
		}
		if parsed.Sender != orig.Sender {
			t.Errorf("sender mismatch: got %q, want %q (line: %s)", parsed.Sender, orig.Sender, line)
		}
		if parsed.Content != orig.Content {
			t.Errorf("content mismatch: got %q, want %q (line: %s)", parsed.Content, orig.Content, line)
		}
		if parsed.Extra != orig.Extra {
			t.Errorf("extra mismatch: got %q, want %q (line: %s)", parsed.Extra, orig.Extra, line)
		}
		if parsed.Room != orig.Room {
			t.Errorf("room mismatch: got %q, want %q (line: %s)", parsed.Room, orig.Room, line)
		}
	}
}

func TestFormatLogLineRoomTag(t *testing.T) {
	tests := []struct {
		msg    Message
		expect string
	}{
		{Message{Timestamp: refTime, Type: MsgChat, Sender: "alice", Content: "hi", Room: "general"}, "[2026-02-20 15:48:41] @general CHAT [alice]:hi"},
		{Message{Timestamp: refTime, Type: MsgJoin, Sender: "bob", Room: "dev"}, "[2026-02-20 15:48:41] @dev JOIN bob"},
		{Message{Timestamp: refTime, Type: MsgLeave, Sender: "bob", Extra: "voluntary", Room: "general"}, "[2026-02-20 15:48:41] @general LEAVE bob voluntary"},
		{Message{Timestamp: refTime, Type: MsgNameChange, Sender: "New", Extra: "Old", Room: "dev"}, "[2026-02-20 15:48:41] @dev NAMECHANGE Old New"},
		{Message{Timestamp: refTime, Type: MsgAnnouncement, Content: "test", Extra: "admin", Room: "general"}, "[2026-02-20 15:48:41] @general ANNOUNCE [admin]:test"},
		{Message{Timestamp: refTime, Type: MsgModeration, Sender: "alice", Content: "kicked", Extra: "bob", Room: "general"}, "[2026-02-20 15:48:41] @general MOD kicked alice bob"},
	}
	for _, tt := range tests {
		got := tt.msg.FormatLogLine()
		if got != tt.expect {
			t.Errorf("FormatLogLine() = %q, want %q", got, tt.expect)
		}
	}
}

func TestFormatLogLineServerEventNoRoom(t *testing.T) {
	msg := Message{Timestamp: refTime, Type: MsgServerEvent, Content: "started", Room: "general"}
	got := msg.FormatLogLine()
	want := "[2026-02-20 15:48:41] SERVER started"
	if got != want {
		t.Errorf("FormatLogLine() = %q, want %q (ServerEvent should never have room tag)", got, want)
	}
}

func TestParseLogLineRoundTripWithRoom(t *testing.T) {
	msg := Message{Timestamp: refTime, Type: MsgChat, Sender: "alice", Content: "hello", Room: "dev"}
	line := msg.FormatLogLine()
	parsed, err := ParseLogLine(line)
	if err != nil {
		t.Fatalf("ParseLogLine(%q) error: %v", line, err)
	}
	if parsed.Room != "dev" {
		t.Errorf("Room mismatch: got %q, want %q", parsed.Room, "dev")
	}
	if parsed.Sender != "alice" || parsed.Content != "hello" {
		t.Errorf("fields not preserved: sender=%q content=%q", parsed.Sender, parsed.Content)
	}
}

func TestParseLogLineBackwardCompatible(t *testing.T) {
	// Old-format log line without @room tag
	line := "[2026-02-20 15:48:41] CHAT [alice]:hello"
	parsed, err := ParseLogLine(line)
	if err != nil {
		t.Fatalf("ParseLogLine(%q) error: %v", line, err)
	}
	if parsed.Room != "" {
		t.Errorf("old-format line should have Room=\"\", got %q", parsed.Room)
	}
	if parsed.Sender != "alice" || parsed.Content != "hello" {
		t.Errorf("fields not preserved: sender=%q content=%q", parsed.Sender, parsed.Content)
	}
}

func TestParseLogLineErrors(t *testing.T) {
	bad := []string{
		"",
		"no bracket",
		"[bad timestamp] CHAT [a]:b",
		"[2026-02-20 15:48:41] UNKNOWN_TYPE foo",
	}
	for _, line := range bad {
		_, err := ParseLogLine(line)
		if err == nil {
			t.Errorf("expected error for %q", line)
		}
	}
}

func TestLogLineDistinct(t *testing.T) {
	// Each message type should produce a log line with a distinct prefix keyword
	types := []MessageType{MsgChat, MsgJoin, MsgLeave, MsgNameChange, MsgAnnouncement, MsgModeration, MsgServerEvent}
	prefixes := make(map[string]bool)
	for _, mt := range types {
		msg := Message{Timestamp: refTime, Type: mt, Sender: "a", Content: "b", Extra: "c", Room: "general"}
		line := msg.FormatLogLine()
		// extract type keyword: everything between "] " and the next space
		// skip the @room tag if present
		after := line[strings.Index(line, "] ")+2:]
		if len(after) > 0 && after[0] == '@' {
			spaceIdx := strings.IndexByte(after, ' ')
			if spaceIdx >= 0 {
				after = after[spaceIdx+1:]
			}
		}
		keyword := strings.SplitN(after, " ", 2)[0]
		if prefixes[keyword] {
			t.Errorf("duplicate log keyword %q for type %d", keyword, mt)
		}
		prefixes[keyword] = true
	}
}
