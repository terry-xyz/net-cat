package models

import (
	"fmt"
	"strings"
	"time"
)

// MessageType identifies the kind of chat event.
type MessageType int

const (
	MsgChat MessageType = iota
	MsgJoin
	MsgLeave
	MsgNameChange
	MsgAnnouncement
	MsgModeration
	MsgServerEvent
)

// Message represents a single chat event stored in history and logs.
// Field semantics vary by Type:
//
//	MsgChat:         Sender=username, Content=message text
//	MsgJoin:         Sender=username
//	MsgLeave:        Sender=username, Extra=reason (voluntary|drop|kicked|banned)
//	MsgNameChange:   Sender=new name, Extra=old name
//	MsgAnnouncement: Content=message text, Extra=announcer name
//	MsgModeration:   Sender=target, Content=action verb, Extra=admin name
//	MsgServerEvent:  Content=description
type Message struct {
	Timestamp time.Time
	Sender    string
	Content   string
	Type      MessageType
	Extra     string
	Room      string
}

// FormatTimestamp formats a time as YYYY-MM-DD HH:MM:SS in 24-hour local time.
func FormatTimestamp(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
		t.Year(), int(t.Month()), t.Day(),
		t.Hour(), t.Minute(), t.Second())
}

func FormatChat(t time.Time, username, content string) string {
	return fmt.Sprintf("[%s][%s]:%s", FormatTimestamp(t), username, content)
}

func FormatPrompt(t time.Time, username string) string {
	return fmt.Sprintf("[%s][%s]:", FormatTimestamp(t), username)
}

func FormatJoin(username string) string {
	return fmt.Sprintf("%s has joined our chat...", username)
}

func FormatLeave(username string) string {
	return fmt.Sprintf("%s has left our chat...", username)
}

func FormatNameChange(oldName, newName string) string {
	return fmt.Sprintf("%s changed their name to %s", oldName, newName)
}

func FormatAnnouncement(message string) string {
	return fmt.Sprintf("[ANNOUNCEMENT]: %s", message)
}

func FormatModeration(target, action, admin string) string {
	return fmt.Sprintf("%s was %s by %s", target, action, admin)
}

func FormatWhisperReceive(t time.Time, sender, message string) string {
	return fmt.Sprintf("[%s][PM from %s]: %s", FormatTimestamp(t), sender, message)
}

func FormatWhisperSend(t time.Time, recipient, message string) string {
	return fmt.Sprintf("[%s][PM to %s]: %s", FormatTimestamp(t), recipient, message)
}

// Display returns the string a client sees for this event.
func (m Message) Display() string {
	switch m.Type {
	case MsgChat:
		return FormatChat(m.Timestamp, m.Sender, m.Content)
	case MsgJoin:
		return FormatJoin(m.Sender)
	case MsgLeave:
		return FormatLeave(m.Sender)
	case MsgNameChange:
		return FormatNameChange(m.Extra, m.Sender)
	case MsgAnnouncement:
		return FormatAnnouncement(m.Content)
	case MsgModeration:
		return FormatModeration(m.Sender, m.Content, m.Extra)
	case MsgServerEvent:
		return m.Content
	default:
		return m.Content
	}
}

// FormatLogLine produces a parseable line for the daily log file.
func (m Message) FormatLogLine() string {
	ts := FormatTimestamp(m.Timestamp)
	// Room tag inserted between timestamp and type keyword for all types except MsgServerEvent
	roomTag := ""
	if m.Type != MsgServerEvent && m.Room != "" {
		roomTag = "@" + m.Room + " "
	}
	switch m.Type {
	case MsgChat:
		return fmt.Sprintf("[%s] %sCHAT [%s]:%s", ts, roomTag, m.Sender, m.Content)
	case MsgJoin:
		return fmt.Sprintf("[%s] %sJOIN %s", ts, roomTag, m.Sender)
	case MsgLeave:
		reason := "voluntary"
		if m.Extra != "" {
			reason = m.Extra
		}
		return fmt.Sprintf("[%s] %sLEAVE %s %s", ts, roomTag, m.Sender, reason)
	case MsgNameChange:
		return fmt.Sprintf("[%s] %sNAMECHANGE %s %s", ts, roomTag, m.Extra, m.Sender)
	case MsgAnnouncement:
		return fmt.Sprintf("[%s] %sANNOUNCE [%s]:%s", ts, roomTag, m.Extra, m.Content)
	case MsgModeration:
		return fmt.Sprintf("[%s] %sMOD %s %s %s", ts, roomTag, m.Content, m.Sender, m.Extra)
	case MsgServerEvent:
		return fmt.Sprintf("[%s] SERVER %s", ts, m.Content)
	default:
		return fmt.Sprintf("[%s] %sUNKNOWN %s", ts, roomTag, m.Content)
	}
}

