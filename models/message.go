package models

import (
	"fmt"
	"strings"
	"time"
)

type logLineParser func(time.Time, string, string) (Message, error)

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

// FormatChat renders a standard chat line with timestamp and username.
func FormatChat(t time.Time, username, content string) string {
	return fmt.Sprintf("[%s][%s]:%s", FormatTimestamp(t), username, content)
}

// FormatPrompt renders the interactive prompt shown before the user types a message.
func FormatPrompt(t time.Time, username string) string {
	return fmt.Sprintf("[%s][%s]:", FormatTimestamp(t), username)
}

// FormatJoin renders the join notice shown to other clients in the room.
func FormatJoin(username string) string {
	return fmt.Sprintf("%s has joined our chat...", username)
}

// FormatLeave renders the leave notice shown to other clients in the room.
func FormatLeave(username string) string {
	return fmt.Sprintf("%s has left our chat...", username)
}

// FormatNameChange renders the notice broadcast after a user changes names.
func FormatNameChange(oldName, newName string) string {
	return fmt.Sprintf("%s changed their name to %s", oldName, newName)
}

// FormatAnnouncement renders a server-wide announcement banner.
func FormatAnnouncement(message string) string {
	return fmt.Sprintf("[ANNOUNCEMENT]: %s", message)
}

// FormatModeration renders kick, ban, mute, unmute, promote, and demote notices.
func FormatModeration(target, action, admin string) string {
	return fmt.Sprintf("%s was %s by %s", target, action, admin)
}

// FormatWhisperReceive renders the private-message line shown to the recipient.
func FormatWhisperReceive(t time.Time, sender, message string) string {
	return fmt.Sprintf("[%s][PM from %s]: %s", FormatTimestamp(t), sender, message)
}

// FormatWhisperSend renders the private-message echo shown back to the sender.
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

	ts, rest, err := parseLogTimestamp(line)
	if err != nil {
		return Message{}, err
	}
	room, rest, err := parseLogRoomTag(rest)
	if err != nil {
		return Message{}, err
	}
	keyword, payload, err := splitLogKeyword(rest)
	if err != nil {
		return Message{}, err
	}
	parser, ok := logLineParsers[keyword]
	if !ok {
		return Message{}, fmt.Errorf("unknown log line type")
	}
	return parser(ts, payload, room)
}

var logLineParsers = map[string]logLineParser{
	"CHAT":       parseChatLogLine,
	"JOIN":       parseJoinLogLine,
	"LEAVE":      parseLeaveLogLine,
	"NAMECHANGE": parseNameChangeLogLine,
	"ANNOUNCE":   parseAnnouncementLogLine,
	"MOD":        parseModerationLogLine,
	"SERVER":     parseServerEventLogLine,
}

func parseLogTimestamp(line string) (time.Time, string, error) {
	if line[0] != '[' {
		return time.Time{}, "", fmt.Errorf("invalid format: missing opening bracket")
	}
	closeBracket := strings.IndexByte(line, ']')
	if closeBracket < 2 {
		return time.Time{}, "", fmt.Errorf("invalid format: malformed timestamp")
	}

	tsStr := line[1:closeBracket]
	var year, month, day, hour, min, sec int
	n, err := fmt.Sscanf(tsStr, "%d-%d-%d %d:%d:%d", &year, &month, &day, &hour, &min, &sec)
	if err != nil || n != 6 {
		return time.Time{}, "", fmt.Errorf("invalid timestamp: %s", tsStr)
	}
	if closeBracket+2 >= len(line) {
		return time.Time{}, "", fmt.Errorf("invalid format: no content after timestamp")
	}

	ts := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)
	return ts, line[closeBracket+2:], nil
}

func parseLogRoomTag(rest string) (string, string, error) {
	if len(rest) == 0 || rest[0] != '@' {
		return "", rest, nil
	}
	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		return "", "", fmt.Errorf("invalid format: room tag without type keyword")
	}
	return rest[1:spaceIdx], rest[spaceIdx+1:], nil
}

func splitLogKeyword(rest string) (string, string, error) {
	if len(rest) == 0 {
		return "", "", fmt.Errorf("invalid format: missing type keyword")
	}
	spaceIdx := strings.IndexByte(rest, ' ')
	if spaceIdx < 0 {
		return rest, "", nil
	}
	return rest[:spaceIdx], rest[spaceIdx+1:], nil
}

func parseChatLogLine(ts time.Time, payload, room string) (Message, error) {
	sender, content, err := parseBracketedPayload(payload, "CHAT")
	if err != nil {
		return Message{}, err
	}
	return Message{Timestamp: ts, Type: MsgChat, Sender: sender, Content: content, Room: room}, nil
}

func parseJoinLogLine(ts time.Time, payload, room string) (Message, error) {
	return Message{Timestamp: ts, Type: MsgJoin, Sender: payload, Room: room}, nil
}

func parseLeaveLogLine(ts time.Time, payload, room string) (Message, error) {
	idx := strings.IndexByte(payload, ' ')
	if idx < 0 {
		return Message{Timestamp: ts, Type: MsgLeave, Sender: payload, Extra: "voluntary", Room: room}, nil
	}
	return Message{Timestamp: ts, Type: MsgLeave, Sender: payload[:idx], Extra: payload[idx+1:], Room: room}, nil
}

func parseNameChangeLogLine(ts time.Time, payload, room string) (Message, error) {
	idx := strings.IndexByte(payload, ' ')
	if idx < 0 {
		return Message{}, fmt.Errorf("invalid NAMECHANGE format")
	}
	return Message{Timestamp: ts, Type: MsgNameChange, Sender: payload[idx+1:], Extra: payload[:idx], Room: room}, nil
}

func parseAnnouncementLogLine(ts time.Time, payload, room string) (Message, error) {
	announcer, content, err := parseBracketedPayload(payload, "ANNOUNCE")
	if err != nil {
		return Message{}, err
	}
	return Message{Timestamp: ts, Type: MsgAnnouncement, Content: content, Extra: announcer, Room: room}, nil
}

func parseModerationLogLine(ts time.Time, payload, room string) (Message, error) {
	fields := strings.SplitN(payload, " ", 3)
	if len(fields) < 3 {
		return Message{}, fmt.Errorf("invalid MOD format")
	}
	return Message{Timestamp: ts, Type: MsgModeration, Content: fields[0], Sender: fields[1], Extra: fields[2], Room: room}, nil
}

func parseServerEventLogLine(ts time.Time, payload, _ string) (Message, error) {
	return Message{Timestamp: ts, Type: MsgServerEvent, Content: payload}, nil
}

func parseBracketedPayload(payload, keyword string) (string, string, error) {
	if len(payload) < 3 || payload[0] != '[' {
		return "", "", fmt.Errorf("invalid %s format", keyword)
	}
	idx := strings.Index(payload, "]:")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid %s format: no closing bracket-colon", keyword)
	}
	return payload[1:idx], payload[idx+2:], nil
}
