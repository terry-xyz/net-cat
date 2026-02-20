package logger

import (
	"net-cat/models"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNew_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	_, err := New(dir)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("logs directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("logs path is not a directory")
	}
}

func TestLogFileNaming(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	now := time.Now()
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgChat,
		Sender:    "alice",
		Content:   "hello",
	})

	expected := filepath.Join(dir, "chat_"+FormatDate(now)+".log")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected log file %s not found: %v", expected, err)
	}
}

func TestLogChatMessage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgChat,
		Sender:    "alice",
		Content:   "hello world",
	})
	l.Close()

	content := readFile(t, dir, now)
	if !strings.Contains(content, "CHAT [alice]:hello world") {
		t.Errorf("log should contain chat message, got: %q", content)
	}
	ts := models.FormatTimestamp(now)
	if !strings.Contains(content, "["+ts+"]") {
		t.Errorf("log should contain timestamp %s, got: %q", ts, content)
	}
}

func TestLogJoinEvent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgJoin,
		Sender:    "bob",
	})
	l.Close()

	content := readFile(t, dir, now)
	if !strings.Contains(content, "JOIN bob") {
		t.Errorf("log should contain join event, got: %q", content)
	}
}

func TestLogLeaveEventVoluntary(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgLeave,
		Sender:    "charlie",
		Extra:     "voluntary",
	})
	l.Close()

	content := readFile(t, dir, now)
	if !strings.Contains(content, "LEAVE charlie voluntary") {
		t.Errorf("log should contain leave event with voluntary reason, got: %q", content)
	}
}

func TestLogLeaveEventDrop(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgLeave,
		Sender:    "dave",
		Extra:     "drop",
	})
	l.Close()

	content := readFile(t, dir, now)
	if !strings.Contains(content, "LEAVE dave drop") {
		t.Errorf("log should contain leave event with drop reason, got: %q", content)
	}
}

func TestLogLeaveEventKicked(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgLeave,
		Sender:    "eve",
		Extra:     "kicked",
	})
	l.Close()

	content := readFile(t, dir, now)
	if !strings.Contains(content, "LEAVE eve kicked") {
		t.Errorf("log should contain leave event with kicked reason, got: %q", content)
	}
}

func TestLogModerationActions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	actions := []struct {
		action string
		target string
		admin  string
		expect string
	}{
		{"kicked", "alice", "admin1", "MOD kicked alice admin1"},
		{"banned", "bob", "admin1", "MOD banned bob admin1"},
		{"muted", "charlie", "admin2", "MOD muted charlie admin2"},
		{"unmuted", "charlie", "admin2", "MOD unmuted charlie admin2"},
		{"promoted", "dave", "Server", "MOD promoted dave Server"},
		{"demoted", "dave", "Server", "MOD demoted dave Server"},
	}

	for _, a := range actions {
		l.Log(models.Message{
			Timestamp: now,
			Type:      models.MsgModeration,
			Sender:    a.target,
			Content:   a.action,
			Extra:     a.admin,
		})
	}
	l.Close()

	content := readFile(t, dir, now)
	for _, a := range actions {
		if !strings.Contains(content, a.expect) {
			t.Errorf("log should contain %q, got: %q", a.expect, content)
		}
	}
}

func TestLogAnnouncement(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgAnnouncement,
		Content:   "Server maintenance at midnight",
		Extra:     "admin1",
	})
	l.Close()

	content := readFile(t, dir, now)
	if !strings.Contains(content, "ANNOUNCE [admin1]:Server maintenance at midnight") {
		t.Errorf("log should contain announcement, got: %q", content)
	}
}

func TestLogServerEvent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgServerEvent,
		Content:   "Server started on port 8989",
	})
	l.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgServerEvent,
		Content:   "Server shutting down",
	})
	l.Close()

	content := readFile(t, dir, now)
	if !strings.Contains(content, "SERVER Server started on port 8989") {
		t.Errorf("log should contain start event, got: %q", content)
	}
	if !strings.Contains(content, "SERVER Server shutting down") {
		t.Errorf("log should contain stop event, got: %q", content)
	}
}

func TestSameDayAppend(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l1, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l1.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgChat,
		Sender:    "alice",
		Content:   "first session",
	})
	l1.Close()

	// Simulate restart: create a new logger pointing to the same directory
	l2, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	l2.Log(models.Message{
		Timestamp: now,
		Type:      models.MsgChat,
		Sender:    "bob",
		Content:   "second session",
	})
	l2.Close()

	content := readFile(t, dir, now)
	if !strings.Contains(content, "first session") {
		t.Error("first session message should be present after restart")
	}
	if !strings.Contains(content, "second session") {
		t.Error("second session message should be appended")
	}
}

func TestConcurrentLogWrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				l.Log(models.Message{
					Timestamp: now,
					Type:      models.MsgChat,
					Sender:    "user",
					Content:   "concurrent message",
				})
			}
		}(i)
	}
	wg.Wait()
	l.Close()

	content := readFile(t, dir, now)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 100 {
		t.Errorf("expected 100 log lines from concurrent writes, got %d", len(lines))
	}
}

func TestNilLoggerSafe(t *testing.T) {
	var l *Logger

	// None of these should panic
	l.Log(models.Message{Timestamp: time.Now(), Type: models.MsgChat, Sender: "x", Content: "y"})
	l.Close()
	if l.FilePath("2026-01-01") != "" {
		t.Error("nil logger FilePath should return empty")
	}
	if l.Dir() != "" {
		t.Error("nil logger Dir should return empty")
	}
}

func TestLogLineSelfContained(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	msgs := []models.Message{
		{Timestamp: now, Type: models.MsgChat, Sender: "alice", Content: "hello"},
		{Timestamp: now, Type: models.MsgJoin, Sender: "bob"},
		{Timestamp: now, Type: models.MsgLeave, Sender: "charlie", Extra: "voluntary"},
		{Timestamp: now, Type: models.MsgNameChange, Sender: "dave2", Extra: "dave"},
		{Timestamp: now, Type: models.MsgAnnouncement, Content: "test", Extra: "admin"},
		{Timestamp: now, Type: models.MsgModeration, Sender: "eve", Content: "kicked", Extra: "admin"},
		{Timestamp: now, Type: models.MsgServerEvent, Content: "Server started"},
	}

	for _, msg := range msgs {
		l.Log(msg)
	}
	l.Close()

	content := readFile(t, dir, now)
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != len(msgs) {
		t.Fatalf("expected %d lines, got %d", len(msgs), len(lines))
	}

	// Each line should be parseable back to a message
	for i, line := range lines {
		parsed, err := models.ParseLogLine(line)
		if err != nil {
			t.Errorf("line %d not parseable: %v (line: %q)", i, err, line)
			continue
		}
		if parsed.Type != msgs[i].Type {
			t.Errorf("line %d: type mismatch: got %d, want %d", i, parsed.Type, msgs[i].Type)
		}
	}
}

func TestFormatDate(t *testing.T) {
	ts := time.Date(2026, 2, 5, 10, 30, 0, 0, time.Local)
	got := FormatDate(ts)
	if got != "2026-02-05" {
		t.Errorf("FormatDate = %q, want %q", got, "2026-02-05")
	}
}

// readFile reads the log file for the given date from the logs directory.
func readFile(t *testing.T, logsDir string, ts time.Time) string {
	t.Helper()
	path := filepath.Join(logsDir, "chat_"+FormatDate(ts)+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read log file %s: %v", path, err)
	}
	return string(data)
}