// ParseLogLine reconstructs a Message from a log line produced by FormatLogLine.
func ParseLogLine(line string) (Message, error) {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return Message{}, fmt.Errorf("empty line")
	}

	if line[0] != '[' {
		return Message{}, fmt.Errorf("invalid format: missing opening bracket")
	}
	closeBracket := strings.IndexByte(line, ']')
	if closeBracket < 2 {
		return Message{}, fmt.Errorf("invalid format: malformed timestamp")
	}

	tsStr := line[1:closeBracket]
	var year, month, day, hour, min, sec int
	n, err := fmt.Sscanf(tsStr, "%d-%d-%d %d:%d:%d", &year, &month, &day, &hour, &min, &sec)
	if err != nil || n != 6 {
		return Message{}, fmt.Errorf("invalid timestamp: %s", tsStr)
	}
	ts := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)

	// After "] " comes an optional @room tag then the type keyword
	if closeBracket+2 >= len(line) {
		return Message{}, fmt.Errorf("invalid format: no content after timestamp")
	}
	rest := line[closeBracket+2:]

	// Extract optional room tag
	var room string
	if len(rest) > 0 && rest[0] == '@' {
		spaceIdx := strings.IndexByte(rest, ' ')
		if spaceIdx < 0 {
			return Message{}, fmt.Errorf("invalid format: room tag without type keyword")
		}
		room = rest[1:spaceIdx]
		rest = rest[spaceIdx+1:]
	}

	if strings.HasPrefix(rest, "CHAT ") {
		inner := rest[5:]
		if len(inner) < 3 || inner[0] != '[' {
			return Message{}, fmt.Errorf("invalid CHAT format")
		}
		idx := strings.Index(inner, "]:")
		if idx < 0 {
			return Message{}, fmt.Errorf("invalid CHAT format: no closing bracket-colon")
		}
		sender := inner[1:idx]
		content := inner[idx+2:]
		return Message{Timestamp: ts, Type: MsgChat, Sender: sender, Content: content, Room: room}, nil
	}

	if strings.HasPrefix(rest, "JOIN ") {
		sender := rest[5:]
		return Message{Timestamp: ts, Type: MsgJoin, Sender: sender, Room: room}, nil
	}

	if strings.HasPrefix(rest, "LEAVE ") {
		parts := rest[6:]
		idx := strings.IndexByte(parts, ' ')
		if idx < 0 {
			return Message{Timestamp: ts, Type: MsgLeave, Sender: parts, Extra: "voluntary", Room: room}, nil
		}
		return Message{Timestamp: ts, Type: MsgLeave, Sender: parts[:idx], Extra: parts[idx+1:], Room: room}, nil
	}

	if strings.HasPrefix(rest, "NAMECHANGE ") {
		parts := rest[11:]
		idx := strings.IndexByte(parts, ' ')
		if idx < 0 {
			return Message{}, fmt.Errorf("invalid NAMECHANGE format")
		}
		return Message{Timestamp: ts, Type: MsgNameChange, Sender: parts[idx+1:], Extra: parts[:idx], Room: room}, nil
	}

	if strings.HasPrefix(rest, "ANNOUNCE ") {
		inner := rest[9:]
		if len(inner) < 3 || inner[0] != '[' {
			return Message{}, fmt.Errorf("invalid ANNOUNCE format")
		}
		idx := strings.Index(inner, "]:")
		if idx < 0 {
			return Message{}, fmt.Errorf("invalid ANNOUNCE format: no closing bracket-colon")
		}
		announcer := inner[1:idx]
		content := inner[idx+2:]
		return Message{Timestamp: ts, Type: MsgAnnouncement, Content: content, Extra: announcer, Room: room}, nil
	}

	if strings.HasPrefix(rest, "MOD ") {
		fields := strings.SplitN(rest[4:], " ", 3)
		if len(fields) < 3 {
			return Message{}, fmt.Errorf("invalid MOD format")
		}
		return Message{Timestamp: ts, Type: MsgModeration, Content: fields[0], Sender: fields[1], Extra: fields[2], Room: room}, nil
	}

	if strings.HasPrefix(rest, "SERVER ") {
		return Message{Timestamp: ts, Type: MsgServerEvent, Content: rest[7:]}, nil
	}

	return Message{}, fmt.Errorf("unknown log line type")
}
