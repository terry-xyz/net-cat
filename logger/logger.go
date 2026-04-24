package logger

import (
	"fmt"
	"github.com/terry-xyz/net-cat/models"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger writes chat events to daily log files in a thread-safe manner.
// All methods are nil-safe: calling any method on a nil Logger is a no-op.
type Logger struct {
	mu      sync.Mutex
	logsDir string
	file    *os.File
	curDate string
	closed  bool
}

// New creates a logger rooted at logsDir and prepares the directory for daily log files.
func New(logsDir string) (*Logger, error) {
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		return nil, err
	}
	return &Logger{logsDir: logsDir}, nil
}

// Log appends one message to the current day log file, switching files when the date changes.
func (l *Logger) Log(msg models.Message) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}

	date := formatDate(msg.Timestamp)
	if err := l.ensureFile(date); err != nil {
		fmt.Fprintf(os.Stderr, "Logger error: %v\n", err)
		return
	}

	line := msg.FormatLogLine() + "\n"
	if _, err := l.file.WriteString(line); err != nil {
		fmt.Fprintf(os.Stderr, "Logger write error: %v\n", err)
	}
}

// FilePath returns the log file path for the given date string (YYYY-MM-DD).
func (l *Logger) FilePath(date string) string {
	if l == nil {
		return ""
	}
	return filepath.Join(l.logsDir, "chat_"+date+".log")
}

// Dir returns the logs directory path.
func (l *Logger) Dir() string {
	if l == nil {
		return ""
	}
	return l.logsDir
}

// Close closes the current log file and prevents future writes. Nil-safe.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// FormatDate returns a date string formatted as YYYY-MM-DD for the given time.
func FormatDate(t time.Time) string {
	return formatDate(t)
}

// ensureFile opens the file for the requested date so writes land in the correct daily log.
func (l *Logger) ensureFile(date string) error {
	if l.curDate == date && l.file != nil {
		return nil
	}
	if l.file != nil {
		l.file.Close()
	}
	fname := filepath.Join(l.logsDir, "chat_"+date+".log")
	f, err := os.OpenFile(fname, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		l.file = nil
		return err
	}
	l.file = f
	l.curDate = date
	return nil
}

// formatDate formats a timestamp as YYYY-MM-DD for internal file selection.
func formatDate(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d", t.Year(), int(t.Month()), t.Day())
}
