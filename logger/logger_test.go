package logger

import (
	"net-cat/models"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNew_CreatesDirectory verifies the scenario described by its name.
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

// TestLogFileNaming verifies the scenario described by its name.
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

// TestLogChatMessage verifies the scenario described by its name.
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

// TestLogJoinEvent verifies the scenario described by its name.
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

// TestLogLeaveEventVoluntary verifies the scenario described by its name.
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

// TestLogLeaveEventDrop verifies the scenario described by its name.
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

// TestLogLeaveEventKicked verifies the scenario described by its name.
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

// TestLogModerationActions verifies the scenario described by its name.
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

// TestLogAnnouncement verifies the scenario described by its name.
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

// TestLogServerEvent verifies the scenario described by its name.
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

// TestSameDayAppend verifies the scenario described by its name.
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

// TestConcurrentLogWrites verifies the scenario described by its name.
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

// TestNilLoggerSafe verifies the scenario described by its name.
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

// TestLogLineSelfContained verifies the scenario described by its name.
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

// TestFormatDate verifies the scenario described by its name.
func TestFormatDate(t *testing.T) {
	ts := time.Date(2026, 2, 5, 10, 30, 0, 0, time.Local)
	got := FormatDate(ts)
	if got != "2026-02-05" {
		t.Errorf("FormatDate = %q, want %q", got, "2026-02-05")
	}
}

// TestLogDayBoundarySwitchesFile verifies the scenario described by its name.
func TestLogDayBoundarySwitchesFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Log a message with a "yesterday" timestamp
	yesterday := time.Date(2026, 2, 20, 23, 59, 59, 0, time.Local)
	l.Log(models.Message{
		Timestamp: yesterday,
		Type:      models.MsgChat,
		Sender:    "alice",
		Content:   "before midnight",
	})

	// Log a message with a "today" timestamp (next day)
	today := time.Date(2026, 2, 21, 0, 0, 1, 0, time.Local)
	l.Log(models.Message{
		Timestamp: today,
		Type:      models.MsgChat,
		Sender:    "alice",
		Content:   "after midnight",
	})

	// Verify yesterday's file has only the "before midnight" message
	yesterdayContent := readFileForDate(t, dir, "2026-02-20")
	if !strings.Contains(yesterdayContent, "before midnight") {
		t.Error("yesterday's log should contain 'before midnight'")
	}
	if strings.Contains(yesterdayContent, "after midnight") {
		t.Error("yesterday's log should NOT contain 'after midnight'")
	}

	// Verify today's file has only the "after midnight" message
	todayContent := readFileForDate(t, dir, "2026-02-21")
	if !strings.Contains(todayContent, "after midnight") {
		t.Error("today's log should contain 'after midnight'")
	}
	if strings.Contains(todayContent, "before midnight") {
		t.Error("today's log should NOT contain 'before midnight'")
	}
}

// TestLogDayBoundaryNoLostEntries verifies the scenario described by its name.
func TestLogDayBoundaryNoLostEntries(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Log messages on both sides of midnight
	day1 := time.Date(2026, 3, 15, 23, 59, 58, 0, time.Local)
	day2 := time.Date(2026, 3, 16, 0, 0, 2, 0, time.Local)

	messages := []struct {
		ts   time.Time
		text string
	}{
		{day1, "msg1"},
		{day1.Add(time.Second), "msg2"},
		{day2, "msg3"},
		{day2.Add(time.Second), "msg4"},
	}

	for _, m := range messages {
		l.Log(models.Message{
			Timestamp: m.ts,
			Type:      models.MsgChat,
			Sender:    "user",
			Content:   m.text,
		})
	}

	// Count total entries across both files
	d1Content := readFileForDate(t, dir, "2026-03-15")
	d2Content := readFileForDate(t, dir, "2026-03-16")

	d1Lines := strings.Split(strings.TrimSpace(d1Content), "\n")
	d2Lines := strings.Split(strings.TrimSpace(d2Content), "\n")

	total := len(d1Lines) + len(d2Lines)
	if total != 4 {
		t.Errorf("expected 4 total entries across both files, got %d (day1: %d, day2: %d)",
			total, len(d1Lines), len(d2Lines))
	}
}

// TestLogPriorDayFileUnmodified verifies the scenario described by its name.
func TestLogPriorDayFileUnmodified(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write to a prior day
	priorDay := time.Date(2026, 2, 19, 12, 0, 0, 0, time.Local)
	l.Log(models.Message{
		Timestamp: priorDay,
		Type:      models.MsgChat,
		Sender:    "alice",
		Content:   "old message",
	})
	l.Close()

	// Record the prior day's file content
	priorContent := readFileForDate(t, dir, "2026-02-19")

	// Create a new logger (simulate restart) and write to current day
	l2, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	currentDay := time.Date(2026, 2, 21, 12, 0, 0, 0, time.Local)
	l2.Log(models.Message{
		Timestamp: currentDay,
		Type:      models.MsgChat,
		Sender:    "bob",
		Content:   "new message",
	})
	l2.Close()

	// Prior day's file should be unchanged
	priorContentAfter := readFileForDate(t, dir, "2026-02-19")
	if priorContent != priorContentAfter {
		t.Error("prior day's log file was modified when writing to current day")
	}
}

// TestLogWritePermissionDenied verifies that when the log file cannot be written
// (e.g. permission denied), the logger degrades gracefully: it prints an error to
// stderr but does NOT panic or crash. As required by spec 12 edge case:
// "Log file permissions prevent writing: error to console, chat continues."
// TestLogWritePermissionDenied verifies the scenario described by its name.
func TestLogWritePermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test not reliable on Windows")
	}

	dir := filepath.Join(t.TempDir(), "logs")
	l, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// Write one message successfully to create the log file
	ts := time.Date(2026, 2, 24, 10, 0, 0, 0, time.Local)
	l.Log(models.Message{
		Timestamp: ts,
		Type:      models.MsgChat,
		Sender:    "alice",
		Content:   "before permission change",
	})
	l.Close()

	// Make the directory read-only to prevent file creation/writing
	if err := os.Chmod(dir, 0444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0700) // restore for cleanup

	// Create a new logger pointing at the read-only directory
	// New() itself may fail since it calls MkdirAll, so we construct manually
	l2 := &Logger{logsDir: dir}

	// Capture stderr to verify error message
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// This should NOT panic — it should log an error to stderr and return
	l2.Log(models.Message{
		Timestamp: ts,
		Type:      models.MsgChat,
		Sender:    "bob",
		Content:   "after permission change",
	})

	w.Close()
	var stderrBuf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, readErr := r.Read(tmp)
		if n > 0 {
			stderrBuf.Write(tmp[:n])
		}
		if readErr != nil {
			break
		}
	}
	r.Close()
	os.Stderr = oldStderr

	stderrOutput := stderrBuf.String()
	if !strings.Contains(stderrOutput, "Logger error") && !strings.Contains(stderrOutput, "Logger write error") {
		t.Errorf("permission denied should produce stderr error, got: %q", stderrOutput)
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

// readFileForDate reads the log file for a specific date string (YYYY-MM-DD).
func readFileForDate(t *testing.T, logsDir, date string) string {
	t.Helper()
	path := filepath.Join(logsDir, "chat_"+date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read log file %s: %v", path, err)
	}
	return string(data)
}
