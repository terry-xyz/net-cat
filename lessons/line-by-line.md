# Net-Cat Line By Line

This file is the primary study artifact for the repository. It covers the production Go code only, in source order, and mirrors each actual code line with a direct explanation. Blank lines and source comments are preserved for readability but are not annotated. Tests are intentionally excluded from the walkthrough and should only inform concepts or risks when they reveal behavior the production code relies on.

## Covered Files

- `main.go`
- `cmd/commands.go`
- `models/message.go`
- `logger/logger.go`
- `client/client.go`
- `server/server.go`
- `server/room.go`
- `server/clients.go`
- `server/handler.go`
- `server/commands.go`
- `server/admins.go`
- `server/history.go`
- `server/moderation.go`
- `server/operator.go`

## `main.go`

Bootstraps the process, validates CLI input, wires logging and signal handling, and starts the server runtime.

```go
// L1: Declares `main` as the package for this directory so the compiler groups this file with the rest of that package.
package main

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L5: Imports `net-cat/logger` so this file can call functionality from its `logger` package.
	"net-cat/logger"
// L6: Imports `net-cat/server` so this file can call functionality from its `server` package.
	"net-cat/server"
// L7: Imports `os` so this file can call functionality from its `os` package.
	"os"
// L8: Imports `os/signal` so this file can call functionality from its `signal` package.
	"os/signal"
// L9: Closes the import block after listing all package dependencies.
)

// L11: Declares the `main` function, which starts a named unit of behavior other code can call.
func main() {
// L12: Creates `port` as a new local binding so later lines can reuse this computed value.
	port := "8989"
// L13: Evaluates `len(os.Args) > 2` and enters the guarded branch only when that condition holds.
	if len(os.Args) > 2 {
// L14: Calls `fmt.Println` here for its side effects or returned value in the surrounding control flow.
		fmt.Println("[USAGE]: ./TCPChat $port")
// L15: Calls `os.Exit` here for its side effects or returned value in the surrounding control flow.
		os.Exit(1)
// L16: Closes the current block and returns control to the surrounding scope.
	}
// L17: Evaluates `len(os.Args) == 2` and enters the guarded branch only when that condition holds.
	if len(os.Args) == 2 {
// L18: Updates `port` so subsequent logic sees the new state.
		port = os.Args[1]
// L19: Closes the current block and returns control to the surrounding scope.
	}
// L20: Evaluates `!isValidPort(port)` and enters the guarded branch only when that condition holds.
	if !isValidPort(port) {
// L21: Calls `fmt.Println` here for its side effects or returned value in the surrounding control flow.
		fmt.Println("[USAGE]: ./TCPChat $port")
// L22: Calls `os.Exit` here for its side effects or returned value in the surrounding control flow.
		os.Exit(1)
// L23: Closes the current block and returns control to the surrounding scope.
	}

// L25: Creates `srv` from the result of `server.New`, capturing fresh state for the rest of this scope.
	srv := server.New(port)

// L27: Creates `l, err` from the result of `logger.New`, capturing fresh state for the rest of this scope.
	l, err := logger.New("logs")
// L28: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
	if err != nil {
// L29: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logger: %v\n", err)
// L30: Closes the current block and returns control to the surrounding scope.
	}
// L31: Updates `srv.Logger` so subsequent logic sees the new state.
	srv.Logger = l

// L33: Creates `sigChan` from the result of `make`, capturing fresh state for the rest of this scope.
	sigChan := make(chan os.Signal, 1)
// L34: Calls `signal.Notify` here for its side effects or returned value in the surrounding control flow.
	signal.Notify(sigChan, os.Interrupt)
// L35: Launches the following call in a new goroutine so it can run concurrently with the current path.
	go func() {
// L36: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
		<-sigChan
// L37: Calls `srv.Shutdown` here for its side effects or returned value in the surrounding control flow.
		srv.Shutdown()
		// Drain subsequent signals so the default handler does not terminate
		// the process before shutdown completes
// L40: Starts a loop controlled by `range sigChan`, repeating until the loop condition or range is exhausted.
		for range sigChan {
// L41: Closes the current block and returns control to the surrounding scope.
		}
// L42: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	}()

	// Start operator terminal (reads commands from stdin)
// L45: Launches the following call in a new goroutine so it can run concurrently with the current path.
	go srv.StartOperator(os.Stdin)

// L47: Evaluates `err := srv.Start(); err != nil` and enters the guarded branch only when that condition holds.
	if err := srv.Start(); err != nil {
// L48: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
// L49: Calls `os.Exit` here for its side effects or returned value in the surrounding control flow.
		os.Exit(1)
// L50: Closes the current block and returns control to the surrounding scope.
	}
// L51: Closes the current block and returns control to the surrounding scope.
}

// isValidPort validates a port string using byte-range checks (no strconv).
// L54: Declares the `isValidPort` function, which starts a named unit of behavior other code can call.
func isValidPort(s string) bool {
// L55: Evaluates `len(s) == 0` and enters the guarded branch only when that condition holds.
	if len(s) == 0 {
// L56: Returns the listed values to the caller, ending the current function at this point.
		return false
// L57: Closes the current block and returns control to the surrounding scope.
	}
// L58: Creates `port` as a new local binding so later lines can reuse this computed value.
	port := 0
// L59: Starts a loop controlled by `_, b := range []byte(s)`, repeating until the loop condition or range is exhausted.
	for _, b := range []byte(s) {
// L60: Evaluates `b < '0' || b > '9'` and enters the guarded branch only when that condition holds.
		if b < '0' || b > '9' {
// L61: Returns the listed values to the caller, ending the current function at this point.
			return false
// L62: Closes the current block and returns control to the surrounding scope.
		}
// L63: Updates `port` with the result of `int`, replacing its previous value.
		port = port*10 + int(b-'0')
// L64: Evaluates `port > 65535` and enters the guarded branch only when that condition holds.
		if port > 65535 {
// L65: Returns the listed values to the caller, ending the current function at this point.
			return false
// L66: Closes the current block and returns control to the surrounding scope.
		}
// L67: Closes the current block and returns control to the surrounding scope.
	}
// L68: Returns the listed values to the caller, ending the current function at this point.
	return port >= 1
// L69: Closes the current block and returns control to the surrounding scope.
}
```

## `cmd/commands.go`

Defines the command catalog, privilege metadata, and parsing helpers shared by chat users and the operator console.

```go
// L1: Declares `cmd` as the package for this directory so the compiler groups this file with the rest of that package.
package cmd

// L3: Imports strings because later code relies on APIs from that package.
import "strings"

// PrivilegeLevel controls who may invoke a command.
// L6: Defines the `PrivilegeLevel` type alias or named type so the rest of the package can express this concept explicitly.
type PrivilegeLevel int

// L8: Starts a constant block for related immutable values used throughout this file.
const (
// L9: Declares `PrivUser` inside the package-level const block so other code in this package can reuse it.
	PrivUser     PrivilegeLevel = iota // any connected client
// L10: Declares `PrivAdmin` inside the package-level const block so other code in this package can reuse it.
	PrivAdmin                          // promoted admin or server operator
// L11: Declares `PrivOperator` inside the package-level const block so other code in this package can reuse it.
	PrivOperator                       // server operator terminal only
// L12: Closes the grouped declaration block.
)

// CommandDef describes a registered command.
// L15: Defines the `CommandDef` struct, which groups related state that this package manages together.
type CommandDef struct {
// L16: Adds the `Name` field to the struct so instances can hold that piece of state.
	Name        string
// L17: Adds the `MinPriv` field to the struct so instances can hold that piece of state.
	MinPriv     PrivilegeLevel
// L18: Adds the `Usage` field to the struct so instances can hold that piece of state.
	Usage       string
// L19: Adds the `Description` field to the struct so instances can hold that piece of state.
	Description string
// L20: Closes the struct definition after listing all of its fields.
}

// Commands is the canonical command registry.
// L23: Declares `Commands` in the current scope so later lines can fill or mutate it as needed.
var Commands = map[string]CommandDef{
// L24: Keeps this element in the surrounding multiline literal or call expression.
	"list":     {Name: "list", MinPriv: PrivUser, Usage: "/list", Description: "List connected clients with idle times"},
// L25: Keeps this element in the surrounding multiline literal or call expression.
	"quit":     {Name: "quit", MinPriv: PrivUser, Usage: "/quit", Description: "Disconnect from chat"},
// L26: Keeps this element in the surrounding multiline literal or call expression.
	"name":     {Name: "name", MinPriv: PrivUser, Usage: "/name <newname>", Description: "Change your display name"},
// L27: Keeps this element in the surrounding multiline literal or call expression.
	"whisper":  {Name: "whisper", MinPriv: PrivUser, Usage: "/whisper <name> <message>", Description: "Send a private message"},
// L28: Keeps this element in the surrounding multiline literal or call expression.
	"help":     {Name: "help", MinPriv: PrivUser, Usage: "/help", Description: "Show available commands"},
// L29: Keeps this element in the surrounding multiline literal or call expression.
	"rooms":    {Name: "rooms", MinPriv: PrivUser, Usage: "/rooms", Description: "List available rooms with client counts"},
// L30: Keeps this element in the surrounding multiline literal or call expression.
	"switch":   {Name: "switch", MinPriv: PrivUser, Usage: "/switch <room>", Description: "Switch to another room"},
// L31: Keeps this element in the surrounding multiline literal or call expression.
	"create":   {Name: "create", MinPriv: PrivUser, Usage: "/create <room>", Description: "Create and switch to a new room"},
// L32: Keeps this element in the surrounding multiline literal or call expression.
	"kick":     {Name: "kick", MinPriv: PrivAdmin, Usage: "/kick <name>", Description: "Kick a user from chat"},
// L33: Keeps this element in the surrounding multiline literal or call expression.
	"ban":      {Name: "ban", MinPriv: PrivAdmin, Usage: "/ban <name>", Description: "Ban a user from chat"},
// L34: Keeps this element in the surrounding multiline literal or call expression.
	"mute":     {Name: "mute", MinPriv: PrivAdmin, Usage: "/mute <name>", Description: "Mute a user"},
// L35: Keeps this element in the surrounding multiline literal or call expression.
	"unmute":   {Name: "unmute", MinPriv: PrivAdmin, Usage: "/unmute <name>", Description: "Unmute a user"},
// L36: Keeps this element in the surrounding multiline literal or call expression.
	"announce": {Name: "announce", MinPriv: PrivAdmin, Usage: "/announce <message>", Description: "Broadcast an announcement"},
// L37: Keeps this element in the surrounding multiline literal or call expression.
	"promote":  {Name: "promote", MinPriv: PrivOperator, Usage: "/promote <name>", Description: "Promote a user to admin"},
// L38: Keeps this element in the surrounding multiline literal or call expression.
	"demote":   {Name: "demote", MinPriv: PrivOperator, Usage: "/demote <name>", Description: "Demote an admin"},
// L39: Closes the current block and returns control to the surrounding scope.
}

// CommandOrder defines the display order used by /help.
// L42: Declares `CommandOrder` in the current scope so later lines can fill or mutate it as needed.
var CommandOrder = []string{
// L43: Keeps this element in the surrounding multiline literal or call expression.
	"list", "rooms", "switch", "create", "quit", "name", "whisper", "help",
// L44: Keeps this element in the surrounding multiline literal or call expression.
	"kick", "ban", "mute", "unmute", "announce",
// L45: Keeps this element in the surrounding multiline literal or call expression.
	"promote", "demote",
// L46: Closes the current block and returns control to the surrounding scope.
}

// ParseCommand splits a /-prefixed input into command name and trimmed arguments.
// Returns isCommand=false for non-command input.
// L50: Declares the `ParseCommand` function, which starts a named unit of behavior other code can call.
func ParseCommand(input string) (name string, args string, isCommand bool) {
// L51: Evaluates `len(input) == 0 || input[0] != '/'` and enters the guarded branch only when that condition holds.
	if len(input) == 0 || input[0] != '/' {
// L52: Returns the listed values to the caller, ending the current function at this point.
		return "", "", false
// L53: Closes the current block and returns control to the surrounding scope.
	}
// L54: Creates `rest` as a new local binding so later lines can reuse this computed value.
	rest := input[1:]
// L55: Evaluates `len(rest) == 0` and enters the guarded branch only when that condition holds.
	if len(rest) == 0 {
// L56: Returns the listed values to the caller, ending the current function at this point.
		return "", "", true // lone "/"
// L57: Closes the current block and returns control to the surrounding scope.
	}
// L58: Creates `idx` from the result of `strings.IndexByte`, capturing fresh state for the rest of this scope.
	idx := strings.IndexByte(rest, ' ')
// L59: Evaluates `idx < 0` and enters the guarded branch only when that condition holds.
	if idx < 0 {
// L60: Returns the listed values to the caller, ending the current function at this point.
		return rest, "", true
// L61: Closes the current block and returns control to the surrounding scope.
	}
// L62: Returns the listed values to the caller, ending the current function at this point.
	return rest[:idx], strings.TrimSpace(rest[idx+1:]), true
// L63: Closes the current block and returns control to the surrounding scope.
}

// GetPrivilegeLevel maps boolean flags to the corresponding level.
// L66: Declares the `GetPrivilegeLevel` function, which starts a named unit of behavior other code can call.
func GetPrivilegeLevel(isAdmin, isOperator bool) PrivilegeLevel {
// L67: Evaluates `isOperator` and enters the guarded branch only when that condition holds.
	if isOperator {
// L68: Returns the listed values to the caller, ending the current function at this point.
		return PrivOperator
// L69: Closes the current block and returns control to the surrounding scope.
	}
// L70: Evaluates `isAdmin` and enters the guarded branch only when that condition holds.
	if isAdmin {
// L71: Returns the listed values to the caller, ending the current function at this point.
		return PrivAdmin
// L72: Closes the current block and returns control to the surrounding scope.
	}
// L73: Returns the listed values to the caller, ending the current function at this point.
	return PrivUser
// L74: Closes the current block and returns control to the surrounding scope.
}
```

## `models/message.go`

Defines the canonical event model for chat activity, terminal rendering, log formatting, and log replay.

```go
// L1: Declares `models` as the package for this directory so the compiler groups this file with the rest of that package.
package models

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L5: Imports `strings` so this file can call functionality from its `strings` package.
	"strings"
// L6: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L7: Closes the import block after listing all package dependencies.
)

// MessageType identifies the kind of chat event.
// L10: Defines the `MessageType` type alias or named type so the rest of the package can express this concept explicitly.
type MessageType int

// L12: Starts a constant block for related immutable values used throughout this file.
const (
// L13: Declares `MsgChat` inside the package-level const block so other code in this package can reuse it.
	MsgChat MessageType = iota
// L14: Declares `MsgJoin` inside the package-level const block so other code in this package can reuse it.
	MsgJoin
// L15: Declares `MsgLeave` inside the package-level const block so other code in this package can reuse it.
	MsgLeave
// L16: Declares `MsgNameChange` inside the package-level const block so other code in this package can reuse it.
	MsgNameChange
// L17: Declares `MsgAnnouncement` inside the package-level const block so other code in this package can reuse it.
	MsgAnnouncement
// L18: Declares `MsgModeration` inside the package-level const block so other code in this package can reuse it.
	MsgModeration
// L19: Declares `MsgServerEvent` inside the package-level const block so other code in this package can reuse it.
	MsgServerEvent
// L20: Closes the grouped declaration block.
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
// L32: Defines the `Message` struct, which groups related state that this package manages together.
type Message struct {
// L33: Adds the `Timestamp` field to the struct so instances can hold that piece of state.
	Timestamp time.Time
// L34: Adds the `Sender` field to the struct so instances can hold that piece of state.
	Sender    string
// L35: Adds the `Content` field to the struct so instances can hold that piece of state.
	Content   string
// L36: Adds the `Type` field to the struct so instances can hold that piece of state.
	Type      MessageType
// L37: Adds the `Extra` field to the struct so instances can hold that piece of state.
	Extra     string
// L38: Adds the `Room` field to the struct so instances can hold that piece of state.
	Room      string
// L39: Closes the struct definition after listing all of its fields.
}

// FormatTimestamp formats a time as YYYY-MM-DD HH:MM:SS in 24-hour local time.
// L42: Declares the `FormatTimestamp` function, which starts a named unit of behavior other code can call.
func FormatTimestamp(t time.Time) string {
// L43: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
// L44: Keeps this element in the surrounding multiline literal or call expression.
		t.Year(), int(t.Month()), t.Day(),
// L45: Calls `t.Hour` here for its side effects or returned value in the surrounding control flow.
		t.Hour(), t.Minute(), t.Second())
// L46: Closes the current block and returns control to the surrounding scope.
}

// L48: Declares the `FormatChat` function, which starts a named unit of behavior other code can call.
func FormatChat(t time.Time, username, content string) string {
// L49: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("[%s][%s]:%s", FormatTimestamp(t), username, content)
// L50: Closes the current block and returns control to the surrounding scope.
}

// L52: Declares the `FormatPrompt` function, which starts a named unit of behavior other code can call.
func FormatPrompt(t time.Time, username string) string {
// L53: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("[%s][%s]:", FormatTimestamp(t), username)
// L54: Closes the current block and returns control to the surrounding scope.
}

// L56: Declares the `FormatJoin` function, which starts a named unit of behavior other code can call.
func FormatJoin(username string) string {
// L57: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("%s has joined our chat...", username)
// L58: Closes the current block and returns control to the surrounding scope.
}

// L60: Declares the `FormatLeave` function, which starts a named unit of behavior other code can call.
func FormatLeave(username string) string {
// L61: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("%s has left our chat...", username)
// L62: Closes the current block and returns control to the surrounding scope.
}

// L64: Declares the `FormatNameChange` function, which starts a named unit of behavior other code can call.
func FormatNameChange(oldName, newName string) string {
// L65: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("%s changed their name to %s", oldName, newName)
// L66: Closes the current block and returns control to the surrounding scope.
}

// L68: Declares the `FormatAnnouncement` function, which starts a named unit of behavior other code can call.
func FormatAnnouncement(message string) string {
// L69: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("[ANNOUNCEMENT]: %s", message)
// L70: Closes the current block and returns control to the surrounding scope.
}

// L72: Declares the `FormatModeration` function, which starts a named unit of behavior other code can call.
func FormatModeration(target, action, admin string) string {
// L73: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("%s was %s by %s", target, action, admin)
// L74: Closes the current block and returns control to the surrounding scope.
}

// L76: Declares the `FormatWhisperReceive` function, which starts a named unit of behavior other code can call.
func FormatWhisperReceive(t time.Time, sender, message string) string {
// L77: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("[%s][PM from %s]: %s", FormatTimestamp(t), sender, message)
// L78: Closes the current block and returns control to the surrounding scope.
}

// L80: Declares the `FormatWhisperSend` function, which starts a named unit of behavior other code can call.
func FormatWhisperSend(t time.Time, recipient, message string) string {
// L81: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("[%s][PM to %s]: %s", FormatTimestamp(t), recipient, message)
// L82: Closes the current block and returns control to the surrounding scope.
}

// Display returns the string a client sees for this event.
// L85: Declares the `Display` method on `m Message`, creating a reusable behavior entrypoint tied to that receiver state.
func (m Message) Display() string {
// L86: Starts a switch on `m.Type` so the following cases can branch on that value cleanly.
	switch m.Type {
// L87: Selects the `MsgChat` branch inside the surrounding switch or select.
	case MsgChat:
// L88: Returns the listed values to the caller, ending the current function at this point.
		return FormatChat(m.Timestamp, m.Sender, m.Content)
// L89: Selects the `MsgJoin` branch inside the surrounding switch or select.
	case MsgJoin:
// L90: Returns the listed values to the caller, ending the current function at this point.
		return FormatJoin(m.Sender)
// L91: Selects the `MsgLeave` branch inside the surrounding switch or select.
	case MsgLeave:
// L92: Returns the listed values to the caller, ending the current function at this point.
		return FormatLeave(m.Sender)
// L93: Selects the `MsgNameChange` branch inside the surrounding switch or select.
	case MsgNameChange:
// L94: Returns the listed values to the caller, ending the current function at this point.
		return FormatNameChange(m.Extra, m.Sender)
// L95: Selects the `MsgAnnouncement` branch inside the surrounding switch or select.
	case MsgAnnouncement:
// L96: Returns the listed values to the caller, ending the current function at this point.
		return FormatAnnouncement(m.Content)
// L97: Selects the `MsgModeration` branch inside the surrounding switch or select.
	case MsgModeration:
// L98: Returns the listed values to the caller, ending the current function at this point.
		return FormatModeration(m.Sender, m.Content, m.Extra)
// L99: Selects the `MsgServerEvent` branch inside the surrounding switch or select.
	case MsgServerEvent:
// L100: Returns the listed values to the caller, ending the current function at this point.
		return m.Content
// L101: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
	default:
// L102: Returns the listed values to the caller, ending the current function at this point.
		return m.Content
// L103: Closes the current block and returns control to the surrounding scope.
	}
// L104: Closes the current block and returns control to the surrounding scope.
}

// FormatLogLine produces a parseable line for the daily log file.
// L107: Declares the `FormatLogLine` method on `m Message`, creating a reusable behavior entrypoint tied to that receiver state.
func (m Message) FormatLogLine() string {
// L108: Creates `ts` from the result of `FormatTimestamp`, capturing fresh state for the rest of this scope.
	ts := FormatTimestamp(m.Timestamp)
	// Room tag inserted between timestamp and type keyword for all types except MsgServerEvent
// L110: Creates `roomTag` as a new local binding so later lines can reuse this computed value.
	roomTag := ""
// L111: Evaluates `m.Type != MsgServerEvent && m.Room != ""` and enters the guarded branch only when that condition holds.
	if m.Type != MsgServerEvent && m.Room != "" {
// L112: Updates `roomTag` so subsequent logic sees the new state.
		roomTag = "@" + m.Room + " "
// L113: Closes the current block and returns control to the surrounding scope.
	}
// L114: Starts a switch on `m.Type` so the following cases can branch on that value cleanly.
	switch m.Type {
// L115: Selects the `MsgChat` branch inside the surrounding switch or select.
	case MsgChat:
// L116: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Sprintf("[%s] %sCHAT [%s]:%s", ts, roomTag, m.Sender, m.Content)
// L117: Selects the `MsgJoin` branch inside the surrounding switch or select.
	case MsgJoin:
// L118: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Sprintf("[%s] %sJOIN %s", ts, roomTag, m.Sender)
// L119: Selects the `MsgLeave` branch inside the surrounding switch or select.
	case MsgLeave:
// L120: Creates `reason` as a new local binding so later lines can reuse this computed value.
		reason := "voluntary"
// L121: Evaluates `m.Extra != ""` and enters the guarded branch only when that condition holds.
		if m.Extra != "" {
// L122: Updates `reason` so subsequent logic sees the new state.
			reason = m.Extra
// L123: Closes the current block and returns control to the surrounding scope.
		}
// L124: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Sprintf("[%s] %sLEAVE %s %s", ts, roomTag, m.Sender, reason)
// L125: Selects the `MsgNameChange` branch inside the surrounding switch or select.
	case MsgNameChange:
// L126: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Sprintf("[%s] %sNAMECHANGE %s %s", ts, roomTag, m.Extra, m.Sender)
// L127: Selects the `MsgAnnouncement` branch inside the surrounding switch or select.
	case MsgAnnouncement:
// L128: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Sprintf("[%s] %sANNOUNCE [%s]:%s", ts, roomTag, m.Extra, m.Content)
// L129: Selects the `MsgModeration` branch inside the surrounding switch or select.
	case MsgModeration:
// L130: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Sprintf("[%s] %sMOD %s %s %s", ts, roomTag, m.Content, m.Sender, m.Extra)
// L131: Selects the `MsgServerEvent` branch inside the surrounding switch or select.
	case MsgServerEvent:
// L132: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Sprintf("[%s] SERVER %s", ts, m.Content)
// L133: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
	default:
// L134: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Sprintf("[%s] %sUNKNOWN %s", ts, roomTag, m.Content)
// L135: Closes the current block and returns control to the surrounding scope.
	}
// L136: Closes the current block and returns control to the surrounding scope.
}

// ParseLogLine reconstructs a Message from a log line produced by FormatLogLine.
// L139: Declares the `ParseLogLine` function, which starts a named unit of behavior other code can call.
func ParseLogLine(line string) (Message, error) {
// L140: Updates `line` with the result of `strings.TrimSpace`, replacing its previous value.
	line = strings.TrimSpace(line)
// L141: Evaluates `len(line) == 0` and enters the guarded branch only when that condition holds.
	if len(line) == 0 {
// L142: Returns the listed values to the caller, ending the current function at this point.
		return Message{}, fmt.Errorf("empty line")
// L143: Closes the current block and returns control to the surrounding scope.
	}

// L145: Evaluates `line[0] != '['` and enters the guarded branch only when that condition holds.
	if line[0] != '[' {
// L146: Returns the listed values to the caller, ending the current function at this point.
		return Message{}, fmt.Errorf("invalid format: missing opening bracket")
// L147: Closes the current block and returns control to the surrounding scope.
	}
// L148: Creates `closeBracket` from the result of `strings.IndexByte`, capturing fresh state for the rest of this scope.
	closeBracket := strings.IndexByte(line, ']')
// L149: Evaluates `closeBracket < 2` and enters the guarded branch only when that condition holds.
	if closeBracket < 2 {
// L150: Returns the listed values to the caller, ending the current function at this point.
		return Message{}, fmt.Errorf("invalid format: malformed timestamp")
// L151: Closes the current block and returns control to the surrounding scope.
	}

// L153: Creates `tsStr` as a new local binding so later lines can reuse this computed value.
	tsStr := line[1:closeBracket]
// L154: Declares `year,` in the current scope so later lines can fill or mutate it as needed.
	var year, month, day, hour, min, sec int
// L155: Creates `n, err` from the result of `fmt.Sscanf`, capturing fresh state for the rest of this scope.
	n, err := fmt.Sscanf(tsStr, "%d-%d-%d %d:%d:%d", &year, &month, &day, &hour, &min, &sec)
// L156: Evaluates `err != nil || n != 6` and enters the guarded branch only when that condition holds.
	if err != nil || n != 6 {
// L157: Returns the listed values to the caller, ending the current function at this point.
		return Message{}, fmt.Errorf("invalid timestamp: %s", tsStr)
// L158: Closes the current block and returns control to the surrounding scope.
	}
// L159: Creates `ts` from the result of `time.Date`, capturing fresh state for the rest of this scope.
	ts := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)

	// After "] " comes an optional @room tag then the type keyword
// L162: Evaluates `closeBracket+2 >= len(line)` and enters the guarded branch only when that condition holds.
	if closeBracket+2 >= len(line) {
// L163: Returns the listed values to the caller, ending the current function at this point.
		return Message{}, fmt.Errorf("invalid format: no content after timestamp")
// L164: Closes the current block and returns control to the surrounding scope.
	}
// L165: Creates `rest` as a new local binding so later lines can reuse this computed value.
	rest := line[closeBracket+2:]

	// Extract optional room tag
// L168: Declares `room` in the current scope so later lines can fill or mutate it as needed.
	var room string
// L169: Evaluates `len(rest) > 0 && rest[0] == '@'` and enters the guarded branch only when that condition holds.
	if len(rest) > 0 && rest[0] == '@' {
// L170: Creates `spaceIdx` from the result of `strings.IndexByte`, capturing fresh state for the rest of this scope.
		spaceIdx := strings.IndexByte(rest, ' ')
// L171: Evaluates `spaceIdx < 0` and enters the guarded branch only when that condition holds.
		if spaceIdx < 0 {
// L172: Returns the listed values to the caller, ending the current function at this point.
			return Message{}, fmt.Errorf("invalid format: room tag without type keyword")
// L173: Closes the current block and returns control to the surrounding scope.
		}
// L174: Updates `room` so subsequent logic sees the new state.
		room = rest[1:spaceIdx]
// L175: Updates `rest` so subsequent logic sees the new state.
		rest = rest[spaceIdx+1:]
// L176: Closes the current block and returns control to the surrounding scope.
	}

// L178: Evaluates `strings.HasPrefix(rest, "CHAT ")` and enters the guarded branch only when that condition holds.
	if strings.HasPrefix(rest, "CHAT ") {
// L179: Creates `inner` as a new local binding so later lines can reuse this computed value.
		inner := rest[5:]
// L180: Evaluates `len(inner) < 3 || inner[0] != '['` and enters the guarded branch only when that condition holds.
		if len(inner) < 3 || inner[0] != '[' {
// L181: Returns the listed values to the caller, ending the current function at this point.
			return Message{}, fmt.Errorf("invalid CHAT format")
// L182: Closes the current block and returns control to the surrounding scope.
		}
// L183: Creates `idx` from the result of `strings.Index`, capturing fresh state for the rest of this scope.
		idx := strings.Index(inner, "]:")
// L184: Evaluates `idx < 0` and enters the guarded branch only when that condition holds.
		if idx < 0 {
// L185: Returns the listed values to the caller, ending the current function at this point.
			return Message{}, fmt.Errorf("invalid CHAT format: no closing bracket-colon")
// L186: Closes the current block and returns control to the surrounding scope.
		}
// L187: Creates `sender` as a new local binding so later lines can reuse this computed value.
		sender := inner[1:idx]
// L188: Creates `content` as a new local binding so later lines can reuse this computed value.
		content := inner[idx+2:]
// L189: Returns the listed values to the caller, ending the current function at this point.
		return Message{Timestamp: ts, Type: MsgChat, Sender: sender, Content: content, Room: room}, nil
// L190: Closes the current block and returns control to the surrounding scope.
	}

// L192: Evaluates `strings.HasPrefix(rest, "JOIN ")` and enters the guarded branch only when that condition holds.
	if strings.HasPrefix(rest, "JOIN ") {
// L193: Creates `sender` as a new local binding so later lines can reuse this computed value.
		sender := rest[5:]
// L194: Returns the listed values to the caller, ending the current function at this point.
		return Message{Timestamp: ts, Type: MsgJoin, Sender: sender, Room: room}, nil
// L195: Closes the current block and returns control to the surrounding scope.
	}

// L197: Evaluates `strings.HasPrefix(rest, "LEAVE ")` and enters the guarded branch only when that condition holds.
	if strings.HasPrefix(rest, "LEAVE ") {
// L198: Creates `parts` as a new local binding so later lines can reuse this computed value.
		parts := rest[6:]
// L199: Creates `idx` from the result of `strings.IndexByte`, capturing fresh state for the rest of this scope.
		idx := strings.IndexByte(parts, ' ')
// L200: Evaluates `idx < 0` and enters the guarded branch only when that condition holds.
		if idx < 0 {
// L201: Returns the listed values to the caller, ending the current function at this point.
			return Message{Timestamp: ts, Type: MsgLeave, Sender: parts, Extra: "voluntary", Room: room}, nil
// L202: Closes the current block and returns control to the surrounding scope.
		}
// L203: Returns the listed values to the caller, ending the current function at this point.
		return Message{Timestamp: ts, Type: MsgLeave, Sender: parts[:idx], Extra: parts[idx+1:], Room: room}, nil
// L204: Closes the current block and returns control to the surrounding scope.
	}

// L206: Evaluates `strings.HasPrefix(rest, "NAMECHANGE ")` and enters the guarded branch only when that condition holds.
	if strings.HasPrefix(rest, "NAMECHANGE ") {
// L207: Creates `parts` as a new local binding so later lines can reuse this computed value.
		parts := rest[11:]
// L208: Creates `idx` from the result of `strings.IndexByte`, capturing fresh state for the rest of this scope.
		idx := strings.IndexByte(parts, ' ')
// L209: Evaluates `idx < 0` and enters the guarded branch only when that condition holds.
		if idx < 0 {
// L210: Returns the listed values to the caller, ending the current function at this point.
			return Message{}, fmt.Errorf("invalid NAMECHANGE format")
// L211: Closes the current block and returns control to the surrounding scope.
		}
// L212: Returns the listed values to the caller, ending the current function at this point.
		return Message{Timestamp: ts, Type: MsgNameChange, Sender: parts[idx+1:], Extra: parts[:idx], Room: room}, nil
// L213: Closes the current block and returns control to the surrounding scope.
	}

// L215: Evaluates `strings.HasPrefix(rest, "ANNOUNCE ")` and enters the guarded branch only when that condition holds.
	if strings.HasPrefix(rest, "ANNOUNCE ") {
// L216: Creates `inner` as a new local binding so later lines can reuse this computed value.
		inner := rest[9:]
// L217: Evaluates `len(inner) < 3 || inner[0] != '['` and enters the guarded branch only when that condition holds.
		if len(inner) < 3 || inner[0] != '[' {
// L218: Returns the listed values to the caller, ending the current function at this point.
			return Message{}, fmt.Errorf("invalid ANNOUNCE format")
// L219: Closes the current block and returns control to the surrounding scope.
		}
// L220: Creates `idx` from the result of `strings.Index`, capturing fresh state for the rest of this scope.
		idx := strings.Index(inner, "]:")
// L221: Evaluates `idx < 0` and enters the guarded branch only when that condition holds.
		if idx < 0 {
// L222: Returns the listed values to the caller, ending the current function at this point.
			return Message{}, fmt.Errorf("invalid ANNOUNCE format: no closing bracket-colon")
// L223: Closes the current block and returns control to the surrounding scope.
		}
// L224: Creates `announcer` as a new local binding so later lines can reuse this computed value.
		announcer := inner[1:idx]
// L225: Creates `content` as a new local binding so later lines can reuse this computed value.
		content := inner[idx+2:]
// L226: Returns the listed values to the caller, ending the current function at this point.
		return Message{Timestamp: ts, Type: MsgAnnouncement, Content: content, Extra: announcer, Room: room}, nil
// L227: Closes the current block and returns control to the surrounding scope.
	}

// L229: Evaluates `strings.HasPrefix(rest, "MOD ")` and enters the guarded branch only when that condition holds.
	if strings.HasPrefix(rest, "MOD ") {
// L230: Creates `fields` from the result of `strings.SplitN`, capturing fresh state for the rest of this scope.
		fields := strings.SplitN(rest[4:], " ", 3)
// L231: Evaluates `len(fields) < 3` and enters the guarded branch only when that condition holds.
		if len(fields) < 3 {
// L232: Returns the listed values to the caller, ending the current function at this point.
			return Message{}, fmt.Errorf("invalid MOD format")
// L233: Closes the current block and returns control to the surrounding scope.
		}
// L234: Returns the listed values to the caller, ending the current function at this point.
		return Message{Timestamp: ts, Type: MsgModeration, Content: fields[0], Sender: fields[1], Extra: fields[2], Room: room}, nil
// L235: Closes the current block and returns control to the surrounding scope.
	}

// L237: Evaluates `strings.HasPrefix(rest, "SERVER ")` and enters the guarded branch only when that condition holds.
	if strings.HasPrefix(rest, "SERVER ") {
// L238: Returns the listed values to the caller, ending the current function at this point.
		return Message{Timestamp: ts, Type: MsgServerEvent, Content: rest[7:]}, nil
// L239: Closes the current block and returns control to the surrounding scope.
	}

// L241: Returns the listed values to the caller, ending the current function at this point.
	return Message{}, fmt.Errorf("unknown log line type")
// L242: Closes the current block and returns control to the surrounding scope.
}
```

## `logger/logger.go`

Owns append-only daily log files, synchronized writes, and day-rollover handling for persisted chat history.

```go
// L1: Declares `logger` as the package for this directory so the compiler groups this file with the rest of that package.
package logger

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L5: Imports `net-cat/models` so this file can call functionality from its `models` package.
	"net-cat/models"
// L6: Imports `os` so this file can call functionality from its `os` package.
	"os"
// L7: Imports `path/filepath` so this file can call functionality from its `filepath` package.
	"path/filepath"
// L8: Imports `sync` so this file can call functionality from its `sync` package.
	"sync"
// L9: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L10: Closes the import block after listing all package dependencies.
)

// Logger writes chat events to daily log files in a thread-safe manner.
// All methods are nil-safe: calling any method on a nil Logger is a no-op.
// L14: Defines the `Logger` struct, which groups related state that this package manages together.
type Logger struct {
// L15: Adds the `mu` field to the struct so instances can hold that piece of state.
	mu      sync.Mutex
// L16: Adds the `logsDir` field to the struct so instances can hold that piece of state.
	logsDir string
// L17: Adds the `file` field to the struct so instances can hold that piece of state.
	file    *os.File
// L18: Adds the `curDate` field to the struct so instances can hold that piece of state.
	curDate string
// L19: Adds the `closed` field to the struct so instances can hold that piece of state.
	closed  bool
// L20: Closes the struct definition after listing all of its fields.
}

// New creates a Logger that writes to the given directory.
// Creates the directory if it does not exist.
// L24: Declares the `New` function, which starts a named unit of behavior other code can call.
func New(logsDir string) (*Logger, error) {
// L25: Evaluates `err := os.MkdirAll(logsDir, 0700); err != nil` and enters the guarded branch only when that condition holds.
	if err := os.MkdirAll(logsDir, 0700); err != nil {
// L26: Returns the listed values to the caller, ending the current function at this point.
		return nil, err
// L27: Closes the current block and returns control to the surrounding scope.
	}
// L28: Returns the listed values to the caller, ending the current function at this point.
	return &Logger{logsDir: logsDir}, nil
// L29: Closes the current block and returns control to the surrounding scope.
}

// Log writes a message to the daily log file. Thread-safe and nil-safe.
// After Close is called, Log is a no-op (prevents file reopening by late goroutines).
// L33: Declares the `Log` method on `l *Logger`, creating a reusable behavior entrypoint tied to that receiver state.
func (l *Logger) Log(msg models.Message) {
// L34: Evaluates `l == nil` and enters the guarded branch only when that condition holds.
	if l == nil {
// L35: Returns immediately from the current function without additional values.
		return
// L36: Closes the current block and returns control to the surrounding scope.
	}
// L37: Calls `l.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	l.mu.Lock()
// L38: Schedules this cleanup or follow-up call to run when the current function returns.
	defer l.mu.Unlock()

// L40: Evaluates `l.closed` and enters the guarded branch only when that condition holds.
	if l.closed {
// L41: Returns immediately from the current function without additional values.
		return
// L42: Closes the current block and returns control to the surrounding scope.
	}

// L44: Creates `date` from the result of `formatDate`, capturing fresh state for the rest of this scope.
	date := formatDate(msg.Timestamp)
// L45: Evaluates `err := l.ensureFile(date); err != nil` and enters the guarded branch only when that condition holds.
	if err := l.ensureFile(date); err != nil {
// L46: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Logger error: %v\n", err)
// L47: Returns immediately from the current function without additional values.
		return
// L48: Closes the current block and returns control to the surrounding scope.
	}

// L50: Creates `line` from the result of `msg.FormatLogLine`, capturing fresh state for the rest of this scope.
	line := msg.FormatLogLine() + "\n"
// L51: Evaluates `_, err := l.file.WriteString(line); err != nil` and enters the guarded branch only when that condition holds.
	if _, err := l.file.WriteString(line); err != nil {
// L52: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Logger write error: %v\n", err)
// L53: Closes the current block and returns control to the surrounding scope.
	}
// L54: Closes the current block and returns control to the surrounding scope.
}

// FilePath returns the log file path for the given date string (YYYY-MM-DD).
// L57: Declares the `FilePath` method on `l *Logger`, creating a reusable behavior entrypoint tied to that receiver state.
func (l *Logger) FilePath(date string) string {
// L58: Evaluates `l == nil` and enters the guarded branch only when that condition holds.
	if l == nil {
// L59: Returns the listed values to the caller, ending the current function at this point.
		return ""
// L60: Closes the current block and returns control to the surrounding scope.
	}
// L61: Returns the listed values to the caller, ending the current function at this point.
	return filepath.Join(l.logsDir, "chat_"+date+".log")
// L62: Closes the current block and returns control to the surrounding scope.
}

// Dir returns the logs directory path.
// L65: Declares the `Dir` method on `l *Logger`, creating a reusable behavior entrypoint tied to that receiver state.
func (l *Logger) Dir() string {
// L66: Evaluates `l == nil` and enters the guarded branch only when that condition holds.
	if l == nil {
// L67: Returns the listed values to the caller, ending the current function at this point.
		return ""
// L68: Closes the current block and returns control to the surrounding scope.
	}
// L69: Returns the listed values to the caller, ending the current function at this point.
	return l.logsDir
// L70: Closes the current block and returns control to the surrounding scope.
}

// Close closes the current log file and prevents future writes. Nil-safe.
// L73: Declares the `Close` method on `l *Logger`, creating a reusable behavior entrypoint tied to that receiver state.
func (l *Logger) Close() error {
// L74: Evaluates `l == nil` and enters the guarded branch only when that condition holds.
	if l == nil {
// L75: Returns the listed values to the caller, ending the current function at this point.
		return nil
// L76: Closes the current block and returns control to the surrounding scope.
	}
// L77: Calls `l.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	l.mu.Lock()
// L78: Schedules this cleanup or follow-up call to run when the current function returns.
	defer l.mu.Unlock()
// L79: Updates `l.closed` so subsequent logic sees the new state.
	l.closed = true
// L80: Evaluates `l.file != nil` and enters the guarded branch only when that condition holds.
	if l.file != nil {
// L81: Creates `err` from the result of `l.file.Close`, capturing fresh state for the rest of this scope.
		err := l.file.Close()
// L82: Updates `l.file` so subsequent logic sees the new state.
		l.file = nil
// L83: Returns the listed values to the caller, ending the current function at this point.
		return err
// L84: Closes the current block and returns control to the surrounding scope.
	}
// L85: Returns the listed values to the caller, ending the current function at this point.
	return nil
// L86: Closes the current block and returns control to the surrounding scope.
}

// FormatDate returns a date string formatted as YYYY-MM-DD for the given time.
// L89: Declares the `FormatDate` function, which starts a named unit of behavior other code can call.
func FormatDate(t time.Time) string {
// L90: Returns the listed values to the caller, ending the current function at this point.
	return formatDate(t)
// L91: Closes the current block and returns control to the surrounding scope.
}

// L93: Declares the `ensureFile` method on `l *Logger`, creating a reusable behavior entrypoint tied to that receiver state.
func (l *Logger) ensureFile(date string) error {
// L94: Evaluates `l.curDate == date && l.file != nil` and enters the guarded branch only when that condition holds.
	if l.curDate == date && l.file != nil {
// L95: Returns the listed values to the caller, ending the current function at this point.
		return nil
// L96: Closes the current block and returns control to the surrounding scope.
	}
// L97: Evaluates `l.file != nil` and enters the guarded branch only when that condition holds.
	if l.file != nil {
// L98: Calls `l.file.Close` here for its side effects or returned value in the surrounding control flow.
		l.file.Close()
// L99: Closes the current block and returns control to the surrounding scope.
	}
// L100: Creates `fname` from the result of `filepath.Join`, capturing fresh state for the rest of this scope.
	fname := filepath.Join(l.logsDir, "chat_"+date+".log")
// L101: Creates `f, err` from the result of `os.OpenFile`, capturing fresh state for the rest of this scope.
	f, err := os.OpenFile(fname, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
// L102: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
	if err != nil {
// L103: Updates `l.file` so subsequent logic sees the new state.
		l.file = nil
// L104: Returns the listed values to the caller, ending the current function at this point.
		return err
// L105: Closes the current block and returns control to the surrounding scope.
	}
// L106: Updates `l.file` so subsequent logic sees the new state.
	l.file = f
// L107: Updates `l.curDate` so subsequent logic sees the new state.
	l.curDate = date
// L108: Returns the listed values to the caller, ending the current function at this point.
	return nil
// L109: Closes the current block and returns control to the surrounding scope.
}

// L111: Declares the `formatDate` function, which starts a named unit of behavior other code can call.
func formatDate(t time.Time) string {
// L112: Returns the listed values to the caller, ending the current function at this point.
	return fmt.Sprintf("%04d-%02d-%02d", t.Year(), int(t.Month()), t.Day())
// L113: Closes the current block and returns control to the surrounding scope.
}
```

## `client/client.go`

Wraps each network connection with serialized output, interactive input tracking, prompt redraw support, and safe shutdown.

```go
// L1: Declares `client` as the package for this directory so the compiler groups this file with the rest of that package.
package client

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `bufio` so this file can call functionality from its `bufio` package.
	"bufio"
// L5: Imports `io` so this file can call functionality from its `io` package.
	"io"
// L6: Imports `net` so this file can call functionality from its `net` package.
	"net"
// L7: Imports `sync` so this file can call functionality from its `sync` package.
	"sync"
// L8: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L9: Closes the import block after listing all package dependencies.
)

// L11: Starts a constant block for related immutable values used throughout this file.
const (
// L12: Declares `maxLineLength` inside the package-level const block so other code in this package can reuse it.
	maxLineLength     = 1048576 // 1 MB scanner buffer limit
// L13: Declares `msgChanSize` inside the package-level const block so other code in this package can reuse it.
	msgChanSize       = 4096   // large enough for echo messages of max-length lines
// L14: Declares `maxInteractiveBuf` inside the package-level const block so other code in this package can reuse it.
	maxInteractiveBuf = 4096   // max partial input tracked for redraw
// L15: Closes the grouped declaration block.
)

// Write-message types for the writeLoop channel.
// L18: Starts a constant block for related immutable values used throughout this file.
const (
// L19: Declares `wmMessage` inside the package-level const block so other code in this package can reuse it.
	wmMessage   = iota // broadcast/notification: in echoMode clear+redraw; else raw write
// L20: Declares `wmPrompt` inside the package-level const block so other code in this package can reuse it.
	wmPrompt          // set prompt, clear inputBuf, write data
// L21: Declares `wmEcho` inside the package-level const block so other code in this package can reuse it.
	wmEcho            // append char to inputBuf, write char
// L22: Declares `wmBackspace` inside the package-level const block so other code in this package can reuse it.
	wmBackspace       // remove last from inputBuf, write \b \b
// L23: Declares `wmNewline` inside the package-level const block so other code in this package can reuse it.
	wmNewline         // clear inputBuf/prompt, write \r\n
// L24: Closes the grouped declaration block.
)

// writeMsg is the internal message type for the write channel.
// L27: Defines the `writeMsg` struct, which groups related state that this package manages together.
type writeMsg struct {
// L28: Adds the `data` field to the struct so instances can hold that piece of state.
	data    string
// L29: Adds the `msgType` field to the struct so instances can hold that piece of state.
	msgType int
// L30: Closes the struct definition after listing all of its fields.
}

// Client represents a single connected user with its own write goroutine.
// L33: Defines the `Client` struct, which groups related state that this package manages together.
type Client struct {
// L34: Adds the `Conn` field to the struct so instances can hold that piece of state.
	Conn     net.Conn
// L35: Adds the `Username` field to the struct so instances can hold that piece of state.
	Username string
// L36: Adds the `JoinTime` field to the struct so instances can hold that piece of state.
	JoinTime time.Time
// L37: Adds the `IP` field to the struct so instances can hold that piece of state.
	IP       string
// L38: Adds the `Room` field to the struct so instances can hold that piece of state.
	Room     string

	// lastActivity, muted, admin are accessed from multiple goroutines (handler,
	// operator, heartbeat) — protected by mu alongside lastInput et al.
// L42: Adds the `lastActivity` field to the struct so instances can hold that piece of state.
	lastActivity time.Time
// L43: Adds the `muted` field to the struct so instances can hold that piece of state.
	muted        bool
// L44: Adds the `admin` field to the struct so instances can hold that piece of state.
	admin        bool

// L46: Adds the `msgChan` field to the struct so instances can hold that piece of state.
	msgChan   chan writeMsg
// L47: Adds the `done` field to the struct so instances can hold that piece of state.
	done      chan struct{}
// L48: Adds the `closeOnce` field to the struct so instances can hold that piece of state.
	closeOnce sync.Once
// L49: Adds the `scanner` field to the struct so instances can hold that piece of state.
	scanner   *bufio.Scanner

	// mu protects fields accessed concurrently by multiple goroutines (handler,
	// operator, heartbeat): lastInput, disconnectReason, echoMode, lastActivity,
	// muted, and admin.
// L54: Adds the `mu` field to the struct so instances can hold that piece of state.
	mu               sync.Mutex
// L55: Adds the `lastInput` field to the struct so instances can hold that piece of state.
	lastInput        time.Time
// L56: Adds the `disconnectReason` field to the struct so instances can hold that piece of state.
	disconnectReason string
// L57: Adds the `echoMode` field to the struct so instances can hold that piece of state.
	echoMode         bool // true after onboarding; enables input continuity

	// The following fields are ONLY accessed by the writeLoop goroutine:
// L60: Adds the `inputBuf` field to the struct so instances can hold that piece of state.
	inputBuf []byte // partial input typed so far (server-side tracking)
// L61: Adds the `prompt` field to the struct so instances can hold that piece of state.
	prompt   string // current prompt string for redraw

	// skipLF is ONLY accessed by the handler goroutine (ReadLineInteractive):
// L64: Adds the `skipLF` field to the struct so instances can hold that piece of state.
	skipLF bool // skip next \n after \r (for \r\n handling)
// L65: Closes the struct definition after listing all of its fields.
}

// NewClient wraps a connection and starts the background write goroutine.
// L68: Declares the `NewClient` function, which starts a named unit of behavior other code can call.
func NewClient(conn net.Conn) *Client {
// L69: Creates `c` as a new local binding so later lines can reuse this computed value.
	c := &Client{
// L70: Keeps this element in the surrounding multiline literal or call expression.
		Conn:    conn,
// L71: Keeps this element in the surrounding multiline literal or call expression.
		IP:      conn.RemoteAddr().String(),
// L72: Keeps this element in the surrounding multiline literal or call expression.
		msgChan: make(chan writeMsg, msgChanSize),
// L73: Keeps this element in the surrounding multiline literal or call expression.
		done:    make(chan struct{}),
// L74: Closes the current block and returns control to the surrounding scope.
	}
// L75: Creates `scanner` from the result of `bufio.NewScanner`, capturing fresh state for the rest of this scope.
	scanner := bufio.NewScanner(conn)
// L76: Calls `scanner.Buffer` here for its side effects or returned value in the surrounding control flow.
	scanner.Buffer(make([]byte, 4096), maxLineLength)
// L77: Updates `c.scanner` so subsequent logic sees the new state.
	c.scanner = scanner
// L78: Launches the following call in a new goroutine so it can run concurrently with the current path.
	go c.writeLoop()
// L79: Returns the listed values to the caller, ending the current function at this point.
	return c
// L80: Closes the current block and returns control to the surrounding scope.
}

// Send enqueues a message for delivery. Non-blocking: drops if channel is full.
// L83: Declares the `Send` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) Send(msg string) {
// L84: Calls `c.enqueue` here for its side effects or returned value in the surrounding control flow.
	c.enqueue(writeMsg{data: msg, msgType: wmMessage})
// L85: Closes the current block and returns control to the surrounding scope.
}

// SendPrompt enqueues a prompt message and tells the writeLoop to update the
// tracked prompt (clears inputBuf). Used after the client submits a line.
// L89: Declares the `SendPrompt` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) SendPrompt(prompt string) {
// L90: Calls `c.enqueue` here for its side effects or returned value in the surrounding control flow.
	c.enqueue(writeMsg{data: prompt, msgType: wmPrompt})
// L91: Closes the current block and returns control to the surrounding scope.
}

// L93: Declares the `enqueue` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) enqueue(m writeMsg) {
// L94: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
	select {
// L95: Selects the `<-c.done` branch inside the surrounding switch or select.
	case <-c.done:
// L96: Returns immediately from the current function without additional values.
		return
// L97: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
	default:
// L98: Closes the current block and returns control to the surrounding scope.
	}
// L99: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
	select {
// L100: Selects the `c.msgChan <- m` branch inside the surrounding switch or select.
	case c.msgChan <- m:
// L101: Selects the `<-c.done` branch inside the surrounding switch or select.
	case <-c.done:
// L102: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
	default:
		// channel full – drop for this client to protect others
// L104: Closes the current block and returns control to the surrounding scope.
	}
// L105: Closes the current block and returns control to the surrounding scope.
}

// ReadLine blocks until a full line is available. Strips \r\n → content only.
// Used during onboarding (before echoMode is enabled).
// L109: Declares the `ReadLine` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) ReadLine() (string, error) {
// L110: Evaluates `c.scanner.Scan()` and enters the guarded branch only when that condition holds.
	if c.scanner.Scan() {
// L111: Returns the listed values to the caller, ending the current function at this point.
		return c.scanner.Text(), nil
// L112: Closes the current block and returns control to the surrounding scope.
	}
// L113: Evaluates `err := c.scanner.Err(); err != nil` and enters the guarded branch only when that condition holds.
	if err := c.scanner.Err(); err != nil {
// L114: Returns the listed values to the caller, ending the current function at this point.
		return "", err
// L115: Closes the current block and returns control to the surrounding scope.
	}
// L116: Returns the listed values to the caller, ending the current function at this point.
	return "", io.EOF
// L117: Closes the current block and returns control to the surrounding scope.
}

// Close tears down the connection and stops the write goroutine. Safe to call multiple times.
// L120: Declares the `Close` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) Close() {
// L121: Calls `c.closeOnce.Do` here for its side effects or returned value in the surrounding control flow.
	c.closeOnce.Do(func() {
// L122: Calls `close` here for its side effects or returned value in the surrounding control flow.
		close(c.done)
// L123: Calls `c.Conn.Close` here for its side effects or returned value in the surrounding control flow.
		c.Conn.Close()
// L124: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	})
// L125: Closes the current block and returns control to the surrounding scope.
}

// IsClosed reports whether Close has been called.
// L128: Declares the `IsClosed` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) IsClosed() bool {
// L129: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
	select {
// L130: Selects the `<-c.done` branch inside the surrounding switch or select.
	case <-c.done:
// L131: Returns the listed values to the caller, ending the current function at this point.
		return true
// L132: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
	default:
// L133: Returns the listed values to the caller, ending the current function at this point.
		return false
// L134: Closes the current block and returns control to the surrounding scope.
	}
// L135: Closes the current block and returns control to the surrounding scope.
}

// Done returns a channel that is closed when the client is shutting down.
// L138: Declares the `Done` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) Done() <-chan struct{} {
// L139: Returns the listed values to the caller, ending the current function at this point.
	return c.done
// L140: Closes the current block and returns control to the surrounding scope.
}

// ResetScanner creates a fresh scanner for the connection.
// Used after queue admission where the scanner may be in an error state
// due to a read deadline used to cancel the monitoring goroutine.
// L145: Declares the `ResetScanner` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) ResetScanner() {
// L146: Creates `scanner` from the result of `bufio.NewScanner`, capturing fresh state for the rest of this scope.
	scanner := bufio.NewScanner(c.Conn)
// L147: Calls `scanner.Buffer` here for its side effects or returned value in the surrounding control flow.
	scanner.Buffer(make([]byte, 4096), maxLineLength)
// L148: Updates `c.scanner` so subsequent logic sees the new state.
	c.scanner = scanner
// L149: Closes the current block and returns control to the surrounding scope.
}

// SetLastInput records the time the client last sent any data (for heartbeat tracking).
// L152: Declares the `SetLastInput` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) SetLastInput(t time.Time) {
// L153: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L154: Updates `c.lastInput` so subsequent logic sees the new state.
	c.lastInput = t
// L155: Calls `c.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Unlock()
// L156: Closes the current block and returns control to the surrounding scope.
}

// GetLastInput returns the time the client last sent any data.
// L159: Declares the `GetLastInput` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) GetLastInput() time.Time {
// L160: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L161: Schedules this cleanup or follow-up call to run when the current function returns.
	defer c.mu.Unlock()
// L162: Returns the listed values to the caller, ending the current function at this point.
	return c.lastInput
// L163: Closes the current block and returns control to the surrounding scope.
}

// SetDisconnectReason atomically sets the disconnect reason if not already set.
// Used to distinguish voluntary, dropped, kicked, and banned disconnects.
// L167: Declares the `SetDisconnectReason` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) SetDisconnectReason(reason string) {
// L168: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L169: Evaluates `c.disconnectReason == ""` and enters the guarded branch only when that condition holds.
	if c.disconnectReason == "" {
// L170: Updates `c.disconnectReason` so subsequent logic sees the new state.
		c.disconnectReason = reason
// L171: Closes the current block and returns control to the surrounding scope.
	}
// L172: Calls `c.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Unlock()
// L173: Closes the current block and returns control to the surrounding scope.
}

// GetDisconnectReason returns the current disconnect reason.
// L176: Declares the `GetDisconnectReason` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) GetDisconnectReason() string {
// L177: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L178: Schedules this cleanup or follow-up call to run when the current function returns.
	defer c.mu.Unlock()
// L179: Returns the listed values to the caller, ending the current function at this point.
	return c.disconnectReason
// L180: Closes the current block and returns control to the surrounding scope.
}

// ForceDisconnectReason sets the disconnect reason unconditionally (for moderation).
// L183: Declares the `ForceDisconnectReason` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) ForceDisconnectReason(reason string) {
// L184: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L185: Updates `c.disconnectReason` so subsequent logic sees the new state.
	c.disconnectReason = reason
// L186: Calls `c.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Unlock()
// L187: Closes the current block and returns control to the surrounding scope.
}

// SetLastActivity records the time the client last sent a chat message.
// L190: Declares the `SetLastActivity` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) SetLastActivity(t time.Time) {
// L191: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L192: Updates `c.lastActivity` so subsequent logic sees the new state.
	c.lastActivity = t
// L193: Calls `c.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Unlock()
// L194: Closes the current block and returns control to the surrounding scope.
}

// GetLastActivity returns the time the client last sent a chat message.
// L197: Declares the `GetLastActivity` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) GetLastActivity() time.Time {
// L198: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L199: Schedules this cleanup or follow-up call to run when the current function returns.
	defer c.mu.Unlock()
// L200: Returns the listed values to the caller, ending the current function at this point.
	return c.lastActivity
// L201: Closes the current block and returns control to the surrounding scope.
}

// SetMuted sets whether the client is muted.
// L204: Declares the `SetMuted` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) SetMuted(v bool) {
// L205: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L206: Updates `c.muted` so subsequent logic sees the new state.
	c.muted = v
// L207: Calls `c.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Unlock()
// L208: Closes the current block and returns control to the surrounding scope.
}

// IsMuted reports whether the client is muted.
// L211: Declares the `IsMuted` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) IsMuted() bool {
// L212: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L213: Schedules this cleanup or follow-up call to run when the current function returns.
	defer c.mu.Unlock()
// L214: Returns the listed values to the caller, ending the current function at this point.
	return c.muted
// L215: Closes the current block and returns control to the surrounding scope.
}

// SetAdmin sets whether the client has admin privileges.
// L218: Declares the `SetAdmin` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) SetAdmin(v bool) {
// L219: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L220: Updates `c.admin` so subsequent logic sees the new state.
	c.admin = v
// L221: Calls `c.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Unlock()
// L222: Closes the current block and returns control to the surrounding scope.
}

// IsAdmin reports whether the client has admin privileges.
// L225: Declares the `IsAdmin` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) IsAdmin() bool {
// L226: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L227: Schedules this cleanup or follow-up call to run when the current function returns.
	defer c.mu.Unlock()
// L228: Returns the listed values to the caller, ending the current function at this point.
	return c.admin
// L229: Closes the current block and returns control to the surrounding scope.
}

// ---------- writeLoop: single goroutine responsible for all Conn writes ----------

// writeLoop drains the message channel and writes to the connection.
// It is the sole writer to Conn (except heartbeat null-byte probes).
// In echoMode it preserves the client's partial input across incoming messages.
// L236: Declares the `writeLoop` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) writeLoop() {
// L237: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L238: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
		select {
// L239: Selects the `msg := <-c.msgChan` branch inside the surrounding switch or select.
		case msg := <-c.msgChan:
// L240: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
			c.mu.Lock()
// L241: Creates `echo` as a new local binding so later lines can reuse this computed value.
			echo := c.echoMode
// L242: Calls `c.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
			c.mu.Unlock()

// L244: Evaluates `!echo` and enters the guarded branch only when that condition holds.
			if !echo {
				// Pre-onboarding: raw write
// L246: Evaluates `_, err := c.Conn.Write([]byte(msg.data)); err != nil` and enters the guarded branch only when that condition holds.
				if _, err := c.Conn.Write([]byte(msg.data)); err != nil {
// L247: Returns immediately from the current function without additional values.
					return
// L248: Closes the current block and returns control to the surrounding scope.
				}
// L249: Skips the rest of the current loop iteration and starts the next iteration immediately.
				continue
// L250: Closes the current block and returns control to the surrounding scope.
			}

			// echoMode active: handle by message type
// L253: Starts a switch on `msg.msgType` so the following cases can branch on that value cleanly.
			switch msg.msgType {
// L254: Selects the `wmMessage` branch inside the surrounding switch or select.
			case wmMessage:
// L255: Calls `c.writeWithContinuity` here for its side effects or returned value in the surrounding control flow.
				c.writeWithContinuity(msg.data)
// L256: Selects the `wmPrompt` branch inside the surrounding switch or select.
			case wmPrompt:
// L257: Updates `c.prompt` so subsequent logic sees the new state.
				c.prompt = msg.data
// L258: Updates `c.inputBuf` so subsequent logic sees the new state.
				c.inputBuf = c.inputBuf[:0]
// L259: Calls `c.Conn.Write` here for its side effects or returned value in the surrounding control flow.
				c.Conn.Write([]byte(msg.data))
// L260: Selects the `wmEcho` branch inside the surrounding switch or select.
			case wmEcho:
// L261: Evaluates `len(msg.data) > 0 && len(c.inputBuf) < maxInteractiveBuf` and enters the guarded branch only when that condition holds.
				if len(msg.data) > 0 && len(c.inputBuf) < maxInteractiveBuf {
// L262: Updates `c.inputBuf` with the result of `append`, replacing its previous value.
					c.inputBuf = append(c.inputBuf, msg.data[0])
// L263: Closes the current block and returns control to the surrounding scope.
				}
// L264: Selects the `wmBackspace` branch inside the surrounding switch or select.
			case wmBackspace:
// L265: Evaluates `len(c.inputBuf) > 0` and enters the guarded branch only when that condition holds.
				if len(c.inputBuf) > 0 {
// L266: Updates `c.inputBuf` with the result of `len`, replacing its previous value.
					c.inputBuf = c.inputBuf[:len(c.inputBuf)-1]
// L267: Closes the current block and returns control to the surrounding scope.
				}
// L268: Selects the `wmNewline` branch inside the surrounding switch or select.
			case wmNewline:
// L269: Updates `c.inputBuf` so subsequent logic sees the new state.
				c.inputBuf = c.inputBuf[:0]
// L270: Updates `c.prompt` so subsequent logic sees the new state.
				c.prompt = ""
// L271: Closes the current block and returns control to the surrounding scope.
			}
// L272: Selects the `<-c.done` branch inside the surrounding switch or select.
		case <-c.done:
// L273: Returns immediately from the current function without additional values.
			return
// L274: Closes the current block and returns control to the surrounding scope.
		}
// L275: Closes the current block and returns control to the surrounding scope.
	}
// L276: Closes the current block and returns control to the surrounding scope.
}

// writeWithContinuity clears the current prompt+input line, writes the message,
// then redraws the prompt and partial input. All output is batched into a single
// Conn.Write to avoid partial-write blocking on synchronous pipes. Only called
// from writeLoop.
// L282: Declares the `writeWithContinuity` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) writeWithContinuity(msg string) {
// L283: Creates `hasPrompt` from the result of `len`, capturing fresh state for the rest of this scope.
	hasPrompt := len(c.prompt) > 0
// L284: Creates `hasInput` from the result of `len`, capturing fresh state for the rest of this scope.
	hasInput := len(c.inputBuf) > 0

	// Pre-calculate total size for a single allocation
// L287: Creates `size` from the result of `len`, capturing fresh state for the rest of this scope.
	size := len(msg)
// L288: Evaluates `hasPrompt || hasInput` and enters the guarded branch only when that condition holds.
	if hasPrompt || hasInput {
// L289: Updates `size +` so subsequent logic sees the new state.
		size += 4 // "\r\033[K"
// L290: Closes the current block and returns control to the surrounding scope.
	}
// L291: Evaluates `hasPrompt` and enters the guarded branch only when that condition holds.
	if hasPrompt {
// L292: Updates `size +` with the result of `len`, replacing its previous value.
		size += len(c.prompt)
// L293: Closes the current block and returns control to the surrounding scope.
	}
// L294: Evaluates `hasInput` and enters the guarded branch only when that condition holds.
	if hasInput {
// L295: Updates `size +` with the result of `len`, replacing its previous value.
		size += len(c.inputBuf)
// L296: Closes the current block and returns control to the surrounding scope.
	}

// L298: Creates `buf` from the result of `make`, capturing fresh state for the rest of this scope.
	buf := make([]byte, 0, size)
// L299: Evaluates `hasPrompt || hasInput` and enters the guarded branch only when that condition holds.
	if hasPrompt || hasInput {
// L300: Updates `buf` with the result of `append`, replacing its previous value.
		buf = append(buf, '\r')
// L301: Updates `buf` with the result of `append`, replacing its previous value.
		buf = append(buf, "\033[K"...)
// L302: Closes the current block and returns control to the surrounding scope.
	}
// L303: Updates `buf` with the result of `append`, replacing its previous value.
	buf = append(buf, msg...)
// L304: Evaluates `hasPrompt` and enters the guarded branch only when that condition holds.
	if hasPrompt {
// L305: Updates `buf` with the result of `append`, replacing its previous value.
		buf = append(buf, c.prompt...)
// L306: Closes the current block and returns control to the surrounding scope.
	}
// L307: Evaluates `hasInput` and enters the guarded branch only when that condition holds.
	if hasInput {
// L308: Updates `buf` with the result of `append`, replacing its previous value.
		buf = append(buf, c.inputBuf...)
// L309: Closes the current block and returns control to the surrounding scope.
	}
// L310: Calls `c.Conn.Write` here for its side effects or returned value in the surrounding control flow.
	c.Conn.Write(buf)
// L311: Closes the current block and returns control to the surrounding scope.
}

// ---------- echo mode & interactive reading ----------

// SetEchoMode enables character-at-a-time input handling with input continuity.
// L316: Declares the `SetEchoMode` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) SetEchoMode(enabled bool) {
// L317: Calls `c.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Lock()
// L318: Updates `c.echoMode` so subsequent logic sees the new state.
	c.echoMode = enabled
// L319: Calls `c.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	c.mu.Unlock()
// L320: Closes the current block and returns control to the surrounding scope.
}

// ReadLineInteractive reads input byte-by-byte, sending echo/backspace/newline
// commands through the writeLoop channel. This avoids direct Conn writes from
// the handler goroutine, preventing deadlocks with synchronous pipes.
// Returns the complete line (without newline) when Enter is pressed.
// L326: Declares the `ReadLineInteractive` method on `c *Client`, creating a reusable behavior entrypoint tied to that receiver state.
func (c *Client) ReadLineInteractive() (string, error) {
// L327: Creates `buf` from the result of `make`, capturing fresh state for the rest of this scope.
	buf := make([]byte, 1)
// L328: Declares `line` in the current scope so later lines can fill or mutate it as needed.
	var line []byte
// L329: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L330: Creates `n, err` from the result of `c.Conn.Read`, capturing fresh state for the rest of this scope.
		n, err := c.Conn.Read(buf)
// L331: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
		if err != nil {
// L332: Returns the listed values to the caller, ending the current function at this point.
			return "", err
// L333: Closes the current block and returns control to the surrounding scope.
		}
// L334: Evaluates `n == 0` and enters the guarded branch only when that condition holds.
		if n == 0 {
// L335: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L336: Closes the current block and returns control to the surrounding scope.
		}
// L337: Creates `b` as a new local binding so later lines can reuse this computed value.
		b := buf[0]

		// Handle \r\n: after \r we skip the next \n
// L340: Evaluates `c.skipLF && b == '\n'` and enters the guarded branch only when that condition holds.
		if c.skipLF && b == '\n' {
// L341: Updates `c.skipLF` so subsequent logic sees the new state.
			c.skipLF = false
// L342: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L343: Closes the current block and returns control to the surrounding scope.
		}
// L344: Updates `c.skipLF` so subsequent logic sees the new state.
		c.skipLF = false

// L346: Starts a switch on `` so the following cases can branch on that value cleanly.
		switch {
// L347: Selects the `b == '\r'` branch inside the surrounding switch or select.
		case b == '\r':
// L348: Updates `c.skipLF` so subsequent logic sees the new state.
			c.skipLF = true
// L349: Calls `c.enqueue` here for its side effects or returned value in the surrounding control flow.
			c.enqueue(writeMsg{msgType: wmNewline})
// L350: Returns the listed values to the caller, ending the current function at this point.
			return string(line), nil

// L352: Selects the `b == '\n'` branch inside the surrounding switch or select.
		case b == '\n':
// L353: Calls `c.enqueue` here for its side effects or returned value in the surrounding control flow.
			c.enqueue(writeMsg{msgType: wmNewline})
// L354: Returns the listed values to the caller, ending the current function at this point.
			return string(line), nil

// L356: Selects the `b == 0x7F || b == 0x08: // Backspace / Delete` branch inside the surrounding switch or select.
		case b == 0x7F || b == 0x08: // Backspace / Delete
// L357: Evaluates `len(line) > 0` and enters the guarded branch only when that condition holds.
			if len(line) > 0 {
// L358: Updates `line` with the result of `len`, replacing its previous value.
				line = line[:len(line)-1]
// L359: Calls `c.enqueue` here for its side effects or returned value in the surrounding control flow.
				c.enqueue(writeMsg{msgType: wmBackspace})
// L360: Closes the current block and returns control to the surrounding scope.
			}

// L362: Selects the `b == 0x00: // Null byte from heartbeat probe — ignore` branch inside the surrounding switch or select.
		case b == 0x00: // Null byte from heartbeat probe — ignore
// L363: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue

// L365: Selects the `b >= 0x20 && b <= 0x7E: // Printable ASCII` branch inside the surrounding switch or select.
		case b >= 0x20 && b <= 0x7E: // Printable ASCII
// L366: Updates `line` with the result of `append`, replacing its previous value.
			line = append(line, b)
// L367: Calls `c.enqueue` here for its side effects or returned value in the surrounding control flow.
			c.enqueue(writeMsg{data: string([]byte{b}), msgType: wmEcho})

// L369: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
		default:
			// Non-printable control character — ignore
// L371: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L372: Closes the current block and returns control to the surrounding scope.
		}
// L373: Closes the current block and returns control to the surrounding scope.
	}
// L374: Closes the current block and returns control to the surrounding scope.
}
```

## `server/server.go`

Defines the top-level server state and orchestrates startup, acceptance, shutdown, heartbeat probing, and daily maintenance.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L5: Imports `io` so this file can call functionality from its `io` package.
	"io"
// L6: Imports `net` so this file can call functionality from its `net` package.
	"net"
// L7: Imports `net-cat/client` so this file can call functionality from its `client` package.
	"net-cat/client"
// L8: Imports `net-cat/logger` so this file can call functionality from its `logger` package.
	"net-cat/logger"
// L9: Imports `net-cat/models` so this file can call functionality from its `models` package.
	"net-cat/models"
// L10: Imports `os` so this file can call functionality from its `os` package.
	"os"
// L11: Imports `sync` so this file can call functionality from its `sync` package.
	"sync"
// L12: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L13: Closes the import block after listing all package dependencies.
)

// QueueEntry represents a client waiting for a slot to open.
// L16: Defines the `QueueEntry` struct, which groups related state that this package manages together.
type QueueEntry struct {
// L17: Adds the `client` field to the struct so instances can hold that piece of state.
	client *client.Client
// L18: Adds the `admit` field to the struct so instances can hold that piece of state.
	admit  chan struct{} // closed when the client is admitted
// L19: Closes the struct definition after listing all of its fields.
}

// Server manages the TCP listener, connected clients, and chat history.
// L22: Defines the `Server` struct, which groups related state that this package manages together.
type Server struct {
// L23: Adds the `port` field to the struct so instances can hold that piece of state.
	port            string
// L24: Adds the `listener` field to the struct so instances can hold that piece of state.
	listener        net.Listener
// L25: Adds the `clients` field to the struct so instances can hold that piece of state.
	clients         map[string]*client.Client    // global username→client for cross-room lookups
// L26: Adds the `allClients` field to the struct so instances can hold that piece of state.
	allClients      map[*client.Client]struct{}   // all connections in any phase (name-prompt, queued, active)
// L27: Adds the `mu` field to the struct so instances can hold that piece of state.
	mu              sync.RWMutex
// L28: Adds the `rooms` field to the struct so instances can hold that piece of state.
	rooms           map[string]*Room              // room name → Room (history+queue are per-room)
// L29: Adds the `DefaultRoom` field to the struct so instances can hold that piece of state.
	DefaultRoom     string                        // default room name ("general")
// L30: Adds the `reservedNames` field to the struct so instances can hold that piece of state.
	reservedNames   map[string]bool
// L31: Adds the `quit` field to the struct so instances can hold that piece of state.
	quit            chan struct{}
// L32: Adds the `shutdownOnce` field to the struct so instances can hold that piece of state.
	shutdownOnce    sync.Once
// L33: Adds the `shutdownDone` field to the struct so instances can hold that piece of state.
	shutdownDone    chan struct{}
// L34: Adds the `ShutdownTimeout` field to the struct so instances can hold that piece of state.
	ShutdownTimeout time.Duration // defaults to 5s; override in tests for faster execution
// L35: Adds the `Logger` field to the struct so instances can hold that piece of state.
	Logger          *logger.Logger

	// IP-based moderation (protected by mu)
// L38: Adds the `kickedIPs` field to the struct so instances can hold that piece of state.
	kickedIPs map[string]time.Time // host IP -> cooldown expiry
// L39: Adds the `bannedIPs` field to the struct so instances can hold that piece of state.
	bannedIPs map[string]bool      // host IP -> banned for server session

	// Admin persistence
// L42: Adds the `admins` field to the struct so instances can hold that piece of state.
	admins     map[string]bool // known admin usernames, protected by mu
// L43: Adds the `adminsFile` field to the struct so instances can hold that piece of state.
	adminsFile string          // path to admins.json

	// Operator terminal output (defaults to os.Stdout)
// L46: Adds the `OperatorOutput` field to the struct so instances can hold that piece of state.
	OperatorOutput io.Writer

	// Heartbeat configuration (zero values use defaults: 10s interval, 5s timeout)
// L49: Adds the `HeartbeatInterval` field to the struct so instances can hold that piece of state.
	HeartbeatInterval time.Duration // how often to check idle clients (default 10s)
// L50: Adds the `HeartbeatTimeout` field to the struct so instances can hold that piece of state.
	HeartbeatTimeout  time.Duration // write probe deadline (default 5s)
// L51: Closes the struct definition after listing all of its fields.
}

// New creates a server that will listen on the given port.
// L54: Declares the `New` function, which starts a named unit of behavior other code can call.
func New(port string) *Server {
// L55: Creates `s` as a new local binding so later lines can reuse this computed value.
	s := &Server{
// L56: Keeps this element in the surrounding multiline literal or call expression.
		port:       port,
// L57: Keeps this element in the surrounding multiline literal or call expression.
		clients:    make(map[string]*client.Client),
// L58: Keeps this element in the surrounding multiline literal or call expression.
		allClients: make(map[*client.Client]struct{}),
// L59: Keeps this element in the surrounding multiline literal or call expression.
		rooms:      make(map[string]*Room),
// L60: Keeps this element in the surrounding multiline literal or call expression.
		DefaultRoom: "general",
// L61: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
		reservedNames: map[string]bool{
// L62: Keeps this element in the surrounding multiline literal or call expression.
			"Server": true,
// L63: Keeps this element in the surrounding multiline literal or call expression.
		},
// L64: Keeps this element in the surrounding multiline literal or call expression.
		quit:           make(chan struct{}),
// L65: Keeps this element in the surrounding multiline literal or call expression.
		shutdownDone:   make(chan struct{}),
// L66: Keeps this element in the surrounding multiline literal or call expression.
		kickedIPs:      make(map[string]time.Time),
// L67: Keeps this element in the surrounding multiline literal or call expression.
		bannedIPs:      make(map[string]bool),
// L68: Keeps this element in the surrounding multiline literal or call expression.
		admins:         make(map[string]bool),
// L69: Keeps this element in the surrounding multiline literal or call expression.
		adminsFile:     "admins.json",
// L70: Keeps this element in the surrounding multiline literal or call expression.
		OperatorOutput: os.Stdout,
// L71: Closes the current block and returns control to the surrounding scope.
	}
	// Ensure the default room always exists
// L73: Updates `s.rooms[s.DefaultRoom]` with the result of `newRoom`, replacing its previous value.
	s.rooms[s.DefaultRoom] = newRoom(s.DefaultRoom)
// L74: Returns the listed values to the caller, ending the current function at this point.
	return s
// L75: Closes the current block and returns control to the surrounding scope.
}

// Start opens the TCP listener and blocks in the accept loop until shutdown.
// L78: Declares the `Start` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) Start() error {
// L79: Declares `err` in the current scope so later lines can fill or mutate it as needed.
	var err error
// L80: Updates `s.listener, err` with the result of `net.Listen`, replacing its previous value.
	s.listener, err = net.Listen("tcp", ":"+s.port)
// L81: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
	if err != nil {
// L82: Returns the listed values to the caller, ending the current function at this point.
		return err
// L83: Closes the current block and returns control to the surrounding scope.
	}
// L84: Calls `fmt.Printf` here for its side effects or returned value in the surrounding control flow.
	fmt.Printf("Listening on the port :%s\n", s.port)
// L85: Calls `s.Logger.Log` here for its side effects or returned value in the surrounding control flow.
	s.Logger.Log(models.Message{
// L86: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L87: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgServerEvent,
// L88: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "Server started on port " + s.port,
// L89: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	})
// L90: Calls `s.LoadAdmins` here for its side effects or returned value in the surrounding control flow.
	s.LoadAdmins()
// L91: Calls `s.RecoverHistory` here for its side effects or returned value in the surrounding control flow.
	s.RecoverHistory()
// L92: Launches the following call in a new goroutine so it can run concurrently with the current path.
	go s.startMidnightWatcher()
// L93: Calls `s.acceptLoop` here for its side effects or returned value in the surrounding control flow.
	s.acceptLoop()
// L94: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	<-s.shutdownDone
// L95: Returns the listed values to the caller, ending the current function at this point.
	return nil
// L96: Closes the current block and returns control to the surrounding scope.
}

// Shutdown sends the goodbye message, waits for clients to disconnect, then
// force-closes remaining connections. Idempotent via sync.Once.
// L100: Declares the `Shutdown` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) Shutdown() {
// L101: Calls `s.shutdownOnce.Do` here for its side effects or returned value in the surrounding control flow.
	s.shutdownOnce.Do(func() {
// L102: Calls `close` here for its side effects or returned value in the surrounding control flow.
		close(s.quit)

		// Stop accepting new connections
// L105: Evaluates `s.listener != nil` and enters the guarded branch only when that condition holds.
		if s.listener != nil {
// L106: Calls `s.listener.Close` here for its side effects or returned value in the surrounding control flow.
			s.listener.Close()
// L107: Closes the current block and returns control to the surrounding scope.
		}

		// Send goodbye to ALL tracked connections (active, queued, and name-prompt)
// L110: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
		s.mu.RLock()
// L111: Starts a loop controlled by `c := range s.allClients`, repeating until the loop condition or range is exhausted.
		for c := range s.allClients {
// L112: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send("Server is shutting down. Goodbye!\n")
// L113: Closes the current block and returns control to the surrounding scope.
		}
// L114: Calls `s.mu.RUnlock` here for its side effects or returned value in the surrounding control flow.
		s.mu.RUnlock()

		// Wait up to ShutdownTimeout for clients to disconnect voluntarily
// L117: Creates `timeout` as a new local binding so later lines can reuse this computed value.
		timeout := s.ShutdownTimeout
// L118: Evaluates `timeout == 0` and enters the guarded branch only when that condition holds.
		if timeout == 0 {
// L119: Updates `timeout` so subsequent logic sees the new state.
			timeout = 5 * time.Second
// L120: Closes the current block and returns control to the surrounding scope.
		}
// L121: Creates `deadline` from the result of `time.Now`, capturing fresh state for the rest of this scope.
		deadline := time.Now().Add(timeout)
// L122: Starts a loop controlled by `time.Now().Before(deadline)`, repeating until the loop condition or range is exhausted.
		for time.Now().Before(deadline) {
// L123: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
			s.mu.RLock()
// L124: Creates `remaining` from the result of `len`, capturing fresh state for the rest of this scope.
			remaining := len(s.allClients)
// L125: Calls `s.mu.RUnlock` here for its side effects or returned value in the surrounding control flow.
			s.mu.RUnlock()
// L126: Evaluates `remaining == 0` and enters the guarded branch only when that condition holds.
			if remaining == 0 {
// L127: Breaks out of the current loop or switch so execution resumes after that block.
				break
// L128: Closes the current block and returns control to the surrounding scope.
			}
// L129: Calls `time.Sleep` here for its side effects or returned value in the surrounding control flow.
			time.Sleep(50 * time.Millisecond)
// L130: Closes the current block and returns control to the surrounding scope.
		}

		// Force-close any remaining connections
// L133: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
		s.mu.RLock()
// L134: Starts a loop controlled by `c := range s.allClients`, repeating until the loop condition or range is exhausted.
		for c := range s.allClients {
// L135: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
			c.Close()
// L136: Closes the current block and returns control to the surrounding scope.
		}
// L137: Calls `s.mu.RUnlock` here for its side effects or returned value in the surrounding control flow.
		s.mu.RUnlock()

		// Wait for handler goroutine cleanup (leave logging, untracking)
// L140: Calls `time.Sleep` here for its side effects or returned value in the surrounding control flow.
		time.Sleep(200 * time.Millisecond)

		// Log shutdown synchronously — guaranteed before process exit
// L143: Calls `s.Logger.Log` here for its side effects or returned value in the surrounding control flow.
		s.Logger.Log(models.Message{
// L144: Keeps this element in the surrounding multiline literal or call expression.
			Timestamp: time.Now(),
// L145: Keeps this element in the surrounding multiline literal or call expression.
			Type:      models.MsgServerEvent,
// L146: Keeps this element in the surrounding multiline literal or call expression.
			Content:   "Server shutting down",
// L147: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
		})
// L148: Calls `s.Logger.Close` here for its side effects or returned value in the surrounding control flow.
		s.Logger.Close()

// L150: Calls `close` here for its side effects or returned value in the surrounding control flow.
		close(s.shutdownDone)
// L151: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	})
// L152: Closes the current block and returns control to the surrounding scope.
}

// L154: Declares the `acceptLoop` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) acceptLoop() {
// L155: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L156: Creates `conn, err` from the result of `s.listener.Accept`, capturing fresh state for the rest of this scope.
		conn, err := s.listener.Accept()
// L157: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
		if err != nil {
// L158: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
			select {
// L159: Selects the `<-s.quit` branch inside the surrounding switch or select.
			case <-s.quit:
// L160: Returns immediately from the current function without additional values.
				return
// L161: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
			default:
// L162: Skips the rest of the current loop iteration and starts the next iteration immediately.
				continue
// L163: Closes the current block and returns control to the surrounding scope.
			}
// L164: Closes the current block and returns control to the surrounding scope.
		}
// L165: Launches the following call in a new goroutine so it can run concurrently with the current path.
		go s.handleConnection(conn)
// L166: Closes the current block and returns control to the surrounding scope.
	}
// L167: Closes the current block and returns control to the surrounding scope.
}

// ---------- queue management ----------

// MaxActiveClients is the maximum number of clients that can be actively chatting per room.
// L172: Updates `const MaxActiveClients` so subsequent logic sees the new state.
const MaxActiveClients = 10

// GetQueueLength returns the total number of queued clients across all rooms.
// L175: Declares the `GetQueueLength` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetQueueLength() int {
// L176: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L177: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L178: Creates `total` as a new local binding so later lines can reuse this computed value.
	total := 0
// L179: Starts a loop controlled by `_, r := range s.rooms`, repeating until the loop condition or range is exhausted.
	for _, r := range s.rooms {
// L180: Updates `total +` with the result of `len`, replacing its previous value.
		total += len(r.queue)
// L181: Closes the current block and returns control to the surrounding scope.
	}
// L182: Returns the listed values to the caller, ending the current function at this point.
	return total
// L183: Closes the current block and returns control to the surrounding scope.
}

// RemoveFromQueueByIP removes all queue entries across all rooms whose IP matches
// the given address and returns their clients.
// L187: Declares the `RemoveFromQueueByIP` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) RemoveFromQueueByIP(ip string) []*client.Client {
// L188: Returns the listed values to the caller, ending the current function at this point.
	return s.RemoveFromAllRoomQueuesByIP(ip)
// L189: Closes the current block and returns control to the surrounding scope.
}

// IsShuttingDown reports whether the server is in the shutdown process.
// L192: Declares the `IsShuttingDown` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) IsShuttingDown() bool {
// L193: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
	select {
// L194: Selects the `<-s.quit` branch inside the surrounding switch or select.
	case <-s.quit:
// L195: Returns the listed values to the caller, ending the current function at this point.
		return true
// L196: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
	default:
// L197: Returns the listed values to the caller, ending the current function at this point.
		return false
// L198: Closes the current block and returns control to the surrounding scope.
	}
// L199: Closes the current block and returns control to the surrounding scope.
}

// ---------- heartbeat ----------

// startHeartbeat runs a per-client goroutine that periodically probes the connection
// to detect dead/ghost clients. A null byte (\x00) write probe is used because it is
// invisible to most terminal emulators (including netcat).
//
// The probe runs in a separate goroutine to avoid calling SetWriteDeadline, which
// would interfere with the client's writeLoop. Instead, a timer detects whether the
// probe completes in time:
//   - If the write returns an error (io.ErrClosedPipe, ECONNRESET, etc.): dead client.
//   - If the write doesn't complete within the timeout: slow/unstable — warn but keep alive.
//   - If the write completes quickly: healthy connection.
//
// For real TCP connections, TCP keepalive (enabled in handleConnection) provides an
// additional layer of dead peer detection at the OS level.
// L216: Declares the `startHeartbeat` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) startHeartbeat(c *client.Client) {
// L217: Creates `interval` as a new local binding so later lines can reuse this computed value.
	interval := s.HeartbeatInterval
// L218: Evaluates `interval == 0` and enters the guarded branch only when that condition holds.
	if interval == 0 {
// L219: Updates `interval` so subsequent logic sees the new state.
		interval = 10 * time.Second
// L220: Closes the current block and returns control to the surrounding scope.
	}
// L221: Creates `timeout` as a new local binding so later lines can reuse this computed value.
	timeout := s.HeartbeatTimeout
// L222: Evaluates `timeout == 0` and enters the guarded branch only when that condition holds.
	if timeout == 0 {
// L223: Updates `timeout` so subsequent logic sees the new state.
		timeout = 5 * time.Second
// L224: Closes the current block and returns control to the surrounding scope.
	}

// L226: Creates `ticker` from the result of `time.NewTicker`, capturing fresh state for the rest of this scope.
	ticker := time.NewTicker(interval)
// L227: Schedules this cleanup or follow-up call to run when the current function returns.
	defer ticker.Stop()

// L229: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L230: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
		select {
// L231: Selects the `<-ticker.C` branch inside the surrounding switch or select.
		case <-ticker.C:
// L232: Evaluates `c.IsClosed()` and enters the guarded branch only when that condition holds.
			if c.IsClosed() {
// L233: Returns immediately from the current function without additional values.
				return
// L234: Closes the current block and returns control to the surrounding scope.
			}
			// Active sender exemption: client recently sent data, skip probe
// L236: Evaluates `time.Since(c.GetLastInput()) < interval` and enters the guarded branch only when that condition holds.
			if time.Since(c.GetLastInput()) < interval {
// L237: Skips the rest of the current loop iteration and starts the next iteration immediately.
				continue
// L238: Closes the current block and returns control to the surrounding scope.
			}
			// Write probe in a goroutine to avoid blocking and to avoid
			// calling SetWriteDeadline (which would interfere with writeLoop).
// L241: Creates `probeResult` from the result of `make`, capturing fresh state for the rest of this scope.
			probeResult := make(chan error, 1)
// L242: Creates `start` from the result of `time.Now`, capturing fresh state for the rest of this scope.
			start := time.Now()
// L243: Launches the following call in a new goroutine so it can run concurrently with the current path.
			go func() {
// L244: Creates `_, err` from the result of `c.Conn.Write`, capturing fresh state for the rest of this scope.
				_, err := c.Conn.Write([]byte{0})
// L245: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
				probeResult <- err
// L246: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
			}()

// L248: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
			select {
// L249: Selects the `err := <-probeResult` branch inside the surrounding switch or select.
			case err := <-probeResult:
// L250: Creates `elapsed` from the result of `time.Since`, capturing fresh state for the rest of this scope.
				elapsed := time.Since(start)
// L251: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
				if err != nil {
					// Non-timeout write error — connection is truly broken
// L253: Calls `c.SetDisconnectReason` here for its side effects or returned value in the surrounding control flow.
					c.SetDisconnectReason("drop")
// L254: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
					c.Close()
// L255: Returns immediately from the current function without additional values.
					return
// L256: Closes the current block and returns control to the surrounding scope.
				}
				// Write succeeded; warn if slow
// L258: Evaluates `elapsed > timeout/2` and enters the guarded branch only when that condition holds.
				if elapsed > timeout/2 {
// L259: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
					c.Send("Connection unstable...\n")
// L260: Closes the current block and returns control to the surrounding scope.
				}
// L261: Selects the `<-time.After(timeout)` branch inside the surrounding switch or select.
			case <-time.After(timeout):
				// Write probe timed out — client is unresponsive.
				// Per spec 11: "A client that fails to respond within 5 seconds
				// of a health check is treated as disconnected." Disconnect now;
				// the deferred cleanup in handleConnection handles leave broadcast,
				// logging, and queue admission.
// L267: Calls `c.SetDisconnectReason` here for its side effects or returned value in the surrounding control flow.
				c.SetDisconnectReason("drop")
// L268: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
				c.Close()
// L269: Returns immediately from the current function without additional values.
				return
// L270: Selects the `<-c.Done()` branch inside the surrounding switch or select.
			case <-c.Done():
// L271: Returns immediately from the current function without additional values.
				return
// L272: Selects the `<-s.quit` branch inside the surrounding switch or select.
			case <-s.quit:
// L273: Returns immediately from the current function without additional values.
				return
// L274: Closes the current block and returns control to the surrounding scope.
			}
// L275: Selects the `<-c.Done()` branch inside the surrounding switch or select.
		case <-c.Done():
// L276: Returns immediately from the current function without additional values.
			return
// L277: Selects the `<-s.quit` branch inside the surrounding switch or select.
		case <-s.quit:
// L278: Returns immediately from the current function without additional values.
			return
// L279: Closes the current block and returns control to the surrounding scope.
		}
// L280: Closes the current block and returns control to the surrounding scope.
	}
// L281: Closes the current block and returns control to the surrounding scope.
}

// ---------- midnight log rotation ----------

// startMidnightWatcher runs a goroutine that detects day boundaries and resets
// in-memory history at midnight. The logger already handles file switching based
// on message timestamps (via ensureFile), so this only needs to clear the history
// so that clients joining after midnight see only the new day's events.
// L289: Declares the `startMidnightWatcher` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) startMidnightWatcher() {
// L290: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L291: Creates `now` from the result of `time.Now`, capturing fresh state for the rest of this scope.
		now := time.Now()
// L292: Creates `nextMidnight` from the result of `time.Date`, capturing fresh state for the rest of this scope.
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
// L293: Creates `duration` from the result of `nextMidnight.Sub`, capturing fresh state for the rest of this scope.
		duration := nextMidnight.Sub(now)

// L295: Creates `timer` from the result of `time.NewTimer`, capturing fresh state for the rest of this scope.
		timer := time.NewTimer(duration)
// L296: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
		select {
// L297: Selects the `<-timer.C` branch inside the surrounding switch or select.
		case <-timer.C:
// L298: Calls `s.ClearHistory` here for its side effects or returned value in the surrounding control flow.
			s.ClearHistory()
// L299: Selects the `<-s.quit` branch inside the surrounding switch or select.
		case <-s.quit:
// L300: Calls `timer.Stop` here for its side effects or returned value in the surrounding control flow.
			timer.Stop()
// L301: Returns immediately from the current function without additional values.
			return
// L302: Closes the current block and returns control to the surrounding scope.
		}
// L303: Closes the current block and returns control to the surrounding scope.
	}
// L304: Closes the current block and returns control to the surrounding scope.
}
```

## `server/room.go`

Defines room-local state, room membership, history retention, and waiting-queue admission semantics.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L5: Imports `net-cat/client` so this file can call functionality from its `client` package.
	"net-cat/client"
// L6: Imports `net-cat/models` so this file can call functionality from its `models` package.
	"net-cat/models"
// L7: Closes the import block after listing all package dependencies.
)

// Room holds per-room state: members, history, and waiting queue.
// All access is protected by s.mu — no per-room mutex.
// L11: Defines the `Room` struct, which groups related state that this package manages together.
type Room struct {
// L12: Adds the `Name` field to the struct so instances can hold that piece of state.
	Name    string
// L13: Adds the `clients` field to the struct so instances can hold that piece of state.
	clients map[string]*client.Client
// L14: Adds the `history` field to the struct so instances can hold that piece of state.
	history []models.Message
// L15: Adds the `queue` field to the struct so instances can hold that piece of state.
	queue   []*QueueEntry
// L16: Closes the struct definition after listing all of its fields.
}

// L18: Declares the `newRoom` function, which starts a named unit of behavior other code can call.
func newRoom(name string) *Room {
// L19: Returns the listed values to the caller, ending the current function at this point.
	return &Room{
// L20: Keeps this element in the surrounding multiline literal or call expression.
		Name:    name,
// L21: Keeps this element in the surrounding multiline literal or call expression.
		clients: make(map[string]*client.Client),
// L22: Closes the current block and returns control to the surrounding scope.
	}
// L23: Closes the current block and returns control to the surrounding scope.
}

// ---------- room management (all require s.mu held) ----------

// getOrCreateRoom returns the room, creating it if needed. Must hold s.mu write lock.
// L28: Declares the `getOrCreateRoom` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) getOrCreateRoom(name string) *Room {
// L29: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[name]
// L30: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L31: Updates `r` with the result of `newRoom`, replacing its previous value.
		r = newRoom(name)
// L32: Updates `s.rooms[name]` so subsequent logic sees the new state.
		s.rooms[name] = r
// L33: Closes the current block and returns control to the surrounding scope.
	}
// L34: Returns the listed values to the caller, ending the current function at this point.
	return r
// L35: Closes the current block and returns control to the surrounding scope.
}

// deleteRoomIfEmpty removes a room from the map if it has no clients and no queue.
// Never deletes the DefaultRoom.
// L39: Declares the `deleteRoomIfEmpty` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) deleteRoomIfEmpty(name string) {
// L40: Evaluates `name == s.DefaultRoom` and enters the guarded branch only when that condition holds.
	if name == s.DefaultRoom {
// L41: Returns immediately from the current function without additional values.
		return
// L42: Closes the current block and returns control to the surrounding scope.
	}
// L43: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L44: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L45: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[name]
// L46: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L47: Returns immediately from the current function without additional values.
		return
// L48: Closes the current block and returns control to the surrounding scope.
	}
// L49: Evaluates `len(r.clients) == 0 && len(r.queue) == 0` and enters the guarded branch only when that condition holds.
	if len(r.clients) == 0 && len(r.queue) == 0 {
// L50: Calls `delete` here for its side effects or returned value in the surrounding control flow.
		delete(s.rooms, name)
// L51: Closes the current block and returns control to the surrounding scope.
	}
// L52: Closes the current block and returns control to the surrounding scope.
}

// JoinRoom moves a client from their current room (if any) into the target room.
// Must hold s.mu write lock.
// L56: Declares the `JoinRoom` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) JoinRoom(c *client.Client, roomName string) {
	// Remove from old room if any
// L58: Evaluates `c.Room != ""` and enters the guarded branch only when that condition holds.
	if c.Room != "" {
// L59: Evaluates `oldRoom, ok := s.rooms[c.Room]; ok` and enters the guarded branch only when that condition holds.
		if oldRoom, ok := s.rooms[c.Room]; ok {
// L60: Calls `delete` here for its side effects or returned value in the surrounding control flow.
			delete(oldRoom.clients, c.Username)
// L61: Closes the current block and returns control to the surrounding scope.
		}
// L62: Closes the current block and returns control to the surrounding scope.
	}
// L63: Creates `r` from the result of `s.getOrCreateRoom`, capturing fresh state for the rest of this scope.
	r := s.getOrCreateRoom(roomName)
// L64: Updates `r.clients[c.Username]` so subsequent logic sees the new state.
	r.clients[c.Username] = c
// L65: Updates `c.Room` so subsequent logic sees the new state.
	c.Room = roomName
// L66: Closes the current block and returns control to the surrounding scope.
}

// ---------- room-scoped broadcast ----------

// BroadcastRoom sends msg to every client in the room except exclude.
// L71: Declares the `BroadcastRoom` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) BroadcastRoom(roomName, msg string, exclude string) {
// L72: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L73: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L74: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L75: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L76: Returns immediately from the current function without additional values.
		return
// L77: Closes the current block and returns control to the surrounding scope.
	}
// L78: Starts a loop controlled by `name, c := range r.clients`, repeating until the loop condition or range is exhausted.
	for name, c := range r.clients {
// L79: Evaluates `name != exclude` and enters the guarded branch only when that condition holds.
		if name != exclude {
// L80: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send(msg)
// L81: Closes the current block and returns control to the surrounding scope.
		}
// L82: Closes the current block and returns control to the surrounding scope.
	}
// L83: Closes the current block and returns control to the surrounding scope.
}

// BroadcastRoomAll sends msg to every client in the room.
// L86: Declares the `BroadcastRoomAll` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) BroadcastRoomAll(roomName, msg string) {
// L87: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L88: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L89: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L90: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L91: Returns immediately from the current function without additional values.
		return
// L92: Closes the current block and returns control to the surrounding scope.
	}
// L93: Starts a loop controlled by `_, c := range r.clients`, repeating until the loop condition or range is exhausted.
	for _, c := range r.clients {
// L94: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(msg)
// L95: Closes the current block and returns control to the surrounding scope.
	}
// L96: Closes the current block and returns control to the surrounding scope.
}

// BroadcastAllRooms sends msg to every connected client across all rooms.
// L99: Declares the `BroadcastAllRooms` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) BroadcastAllRooms(msg string) {
// L100: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L101: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L102: Starts a loop controlled by `_, c := range s.clients`, repeating until the loop condition or range is exhausted.
	for _, c := range s.clients {
// L103: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(msg)
// L104: Closes the current block and returns control to the surrounding scope.
	}
// L105: Closes the current block and returns control to the surrounding scope.
}

// ---------- room history ----------

// GetRoomHistory returns a copy of the room's history.
// L110: Declares the `GetRoomHistory` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetRoomHistory(roomName string) []models.Message {
// L111: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L112: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L113: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L114: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L115: Returns the listed values to the caller, ending the current function at this point.
		return nil
// L116: Closes the current block and returns control to the surrounding scope.
	}
// L117: Creates `out` from the result of `make`, capturing fresh state for the rest of this scope.
	out := make([]models.Message, len(r.history))
// L118: Calls `copy` here for its side effects or returned value in the surrounding control flow.
	copy(out, r.history)
// L119: Returns the listed values to the caller, ending the current function at this point.
	return out
// L120: Closes the current block and returns control to the surrounding scope.
}

// AddRoomHistory appends a message to the room's history.
// L123: Declares the `AddRoomHistory` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) AddRoomHistory(roomName string, msg models.Message) {
// L124: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L125: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L126: Creates `r` from the result of `s.getOrCreateRoom`, capturing fresh state for the rest of this scope.
	r := s.getOrCreateRoom(roomName)
// L127: Updates `r.history` with the result of `append`, replacing its previous value.
	r.history = append(r.history, msg)
// L128: Closes the current block and returns control to the surrounding scope.
}

// recordRoomEvent sets the Room field on the message, adds to room history, and logs.
// L131: Declares the `recordRoomEvent` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) recordRoomEvent(roomName string, msg models.Message) {
// L132: Updates `msg.Room` so subsequent logic sees the new state.
	msg.Room = roomName
// L133: Calls `s.AddRoomHistory` here for its side effects or returned value in the surrounding control flow.
	s.AddRoomHistory(roomName, msg)
// L134: Calls `s.Logger.Log` here for its side effects or returned value in the surrounding control flow.
	s.Logger.Log(msg)
// L135: Closes the current block and returns control to the surrounding scope.
}

// ---------- room queries ----------

// GetRoomNames returns a sorted list of room names.
// L140: Declares the `GetRoomNames` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetRoomNames() []string {
// L141: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L142: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L143: Creates `names` from the result of `make`, capturing fresh state for the rest of this scope.
	names := make([]string, 0, len(s.rooms))
// L144: Starts a loop controlled by `name := range s.rooms`, repeating until the loop condition or range is exhausted.
	for name := range s.rooms {
// L145: Updates `names` with the result of `append`, replacing its previous value.
		names = append(names, name)
// L146: Closes the current block and returns control to the surrounding scope.
	}
	// insertion sort
// L148: Starts a loop controlled by `i := 1; i < len(names); i++`, repeating until the loop condition or range is exhausted.
	for i := 1; i < len(names); i++ {
// L149: Creates `key` as a new local binding so later lines can reuse this computed value.
		key := names[i]
// L150: Creates `j` as a new local binding so later lines can reuse this computed value.
		j := i - 1
// L151: Starts a loop controlled by `j >= 0 && names[j] > key`, repeating until the loop condition or range is exhausted.
		for j >= 0 && names[j] > key {
// L152: Updates `names[j+1]` so subsequent logic sees the new state.
			names[j+1] = names[j]
// L153: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
			j--
// L154: Closes the current block and returns control to the surrounding scope.
		}
// L155: Updates `names[j+1]` so subsequent logic sees the new state.
		names[j+1] = key
// L156: Closes the current block and returns control to the surrounding scope.
	}
// L157: Returns the listed values to the caller, ending the current function at this point.
	return names
// L158: Closes the current block and returns control to the surrounding scope.
}

// GetRoomClientCount returns the number of clients in a room.
// L161: Declares the `GetRoomClientCount` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetRoomClientCount(roomName string) int {
// L162: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L163: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L164: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L165: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L166: Returns the listed values to the caller, ending the current function at this point.
		return 0
// L167: Closes the current block and returns control to the surrounding scope.
	}
// L168: Returns the listed values to the caller, ending the current function at this point.
	return len(r.clients)
// L169: Closes the current block and returns control to the surrounding scope.
}

// GetRoomClientNames returns a sorted list of client names in a room.
// L172: Declares the `GetRoomClientNames` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetRoomClientNames(roomName string) []string {
// L173: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L174: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L175: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L176: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L177: Returns the listed values to the caller, ending the current function at this point.
		return nil
// L178: Closes the current block and returns control to the surrounding scope.
	}
// L179: Creates `names` from the result of `make`, capturing fresh state for the rest of this scope.
	names := make([]string, 0, len(r.clients))
// L180: Starts a loop controlled by `name := range r.clients`, repeating until the loop condition or range is exhausted.
	for name := range r.clients {
// L181: Updates `names` with the result of `append`, replacing its previous value.
		names = append(names, name)
// L182: Closes the current block and returns control to the surrounding scope.
	}
	// insertion sort
// L184: Starts a loop controlled by `i := 1; i < len(names); i++`, repeating until the loop condition or range is exhausted.
	for i := 1; i < len(names); i++ {
// L185: Creates `key` as a new local binding so later lines can reuse this computed value.
		key := names[i]
// L186: Creates `j` as a new local binding so later lines can reuse this computed value.
		j := i - 1
// L187: Starts a loop controlled by `j >= 0 && names[j] > key`, repeating until the loop condition or range is exhausted.
		for j >= 0 && names[j] > key {
// L188: Updates `names[j+1]` so subsequent logic sees the new state.
			names[j+1] = names[j]
// L189: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
			j--
// L190: Closes the current block and returns control to the surrounding scope.
		}
// L191: Updates `names[j+1]` so subsequent logic sees the new state.
		names[j+1] = key
// L192: Closes the current block and returns control to the surrounding scope.
	}
// L193: Returns the listed values to the caller, ending the current function at this point.
	return names
// L194: Closes the current block and returns control to the surrounding scope.
}

// checkRoomCapacity returns true if the room has space for another client.
// L197: Declares the `checkRoomCapacity` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) checkRoomCapacity(roomName string) bool {
// L198: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L199: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L200: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L201: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L202: Returns the listed values to the caller, ending the current function at this point.
		return true // room doesn't exist yet, will be created with 0 members
// L203: Closes the current block and returns control to the surrounding scope.
	}
// L204: Returns the listed values to the caller, ending the current function at this point.
	return len(r.clients) < MaxActiveClients
// L205: Closes the current block and returns control to the surrounding scope.
}

// ---------- room-scoped queue management ----------

// admitFromRoomQueue admits the first valid queued client in a specific room.
// No-op during shutdown.
// L211: Declares the `admitFromRoomQueue` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) admitFromRoomQueue(roomName string) {
// L212: Evaluates `s.IsShuttingDown()` and enters the guarded branch only when that condition holds.
	if s.IsShuttingDown() {
// L213: Returns immediately from the current function without additional values.
		return
// L214: Closes the current block and returns control to the surrounding scope.
	}
// L215: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L216: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L217: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L218: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L219: Returns immediately from the current function without additional values.
		return
// L220: Closes the current block and returns control to the surrounding scope.
	}
// L221: Starts a loop controlled by `len(r.queue) > 0`, repeating until the loop condition or range is exhausted.
	for len(r.queue) > 0 {
// L222: Creates `entry` as a new local binding so later lines can reuse this computed value.
		entry := r.queue[0]
// L223: Updates `r.queue` so subsequent logic sees the new state.
		r.queue = r.queue[1:]
// L224: Evaluates `entry.client.IsClosed()` and enters the guarded branch only when that condition holds.
		if entry.client.IsClosed() {
// L225: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L226: Closes the current block and returns control to the surrounding scope.
		}
// L227: Calls `close` here for its side effects or returned value in the surrounding control flow.
		close(entry.admit)
// L228: Starts a loop controlled by `i, e := range r.queue`, repeating until the loop condition or range is exhausted.
		for i, e := range r.queue {
// L229: Calls `e.client.Send` here for its side effects or returned value in the surrounding control flow.
			e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
// L230: Closes the current block and returns control to the surrounding scope.
		}
// L231: Returns immediately from the current function without additional values.
		return
// L232: Closes the current block and returns control to the surrounding scope.
	}
// L233: Closes the current block and returns control to the surrounding scope.
}

// removeFromRoomQueue removes the given entry from a room's queue and sends position updates.
// L236: Declares the `removeFromRoomQueue` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) removeFromRoomQueue(roomName string, entry *QueueEntry) {
// L237: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L238: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L239: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L240: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L241: Returns immediately from the current function without additional values.
		return
// L242: Closes the current block and returns control to the surrounding scope.
	}
// L243: Starts a loop controlled by `i, e := range r.queue`, repeating until the loop condition or range is exhausted.
	for i, e := range r.queue {
// L244: Evaluates `e == entry` and enters the guarded branch only when that condition holds.
		if e == entry {
// L245: Updates `r.queue` with the result of `append`, replacing its previous value.
			r.queue = append(r.queue[:i], r.queue[i+1:]...)
// L246: Breaks out of the current loop or switch so execution resumes after that block.
			break
// L247: Closes the current block and returns control to the surrounding scope.
		}
// L248: Closes the current block and returns control to the surrounding scope.
	}
// L249: Starts a loop controlled by `i, e := range r.queue`, repeating until the loop condition or range is exhausted.
	for i, e := range r.queue {
// L250: Calls `e.client.Send` here for its side effects or returned value in the surrounding control flow.
		e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
// L251: Closes the current block and returns control to the surrounding scope.
	}
// L252: Closes the current block and returns control to the surrounding scope.
}

// RemoveFromQueueByIP removes all queue entries across all rooms whose IP matches
// and returns their clients.
// L256: Declares the `RemoveFromAllRoomQueuesByIP` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) RemoveFromAllRoomQueuesByIP(ip string) []*client.Client {
// L257: Creates `host` from the result of `extractHost`, capturing fresh state for the rest of this scope.
	host := extractHost(ip)
// L258: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L259: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()

// L261: Declares `removed` in the current scope so later lines can fill or mutate it as needed.
	var removed []*client.Client
// L262: Starts a loop controlled by `_, r := range s.rooms`, repeating until the loop condition or range is exhausted.
	for _, r := range s.rooms {
// L263: Declares `remaining` in the current scope so later lines can fill or mutate it as needed.
		var remaining []*QueueEntry
// L264: Starts a loop controlled by `_, e := range r.queue`, repeating until the loop condition or range is exhausted.
		for _, e := range r.queue {
// L265: Evaluates `extractHost(e.client.IP) == host` and enters the guarded branch only when that condition holds.
			if extractHost(e.client.IP) == host {
// L266: Updates `removed` with the result of `append`, replacing its previous value.
				removed = append(removed, e.client)
// L267: Closes the previous branch and opens the fallback path used when earlier conditions failed.
			} else {
// L268: Updates `remaining` with the result of `append`, replacing its previous value.
				remaining = append(remaining, e)
// L269: Closes the current block and returns control to the surrounding scope.
			}
// L270: Closes the current block and returns control to the surrounding scope.
		}
// L271: Evaluates `len(removed) > 0` and enters the guarded branch only when that condition holds.
		if len(removed) > 0 {
// L272: Updates `r.queue` so subsequent logic sees the new state.
			r.queue = remaining
// L273: Starts a loop controlled by `i, e := range r.queue`, repeating until the loop condition or range is exhausted.
			for i, e := range r.queue {
// L274: Calls `e.client.Send` here for its side effects or returned value in the surrounding control flow.
				e.client.Send(fmt.Sprintf("You are now #%d in the queue.\n", i+1))
// L275: Closes the current block and returns control to the surrounding scope.
			}
// L276: Closes the current block and returns control to the surrounding scope.
		}
// L277: Closes the current block and returns control to the surrounding scope.
	}
// L278: Returns the listed values to the caller, ending the current function at this point.
	return removed
// L279: Closes the current block and returns control to the surrounding scope.
}
```

## `server/clients.go`

Maintains global client registries and shared helpers for registration, lookup, broadcast, rename, and removal.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `errors` so this file can call functionality from its `errors` package.
	"errors"
// L5: Imports `net-cat/client` so this file can call functionality from its `client` package.
	"net-cat/client"
// L6: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L7: Closes the import block after listing all package dependencies.
)

// ---------- connection tracking ----------

// TrackClient registers a connection in allClients for shutdown notification.
// L12: Declares the `TrackClient` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) TrackClient(c *client.Client) {
// L13: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L14: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L15: Updates `s.allClients[c]` so subsequent logic sees the new state.
	s.allClients[c] = struct{}{}
// L16: Closes the current block and returns control to the surrounding scope.
}

// UntrackClient removes a connection from allClients.
// L19: Declares the `UntrackClient` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) UntrackClient(c *client.Client) {
// L20: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L21: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L22: Calls `delete` here for its side effects or returned value in the surrounding control flow.
	delete(s.allClients, c)
// L23: Closes the current block and returns control to the surrounding scope.
}

// ---------- client map ----------

// RegisterClient atomically checks uniqueness and adds the client to the global map.
// Room assignment happens separately via JoinRoom. Returns nil on success or a
// generic error if the name is taken or reserved.
// L30: Declares the `RegisterClient` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) RegisterClient(c *client.Client, name string) error {
// L31: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L32: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L33: Evaluates `_, exists := s.clients[name]; exists` and enters the guarded branch only when that condition holds.
	if _, exists := s.clients[name]; exists {
// L34: Returns the listed values to the caller, ending the current function at this point.
		return errors.New("name taken")
// L35: Closes the current block and returns control to the surrounding scope.
	}
// L36: Evaluates `s.reservedNames[name]` and enters the guarded branch only when that condition holds.
	if s.reservedNames[name] {
// L37: Returns the listed values to the caller, ending the current function at this point.
		return errors.New("name reserved")
// L38: Closes the current block and returns control to the surrounding scope.
	}
// L39: Creates `now` from the result of `time.Now`, capturing fresh state for the rest of this scope.
	now := time.Now()
// L40: Updates `c.Username` so subsequent logic sees the new state.
	c.Username = name
// L41: Updates `c.JoinTime` so subsequent logic sees the new state.
	c.JoinTime = now
// L42: Calls `c.SetLastActivity` here for its side effects or returned value in the surrounding control flow.
	c.SetLastActivity(now)
// L43: Updates `s.clients[name]` so subsequent logic sees the new state.
	s.clients[name] = c
// L44: Returns the listed values to the caller, ending the current function at this point.
	return nil
// L45: Closes the current block and returns control to the surrounding scope.
}

// L47: Declares the `RemoveClient` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) RemoveClient(username string) {
// L48: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L49: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L50: Creates `c, ok` as a new local binding so later lines can reuse this computed value.
	c, ok := s.clients[username]
// L51: Evaluates `ok` and enters the guarded branch only when that condition holds.
	if ok {
// L52: Evaluates `c.Room != ""` and enters the guarded branch only when that condition holds.
		if c.Room != "" {
// L53: Evaluates `r, rOk := s.rooms[c.Room]; rOk` and enters the guarded branch only when that condition holds.
			if r, rOk := s.rooms[c.Room]; rOk {
// L54: Calls `delete` here for its side effects or returned value in the surrounding control flow.
				delete(r.clients, username)
// L55: Closes the current block and returns control to the surrounding scope.
			}
// L56: Closes the current block and returns control to the surrounding scope.
		}
// L57: Calls `delete` here for its side effects or returned value in the surrounding control flow.
		delete(s.clients, username)
// L58: Closes the current block and returns control to the surrounding scope.
	}
// L59: Closes the current block and returns control to the surrounding scope.
}

// L61: Declares the `GetClient` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetClient(name string) *client.Client {
// L62: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L63: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L64: Returns the listed values to the caller, ending the current function at this point.
	return s.clients[name]
// L65: Closes the current block and returns control to the surrounding scope.
}

// L67: Declares the `GetClientCount` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetClientCount() int {
// L68: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L69: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L70: Returns the listed values to the caller, ending the current function at this point.
	return len(s.clients)
// L71: Closes the current block and returns control to the surrounding scope.
}

// L73: Declares the `GetClientNames` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetClientNames() []string {
// L74: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L75: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L76: Creates `names` from the result of `make`, capturing fresh state for the rest of this scope.
	names := make([]string, 0, len(s.clients))
// L77: Starts a loop controlled by `n := range s.clients`, repeating until the loop condition or range is exhausted.
	for n := range s.clients {
// L78: Updates `names` with the result of `append`, replacing its previous value.
		names = append(names, n)
// L79: Closes the current block and returns control to the surrounding scope.
	}
// L80: Returns the listed values to the caller, ending the current function at this point.
	return names
// L81: Closes the current block and returns control to the surrounding scope.
}

// L83: Declares the `IsReservedName` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) IsReservedName(name string) bool {
// L84: Returns the listed values to the caller, ending the current function at this point.
	return s.reservedNames[name]
// L85: Closes the current block and returns control to the surrounding scope.
}

// RenameClient atomically swaps the key in the client and room maps.
// Returns false if newName is already taken or reserved.
// L89: Declares the `RenameClient` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) RenameClient(c *client.Client, oldName, newName string) bool {
// L90: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L91: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L92: Evaluates `_, exists := s.clients[newName]; exists` and enters the guarded branch only when that condition holds.
	if _, exists := s.clients[newName]; exists {
// L93: Returns the listed values to the caller, ending the current function at this point.
		return false
// L94: Closes the current block and returns control to the surrounding scope.
	}
// L95: Evaluates `s.reservedNames[newName]` and enters the guarded branch only when that condition holds.
	if s.reservedNames[newName] {
// L96: Returns the listed values to the caller, ending the current function at this point.
		return false
// L97: Closes the current block and returns control to the surrounding scope.
	}
// L98: Calls `delete` here for its side effects or returned value in the surrounding control flow.
	delete(s.clients, oldName)
// L99: Updates `c.Username` so subsequent logic sees the new state.
	c.Username = newName
// L100: Updates `s.clients[newName]` so subsequent logic sees the new state.
	s.clients[newName] = c
	// Update room's client map
// L102: Evaluates `c.Room != ""` and enters the guarded branch only when that condition holds.
	if c.Room != "" {
// L103: Evaluates `r, ok := s.rooms[c.Room]; ok` and enters the guarded branch only when that condition holds.
		if r, ok := s.rooms[c.Room]; ok {
// L104: Calls `delete` here for its side effects or returned value in the surrounding control flow.
			delete(r.clients, oldName)
// L105: Updates `r.clients[newName]` so subsequent logic sees the new state.
			r.clients[newName] = c
// L106: Closes the current block and returns control to the surrounding scope.
		}
// L107: Closes the current block and returns control to the surrounding scope.
	}
// L108: Returns the listed values to the caller, ending the current function at this point.
	return true
// L109: Closes the current block and returns control to the surrounding scope.
}

// GetClientsByIP returns all registered clients whose IP matches the given host.
// Pass exclude to skip a specific username (e.g. the command issuer).
// L113: Declares the `GetClientsByIP` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetClientsByIP(host string, exclude string) []*client.Client {
// L114: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L115: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L116: Declares `result` in the current scope so later lines can fill or mutate it as needed.
	var result []*client.Client
// L117: Starts a loop controlled by `_, c := range s.clients`, repeating until the loop condition or range is exhausted.
	for _, c := range s.clients {
// L118: Evaluates `extractHost(c.IP) == host && c.Username != exclude` and enters the guarded branch only when that condition holds.
		if extractHost(c.IP) == host && c.Username != exclude {
// L119: Updates `result` with the result of `append`, replacing its previous value.
			result = append(result, c)
// L120: Closes the current block and returns control to the surrounding scope.
		}
// L121: Closes the current block and returns control to the surrounding scope.
	}
// L122: Returns the listed values to the caller, ending the current function at this point.
	return result
// L123: Closes the current block and returns control to the surrounding scope.
}

// ---------- broadcast ----------

// Broadcast sends msg to every connected client except the one named exclude.
// L128: Declares the `Broadcast` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) Broadcast(msg string, exclude string) {
// L129: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L130: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L131: Starts a loop controlled by `name, c := range s.clients`, repeating until the loop condition or range is exhausted.
	for name, c := range s.clients {
// L132: Evaluates `name != exclude` and enters the guarded branch only when that condition holds.
		if name != exclude {
// L133: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send(msg)
// L134: Closes the current block and returns control to the surrounding scope.
		}
// L135: Closes the current block and returns control to the surrounding scope.
	}
// L136: Closes the current block and returns control to the surrounding scope.
}

// BroadcastAll sends msg to every connected client.
// L139: Declares the `BroadcastAll` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) BroadcastAll(msg string) {
// L140: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L141: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L142: Starts a loop controlled by `_, c := range s.clients`, repeating until the loop condition or range is exhausted.
	for _, c := range s.clients {
// L143: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(msg)
// L144: Closes the current block and returns control to the surrounding scope.
	}
// L145: Closes the current block and returns control to the surrounding scope.
}
```

## `server/handler.go`

Handles connection onboarding, validation, room selection, queue waiting, message loops, and disconnect cleanup.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L5: Imports `io` so this file can call functionality from its `io` package.
	"io"
// L6: Imports `net` so this file can call functionality from its `net` package.
	"net"
// L7: Imports `net-cat/client` so this file can call functionality from its `client` package.
	"net-cat/client"
// L8: Imports `net-cat/cmd` so this file can call functionality from its `cmd` package.
	"net-cat/cmd"
// L9: Imports `net-cat/models` so this file can call functionality from its `models` package.
	"net-cat/models"
// L10: Imports `strings` so this file can call functionality from its `strings` package.
	"strings"
// L11: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L12: Closes the import block after listing all package dependencies.
)

// WelcomeBanner is the exact ASCII art sent on first connect (no trailing prompt).
// L15: Updates `const WelcomeBanner` so subsequent logic sees the new state.
const WelcomeBanner = "Welcome to TCP-Chat!\n" +
// L16: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"         _nnnn_\n" +
// L17: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"        dGGGGMMb\n" +
// L18: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"       @p~qp~~qMb\n" +
// L19: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"       M|@||@) M|\n" +
// L20: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"       @,----.JM|\n" +
// L21: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"      JS^\\__/  qKL\n" +
// L22: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"     dZP        qKRb\n" +
// L23: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"    dZP          qKKb\n" +
// L24: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"   fZP            SMMb\n" +
// L25: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"   HZM            MMMM\n" +
// L26: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"   FqM            MMMM\n" +
// L27: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	" __| \".        |\\dS\"qML\n" +
// L28: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	" |    `.       | `' \\Zq\n" +
// L29: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"_)      \\.___.,|     .'\n" +
// L30: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"\\____   )MMMMMP|   .'\n" +
// L31: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	"     `-'       `--'\n"

// NamePrompt is re-sent after every failed validation attempt.
// L34: Updates `const NamePrompt` so subsequent logic sees the new state.
const NamePrompt = "[ENTER YOUR NAME]:"

// RoomPrompt is sent during room selection.
// L37: Updates `const RoomPrompt` so subsequent logic sees the new state.
const RoomPrompt = "[ENTER ROOM NAME] (Enter for 'general'):"

// handleConnection manages one TCP connection through onboarding, messaging, and cleanup.
// L40: Declares the `handleConnection` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) handleConnection(conn net.Conn) {
	// Enable aggressive TCP keepalive for faster dead peer detection on real connections
// L42: Evaluates `tcpConn, ok := conn.(*net.TCPConn); ok` and enters the guarded branch only when that condition holds.
	if tcpConn, ok := conn.(*net.TCPConn); ok {
// L43: Calls `tcpConn.SetKeepAlive` here for its side effects or returned value in the surrounding control flow.
		tcpConn.SetKeepAlive(true)
// L44: Calls `tcpConn.SetKeepAlivePeriod` here for its side effects or returned value in the surrounding control flow.
		tcpConn.SetKeepAlivePeriod(5 * time.Second)
// L45: Closes the current block and returns control to the surrounding scope.
	}

// L47: Creates `c` from the result of `client.NewClient`, capturing fresh state for the rest of this scope.
	c := client.NewClient(conn)
// L48: Calls `s.TrackClient` here for its side effects or returned value in the surrounding control flow.
	s.TrackClient(c)
// L49: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.UntrackClient(c)

	// Check IP against kick/ban lists BEFORE welcome banner or queue prompt.
	// Write directly to conn (bypassing the async writeLoop) to guarantee delivery
	// before we close the connection.
// L54: Evaluates `blocked, reason := s.IsIPBlocked(c.IP); blocked` and enters the guarded branch only when that condition holds.
	if blocked, reason := s.IsIPBlocked(c.IP); blocked {
// L55: Calls `conn.Write` here for its side effects or returned value in the surrounding control flow.
		conn.Write([]byte(reason))
// L56: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
		c.Close()
// L57: Returns immediately from the current function without additional values.
		return
// L58: Closes the current block and returns control to the surrounding scope.
	}

	// Reject connections that arrive during shutdown
// L61: Evaluates `s.IsShuttingDown()` and enters the guarded branch only when that condition holds.
	if s.IsShuttingDown() {
// L62: Calls `conn.Write` here for its side effects or returned value in the surrounding control flow.
		conn.Write([]byte("Server is shutting down. Goodbye!\n"))
// L63: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
		c.Close()
// L64: Returns immediately from the current function without additional values.
		return
// L65: Closes the current block and returns control to the surrounding scope.
	}

	// Send welcome banner + name prompt
// L68: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send(WelcomeBanner + NamePrompt)

	// --- Name validation loop (infinite retries) ---
// L71: Creates `registered` as a new local binding so later lines can reuse this computed value.
	registered := false
// L72: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L73: Creates `name, err` from the result of `c.ReadLine`, capturing fresh state for the rest of this scope.
		name, err := c.ReadLine()
// L74: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
		if err != nil {
// L75: Evaluates `err != io.EOF` and enters the guarded branch only when that condition holds.
			if err != io.EOF {
// L76: Calls `s.Logger.Log` here for its side effects or returned value in the surrounding control flow.
				s.Logger.Log(models.Message{
// L77: Keeps this element in the surrounding multiline literal or call expression.
					Timestamp: time.Now(),
// L78: Keeps this element in the surrounding multiline literal or call expression.
					Type:      models.MsgServerEvent,
// L79: Keeps this element in the surrounding multiline literal or call expression.
					Content:   fmt.Sprintf("Connection error during onboarding from %s", c.IP),
// L80: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
				})
// L81: Closes the current block and returns control to the surrounding scope.
			}
// L82: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
			c.Close()
// L83: Returns immediately from the current function without additional values.
			return
// L84: Closes the current block and returns control to the surrounding scope.
		}

// L86: Evaluates `valErr := validateName(name); valErr != nil` and enters the guarded branch only when that condition holds.
		if valErr := validateName(name); valErr != nil {
// L87: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send(valErr.Error() + "\n" + NamePrompt)
// L88: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L89: Closes the current block and returns control to the surrounding scope.
		}
// L90: Evaluates `s.IsReservedName(name)` and enters the guarded branch only when that condition holds.
		if s.IsReservedName(name) {
// L91: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send("Name '" + name + "' is reserved.\n" + NamePrompt)
// L92: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L93: Closes the current block and returns control to the surrounding scope.
		}
// L94: Evaluates `err := s.RegisterClient(c, name); err != nil` and enters the guarded branch only when that condition holds.
		if err := s.RegisterClient(c, name); err != nil {
// L95: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send("Name is already taken.\n" + NamePrompt)
// L96: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L97: Closes the current block and returns control to the surrounding scope.
		}
// L98: Updates `registered` so subsequent logic sees the new state.
		registered = true
// L99: Breaks out of the current loop or switch so execution resumes after that block.
		break
// L100: Closes the current block and returns control to the surrounding scope.
	}
// L101: Evaluates `!registered` and enters the guarded branch only when that condition holds.
	if !registered {
// L102: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
		c.Close()
// L103: Returns immediately from the current function without additional values.
		return
// L104: Closes the current block and returns control to the surrounding scope.
	}

	// Auto-restore admin privileges for known admins
// L107: Evaluates `s.IsKnownAdmin(c.Username)` and enters the guarded branch only when that condition holds.
	if s.IsKnownAdmin(c.Username) {
// L108: Calls `c.SetAdmin` here for its side effects or returned value in the surrounding control flow.
		c.SetAdmin(true)
// L109: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Welcome back, admin!\n")
// L110: Closes the current block and returns control to the surrounding scope.
	}

	// --- Room selection ---
// L113: Creates `roomName` from the result of `s.readRoomChoice`, capturing fresh state for the rest of this scope.
	roomName := s.readRoomChoice(c)
// L114: Evaluates `roomName == ""` and enters the guarded branch only when that condition holds.
	if roomName == "" {
		// Client disconnected during room selection
// L116: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
		s.RemoveClient(c.Username)
// L117: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
		c.Close()
// L118: Returns immediately from the current function without additional values.
		return
// L119: Closes the current block and returns control to the surrounding scope.
	}

	// Check room capacity and offer queue if full
// L122: Evaluates `!s.checkOrQueueRoom(c, roomName)` and enters the guarded branch only when that condition holds.
	if !s.checkOrQueueRoom(c, roomName) {
// L123: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
		s.RemoveClient(c.Username)
// L124: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
		c.Close()
// L125: Returns immediately from the current function without additional values.
		return
// L126: Closes the current block and returns control to the surrounding scope.
	}

	// Join the selected room
// L129: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L130: Calls `s.JoinRoom` here for its side effects or returned value in the surrounding control flow.
	s.JoinRoom(c, roomName)
// L131: Calls `s.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Unlock()

	// Cleanup runs on any exit from this point (disconnect, /quit, kick, etc.)
// L134: Schedules this cleanup or follow-up call to run when the current function returns.
	defer func() {
// L135: Creates `username` as a new local binding so later lines can reuse this computed value.
		username := c.Username
// L136: Creates `currentRoom` as a new local binding so later lines can reuse this computed value.
		currentRoom := c.Room
// L137: Starts a switch on `c.GetDisconnectReason()` so the following cases can branch on that value cleanly.
		switch c.GetDisconnectReason() {
// L138: Selects the `"kicked", "banned"` branch inside the surrounding switch or select.
		case "kicked", "banned":
			// moderation handler already removed from map + broadcast + logged
// L140: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
		default:
// L141: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
			s.RemoveClient(username)
// L142: Creates `reason` from the result of `c.GetDisconnectReason`, capturing fresh state for the rest of this scope.
			reason := c.GetDisconnectReason()
// L143: Evaluates `reason == ""` and enters the guarded branch only when that condition holds.
			if reason == "" {
// L144: Updates `reason` so subsequent logic sees the new state.
				reason = "voluntary"
// L145: Closes the current block and returns control to the surrounding scope.
			}
// L146: Creates `leaveMsg` as a new local binding so later lines can reuse this computed value.
			leaveMsg := models.Message{
// L147: Keeps this element in the surrounding multiline literal or call expression.
				Timestamp: time.Now(),
// L148: Keeps this element in the surrounding multiline literal or call expression.
				Sender:    username,
// L149: Keeps this element in the surrounding multiline literal or call expression.
				Type:      models.MsgLeave,
// L150: Keeps this element in the surrounding multiline literal or call expression.
				Extra:     reason,
// L151: Closes the current block and returns control to the surrounding scope.
			}
// L152: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
			s.recordRoomEvent(currentRoom, leaveMsg)
// L153: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
			s.BroadcastRoom(currentRoom, models.FormatLeave(username)+"\n", username)
// L154: Closes the current block and returns control to the surrounding scope.
		}
// L155: Calls `s.admitFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
		s.admitFromRoomQueue(currentRoom)
// L156: Calls `s.deleteRoomIfEmpty` here for its side effects or returned value in the surrounding control flow.
		s.deleteRoomIfEmpty(currentRoom)
// L157: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
		c.Close()
// L158: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	}()

	// Deliver room history
// L161: Starts a loop controlled by `_, msg := range s.GetRoomHistory(c.Room)`, repeating until the loop condition or range is exhausted.
	for _, msg := range s.GetRoomHistory(c.Room) {
// L162: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(msg.Display() + "\n")
// L163: Closes the current block and returns control to the surrounding scope.
	}

	// Enable character-at-a-time echo mode for input continuity
// L166: Calls `c.SetEchoMode` here for its side effects or returned value in the surrounding control flow.
	c.SetEchoMode(true)

	// First prompt (uses SendPrompt so writeLoop tracks the prompt for redraw)
// L169: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))

	// Broadcast join to the room
// L172: Creates `joinMsg` as a new local binding so later lines can reuse this computed value.
	joinMsg := models.Message{
// L173: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L174: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    c.Username,
// L175: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgJoin,
// L176: Closes the current block and returns control to the surrounding scope.
	}
// L177: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(c.Room, joinMsg)
// L178: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(c.Room, models.FormatJoin(c.Username)+"\n", c.Username)

	// Initialize heartbeat tracking and start the health check goroutine
// L181: Calls `c.SetLastInput` here for its side effects or returned value in the surrounding control flow.
	c.SetLastInput(time.Now())
// L182: Launches the following call in a new goroutine so it can run concurrently with the current path.
	go s.startHeartbeat(c)

	// --- Message loop (character-at-a-time reading with echo) ---
// L185: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L186: Creates `line, err` from the result of `c.ReadLineInteractive`, capturing fresh state for the rest of this scope.
		line, err := c.ReadLineInteractive()
// L187: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
		if err != nil {
// L188: Calls `c.SetDisconnectReason` here for its side effects or returned value in the surrounding control flow.
			c.SetDisconnectReason("drop")
// L189: Evaluates `err != io.EOF` and enters the guarded branch only when that condition holds.
			if err != io.EOF {
// L190: Calls `s.Logger.Log` here for its side effects or returned value in the surrounding control flow.
				s.Logger.Log(models.Message{
// L191: Keeps this element in the surrounding multiline literal or call expression.
					Timestamp: time.Now(),
// L192: Keeps this element in the surrounding multiline literal or call expression.
					Type:      models.MsgServerEvent,
// L193: Keeps this element in the surrounding multiline literal or call expression.
					Content:   fmt.Sprintf("Connection error for %s: %v", c.Username, err),
// L194: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
				})
// L195: Closes the current block and returns control to the surrounding scope.
			}
// L196: Returns immediately from the current function without additional values.
			return
// L197: Closes the current block and returns control to the surrounding scope.
		}
		// Any input from the client proves they are alive (heartbeat tracking)
// L199: Calls `c.SetLastInput` here for its side effects or returned value in the surrounding control flow.
		c.SetLastInput(time.Now())
// L200: Creates `cmdName, args, isCmd` from the result of `cmd.ParseCommand`, capturing fresh state for the rest of this scope.
		cmdName, args, isCmd := cmd.ParseCommand(line)
// L201: Evaluates `isCmd` and enters the guarded branch only when that condition holds.
		if isCmd {
// L202: Evaluates `s.dispatchCommand(c, cmdName, args)` and enters the guarded branch only when that condition holds.
			if s.dispatchCommand(c, cmdName, args) {
// L203: Returns the listed values to the caller, ending the current function at this point.
				return // /quit
// L204: Closes the current block and returns control to the surrounding scope.
			}
// L205: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L206: Closes the current block and returns control to the surrounding scope.
		}
// L207: Calls `s.handleChatMessage` here for its side effects or returned value in the surrounding control flow.
		s.handleChatMessage(c, line)
// L208: Closes the current block and returns control to the surrounding scope.
	}
// L209: Closes the current block and returns control to the surrounding scope.
}

// ---------- room selection ----------

// sendRoomSelection lists available rooms with counts and sends the room prompt.
// L214: Declares the `sendRoomSelection` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) sendRoomSelection(c *client.Client) {
// L215: Creates `roomNames` from the result of `s.GetRoomNames`, capturing fresh state for the rest of this scope.
	roomNames := s.GetRoomNames()
// L216: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send("Available rooms:\n")
// L217: Starts a loop controlled by `_, rn := range roomNames`, repeating until the loop condition or range is exhausted.
	for _, rn := range roomNames {
// L218: Creates `count` from the result of `s.GetRoomClientCount`, capturing fresh state for the rest of this scope.
		count := s.GetRoomClientCount(rn)
// L219: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(fmt.Sprintf("  %s (%d/%d users)\n", rn, count, MaxActiveClients))
// L220: Closes the current block and returns control to the surrounding scope.
	}
// L221: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send(RoomPrompt)
// L222: Closes the current block and returns control to the surrounding scope.
}

// readRoomChoice sends room selection prompt and reads the client's choice.
// Returns the room name, or "" if the client disconnected.
// L226: Declares the `readRoomChoice` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) readRoomChoice(c *client.Client) string {
// L227: Calls `s.sendRoomSelection` here for its side effects or returned value in the surrounding control flow.
	s.sendRoomSelection(c)
// L228: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L229: Creates `line, err` from the result of `c.ReadLine`, capturing fresh state for the rest of this scope.
		line, err := c.ReadLine()
// L230: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
		if err != nil {
// L231: Returns the listed values to the caller, ending the current function at this point.
			return ""
// L232: Closes the current block and returns control to the surrounding scope.
		}
// L233: Updates `line` with the result of `strings.TrimSpace`, replacing its previous value.
		line = strings.TrimSpace(line)
// L234: Evaluates `line == ""` and enters the guarded branch only when that condition holds.
		if line == "" {
// L235: Returns the listed values to the caller, ending the current function at this point.
			return s.DefaultRoom
// L236: Closes the current block and returns control to the surrounding scope.
		}
// L237: Evaluates `err := validateRoomName(line); err != nil` and enters the guarded branch only when that condition holds.
		if err := validateRoomName(line); err != nil {
// L238: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send(err.Error() + "\n" + RoomPrompt)
// L239: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L240: Closes the current block and returns control to the surrounding scope.
		}
// L241: Returns the listed values to the caller, ending the current function at this point.
		return line
// L242: Closes the current block and returns control to the surrounding scope.
	}
// L243: Closes the current block and returns control to the surrounding scope.
}

// ---------- capacity check and queue (per-room) ----------

// checkOrQueueRoom returns true if the client can proceed to join the room.
// If the room is at capacity, offers a queue position and blocks until
// admitted, declined, or the server shuts down.
// L250: Declares the `checkOrQueueRoom` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) checkOrQueueRoom(c *client.Client, roomName string) bool {
// L251: Evaluates `s.IsShuttingDown()` and enters the guarded branch only when that condition holds.
	if s.IsShuttingDown() {
// L252: Returns the listed values to the caller, ending the current function at this point.
		return false
// L253: Closes the current block and returns control to the surrounding scope.
	}

// L255: Evaluates `s.checkRoomCapacity(roomName)` and enters the guarded branch only when that condition holds.
	if s.checkRoomCapacity(roomName) {
// L256: Returns the listed values to the caller, ending the current function at this point.
		return true
// L257: Closes the current block and returns control to the surrounding scope.
	}

	// Room is full — offer queue
// L260: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L261: Creates `r` from the result of `s.getOrCreateRoom`, capturing fresh state for the rest of this scope.
	r := s.getOrCreateRoom(roomName)
// L262: Creates `entry` as a new local binding so later lines can reuse this computed value.
	entry := &QueueEntry{
// L263: Keeps this element in the surrounding multiline literal or call expression.
		client: c,
// L264: Keeps this element in the surrounding multiline literal or call expression.
		admit:  make(chan struct{}),
// L265: Closes the current block and returns control to the surrounding scope.
	}
// L266: Updates `r.queue` with the result of `append`, replacing its previous value.
	r.queue = append(r.queue, entry)
// L267: Creates `pos` from the result of `len`, capturing fresh state for the rest of this scope.
	pos := len(r.queue)
// L268: Calls `s.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Unlock()

// L270: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send(fmt.Sprintf("Room '%s' is full. You are #%d in the queue. Would you like to wait? (yes/no)\n", roomName, pos))

	// Read yes/no response
// L273: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
	for {
// L274: Creates `line, err` from the result of `c.ReadLine`, capturing fresh state for the rest of this scope.
		line, err := c.ReadLine()
// L275: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
		if err != nil {
// L276: Calls `s.removeFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
			s.removeFromRoomQueue(roomName, entry)
// L277: Returns the listed values to the caller, ending the current function at this point.
			return false
// L278: Closes the current block and returns control to the surrounding scope.
		}
// L279: Updates `line` with the result of `strings.TrimSpace`, replacing its previous value.
		line = strings.TrimSpace(line)
// L280: Starts a switch on `line` so the following cases can branch on that value cleanly.
		switch line {
// L281: Selects the `"yes"` branch inside the surrounding switch or select.
		case "yes":
// L282: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send("Waiting for a slot to open... (press Ctrl+C to cancel)\n")
// L283: Returns the listed values to the caller, ending the current function at this point.
			return s.waitForRoomAdmission(c, roomName, entry)
// L284: Selects the `"no"` branch inside the surrounding switch or select.
		case "no":
// L285: Calls `s.removeFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
			s.removeFromRoomQueue(roomName, entry)
// L286: Returns the listed values to the caller, ending the current function at this point.
			return false
// L287: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
		default:
// L288: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send("Invalid input. Please type 'yes' or 'no'.\n")
// L289: Closes the current block and returns control to the surrounding scope.
		}
// L290: Closes the current block and returns control to the surrounding scope.
	}
// L291: Closes the current block and returns control to the surrounding scope.
}

// waitForRoomAdmission blocks until the client is admitted from the room queue,
// disconnects, or the server shuts down. Returns true if admitted.
// L295: Declares the `waitForRoomAdmission` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) waitForRoomAdmission(c *client.Client, roomName string, entry *QueueEntry) bool {
// L296: Creates `readDone` from the result of `make`, capturing fresh state for the rest of this scope.
	readDone := make(chan error, 1)
// L297: Creates `monitorDone` from the result of `make`, capturing fresh state for the rest of this scope.
	monitorDone := make(chan struct{})
// L298: Launches the following call in a new goroutine so it can run concurrently with the current path.
	go func() {
// L299: Schedules this cleanup or follow-up call to run when the current function returns.
		defer close(monitorDone)
// L300: Starts a loop controlled by ``, repeating until the loop condition or range is exhausted.
		for {
// L301: Creates `_, err` from the result of `c.ReadLine`, capturing fresh state for the rest of this scope.
			_, err := c.ReadLine()
// L302: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
			if err != nil {
// L303: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
				select {
// L304: Selects the `readDone <- err` branch inside the surrounding switch or select.
				case readDone <- err:
// L305: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
				default:
// L306: Closes the current block and returns control to the surrounding scope.
				}
// L307: Returns immediately from the current function without additional values.
				return
// L308: Closes the current block and returns control to the surrounding scope.
			}
// L309: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
			select {
// L310: Selects the `<-entry.admit` branch inside the surrounding switch or select.
			case <-entry.admit:
// L311: Returns immediately from the current function without additional values.
				return
// L312: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
			default:
// L313: Closes the current block and returns control to the surrounding scope.
			}
// L314: Closes the current block and returns control to the surrounding scope.
		}
// L315: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
	}()

// L317: Starts a channel select so the goroutine can react to whichever communication path becomes ready first.
	select {
// L318: Selects the `<-entry.admit` branch inside the surrounding switch or select.
	case <-entry.admit:
// L319: Calls `c.Conn.SetReadDeadline` here for its side effects or returned value in the surrounding control flow.
		c.Conn.SetReadDeadline(time.Now())
// L320: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
		<-monitorDone
// L321: Calls `c.Conn.SetReadDeadline` here for its side effects or returned value in the surrounding control flow.
		c.Conn.SetReadDeadline(time.Time{})
// L322: Calls `c.ResetScanner` here for its side effects or returned value in the surrounding control flow.
		c.ResetScanner()
// L323: Returns the listed values to the caller, ending the current function at this point.
		return true
// L324: Selects the `<-s.quit` branch inside the surrounding switch or select.
	case <-s.quit:
// L325: Calls `time.Sleep` here for its side effects or returned value in the surrounding control flow.
		time.Sleep(100 * time.Millisecond)
// L326: Calls `s.removeFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
		s.removeFromRoomQueue(roomName, entry)
// L327: Returns the listed values to the caller, ending the current function at this point.
		return false
// L328: Selects the `<-readDone` branch inside the surrounding switch or select.
	case <-readDone:
// L329: Calls `s.removeFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
		s.removeFromRoomQueue(roomName, entry)
// L330: Returns the listed values to the caller, ending the current function at this point.
		return false
// L331: Closes the current block and returns control to the surrounding scope.
	}
// L332: Closes the current block and returns control to the surrounding scope.
}

// ---------- name validation ----------

// validateName checks format rules (no uniqueness – that is checked during registration).
// L337: Declares the `validateName` function, which starts a named unit of behavior other code can call.
func validateName(name string) error {
// L338: Evaluates `len(name) == 0` and enters the guarded branch only when that condition holds.
	if len(name) == 0 {
// L339: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Errorf("Name cannot be empty.")
// L340: Closes the current block and returns control to the surrounding scope.
	}
// L341: Creates `allWhitespace` as a new local binding so later lines can reuse this computed value.
	allWhitespace := true
// L342: Starts a loop controlled by `_, b := range []byte(name)`, repeating until the loop condition or range is exhausted.
	for _, b := range []byte(name) {
// L343: Evaluates `b != ' ' && b != '\t' && b != '\r' && b != '\n'` and enters the guarded branch only when that condition holds.
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
// L344: Updates `allWhitespace` so subsequent logic sees the new state.
			allWhitespace = false
// L345: Breaks out of the current loop or switch so execution resumes after that block.
			break
// L346: Closes the current block and returns control to the surrounding scope.
		}
// L347: Closes the current block and returns control to the surrounding scope.
	}
// L348: Evaluates `allWhitespace` and enters the guarded branch only when that condition holds.
	if allWhitespace {
// L349: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Errorf("Name cannot be empty.")
// L350: Closes the current block and returns control to the surrounding scope.
	}
// L351: Starts a loop controlled by `_, b := range []byte(name)`, repeating until the loop condition or range is exhausted.
	for _, b := range []byte(name) {
// L352: Evaluates `b == ' '` and enters the guarded branch only when that condition holds.
		if b == ' ' {
// L353: Returns the listed values to the caller, ending the current function at this point.
			return fmt.Errorf("Name cannot contain spaces.")
// L354: Closes the current block and returns control to the surrounding scope.
		}
// L355: Closes the current block and returns control to the surrounding scope.
	}
// L356: Evaluates `len(name) > 32` and enters the guarded branch only when that condition holds.
	if len(name) > 32 {
// L357: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Errorf("Name too long (max 32 characters).")
// L358: Closes the current block and returns control to the surrounding scope.
	}
// L359: Starts a loop controlled by `_, b := range []byte(name)`, repeating until the loop condition or range is exhausted.
	for _, b := range []byte(name) {
// L360: Evaluates `b < 0x21 || b > 0x7E` and enters the guarded branch only when that condition holds.
		if b < 0x21 || b > 0x7E {
// L361: Returns the listed values to the caller, ending the current function at this point.
			return fmt.Errorf("Name must contain only printable characters.")
// L362: Closes the current block and returns control to the surrounding scope.
		}
// L363: Closes the current block and returns control to the surrounding scope.
	}
// L364: Returns the listed values to the caller, ending the current function at this point.
	return nil
// L365: Closes the current block and returns control to the surrounding scope.
}

// validateRoomName checks room name format (same rules as validateName).
// L368: Declares the `validateRoomName` function, which starts a named unit of behavior other code can call.
func validateRoomName(name string) error {
// L369: Evaluates `len(name) == 0` and enters the guarded branch only when that condition holds.
	if len(name) == 0 {
// L370: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Errorf("Room name cannot be empty.")
// L371: Closes the current block and returns control to the surrounding scope.
	}
// L372: Evaluates `len(name) > 32` and enters the guarded branch only when that condition holds.
	if len(name) > 32 {
// L373: Returns the listed values to the caller, ending the current function at this point.
		return fmt.Errorf("Room name too long (max 32 characters).")
// L374: Closes the current block and returns control to the surrounding scope.
	}
// L375: Starts a loop controlled by `_, b := range []byte(name)`, repeating until the loop condition or range is exhausted.
	for _, b := range []byte(name) {
// L376: Evaluates `b < 0x21 || b > 0x7E` and enters the guarded branch only when that condition holds.
		if b < 0x21 || b > 0x7E {
// L377: Returns the listed values to the caller, ending the current function at this point.
			return fmt.Errorf("Room name must contain only printable characters.")
// L378: Closes the current block and returns control to the surrounding scope.
		}
// L379: Closes the current block and returns control to the surrounding scope.
	}
// L380: Returns the listed values to the caller, ending the current function at this point.
	return nil
// L381: Closes the current block and returns control to the surrounding scope.
}
```

## `server/commands.go`

Implements the user-visible chat command surface, including room actions, moderation, messaging, and admin operations.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L5: Imports `net-cat/client` so this file can call functionality from its `client` package.
	"net-cat/client"
// L6: Imports `net-cat/cmd` so this file can call functionality from its `cmd` package.
	"net-cat/cmd"
// L7: Imports `net-cat/models` so this file can call functionality from its `models` package.
	"net-cat/models"
// L8: Imports `strings` so this file can call functionality from its `strings` package.
	"strings"
// L9: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L10: Closes the import block after listing all package dependencies.
)

// ---------- chat messages ----------

// L14: Declares the `handleChatMessage` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) handleChatMessage(c *client.Client, line string) {
// L15: Evaluates `len(strings.TrimSpace(line)) == 0` and enters the guarded branch only when that condition holds.
	if len(strings.TrimSpace(line)) == 0 {
// L16: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L17: Returns immediately from the current function without additional values.
		return
// L18: Closes the current block and returns control to the surrounding scope.
	}
// L19: Evaluates `len(line) > 2048` and enters the guarded branch only when that condition holds.
	if len(line) > 2048 {
// L20: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Message too long (max 2048 characters).\n")
// L21: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L22: Returns immediately from the current function without additional values.
		return
// L23: Closes the current block and returns control to the surrounding scope.
	}
// L24: Evaluates `c.IsMuted()` and enters the guarded branch only when that condition holds.
	if c.IsMuted() {
// L25: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("You are muted.\n")
// L26: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L27: Returns immediately from the current function without additional values.
		return
// L28: Closes the current block and returns control to the surrounding scope.
	}

// L30: Creates `now` from the result of `time.Now`, capturing fresh state for the rest of this scope.
	now := time.Now()
// L31: Creates `msg` as a new local binding so later lines can reuse this computed value.
	msg := models.Message{
// L32: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: now,
// L33: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    c.Username,
// L34: Keeps this element in the surrounding multiline literal or call expression.
		Content:   line,
// L35: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgChat,
// L36: Closes the current block and returns control to the surrounding scope.
	}
// L37: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(c.Room, msg)
// L38: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(c.Room, msg.Display()+"\n", c.Username)
// L39: Calls `c.SetLastActivity` here for its side effects or returned value in the surrounding control flow.
	c.SetLastActivity(now)
// L40: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L41: Closes the current block and returns control to the surrounding scope.
}

// ---------- command dispatch ----------

// dispatchCommand routes a parsed command. Returns true when the caller should exit (/quit).
// L46: Declares the `dispatchCommand` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) dispatchCommand(c *client.Client, cmdName, args string) bool {
// L47: Creates `def, exists` as a new local binding so later lines can reuse this computed value.
	def, exists := cmd.Commands[cmdName]
// L48: Evaluates `!exists` and enters the guarded branch only when that condition holds.
	if !exists {
// L49: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Unknown command: /" + cmdName + ". Use /help to see available commands.\n")
// L50: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L51: Returns the listed values to the caller, ending the current function at this point.
		return false
// L52: Closes the current block and returns control to the surrounding scope.
	}
// L53: Creates `clientPriv` from the result of `cmd.GetPrivilegeLevel`, capturing fresh state for the rest of this scope.
	clientPriv := cmd.GetPrivilegeLevel(c.IsAdmin(), false)
// L54: Evaluates `def.MinPriv > clientPriv` and enters the guarded branch only when that condition holds.
	if def.MinPriv > clientPriv {
// L55: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Insufficient privileges.\n")
// L56: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L57: Returns the listed values to the caller, ending the current function at this point.
		return false
// L58: Closes the current block and returns control to the surrounding scope.
	}
// L59: Starts a switch on `cmdName` so the following cases can branch on that value cleanly.
	switch cmdName {
// L60: Selects the `"quit"` branch inside the surrounding switch or select.
	case "quit":
// L61: Returns the listed values to the caller, ending the current function at this point.
		return true
// L62: Selects the `"list"` branch inside the surrounding switch or select.
	case "list":
// L63: Calls `s.cmdList` here for its side effects or returned value in the surrounding control flow.
		s.cmdList(c)
// L64: Selects the `"rooms"` branch inside the surrounding switch or select.
	case "rooms":
// L65: Calls `s.cmdRooms` here for its side effects or returned value in the surrounding control flow.
		s.cmdRooms(c)
// L66: Selects the `"switch"` branch inside the surrounding switch or select.
	case "switch":
// L67: Calls `s.cmdSwitch` here for its side effects or returned value in the surrounding control flow.
		s.cmdSwitch(c, args)
// L68: Selects the `"create"` branch inside the surrounding switch or select.
	case "create":
// L69: Calls `s.cmdCreate` here for its side effects or returned value in the surrounding control flow.
		s.cmdCreate(c, args)
// L70: Selects the `"help"` branch inside the surrounding switch or select.
	case "help":
// L71: Calls `s.cmdHelp` here for its side effects or returned value in the surrounding control flow.
		s.cmdHelp(c)
// L72: Selects the `"name"` branch inside the surrounding switch or select.
	case "name":
// L73: Calls `s.cmdName` here for its side effects or returned value in the surrounding control flow.
		s.cmdName(c, args)
// L74: Selects the `"whisper"` branch inside the surrounding switch or select.
	case "whisper":
// L75: Calls `s.cmdWhisper` here for its side effects or returned value in the surrounding control flow.
		s.cmdWhisper(c, args)
// L76: Selects the `"kick"` branch inside the surrounding switch or select.
	case "kick":
// L77: Calls `s.cmdKick` here for its side effects or returned value in the surrounding control flow.
		s.cmdKick(c, args)
// L78: Selects the `"ban"` branch inside the surrounding switch or select.
	case "ban":
// L79: Calls `s.cmdBan` here for its side effects or returned value in the surrounding control flow.
		s.cmdBan(c, args)
// L80: Selects the `"mute"` branch inside the surrounding switch or select.
	case "mute":
// L81: Calls `s.cmdMute` here for its side effects or returned value in the surrounding control flow.
		s.cmdMute(c, args)
// L82: Selects the `"unmute"` branch inside the surrounding switch or select.
	case "unmute":
// L83: Calls `s.cmdUnmute` here for its side effects or returned value in the surrounding control flow.
		s.cmdUnmute(c, args)
// L84: Selects the `"announce"` branch inside the surrounding switch or select.
	case "announce":
// L85: Calls `s.cmdAnnounce` here for its side effects or returned value in the surrounding control flow.
		s.cmdAnnounce(c, args)
// L86: Selects the `"promote"` branch inside the surrounding switch or select.
	case "promote":
// L87: Calls `s.cmdPromote` here for its side effects or returned value in the surrounding control flow.
		s.cmdPromote(c, args)
// L88: Selects the `"demote"` branch inside the surrounding switch or select.
	case "demote":
// L89: Calls `s.cmdDemote` here for its side effects or returned value in the surrounding control flow.
		s.cmdDemote(c, args)
// L90: Closes the current block and returns control to the surrounding scope.
	}
// L91: Returns the listed values to the caller, ending the current function at this point.
	return false
// L92: Closes the current block and returns control to the surrounding scope.
}

// ---------- /list (room-scoped) ----------

// L96: Declares the `cmdList` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdList(c *client.Client) {
// L97: Creates `roomName` as a new local binding so later lines can reuse this computed value.
	roomName := c.Room
// L98: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L99: Defines the `entry` struct, which groups related state that this package manages together.
	type entry struct {
// L100: Adds the `name` field to the struct so instances can hold that piece of state.
		name string
// L101: Adds the `idle` field to the struct so instances can hold that piece of state.
		idle time.Duration
// L102: Closes the struct definition after listing all of its fields.
	}
// L103: Creates `r, ok` as a new local binding so later lines can reuse this computed value.
	r, ok := s.rooms[roomName]
// L104: Evaluates `!ok` and enters the guarded branch only when that condition holds.
	if !ok {
// L105: Calls `s.mu.RUnlock` here for its side effects or returned value in the surrounding control flow.
		s.mu.RUnlock()
// L106: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L107: Returns immediately from the current function without additional values.
		return
// L108: Closes the current block and returns control to the surrounding scope.
	}
// L109: Creates `entries` from the result of `make`, capturing fresh state for the rest of this scope.
	entries := make([]entry, 0, len(r.clients))
// L110: Starts a loop controlled by `n, cl := range r.clients`, repeating until the loop condition or range is exhausted.
	for n, cl := range r.clients {
// L111: Updates `entries` with the result of `append`, replacing its previous value.
		entries = append(entries, entry{name: n, idle: time.Since(cl.GetLastActivity()).Truncate(time.Second)})
// L112: Closes the current block and returns control to the surrounding scope.
	}
// L113: Calls `s.mu.RUnlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RUnlock()

	// simple insertion sort
// L116: Starts a loop controlled by `i := 1; i < len(entries); i++`, repeating until the loop condition or range is exhausted.
	for i := 1; i < len(entries); i++ {
// L117: Creates `key` as a new local binding so later lines can reuse this computed value.
		key := entries[i]
// L118: Creates `j` as a new local binding so later lines can reuse this computed value.
		j := i - 1
// L119: Starts a loop controlled by `j >= 0 && entries[j].name > key.name`, repeating until the loop condition or range is exhausted.
		for j >= 0 && entries[j].name > key.name {
// L120: Updates `entries[j+1]` so subsequent logic sees the new state.
			entries[j+1] = entries[j]
// L121: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
			j--
// L122: Closes the current block and returns control to the surrounding scope.
		}
// L123: Updates `entries[j+1]` so subsequent logic sees the new state.
		entries[j+1] = key
// L124: Closes the current block and returns control to the surrounding scope.
	}

// L126: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send(fmt.Sprintf("Room %s — connected clients:\n", roomName))
// L127: Starts a loop controlled by `_, e := range entries`, repeating until the loop condition or range is exhausted.
	for _, e := range entries {
// L128: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(fmt.Sprintf("  %s (idle: %s)\n", e.name, e.idle.String()))
// L129: Closes the current block and returns control to the surrounding scope.
	}
// L130: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L131: Closes the current block and returns control to the surrounding scope.
}

// ---------- /rooms ----------

// L135: Declares the `cmdRooms` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdRooms(c *client.Client) {
// L136: Creates `roomNames` from the result of `s.GetRoomNames`, capturing fresh state for the rest of this scope.
	roomNames := s.GetRoomNames()
// L137: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send("Available rooms:\n")
// L138: Starts a loop controlled by `_, rn := range roomNames`, repeating until the loop condition or range is exhausted.
	for _, rn := range roomNames {
// L139: Creates `count` from the result of `s.GetRoomClientCount`, capturing fresh state for the rest of this scope.
		count := s.GetRoomClientCount(rn)
// L140: Creates `marker` as a new local binding so later lines can reuse this computed value.
		marker := ""
// L141: Evaluates `rn == c.Room` and enters the guarded branch only when that condition holds.
		if rn == c.Room {
// L142: Updates `marker` so subsequent logic sees the new state.
			marker = " (current)"
// L143: Closes the current block and returns control to the surrounding scope.
		}
// L144: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(fmt.Sprintf("  %s (%d/%d users)%s\n", rn, count, MaxActiveClients, marker))
// L145: Closes the current block and returns control to the surrounding scope.
	}
// L146: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L147: Closes the current block and returns control to the surrounding scope.
}

// ---------- /switch ----------

// L151: Declares the `cmdSwitch` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdSwitch(c *client.Client, args string) {
// L152: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L153: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Usage: /switch <room>\n")
// L154: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L155: Returns immediately from the current function without additional values.
		return
// L156: Closes the current block and returns control to the surrounding scope.
	}
// L157: Creates `targetRoom` as a new local binding so later lines can reuse this computed value.
	targetRoom := args
// L158: Evaluates `err := validateRoomName(targetRoom); err != nil` and enters the guarded branch only when that condition holds.
	if err := validateRoomName(targetRoom); err != nil {
// L159: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(err.Error() + "\n")
// L160: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L161: Returns immediately from the current function without additional values.
		return
// L162: Closes the current block and returns control to the surrounding scope.
	}
// L163: Evaluates `targetRoom == c.Room` and enters the guarded branch only when that condition holds.
	if targetRoom == c.Room {
// L164: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("You are already in room '" + targetRoom + "'.\n")
// L165: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L166: Returns immediately from the current function without additional values.
		return
// L167: Closes the current block and returns control to the surrounding scope.
	}
// L168: Evaluates `!s.checkRoomCapacity(targetRoom)` and enters the guarded branch only when that condition holds.
	if !s.checkRoomCapacity(targetRoom) {
// L169: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Room '" + targetRoom + "' is full.\n")
// L170: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L171: Returns immediately from the current function without additional values.
		return
// L172: Closes the current block and returns control to the surrounding scope.
	}

// L174: Calls `s.switchClientRoom` here for its side effects or returned value in the surrounding control flow.
	s.switchClientRoom(c, targetRoom)
// L175: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L176: Closes the current block and returns control to the surrounding scope.
}

// ---------- /create ----------

// L180: Declares the `cmdCreate` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdCreate(c *client.Client, args string) {
// L181: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L182: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Usage: /create <room>\n")
// L183: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L184: Returns immediately from the current function without additional values.
		return
// L185: Closes the current block and returns control to the surrounding scope.
	}
// L186: Creates `roomName` as a new local binding so later lines can reuse this computed value.
	roomName := args
// L187: Evaluates `err := validateRoomName(roomName); err != nil` and enters the guarded branch only when that condition holds.
	if err := validateRoomName(roomName); err != nil {
// L188: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(err.Error() + "\n")
// L189: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L190: Returns immediately from the current function without additional values.
		return
// L191: Closes the current block and returns control to the surrounding scope.
	}

	// Check if room already exists
// L194: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L195: Creates `_, exists` as a new local binding so later lines can reuse this computed value.
	_, exists := s.rooms[roomName]
// L196: Calls `s.mu.RUnlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RUnlock()
// L197: Evaluates `exists` and enters the guarded branch only when that condition holds.
	if exists {
// L198: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Room '" + roomName + "' already exists. Use /switch " + roomName + " to join it.\n")
// L199: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L200: Returns immediately from the current function without additional values.
		return
// L201: Closes the current block and returns control to the surrounding scope.
	}

// L203: Calls `s.switchClientRoom` here for its side effects or returned value in the surrounding control flow.
	s.switchClientRoom(c, roomName)
// L204: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L205: Closes the current block and returns control to the surrounding scope.
}

// switchClientRoom moves a client from their current room to a new room,
// handling leave/join broadcasts and history delivery.
// L209: Declares the `switchClientRoom` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) switchClientRoom(c *client.Client, newRoom string) {
// L210: Creates `oldRoom` as a new local binding so later lines can reuse this computed value.
	oldRoom := c.Room

	// Broadcast leave to old room
// L213: Creates `leaveMsg` as a new local binding so later lines can reuse this computed value.
	leaveMsg := models.Message{
// L214: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L215: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    c.Username,
// L216: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgLeave,
// L217: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     "switched",
// L218: Closes the current block and returns control to the surrounding scope.
	}
// L219: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(oldRoom, leaveMsg)
// L220: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(oldRoom, models.FormatLeave(c.Username)+"\n", c.Username)

	// Move to new room
// L223: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L224: Calls `s.JoinRoom` here for its side effects or returned value in the surrounding control flow.
	s.JoinRoom(c, newRoom)
// L225: Calls `s.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Unlock()

	// Admit from old room's queue and clean up
// L228: Calls `s.admitFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
	s.admitFromRoomQueue(oldRoom)
// L229: Calls `s.deleteRoomIfEmpty` here for its side effects or returned value in the surrounding control flow.
	s.deleteRoomIfEmpty(oldRoom)

	// Deliver new room's history
// L232: Starts a loop controlled by `_, msg := range s.GetRoomHistory(newRoom)`, repeating until the loop condition or range is exhausted.
	for _, msg := range s.GetRoomHistory(newRoom) {
// L233: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(msg.Display() + "\n")
// L234: Closes the current block and returns control to the surrounding scope.
	}

	// Broadcast join to new room
// L237: Creates `joinMsg` as a new local binding so later lines can reuse this computed value.
	joinMsg := models.Message{
// L238: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L239: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    c.Username,
// L240: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgJoin,
// L241: Closes the current block and returns control to the surrounding scope.
	}
// L242: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(newRoom, joinMsg)
// L243: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(newRoom, models.FormatJoin(c.Username)+"\n", c.Username)

// L245: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send("Switched to room '" + newRoom + "'.\n")
// L246: Closes the current block and returns control to the surrounding scope.
}

// ---------- /help (role-aware) ----------

// L250: Declares the `cmdHelp` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdHelp(c *client.Client) {
// L251: Creates `priv` from the result of `cmd.GetPrivilegeLevel`, capturing fresh state for the rest of this scope.
	priv := cmd.GetPrivilegeLevel(c.IsAdmin(), false)
// L252: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send("Available commands:\n")
// L253: Starts a loop controlled by `_, name := range cmd.CommandOrder`, repeating until the loop condition or range is exhausted.
	for _, name := range cmd.CommandOrder {
// L254: Creates `def` as a new local binding so later lines can reuse this computed value.
		def := cmd.Commands[name]
// L255: Evaluates `def.MinPriv <= priv` and enters the guarded branch only when that condition holds.
		if def.MinPriv <= priv {
// L256: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send(fmt.Sprintf("  %-30s %s\n", def.Usage, def.Description))
// L257: Closes the current block and returns control to the surrounding scope.
		}
// L258: Closes the current block and returns control to the surrounding scope.
	}
// L259: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L260: Closes the current block and returns control to the surrounding scope.
}

// ---------- /name ----------

// L264: Declares the `cmdName` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdName(c *client.Client, args string) {
// L265: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L266: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Usage: /name <newname>\n")
// L267: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L268: Returns immediately from the current function without additional values.
		return
// L269: Closes the current block and returns control to the surrounding scope.
	}
// L270: Creates `newName` as a new local binding so later lines can reuse this computed value.
	newName := args
// L271: Evaluates `err := validateName(newName); err != nil` and enters the guarded branch only when that condition holds.
	if err := validateName(newName); err != nil {
// L272: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(err.Error() + "\n")
// L273: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L274: Returns immediately from the current function without additional values.
		return
// L275: Closes the current block and returns control to the surrounding scope.
	}
// L276: Evaluates `s.IsReservedName(newName)` and enters the guarded branch only when that condition holds.
	if s.IsReservedName(newName) {
// L277: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Name '" + newName + "' is reserved.\n")
// L278: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L279: Returns immediately from the current function without additional values.
		return
// L280: Closes the current block and returns control to the surrounding scope.
	}
// L281: Evaluates `newName == c.Username` and enters the guarded branch only when that condition holds.
	if newName == c.Username {
// L282: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("You already have that name.\n")
// L283: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L284: Returns immediately from the current function without additional values.
		return
// L285: Closes the current block and returns control to the surrounding scope.
	}

// L287: Creates `oldName` as a new local binding so later lines can reuse this computed value.
	oldName := c.Username
// L288: Evaluates `!s.RenameClient(c, oldName, newName)` and enters the guarded branch only when that condition holds.
	if !s.RenameClient(c, oldName, newName) {
// L289: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Name is already taken.\n")
// L290: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L291: Returns immediately from the current function without additional values.
		return
// L292: Closes the current block and returns control to the surrounding scope.
	}

	// Update admins.json if this client is an admin
// L295: Evaluates `c.IsAdmin()` and enters the guarded branch only when that condition holds.
	if c.IsAdmin() {
// L296: Calls `s.RenameAdmin` here for its side effects or returned value in the surrounding control flow.
		s.RenameAdmin(oldName, newName)
// L297: Closes the current block and returns control to the surrounding scope.
	}

// L299: Creates `nameMsg` as a new local binding so later lines can reuse this computed value.
	nameMsg := models.Message{
// L300: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L301: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    newName,
// L302: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgNameChange,
// L303: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     oldName,
// L304: Closes the current block and returns control to the surrounding scope.
	}
// L305: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(c.Room, nameMsg)
// L306: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(c.Room, models.FormatNameChange(oldName, newName)+"\n", "")
// L307: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L308: Closes the current block and returns control to the surrounding scope.
}

// ---------- /whisper (cross-room) ----------

// L312: Declares the `cmdWhisper` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdWhisper(c *client.Client, args string) {
// L313: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L314: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing recipient. Usage: /whisper <name> <message>\n")
// L315: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L316: Returns immediately from the current function without additional values.
		return
// L317: Closes the current block and returns control to the surrounding scope.
	}
// L318: Creates `idx` from the result of `strings.IndexByte`, capturing fresh state for the rest of this scope.
	idx := strings.IndexByte(args, ' ')
// L319: Evaluates `idx < 0` and enters the guarded branch only when that condition holds.
	if idx < 0 {
// L320: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing message. Usage: /whisper <name> <message>\n")
// L321: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L322: Returns immediately from the current function without additional values.
		return
// L323: Closes the current block and returns control to the surrounding scope.
	}
// L324: Creates `recipient` as a new local binding so later lines can reuse this computed value.
	recipient := args[:idx]
// L325: Creates `message` from the result of `strings.TrimSpace`, capturing fresh state for the rest of this scope.
	message := strings.TrimSpace(args[idx+1:])
// L326: Evaluates `len(strings.TrimSpace(message)) == 0` and enters the guarded branch only when that condition holds.
	if len(strings.TrimSpace(message)) == 0 {
// L327: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing message. Usage: /whisper <name> <message>\n")
// L328: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L329: Returns immediately from the current function without additional values.
		return
// L330: Closes the current block and returns control to the surrounding scope.
	}
// L331: Evaluates `len(message) > 2048` and enters the guarded branch only when that condition holds.
	if len(message) > 2048 {
// L332: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Message too long (max 2048 characters).\n")
// L333: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L334: Returns immediately from the current function without additional values.
		return
// L335: Closes the current block and returns control to the surrounding scope.
	}
// L336: Evaluates `recipient == c.Username` and enters the guarded branch only when that condition holds.
	if recipient == c.Username {
// L337: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Cannot whisper to yourself.\n")
// L338: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L339: Returns immediately from the current function without additional values.
		return
// L340: Closes the current block and returns control to the surrounding scope.
	}

// L342: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(recipient)
// L343: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L344: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("User '" + recipient + "' not found. Use /list to see connected users.\n")
// L345: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L346: Returns immediately from the current function without additional values.
		return
// L347: Closes the current block and returns control to the surrounding scope.
	}

// L349: Creates `now` from the result of `time.Now`, capturing fresh state for the rest of this scope.
	now := time.Now()
// L350: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send(models.FormatWhisperReceive(now, c.Username, message) + "\n")
// L351: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
	c.Send(models.FormatWhisperSend(now, recipient, message) + "\n")
// L352: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L353: Closes the current block and returns control to the surrounding scope.
}

// ---------- /kick (same-room only) ----------

// L357: Declares the `cmdKick` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdKick(c *client.Client, args string) {
// L358: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L359: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing target. Usage: /kick <name>\n")
// L360: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L361: Returns immediately from the current function without additional values.
		return
// L362: Closes the current block and returns control to the surrounding scope.
	}
// L363: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L364: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L365: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
// L366: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L367: Returns immediately from the current function without additional values.
		return
// L368: Closes the current block and returns control to the surrounding scope.
	}
// L369: Evaluates `target.Room != c.Room` and enters the guarded branch only when that condition holds.
	if target.Room != c.Room {
// L370: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("User '" + args + "' is not in your room.\n")
// L371: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L372: Returns immediately from the current function without additional values.
		return
// L373: Closes the current block and returns control to the surrounding scope.
	}

// L375: Creates `targetIP` as a new local binding so later lines can reuse this computed value.
	targetIP := target.IP
// L376: Creates `targetRoom` as a new local binding so later lines can reuse this computed value.
	targetRoom := target.Room
// L377: Calls `target.ForceDisconnectReason` here for its side effects or returned value in the surrounding control flow.
	target.ForceDisconnectReason("kicked")
// L378: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
	s.RemoveClient(args)

// L380: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L381: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L382: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L383: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "kicked",
// L384: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L385: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     c.Username,
// L386: Closes the current block and returns control to the surrounding scope.
	}
// L387: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(targetRoom, modMsg)
// L388: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(targetRoom, models.FormatModeration(args, "kicked", c.Username)+"\n", "")
// L389: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send("You have been kicked by " + c.Username + ".\n")
// L390: Calls `target.Close` here for its side effects or returned value in the surrounding control flow.
	target.Close()
// L391: Calls `s.AddKickCooldown` here for its side effects or returned value in the surrounding control flow.
	s.AddKickCooldown(targetIP)
// L392: Calls `s.admitFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
	s.admitFromRoomQueue(targetRoom)
// L393: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L394: Closes the current block and returns control to the surrounding scope.
}

// ---------- /ban (global) ----------

// L398: Declares the `cmdBan` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdBan(c *client.Client, args string) {
// L399: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L400: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing target. Usage: /ban <name>\n")
// L401: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L402: Returns immediately from the current function without additional values.
		return
// L403: Closes the current block and returns control to the surrounding scope.
	}
// L404: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L405: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L406: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
// L407: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L408: Returns immediately from the current function without additional values.
		return
// L409: Closes the current block and returns control to the surrounding scope.
	}

// L411: Creates `targetIP` as a new local binding so later lines can reuse this computed value.
	targetIP := target.IP
// L412: Creates `targetRoom` as a new local binding so later lines can reuse this computed value.
	targetRoom := target.Room
// L413: Creates `bannedHost` from the result of `extractHost`, capturing fresh state for the rest of this scope.
	bannedHost := extractHost(targetIP)
// L414: Calls `target.ForceDisconnectReason` here for its side effects or returned value in the surrounding control flow.
	target.ForceDisconnectReason("banned")
// L415: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
	s.RemoveClient(args)

// L417: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L418: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L419: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L420: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "banned",
// L421: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L422: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     c.Username,
// L423: Closes the current block and returns control to the surrounding scope.
	}
// L424: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(targetRoom, modMsg)
// L425: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(targetRoom, models.FormatModeration(args, "banned", c.Username)+"\n", "")
// L426: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send("You have been banned by " + c.Username + ".\n")
// L427: Calls `target.Close` here for its side effects or returned value in the surrounding control flow.
	target.Close()
// L428: Calls `s.AddBanIP` here for its side effects or returned value in the surrounding control flow.
	s.AddBanIP(targetIP)

	// Disconnect all other active clients sharing the banned IP across all rooms.
// L431: Creates `roomsOpened` as a new local binding so later lines can reuse this computed value.
	roomsOpened := map[string]int{targetRoom: 1}
// L432: Creates `collateral` from the result of `s.GetClientsByIP`, capturing fresh state for the rest of this scope.
	collateral := s.GetClientsByIP(bannedHost, c.Username)
// L433: Starts a loop controlled by `_, cc := range collateral`, repeating until the loop condition or range is exhausted.
	for _, cc := range collateral {
// L434: Calls `cc.ForceDisconnectReason` here for its side effects or returned value in the surrounding control flow.
		cc.ForceDisconnectReason("banned")
// L435: Creates `ccName` as a new local binding so later lines can reuse this computed value.
		ccName := cc.Username
// L436: Creates `ccRoom` as a new local binding so later lines can reuse this computed value.
		ccRoom := cc.Room
// L437: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
		s.RemoveClient(ccName)
// L438: Creates `collateralMsg` as a new local binding so later lines can reuse this computed value.
		collateralMsg := models.Message{
// L439: Keeps this element in the surrounding multiline literal or call expression.
			Timestamp: time.Now(),
// L440: Keeps this element in the surrounding multiline literal or call expression.
			Sender:    ccName,
// L441: Keeps this element in the surrounding multiline literal or call expression.
			Content:   "banned",
// L442: Keeps this element in the surrounding multiline literal or call expression.
			Type:      models.MsgModeration,
// L443: Keeps this element in the surrounding multiline literal or call expression.
			Extra:     c.Username,
// L444: Closes the current block and returns control to the surrounding scope.
		}
// L445: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
		s.recordRoomEvent(ccRoom, collateralMsg)
// L446: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
		s.BroadcastRoom(ccRoom, models.FormatModeration(ccName, "banned", c.Username)+"\n", "")
// L447: Calls `cc.Send` here for its side effects or returned value in the surrounding control flow.
		cc.Send("You have been banned by " + c.Username + ".\n")
// L448: Calls `cc.Close` here for its side effects or returned value in the surrounding control flow.
		cc.Close()
// L449: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
		roomsOpened[ccRoom]++
// L450: Closes the current block and returns control to the surrounding scope.
	}

// L452: Creates `queuedRemoved` from the result of `s.RemoveFromQueueByIP`, capturing fresh state for the rest of this scope.
	queuedRemoved := s.RemoveFromQueueByIP(targetIP)
// L453: Starts a loop controlled by `_, qc := range queuedRemoved`, repeating until the loop condition or range is exhausted.
	for _, qc := range queuedRemoved {
// L454: Calls `qc.Send` here for its side effects or returned value in the surrounding control flow.
		qc.Send("You have been banned by " + c.Username + ".\n")
// L455: Calls `qc.Close` here for its side effects or returned value in the surrounding control flow.
		qc.Close()
// L456: Closes the current block and returns control to the surrounding scope.
	}

// L458: Starts a loop controlled by `rn, count := range roomsOpened`, repeating until the loop condition or range is exhausted.
	for rn, count := range roomsOpened {
// L459: Starts a loop controlled by `i := 0; i < count; i++`, repeating until the loop condition or range is exhausted.
		for i := 0; i < count; i++ {
// L460: Calls `s.admitFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
			s.admitFromRoomQueue(rn)
// L461: Closes the current block and returns control to the surrounding scope.
		}
// L462: Closes the current block and returns control to the surrounding scope.
	}
// L463: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L464: Closes the current block and returns control to the surrounding scope.
}

// ---------- /mute (global, broadcast to all rooms) ----------

// L468: Declares the `cmdMute` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdMute(c *client.Client, args string) {
// L469: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L470: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing target. Usage: /mute <name>\n")
// L471: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L472: Returns immediately from the current function without additional values.
		return
// L473: Closes the current block and returns control to the surrounding scope.
	}
// L474: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L475: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L476: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
// L477: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L478: Returns immediately from the current function without additional values.
		return
// L479: Closes the current block and returns control to the surrounding scope.
	}
// L480: Evaluates `target.IsMuted()` and enters the guarded branch only when that condition holds.
	if target.IsMuted() {
// L481: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(args + " is already muted.\n")
// L482: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L483: Returns immediately from the current function without additional values.
		return
// L484: Closes the current block and returns control to the surrounding scope.
	}

// L486: Calls `target.SetMuted` here for its side effects or returned value in the surrounding control flow.
	target.SetMuted(true)
// L487: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L488: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L489: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L490: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "muted",
// L491: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L492: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     c.Username,
// L493: Closes the current block and returns control to the surrounding scope.
	}
// L494: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(target.Room, modMsg)
// L495: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatModeration(args, "muted", c.Username) + "\n")
// L496: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L497: Closes the current block and returns control to the surrounding scope.
}

// ---------- /unmute ----------

// L501: Declares the `cmdUnmute` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdUnmute(c *client.Client, args string) {
// L502: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L503: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing target. Usage: /unmute <name>\n")
// L504: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L505: Returns immediately from the current function without additional values.
		return
// L506: Closes the current block and returns control to the surrounding scope.
	}
// L507: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L508: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L509: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
// L510: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L511: Returns immediately from the current function without additional values.
		return
// L512: Closes the current block and returns control to the surrounding scope.
	}
// L513: Evaluates `!target.IsMuted()` and enters the guarded branch only when that condition holds.
	if !target.IsMuted() {
// L514: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(args + " is not muted.\n")
// L515: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L516: Returns immediately from the current function without additional values.
		return
// L517: Closes the current block and returns control to the surrounding scope.
	}

// L519: Calls `target.SetMuted` here for its side effects or returned value in the surrounding control flow.
	target.SetMuted(false)
// L520: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L521: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L522: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L523: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "unmuted",
// L524: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L525: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     c.Username,
// L526: Closes the current block and returns control to the surrounding scope.
	}
// L527: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(target.Room, modMsg)
// L528: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatModeration(args, "unmuted", c.Username) + "\n")
// L529: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L530: Closes the current block and returns control to the surrounding scope.
}

// ---------- /announce (server-wide) ----------

// L534: Declares the `cmdAnnounce` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdAnnounce(c *client.Client, args string) {
// L535: Evaluates `len(strings.TrimSpace(args)) == 0` and enters the guarded branch only when that condition holds.
	if len(strings.TrimSpace(args)) == 0 {
// L536: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Usage: /announce <message>\n")
// L537: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L538: Returns immediately from the current function without additional values.
		return
// L539: Closes the current block and returns control to the surrounding scope.
	}
	// Log to all rooms
// L541: Starts a loop controlled by `_, rn := range s.GetRoomNames()`, repeating until the loop condition or range is exhausted.
	for _, rn := range s.GetRoomNames() {
// L542: Creates `announceMsg` as a new local binding so later lines can reuse this computed value.
		announceMsg := models.Message{
// L543: Keeps this element in the surrounding multiline literal or call expression.
			Timestamp: time.Now(),
// L544: Keeps this element in the surrounding multiline literal or call expression.
			Content:   args,
// L545: Keeps this element in the surrounding multiline literal or call expression.
			Type:      models.MsgAnnouncement,
// L546: Keeps this element in the surrounding multiline literal or call expression.
			Extra:     c.Username,
// L547: Closes the current block and returns control to the surrounding scope.
		}
// L548: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
		s.recordRoomEvent(rn, announceMsg)
// L549: Closes the current block and returns control to the surrounding scope.
	}
// L550: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatAnnouncement(args) + "\n")
// L551: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L552: Closes the current block and returns control to the surrounding scope.
}

// ---------- /promote ----------

// L556: Declares the `cmdPromote` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdPromote(c *client.Client, args string) {
// L557: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L558: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing target. Usage: /promote <name>\n")
// L559: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L560: Returns immediately from the current function without additional values.
		return
// L561: Closes the current block and returns control to the surrounding scope.
	}
// L562: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L563: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L564: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
// L565: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L566: Returns immediately from the current function without additional values.
		return
// L567: Closes the current block and returns control to the surrounding scope.
	}
// L568: Evaluates `target.IsAdmin()` and enters the guarded branch only when that condition holds.
	if target.IsAdmin() {
// L569: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(args + " is already an admin.\n")
// L570: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L571: Returns immediately from the current function without additional values.
		return
// L572: Closes the current block and returns control to the surrounding scope.
	}
// L573: Calls `target.SetAdmin` here for its side effects or returned value in the surrounding control flow.
	target.SetAdmin(true)
// L574: Calls `s.AddAdmin` here for its side effects or returned value in the surrounding control flow.
	s.AddAdmin(args)
// L575: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send("You have been promoted to admin.\n")

// L577: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L578: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L579: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L580: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "promoted",
// L581: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L582: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     c.Username,
// L583: Closes the current block and returns control to the surrounding scope.
	}
// L584: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(target.Room, modMsg)
// L585: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatModeration(args, "promoted", c.Username) + "\n")
// L586: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L587: Closes the current block and returns control to the surrounding scope.
}

// ---------- /demote ----------

// L591: Declares the `cmdDemote` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) cmdDemote(c *client.Client, args string) {
// L592: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L593: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("Missing target. Usage: /demote <name>\n")
// L594: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L595: Returns immediately from the current function without additional values.
		return
// L596: Closes the current block and returns control to the surrounding scope.
	}
// L597: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L598: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L599: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send("User '" + args + "' not found. Use /list to see connected users.\n")
// L600: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L601: Returns immediately from the current function without additional values.
		return
// L602: Closes the current block and returns control to the surrounding scope.
	}
// L603: Evaluates `!target.IsAdmin()` and enters the guarded branch only when that condition holds.
	if !target.IsAdmin() {
// L604: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
		c.Send(args + " is not an admin.\n")
// L605: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
		c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L606: Returns immediately from the current function without additional values.
		return
// L607: Closes the current block and returns control to the surrounding scope.
	}
// L608: Calls `target.SetAdmin` here for its side effects or returned value in the surrounding control flow.
	target.SetAdmin(false)
// L609: Calls `s.RemoveAdmin` here for its side effects or returned value in the surrounding control flow.
	s.RemoveAdmin(args)
// L610: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send("Your admin privileges have been revoked.\n")

// L612: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L613: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L614: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L615: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "demoted",
// L616: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L617: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     c.Username,
// L618: Closes the current block and returns control to the surrounding scope.
	}
// L619: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(target.Room, modMsg)
// L620: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatModeration(args, "demoted", c.Username) + "\n")
// L621: Calls `c.SendPrompt` here for its side effects or returned value in the surrounding control flow.
	c.SendPrompt(models.FormatPrompt(time.Now(), c.Username))
// L622: Closes the current block and returns control to the surrounding scope.
}
```

## `server/admins.go`

Persists and reloads promoted admins so operator-granted privileges survive server restarts.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `encoding/json` so this file can call functionality from its `json` package.
	"encoding/json"
// L5: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L6: Imports `os` so this file can call functionality from its `os` package.
	"os"
// L7: Imports `path/filepath` so this file can call functionality from its `filepath` package.
	"path/filepath"
// L8: Closes the import block after listing all package dependencies.
)

// ---------- admin persistence ----------

// LoadAdmins reads admins.json from disk. Missing or corrupt file is handled
// gracefully: the server starts with no saved admins and a console warning.
// L14: Declares the `LoadAdmins` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) LoadAdmins() {
// L15: Creates `path` as a new local binding so later lines can reuse this computed value.
	path := s.adminsFile
// L16: Evaluates `path == ""` and enters the guarded branch only when that condition holds.
	if path == "" {
// L17: Returns immediately from the current function without additional values.
		return
// L18: Closes the current block and returns control to the surrounding scope.
	}

// L20: Creates `f, err` from the result of `os.Open`, capturing fresh state for the rest of this scope.
	f, err := os.Open(path)
// L21: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
	if err != nil {
// L22: Evaluates `!os.IsNotExist(err)` and enters the guarded branch only when that condition holds.
		if !os.IsNotExist(err) {
// L23: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
			fmt.Fprintf(os.Stderr, "Warning: could not open admins.json: %v\n", err)
// L24: Closes the current block and returns control to the surrounding scope.
		}
// L25: Returns immediately from the current function without additional values.
		return
// L26: Closes the current block and returns control to the surrounding scope.
	}
// L27: Schedules this cleanup or follow-up call to run when the current function returns.
	defer f.Close()

// L29: Declares `names` in the current scope so later lines can fill or mutate it as needed.
	var names []string
// L30: Evaluates `err := json.NewDecoder(f).Decode(&names); err != nil` and enters the guarded branch only when that condition holds.
	if err := json.NewDecoder(f).Decode(&names); err != nil {
// L31: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Warning: corrupt admins.json, starting with no saved admins: %v\n", err)
// L32: Returns immediately from the current function without additional values.
		return
// L33: Closes the current block and returns control to the surrounding scope.
	}

// L35: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L36: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L37: Starts a loop controlled by `_, name := range names`, repeating until the loop condition or range is exhausted.
	for _, name := range names {
// L38: Updates `s.admins[name]` so subsequent logic sees the new state.
		s.admins[name] = true
// L39: Closes the current block and returns control to the surrounding scope.
	}
// L40: Closes the current block and returns control to the surrounding scope.
}

// SaveAdmins writes the current admin list to admins.json atomically.
// Writes to a temp file then renames for crash safety.
// L44: Declares the `SaveAdmins` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) SaveAdmins() {
// L45: Creates `path` as a new local binding so later lines can reuse this computed value.
	path := s.adminsFile
// L46: Evaluates `path == ""` and enters the guarded branch only when that condition holds.
	if path == "" {
// L47: Returns immediately from the current function without additional values.
		return
// L48: Closes the current block and returns control to the surrounding scope.
	}

// L50: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L51: Creates `names` from the result of `make`, capturing fresh state for the rest of this scope.
	names := make([]string, 0, len(s.admins))
// L52: Starts a loop controlled by `name := range s.admins`, repeating until the loop condition or range is exhausted.
	for name := range s.admins {
// L53: Updates `names` with the result of `append`, replacing its previous value.
		names = append(names, name)
// L54: Closes the current block and returns control to the surrounding scope.
	}
// L55: Calls `s.mu.RUnlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RUnlock()

	// Sort for deterministic output (simple insertion sort, no sort package)
// L58: Starts a loop controlled by `i := 1; i < len(names); i++`, repeating until the loop condition or range is exhausted.
	for i := 1; i < len(names); i++ {
// L59: Creates `key` as a new local binding so later lines can reuse this computed value.
		key := names[i]
// L60: Creates `j` as a new local binding so later lines can reuse this computed value.
		j := i - 1
// L61: Starts a loop controlled by `j >= 0 && names[j] > key`, repeating until the loop condition or range is exhausted.
		for j >= 0 && names[j] > key {
// L62: Updates `names[j+1]` so subsequent logic sees the new state.
			names[j+1] = names[j]
// L63: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
			j--
// L64: Closes the current block and returns control to the surrounding scope.
		}
// L65: Updates `names[j+1]` so subsequent logic sees the new state.
		names[j+1] = key
// L66: Closes the current block and returns control to the surrounding scope.
	}

// L68: Creates `data, err` from the result of `json.MarshalIndent`, capturing fresh state for the rest of this scope.
	data, err := json.MarshalIndent(names, "", "  ")
// L69: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
	if err != nil {
// L70: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Warning: could not marshal admins.json: %v\n", err)
// L71: Returns immediately from the current function without additional values.
		return
// L72: Closes the current block and returns control to the surrounding scope.
	}

// L74: Creates `dir` from the result of `filepath.Dir`, capturing fresh state for the rest of this scope.
	dir := filepath.Dir(path)
// L75: Creates `tmpFile` from the result of `filepath.Join`, capturing fresh state for the rest of this scope.
	tmpFile := filepath.Join(dir, ".admins.json.tmp")
// L76: Evaluates `err := os.WriteFile(tmpFile, data, 0600); err != nil` and enters the guarded branch only when that condition holds.
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
// L77: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Warning: could not write admins.json: %v\n", err)
// L78: Returns immediately from the current function without additional values.
		return
// L79: Closes the current block and returns control to the surrounding scope.
	}
// L80: Evaluates `err := os.Rename(tmpFile, path); err != nil` and enters the guarded branch only when that condition holds.
	if err := os.Rename(tmpFile, path); err != nil {
// L81: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Warning: could not save admins.json: %v\n", err)
// L82: Closes the current block and returns control to the surrounding scope.
	}
// L83: Closes the current block and returns control to the surrounding scope.
}

// IsKnownAdmin checks if a username is in the persisted admin list.
// L86: Declares the `IsKnownAdmin` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) IsKnownAdmin(name string) bool {
// L87: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L88: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L89: Returns the listed values to the caller, ending the current function at this point.
	return s.admins[name]
// L90: Closes the current block and returns control to the surrounding scope.
}

// AddAdmin adds a username to the persisted admin list and saves.
// L93: Declares the `AddAdmin` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) AddAdmin(name string) {
// L94: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L95: Updates `s.admins[name]` so subsequent logic sees the new state.
	s.admins[name] = true
// L96: Calls `s.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Unlock()
// L97: Calls `s.SaveAdmins` here for its side effects or returned value in the surrounding control flow.
	s.SaveAdmins()
// L98: Closes the current block and returns control to the surrounding scope.
}

// RemoveAdmin removes a username from the persisted admin list and saves.
// L101: Declares the `RemoveAdmin` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) RemoveAdmin(name string) {
// L102: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L103: Calls `delete` here for its side effects or returned value in the surrounding control flow.
	delete(s.admins, name)
// L104: Calls `s.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Unlock()
// L105: Calls `s.SaveAdmins` here for its side effects or returned value in the surrounding control flow.
	s.SaveAdmins()
// L106: Closes the current block and returns control to the surrounding scope.
}

// RenameAdmin updates the persisted admin list when an admin changes their name.
// L109: Declares the `RenameAdmin` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) RenameAdmin(oldName, newName string) {
// L110: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L111: Evaluates `s.admins[oldName]` and enters the guarded branch only when that condition holds.
	if s.admins[oldName] {
// L112: Calls `delete` here for its side effects or returned value in the surrounding control flow.
		delete(s.admins, oldName)
// L113: Updates `s.admins[newName]` so subsequent logic sees the new state.
		s.admins[newName] = true
// L114: Closes the current block and returns control to the surrounding scope.
	}
// L115: Calls `s.mu.Unlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Unlock()
// L116: Calls `s.SaveAdmins` here for its side effects or returned value in the surrounding control flow.
	s.SaveAdmins()
// L117: Closes the current block and returns control to the surrounding scope.
}
```

## `server/history.go`

Resets room history at day boundaries and rebuilds in-memory history by replaying the current log file.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `bufio` so this file can call functionality from its `bufio` package.
	"bufio"
// L5: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L6: Imports `net-cat/logger` so this file can call functionality from its `logger` package.
	"net-cat/logger"
// L7: Imports `net-cat/models` so this file can call functionality from its `models` package.
	"net-cat/models"
// L8: Imports `os` so this file can call functionality from its `os` package.
	"os"
// L9: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L10: Closes the import block after listing all package dependencies.
)

// ---------- history ----------

// ClearHistory removes all in-memory history entries across all rooms.
// Called at midnight to reset history for the new calendar day.
// L16: Declares the `ClearHistory` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) ClearHistory() {
// L17: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L18: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L19: Starts a loop controlled by `_, r := range s.rooms`, repeating until the loop condition or range is exhausted.
	for _, r := range s.rooms {
// L20: Updates `r.history` so subsequent logic sees the new state.
		r.history = r.history[:0]
// L21: Closes the current block and returns control to the surrounding scope.
	}
// L22: Closes the current block and returns control to the surrounding scope.
}

// AddHistory appends a message to the appropriate room's history.
// If msg.Room is empty, uses DefaultRoom.
// L26: Declares the `AddHistory` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) AddHistory(msg models.Message) {
// L27: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L28: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L29: Creates `roomName` as a new local binding so later lines can reuse this computed value.
	roomName := msg.Room
// L30: Evaluates `roomName == ""` and enters the guarded branch only when that condition holds.
	if roomName == "" {
// L31: Updates `roomName` so subsequent logic sees the new state.
		roomName = s.DefaultRoom
// L32: Closes the current block and returns control to the surrounding scope.
	}
// L33: Creates `r` from the result of `s.getOrCreateRoom`, capturing fresh state for the rest of this scope.
	r := s.getOrCreateRoom(roomName)
// L34: Updates `r.history` with the result of `append`, replacing its previous value.
	r.history = append(r.history, msg)
// L35: Closes the current block and returns control to the surrounding scope.
}

// GetHistory returns a combined copy of all room histories for backward compatibility.
// L38: Declares the `GetHistory` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) GetHistory() []models.Message {
// L39: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L40: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.RUnlock()
// L41: Declares `all` in the current scope so later lines can fill or mutate it as needed.
	var all []models.Message
// L42: Starts a loop controlled by `_, r := range s.rooms`, repeating until the loop condition or range is exhausted.
	for _, r := range s.rooms {
// L43: Updates `all` with the result of `append`, replacing its previous value.
		all = append(all, r.history...)
// L44: Closes the current block and returns control to the surrounding scope.
	}
// L45: Returns the listed values to the caller, ending the current function at this point.
	return all
// L46: Closes the current block and returns control to the surrounding scope.
}

// RecoverHistory loads today's log file and reconstructs the in-memory history.
// Only called on startup so that clients connecting after a restart see prior events.
// Server events are excluded (not user-visible). Corrupt lines are skipped with warnings.
// Messages are routed to the correct room based on the @room tag; old lines without
// a room tag are assigned to the DefaultRoom.
// L53: Declares the `RecoverHistory` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) RecoverHistory() {
// L54: Evaluates `s.Logger == nil` and enters the guarded branch only when that condition holds.
	if s.Logger == nil {
// L55: Returns immediately from the current function without additional values.
		return
// L56: Closes the current block and returns control to the surrounding scope.
	}

// L58: Creates `date` from the result of `logger.FormatDate`, capturing fresh state for the rest of this scope.
	date := logger.FormatDate(time.Now())
// L59: Creates `path` from the result of `s.Logger.FilePath`, capturing fresh state for the rest of this scope.
	path := s.Logger.FilePath(date)
// L60: Evaluates `path == ""` and enters the guarded branch only when that condition holds.
	if path == "" {
// L61: Returns immediately from the current function without additional values.
		return
// L62: Closes the current block and returns control to the surrounding scope.
	}

// L64: Creates `f, err` from the result of `os.Open`, capturing fresh state for the rest of this scope.
	f, err := os.Open(path)
// L65: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
	if err != nil {
// L66: Evaluates `os.IsNotExist(err)` and enters the guarded branch only when that condition holds.
		if os.IsNotExist(err) {
// L67: Returns immediately from the current function without additional values.
			return
// L68: Closes the current block and returns control to the surrounding scope.
		}
// L69: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Warning: could not open log file for recovery: %v\n", err)
// L70: Returns immediately from the current function without additional values.
		return
// L71: Closes the current block and returns control to the surrounding scope.
	}
// L72: Schedules this cleanup or follow-up call to run when the current function returns.
	defer f.Close()

// L74: Creates `scanner` from the result of `bufio.NewScanner`, capturing fresh state for the rest of this scope.
	scanner := bufio.NewScanner(f)
// L75: Calls `scanner.Buffer` here for its side effects or returned value in the surrounding control flow.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

// L77: Creates `corrupt` as a new local binding so later lines can reuse this computed value.
	corrupt := 0
// L78: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L79: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L80: Starts a loop controlled by `scanner.Scan()`, repeating until the loop condition or range is exhausted.
	for scanner.Scan() {
// L81: Creates `line` from the result of `scanner.Text`, capturing fresh state for the rest of this scope.
		line := scanner.Text()
// L82: Evaluates `len(line) == 0` and enters the guarded branch only when that condition holds.
		if len(line) == 0 {
// L83: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L84: Closes the current block and returns control to the surrounding scope.
		}
// L85: Creates `msg, err` from the result of `models.ParseLogLine`, capturing fresh state for the rest of this scope.
		msg, err := models.ParseLogLine(line)
// L86: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
		if err != nil {
// L87: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
			corrupt++
// L88: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
			fmt.Fprintf(os.Stderr, "Warning: skipping corrupt log line: %v\n", err)
// L89: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L90: Closes the current block and returns control to the surrounding scope.
		}
// L91: Evaluates `msg.Type == models.MsgServerEvent` and enters the guarded branch only when that condition holds.
		if msg.Type == models.MsgServerEvent {
// L92: Skips the rest of the current loop iteration and starts the next iteration immediately.
			continue
// L93: Closes the current block and returns control to the surrounding scope.
		}
		// Route to correct room; old logs without @room go to DefaultRoom
// L95: Creates `roomName` as a new local binding so later lines can reuse this computed value.
		roomName := msg.Room
// L96: Evaluates `roomName == ""` and enters the guarded branch only when that condition holds.
		if roomName == "" {
// L97: Updates `roomName` so subsequent logic sees the new state.
			roomName = s.DefaultRoom
// L98: Updates `msg.Room` so subsequent logic sees the new state.
			msg.Room = roomName
// L99: Closes the current block and returns control to the surrounding scope.
		}
// L100: Creates `r` from the result of `s.getOrCreateRoom`, capturing fresh state for the rest of this scope.
		r := s.getOrCreateRoom(roomName)
// L101: Updates `r.history` with the result of `append`, replacing its previous value.
		r.history = append(r.history, msg)
// L102: Closes the current block and returns control to the surrounding scope.
	}

// L104: Evaluates `err := scanner.Err(); err != nil` and enters the guarded branch only when that condition holds.
	if err := scanner.Err(); err != nil {
// L105: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Warning: error reading log file: %v\n", err)
// L106: Closes the current block and returns control to the surrounding scope.
	}

// L108: Evaluates `corrupt > 0` and enters the guarded branch only when that condition holds.
	if corrupt > 0 {
// L109: Calls `fmt.Fprintf` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprintf(os.Stderr, "Warning: %d corrupt line(s) skipped during history recovery\n", corrupt)
// L110: Closes the current block and returns control to the surrounding scope.
	}
// L111: Closes the current block and returns control to the surrounding scope.
}
```

## `server/moderation.go`

Defines host-based moderation helpers for extracting durable IP keys from network addresses.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `net` so this file can call functionality from its `net` package.
	"net"
// L5: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L6: Closes the import block after listing all package dependencies.
)

// ---------- IP-based moderation ----------

// extractHost extracts the host part from a "host:port" address string.
// L11: Declares the `extractHost` function, which starts a named unit of behavior other code can call.
func extractHost(addr string) string {
// L12: Creates `host, _, err` from the result of `net.SplitHostPort`, capturing fresh state for the rest of this scope.
	host, _, err := net.SplitHostPort(addr)
// L13: Evaluates `err != nil` and enters the guarded branch only when that condition holds.
	if err != nil {
// L14: Returns the listed values to the caller, ending the current function at this point.
		return addr // bare IP or pipe address
// L15: Closes the current block and returns control to the surrounding scope.
	}
// L16: Returns the listed values to the caller, ending the current function at this point.
	return host
// L17: Closes the current block and returns control to the surrounding scope.
}

// AddKickCooldown blocks an IP from reconnecting for 5 minutes.
// L20: Declares the `AddKickCooldown` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) AddKickCooldown(ip string) {
// L21: Creates `host` from the result of `extractHost`, capturing fresh state for the rest of this scope.
	host := extractHost(ip)
// L22: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L23: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L24: Updates `s.kickedIPs[host]` with the result of `time.Now`, replacing its previous value.
	s.kickedIPs[host] = time.Now().Add(5 * time.Minute)
// L25: Closes the current block and returns control to the surrounding scope.
}

// AddBanIP blocks an IP for the remainder of the server session.
// L28: Declares the `AddBanIP` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) AddBanIP(ip string) {
// L29: Creates `host` from the result of `extractHost`, capturing fresh state for the rest of this scope.
	host := extractHost(ip)
// L30: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L31: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L32: Updates `s.bannedIPs[host]` so subsequent logic sees the new state.
	s.bannedIPs[host] = true
// L33: Closes the current block and returns control to the surrounding scope.
}

// IsIPBlocked checks if an IP is blocked by kick cooldown or ban.
// Returns (blocked, rejection message). Cleans up expired kick cooldowns.
// L37: Declares the `IsIPBlocked` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) IsIPBlocked(ip string) (bool, string) {
// L38: Creates `host` from the result of `extractHost`, capturing fresh state for the rest of this scope.
	host := extractHost(ip)
// L39: Calls `s.mu.Lock` here for its side effects or returned value in the surrounding control flow.
	s.mu.Lock()
// L40: Schedules this cleanup or follow-up call to run when the current function returns.
	defer s.mu.Unlock()
// L41: Evaluates `s.bannedIPs[host]` and enters the guarded branch only when that condition holds.
	if s.bannedIPs[host] {
// L42: Returns the listed values to the caller, ending the current function at this point.
		return true, "You are banned from this server.\n"
// L43: Closes the current block and returns control to the surrounding scope.
	}
// L44: Evaluates `expiry, ok := s.kickedIPs[host]; ok` and enters the guarded branch only when that condition holds.
	if expiry, ok := s.kickedIPs[host]; ok {
// L45: Evaluates `time.Now().Before(expiry)` and enters the guarded branch only when that condition holds.
		if time.Now().Before(expiry) {
// L46: Returns the listed values to the caller, ending the current function at this point.
			return true, "You are temporarily blocked. Try again later.\n"
// L47: Closes the current block and returns control to the surrounding scope.
		}
// L48: Calls `delete` here for its side effects or returned value in the surrounding control flow.
		delete(s.kickedIPs, host) // expired
// L49: Closes the current block and returns control to the surrounding scope.
	}
// L50: Returns the listed values to the caller, ending the current function at this point.
	return false, ""
// L51: Closes the current block and returns control to the surrounding scope.
}
```

## `server/operator.go`

Implements the server-side operator terminal, including privileged command dispatch and administrative introspection.

```go
// L1: Declares `server` as the package for this directory so the compiler groups this file with the rest of that package.
package server

// L3: Starts the import block that declares external packages this file depends on.
import (
// L4: Imports `bufio` so this file can call functionality from its `bufio` package.
	"bufio"
// L5: Imports `fmt` so this file can call functionality from its `fmt` package.
	"fmt"
// L6: Imports `io` so this file can call functionality from its `io` package.
	"io"
// L7: Imports `net-cat/cmd` so this file can call functionality from its `cmd` package.
	"net-cat/cmd"
// L8: Imports `net-cat/models` so this file can call functionality from its `models` package.
	"net-cat/models"
// L9: Imports `strings` so this file can call functionality from its `strings` package.
	"strings"
// L10: Imports `time` so this file can call functionality from its `time` package.
	"time"
// L11: Closes the import block after listing all package dependencies.
)

// ---------- operator terminal ----------

// StartOperator reads commands from the given reader (typically os.Stdin)
// and dispatches them with full operator authority. Blocks until the reader
// is exhausted or the server shuts down.
// L18: Declares the `StartOperator` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) StartOperator(r io.Reader) {
// L19: Creates `scanner` from the result of `bufio.NewScanner`, capturing fresh state for the rest of this scope.
	scanner := bufio.NewScanner(r)
// L20: Calls `scanner.Buffer` here for its side effects or returned value in the surrounding control flow.
	scanner.Buffer(make([]byte, 4096), 1048576)
// L21: Starts a loop controlled by `scanner.Scan()`, repeating until the loop condition or range is exhausted.
	for scanner.Scan() {
// L22: Evaluates `s.IsShuttingDown()` and enters the guarded branch only when that condition holds.
		if s.IsShuttingDown() {
// L23: Returns immediately from the current function without additional values.
			return
// L24: Closes the current block and returns control to the surrounding scope.
		}
// L25: Creates `line` from the result of `scanner.Text`, capturing fresh state for the rest of this scope.
		line := scanner.Text()
// L26: Calls `s.OperatorDispatch` here for its side effects or returned value in the surrounding control flow.
		s.OperatorDispatch(line)
// L27: Closes the current block and returns control to the surrounding scope.
	}
// L28: Closes the current block and returns control to the surrounding scope.
}

// OperatorDispatch parses and executes a single operator terminal input line.
// L31: Declares the `OperatorDispatch` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) OperatorDispatch(input string) {
// L32: Updates `input` with the result of `strings.TrimSpace`, replacing its previous value.
	input = strings.TrimSpace(input)
// L33: Evaluates `input == ""` and enters the guarded branch only when that condition holds.
	if input == "" {
// L34: Returns immediately from the current function without additional values.
		return
// L35: Closes the current block and returns control to the surrounding scope.
	}

// L37: Creates `cmdName, args, isCmd` from the result of `cmd.ParseCommand`, capturing fresh state for the rest of this scope.
	cmdName, args, isCmd := cmd.ParseCommand(input)
// L38: Evaluates `!isCmd` and enters the guarded branch only when that condition holds.
	if !isCmd {
// L39: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Commands must start with /. Use /help to see available commands.\n")
// L40: Returns immediately from the current function without additional values.
		return
// L41: Closes the current block and returns control to the surrounding scope.
	}

// L43: Creates `def, exists` as a new local binding so later lines can reuse this computed value.
	def, exists := cmd.Commands[cmdName]
// L44: Evaluates `!exists` and enters the guarded branch only when that condition holds.
	if !exists {
// L45: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Unknown command: /" + cmdName + ". Use /help to see available commands.\n")
// L46: Returns immediately from the current function without additional values.
		return
// L47: Closes the current block and returns control to the surrounding scope.
	}

	// Operator has full privilege, but some commands are inapplicable
// L50: Starts a switch on `cmdName` so the following cases can branch on that value cleanly.
	switch cmdName {
// L51: Selects the `"quit"` branch inside the surrounding switch or select.
	case "quit":
// L52: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("The /quit command is not applicable to the server operator.\n")
// L53: Selects the `"name"` branch inside the surrounding switch or select.
	case "name":
// L54: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("The /name command is not applicable to the server operator.\n")
// L55: Selects the `"whisper"` branch inside the surrounding switch or select.
	case "whisper":
// L56: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("The /whisper command is not applicable to the server operator.\n")
// L57: Selects the `"switch"` branch inside the surrounding switch or select.
	case "switch":
// L58: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("The /switch command is not applicable to the server operator.\n")
// L59: Selects the `"create"` branch inside the surrounding switch or select.
	case "create":
// L60: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("The /create command is not applicable to the server operator.\n")
// L61: Selects the `"list"` branch inside the surrounding switch or select.
	case "list":
// L62: Calls `s.operatorCmdList` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdList()
// L63: Selects the `"help"` branch inside the surrounding switch or select.
	case "help":
// L64: Calls `s.operatorCmdHelp` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdHelp()
// L65: Selects the `"rooms"` branch inside the surrounding switch or select.
	case "rooms":
// L66: Calls `s.operatorCmdRooms` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdRooms()
// L67: Selects the `"kick"` branch inside the surrounding switch or select.
	case "kick":
// L68: Calls `s.operatorCmdKick` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdKick(args)
// L69: Selects the `"ban"` branch inside the surrounding switch or select.
	case "ban":
// L70: Calls `s.operatorCmdBan` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdBan(args)
// L71: Selects the `"mute"` branch inside the surrounding switch or select.
	case "mute":
// L72: Calls `s.operatorCmdMute` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdMute(args)
// L73: Selects the `"unmute"` branch inside the surrounding switch or select.
	case "unmute":
// L74: Calls `s.operatorCmdUnmute` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdUnmute(args)
// L75: Selects the `"announce"` branch inside the surrounding switch or select.
	case "announce":
// L76: Calls `s.operatorCmdAnnounce` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdAnnounce(args)
// L77: Selects the `"promote"` branch inside the surrounding switch or select.
	case "promote":
// L78: Calls `s.operatorCmdPromote` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdPromote(args)
// L79: Selects the `"demote"` branch inside the surrounding switch or select.
	case "demote":
// L80: Calls `s.operatorCmdDemote` here for its side effects or returned value in the surrounding control flow.
		s.operatorCmdDemote(args)
// L81: Defines the fallback branch used when no earlier case matches or no channel operation is ready.
	default:
// L82: Updates `_` so subsequent logic sees the new state.
		_ = def
// L83: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Unknown command: /" + cmdName + ".\n")
// L84: Closes the current block and returns control to the surrounding scope.
	}
// L85: Closes the current block and returns control to the surrounding scope.
}

// operatorSend writes a message to the operator's output (typically stdout).
// L88: Declares the `operatorSend` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorSend(msg string) {
// L89: Evaluates `s.OperatorOutput != nil` and enters the guarded branch only when that condition holds.
	if s.OperatorOutput != nil {
// L90: Calls `fmt.Fprint` here for its side effects or returned value in the surrounding control flow.
		fmt.Fprint(s.OperatorOutput, msg)
// L91: Closes the current block and returns control to the surrounding scope.
	}
// L92: Closes the current block and returns control to the surrounding scope.
}

// ---------- operator command implementations ----------

// L96: Declares the `operatorCmdList` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdList() {
// L97: Calls `s.mu.RLock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RLock()
// L98: Defines the `entry` struct, which groups related state that this package manages together.
	type entry struct {
// L99: Adds the `name` field to the struct so instances can hold that piece of state.
		name string
// L100: Adds the `idle` field to the struct so instances can hold that piece of state.
		idle time.Duration
// L101: Closes the struct definition after listing all of its fields.
	}

	// Collect sorted room names
// L104: Creates `roomNames` from the result of `make`, capturing fresh state for the rest of this scope.
	roomNames := make([]string, 0, len(s.rooms))
// L105: Starts a loop controlled by `rn := range s.rooms`, repeating until the loop condition or range is exhausted.
	for rn := range s.rooms {
// L106: Updates `roomNames` with the result of `append`, replacing its previous value.
		roomNames = append(roomNames, rn)
// L107: Closes the current block and returns control to the surrounding scope.
	}
	// insertion sort
// L109: Starts a loop controlled by `i := 1; i < len(roomNames); i++`, repeating until the loop condition or range is exhausted.
	for i := 1; i < len(roomNames); i++ {
// L110: Creates `key` as a new local binding so later lines can reuse this computed value.
		key := roomNames[i]
// L111: Creates `j` as a new local binding so later lines can reuse this computed value.
		j := i - 1
// L112: Starts a loop controlled by `j >= 0 && roomNames[j] > key`, repeating until the loop condition or range is exhausted.
		for j >= 0 && roomNames[j] > key {
// L113: Updates `roomNames[j+1]` so subsequent logic sees the new state.
			roomNames[j+1] = roomNames[j]
// L114: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
			j--
// L115: Closes the current block and returns control to the surrounding scope.
		}
// L116: Updates `roomNames[j+1]` so subsequent logic sees the new state.
		roomNames[j+1] = key
// L117: Closes the current block and returns control to the surrounding scope.
	}

// L119: Defines the `roomData` struct, which groups related state that this package manages together.
	type roomData struct {
// L120: Adds the `name` field to the struct so instances can hold that piece of state.
		name    string
// L121: Adds the `entries` field to the struct so instances can hold that piece of state.
		entries []entry
// L122: Adds the `queue` field to the struct so instances can hold that piece of state.
		queue   []string // IPs of queued users
// L123: Closes the struct definition after listing all of its fields.
	}
// L124: Declares `rooms` in the current scope so later lines can fill or mutate it as needed.
	var rooms []roomData
// L125: Starts a loop controlled by `_, rn := range roomNames`, repeating until the loop condition or range is exhausted.
	for _, rn := range roomNames {
// L126: Creates `r` as a new local binding so later lines can reuse this computed value.
		r := s.rooms[rn]
// L127: Creates `entries` from the result of `make`, capturing fresh state for the rest of this scope.
		entries := make([]entry, 0, len(r.clients))
// L128: Starts a loop controlled by `n, cl := range r.clients`, repeating until the loop condition or range is exhausted.
		for n, cl := range r.clients {
// L129: Updates `entries` with the result of `append`, replacing its previous value.
			entries = append(entries, entry{name: n, idle: time.Since(cl.GetLastActivity()).Truncate(time.Second)})
// L130: Closes the current block and returns control to the surrounding scope.
		}
		// sort entries
// L132: Starts a loop controlled by `i := 1; i < len(entries); i++`, repeating until the loop condition or range is exhausted.
		for i := 1; i < len(entries); i++ {
// L133: Creates `key` as a new local binding so later lines can reuse this computed value.
			key := entries[i]
// L134: Creates `j` as a new local binding so later lines can reuse this computed value.
			j := i - 1
// L135: Starts a loop controlled by `j >= 0 && entries[j].name > key.name`, repeating until the loop condition or range is exhausted.
			for j >= 0 && entries[j].name > key.name {
// L136: Updates `entries[j+1]` so subsequent logic sees the new state.
				entries[j+1] = entries[j]
// L137: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
				j--
// L138: Closes the current block and returns control to the surrounding scope.
			}
// L139: Updates `entries[j+1]` so subsequent logic sees the new state.
			entries[j+1] = key
// L140: Closes the current block and returns control to the surrounding scope.
		}
// L141: Declares `queueIPs` in the current scope so later lines can fill or mutate it as needed.
		var queueIPs []string
// L142: Starts a loop controlled by `_, e := range r.queue`, repeating until the loop condition or range is exhausted.
		for _, e := range r.queue {
// L143: Updates `queueIPs` with the result of `append`, replacing its previous value.
			queueIPs = append(queueIPs, extractHost(e.client.IP))
// L144: Closes the current block and returns control to the surrounding scope.
		}
// L145: Updates `rooms` with the result of `append`, replacing its previous value.
		rooms = append(rooms, roomData{name: rn, entries: entries, queue: queueIPs})
// L146: Closes the current block and returns control to the surrounding scope.
	}
// L147: Calls `s.mu.RUnlock` here for its side effects or returned value in the surrounding control flow.
	s.mu.RUnlock()

// L149: Starts a loop controlled by `_, rd := range rooms`, repeating until the loop condition or range is exhausted.
	for _, rd := range rooms {
// L150: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(fmt.Sprintf("Room %s (%d clients):\n", rd.name, len(rd.entries)))
// L151: Starts a loop controlled by `_, e := range rd.entries`, repeating until the loop condition or range is exhausted.
		for _, e := range rd.entries {
// L152: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
			s.operatorSend(fmt.Sprintf("  %s (idle: %s)\n", e.name, e.idle.String()))
// L153: Closes the current block and returns control to the surrounding scope.
		}
// L154: Starts a loop controlled by `i, ip := range rd.queue`, repeating until the loop condition or range is exhausted.
		for i, ip := range rd.queue {
// L155: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
			s.operatorSend(fmt.Sprintf("  [queued #%d] %s\n", i+1, ip))
// L156: Closes the current block and returns control to the surrounding scope.
		}
// L157: Closes the current block and returns control to the surrounding scope.
	}
// L158: Closes the current block and returns control to the surrounding scope.
}

// L160: Declares the `operatorCmdHelp` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdHelp() {
// L161: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend("Available commands:\n")
// L162: Starts a loop controlled by `_, name := range cmd.CommandOrder`, repeating until the loop condition or range is exhausted.
	for _, name := range cmd.CommandOrder {
// L163: Creates `def` as a new local binding so later lines can reuse this computed value.
		def := cmd.Commands[name]
// L164: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(fmt.Sprintf("  %-30s %s\n", def.Usage, def.Description))
// L165: Closes the current block and returns control to the surrounding scope.
	}
// L166: Closes the current block and returns control to the surrounding scope.
}

// L168: Declares the `operatorCmdKick` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdKick(args string) {
// L169: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L170: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Missing target. Usage: /kick <name>\n")
// L171: Returns immediately from the current function without additional values.
		return
// L172: Closes the current block and returns control to the surrounding scope.
	}
// L173: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L174: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
		// Fallback: treat args as IP and search queued users (operator can see IPs via /list)
// L176: Creates `removed` from the result of `s.RemoveFromQueueByIP`, capturing fresh state for the rest of this scope.
		removed := s.RemoveFromQueueByIP(args)
// L177: Evaluates `len(removed) == 0` and enters the guarded branch only when that condition holds.
		if len(removed) == 0 {
// L178: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
			s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
// L179: Returns immediately from the current function without additional values.
			return
// L180: Closes the current block and returns control to the surrounding scope.
		}
// L181: Starts a loop controlled by `_, c := range removed`, repeating until the loop condition or range is exhausted.
		for _, c := range removed {
// L182: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send("You have been kicked by Server.\n")
// L183: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
			c.Close()
// L184: Closes the current block and returns control to the surrounding scope.
		}
// L185: Calls `s.AddKickCooldown` here for its side effects or returned value in the surrounding control flow.
		s.AddKickCooldown(args)
// L186: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(fmt.Sprintf("Queued user(s) from IP %s have been kicked.\n", extractHost(args)))
// L187: Returns immediately from the current function without additional values.
		return
// L188: Closes the current block and returns control to the surrounding scope.
	}

// L190: Creates `targetIP` as a new local binding so later lines can reuse this computed value.
	targetIP := target.IP
// L191: Creates `targetRoom` as a new local binding so later lines can reuse this computed value.
	targetRoom := target.Room
// L192: Calls `target.ForceDisconnectReason` here for its side effects or returned value in the surrounding control flow.
	target.ForceDisconnectReason("kicked")
// L193: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
	s.RemoveClient(args)

// L195: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L196: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L197: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L198: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "kicked",
// L199: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L200: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     "Server",
// L201: Closes the current block and returns control to the surrounding scope.
	}
// L202: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(targetRoom, modMsg)
// L203: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(targetRoom, models.FormatModeration(args, "kicked", "Server")+"\n", "")
// L204: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send("You have been kicked by Server.\n")
// L205: Calls `target.Close` here for its side effects or returned value in the surrounding control flow.
	target.Close()
// L206: Calls `s.AddKickCooldown` here for its side effects or returned value in the surrounding control flow.
	s.AddKickCooldown(targetIP)
// L207: Calls `s.admitFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
	s.admitFromRoomQueue(targetRoom)
// L208: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend(args + " has been kicked.\n")
// L209: Closes the current block and returns control to the surrounding scope.
}

// L211: Declares the `operatorCmdBan` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdBan(args string) {
// L212: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L213: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Missing target. Usage: /ban <name>\n")
// L214: Returns immediately from the current function without additional values.
		return
// L215: Closes the current block and returns control to the surrounding scope.
	}
// L216: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L217: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
		// Fallback: treat args as IP and search queued users (operator can see IPs via /list)
// L219: Creates `removed` from the result of `s.RemoveFromQueueByIP`, capturing fresh state for the rest of this scope.
		removed := s.RemoveFromQueueByIP(args)
// L220: Evaluates `len(removed) == 0` and enters the guarded branch only when that condition holds.
		if len(removed) == 0 {
// L221: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
			s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
// L222: Returns immediately from the current function without additional values.
			return
// L223: Closes the current block and returns control to the surrounding scope.
		}
// L224: Starts a loop controlled by `_, c := range removed`, repeating until the loop condition or range is exhausted.
		for _, c := range removed {
// L225: Calls `c.Send` here for its side effects or returned value in the surrounding control flow.
			c.Send("You have been banned by Server.\n")
// L226: Calls `c.Close` here for its side effects or returned value in the surrounding control flow.
			c.Close()
// L227: Closes the current block and returns control to the surrounding scope.
		}
// L228: Calls `s.AddBanIP` here for its side effects or returned value in the surrounding control flow.
		s.AddBanIP(args)
// L229: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(fmt.Sprintf("Queued user(s) from IP %s have been banned.\n", extractHost(args)))
// L230: Returns immediately from the current function without additional values.
		return
// L231: Closes the current block and returns control to the surrounding scope.
	}

// L233: Creates `targetIP` as a new local binding so later lines can reuse this computed value.
	targetIP := target.IP
// L234: Creates `targetRoom` as a new local binding so later lines can reuse this computed value.
	targetRoom := target.Room
// L235: Creates `bannedHost` from the result of `extractHost`, capturing fresh state for the rest of this scope.
	bannedHost := extractHost(targetIP)
// L236: Calls `target.ForceDisconnectReason` here for its side effects or returned value in the surrounding control flow.
	target.ForceDisconnectReason("banned")
// L237: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
	s.RemoveClient(args)

// L239: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L240: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L241: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L242: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "banned",
// L243: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L244: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     "Server",
// L245: Closes the current block and returns control to the surrounding scope.
	}
// L246: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(targetRoom, modMsg)
// L247: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastRoom(targetRoom, models.FormatModeration(args, "banned", "Server")+"\n", "")
// L248: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send("You have been banned by Server.\n")
// L249: Calls `target.Close` here for its side effects or returned value in the surrounding control flow.
	target.Close()
// L250: Calls `s.AddBanIP` here for its side effects or returned value in the surrounding control flow.
	s.AddBanIP(targetIP)

	// Disconnect all other active clients sharing the banned IP (NAT scenario).
	// Operator is on terminal, not a TCP client, so exclude nobody ("").
	// Track rooms that opened slots for queue admission.
// L255: Creates `roomsOpened` as a new local binding so later lines can reuse this computed value.
	roomsOpened := map[string]int{targetRoom: 1}
// L256: Creates `collateral` from the result of `s.GetClientsByIP`, capturing fresh state for the rest of this scope.
	collateral := s.GetClientsByIP(bannedHost, "")
// L257: Starts a loop controlled by `_, cc := range collateral`, repeating until the loop condition or range is exhausted.
	for _, cc := range collateral {
// L258: Calls `cc.ForceDisconnectReason` here for its side effects or returned value in the surrounding control flow.
		cc.ForceDisconnectReason("banned")
// L259: Creates `ccName` as a new local binding so later lines can reuse this computed value.
		ccName := cc.Username
// L260: Creates `ccRoom` as a new local binding so later lines can reuse this computed value.
		ccRoom := cc.Room
// L261: Calls `s.RemoveClient` here for its side effects or returned value in the surrounding control flow.
		s.RemoveClient(ccName)
// L262: Creates `collateralMsg` as a new local binding so later lines can reuse this computed value.
		collateralMsg := models.Message{
// L263: Keeps this element in the surrounding multiline literal or call expression.
			Timestamp: time.Now(),
// L264: Keeps this element in the surrounding multiline literal or call expression.
			Sender:    ccName,
// L265: Keeps this element in the surrounding multiline literal or call expression.
			Content:   "banned",
// L266: Keeps this element in the surrounding multiline literal or call expression.
			Type:      models.MsgModeration,
// L267: Keeps this element in the surrounding multiline literal or call expression.
			Extra:     "Server",
// L268: Closes the current block and returns control to the surrounding scope.
		}
// L269: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
		s.recordRoomEvent(ccRoom, collateralMsg)
// L270: Calls `s.BroadcastRoom` here for its side effects or returned value in the surrounding control flow.
		s.BroadcastRoom(ccRoom, models.FormatModeration(ccName, "banned", "Server")+"\n", "")
// L271: Calls `cc.Send` here for its side effects or returned value in the surrounding control flow.
		cc.Send("You have been banned by Server.\n")
// L272: Calls `cc.Close` here for its side effects or returned value in the surrounding control flow.
		cc.Close()
// L273: Carries forward the surrounding declaration or expression with the exact value or syntax needed here.
		roomsOpened[ccRoom]++
// L274: Closes the current block and returns control to the surrounding scope.
	}

	// Remove queued users from the banned IP
// L277: Creates `queuedRemoved` from the result of `s.RemoveFromQueueByIP`, capturing fresh state for the rest of this scope.
	queuedRemoved := s.RemoveFromQueueByIP(targetIP)
// L278: Starts a loop controlled by `_, qc := range queuedRemoved`, repeating until the loop condition or range is exhausted.
	for _, qc := range queuedRemoved {
// L279: Calls `qc.Send` here for its side effects or returned value in the surrounding control flow.
		qc.Send("You have been banned by Server.\n")
// L280: Calls `qc.Close` here for its side effects or returned value in the surrounding control flow.
		qc.Close()
// L281: Closes the current block and returns control to the surrounding scope.
	}

// L283: Starts a loop controlled by `rn, count := range roomsOpened`, repeating until the loop condition or range is exhausted.
	for rn, count := range roomsOpened {
// L284: Starts a loop controlled by `i := 0; i < count; i++`, repeating until the loop condition or range is exhausted.
		for i := 0; i < count; i++ {
// L285: Calls `s.admitFromRoomQueue` here for its side effects or returned value in the surrounding control flow.
			s.admitFromRoomQueue(rn)
// L286: Closes the current block and returns control to the surrounding scope.
		}
// L287: Closes the current block and returns control to the surrounding scope.
	}
// L288: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend(args + " has been banned.\n")
// L289: Closes the current block and returns control to the surrounding scope.
}

// L291: Declares the `operatorCmdMute` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdMute(args string) {
// L292: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L293: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Missing target. Usage: /mute <name>\n")
// L294: Returns immediately from the current function without additional values.
		return
// L295: Closes the current block and returns control to the surrounding scope.
	}
// L296: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L297: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L298: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
// L299: Returns immediately from the current function without additional values.
		return
// L300: Closes the current block and returns control to the surrounding scope.
	}
// L301: Evaluates `target.IsMuted()` and enters the guarded branch only when that condition holds.
	if target.IsMuted() {
// L302: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(args + " is already muted.\n")
// L303: Returns immediately from the current function without additional values.
		return
// L304: Closes the current block and returns control to the surrounding scope.
	}

// L306: Calls `target.SetMuted` here for its side effects or returned value in the surrounding control flow.
	target.SetMuted(true)
// L307: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L308: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L309: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L310: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "muted",
// L311: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L312: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     "Server",
// L313: Closes the current block and returns control to the surrounding scope.
	}
// L314: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(target.Room, modMsg)
// L315: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatModeration(args, "muted", "Server") + "\n")
// L316: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend(args + " has been muted.\n")
// L317: Closes the current block and returns control to the surrounding scope.
}

// L319: Declares the `operatorCmdUnmute` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdUnmute(args string) {
// L320: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L321: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Missing target. Usage: /unmute <name>\n")
// L322: Returns immediately from the current function without additional values.
		return
// L323: Closes the current block and returns control to the surrounding scope.
	}
// L324: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L325: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L326: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
// L327: Returns immediately from the current function without additional values.
		return
// L328: Closes the current block and returns control to the surrounding scope.
	}
// L329: Evaluates `!target.IsMuted()` and enters the guarded branch only when that condition holds.
	if !target.IsMuted() {
// L330: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(args + " is not muted.\n")
// L331: Returns immediately from the current function without additional values.
		return
// L332: Closes the current block and returns control to the surrounding scope.
	}

// L334: Calls `target.SetMuted` here for its side effects or returned value in the surrounding control flow.
	target.SetMuted(false)
// L335: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L336: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L337: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L338: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "unmuted",
// L339: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L340: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     "Server",
// L341: Closes the current block and returns control to the surrounding scope.
	}
// L342: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(target.Room, modMsg)
// L343: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatModeration(args, "unmuted", "Server") + "\n")
// L344: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend(args + " has been unmuted.\n")
// L345: Closes the current block and returns control to the surrounding scope.
}

// L347: Declares the `operatorCmdAnnounce` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdAnnounce(args string) {
// L348: Evaluates `len(strings.TrimSpace(args)) == 0` and enters the guarded branch only when that condition holds.
	if len(strings.TrimSpace(args)) == 0 {
// L349: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Usage: /announce <message>\n")
// L350: Returns immediately from the current function without additional values.
		return
// L351: Closes the current block and returns control to the surrounding scope.
	}
	// Log announcement to all rooms
// L353: Starts a loop controlled by `_, rn := range s.GetRoomNames()`, repeating until the loop condition or range is exhausted.
	for _, rn := range s.GetRoomNames() {
// L354: Creates `announceMsg` as a new local binding so later lines can reuse this computed value.
		announceMsg := models.Message{
// L355: Keeps this element in the surrounding multiline literal or call expression.
			Timestamp: time.Now(),
// L356: Keeps this element in the surrounding multiline literal or call expression.
			Content:   args,
// L357: Keeps this element in the surrounding multiline literal or call expression.
			Type:      models.MsgAnnouncement,
// L358: Keeps this element in the surrounding multiline literal or call expression.
			Extra:     "Server",
// L359: Closes the current block and returns control to the surrounding scope.
		}
// L360: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
		s.recordRoomEvent(rn, announceMsg)
// L361: Closes the current block and returns control to the surrounding scope.
	}
// L362: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatAnnouncement(args) + "\n")
// L363: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend("Announcement sent.\n")
// L364: Closes the current block and returns control to the surrounding scope.
}

// L366: Declares the `operatorCmdPromote` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdPromote(args string) {
// L367: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L368: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Missing target. Usage: /promote <name>\n")
// L369: Returns immediately from the current function without additional values.
		return
// L370: Closes the current block and returns control to the surrounding scope.
	}
// L371: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L372: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L373: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
// L374: Returns immediately from the current function without additional values.
		return
// L375: Closes the current block and returns control to the surrounding scope.
	}
// L376: Evaluates `target.IsAdmin()` and enters the guarded branch only when that condition holds.
	if target.IsAdmin() {
// L377: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(args + " is already an admin.\n")
// L378: Returns immediately from the current function without additional values.
		return
// L379: Closes the current block and returns control to the surrounding scope.
	}
// L380: Calls `target.SetAdmin` here for its side effects or returned value in the surrounding control flow.
	target.SetAdmin(true)
// L381: Calls `s.AddAdmin` here for its side effects or returned value in the surrounding control flow.
	s.AddAdmin(args)
// L382: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send("You have been promoted to admin.\n")

// L384: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L385: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L386: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L387: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "promoted",
// L388: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L389: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     "Server",
// L390: Closes the current block and returns control to the surrounding scope.
	}
// L391: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(target.Room, modMsg)
// L392: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatModeration(args, "promoted", "Server") + "\n")
// L393: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend(args + " has been promoted to admin.\n")
// L394: Closes the current block and returns control to the surrounding scope.
}

// L396: Declares the `operatorCmdDemote` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdDemote(args string) {
// L397: Evaluates `args == ""` and enters the guarded branch only when that condition holds.
	if args == "" {
// L398: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("Missing target. Usage: /demote <name>\n")
// L399: Returns immediately from the current function without additional values.
		return
// L400: Closes the current block and returns control to the surrounding scope.
	}
// L401: Creates `target` from the result of `s.GetClient`, capturing fresh state for the rest of this scope.
	target := s.GetClient(args)
// L402: Evaluates `target == nil` and enters the guarded branch only when that condition holds.
	if target == nil {
// L403: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend("User '" + args + "' not found. Use /list to see connected users.\n")
// L404: Returns immediately from the current function without additional values.
		return
// L405: Closes the current block and returns control to the surrounding scope.
	}
// L406: Evaluates `!target.IsAdmin()` and enters the guarded branch only when that condition holds.
	if !target.IsAdmin() {
// L407: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(args + " is not an admin.\n")
// L408: Returns immediately from the current function without additional values.
		return
// L409: Closes the current block and returns control to the surrounding scope.
	}
// L410: Calls `target.SetAdmin` here for its side effects or returned value in the surrounding control flow.
	target.SetAdmin(false)
// L411: Calls `s.RemoveAdmin` here for its side effects or returned value in the surrounding control flow.
	s.RemoveAdmin(args)
// L412: Calls `target.Send` here for its side effects or returned value in the surrounding control flow.
	target.Send("Your admin privileges have been revoked.\n")

// L414: Creates `modMsg` as a new local binding so later lines can reuse this computed value.
	modMsg := models.Message{
// L415: Keeps this element in the surrounding multiline literal or call expression.
		Timestamp: time.Now(),
// L416: Keeps this element in the surrounding multiline literal or call expression.
		Sender:    args,
// L417: Keeps this element in the surrounding multiline literal or call expression.
		Content:   "demoted",
// L418: Keeps this element in the surrounding multiline literal or call expression.
		Type:      models.MsgModeration,
// L419: Keeps this element in the surrounding multiline literal or call expression.
		Extra:     "Server",
// L420: Closes the current block and returns control to the surrounding scope.
	}
// L421: Calls `s.recordRoomEvent` here for its side effects or returned value in the surrounding control flow.
	s.recordRoomEvent(target.Room, modMsg)
// L422: Calls `s.BroadcastAllRooms` here for its side effects or returned value in the surrounding control flow.
	s.BroadcastAllRooms(models.FormatModeration(args, "demoted", "Server") + "\n")
// L423: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend(args + " has been demoted.\n")
// L424: Closes the current block and returns control to the surrounding scope.
}

// L426: Declares the `operatorCmdRooms` method on `s *Server`, creating a reusable behavior entrypoint tied to that receiver state.
func (s *Server) operatorCmdRooms() {
// L427: Creates `roomNames` from the result of `s.GetRoomNames`, capturing fresh state for the rest of this scope.
	roomNames := s.GetRoomNames()
// L428: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
	s.operatorSend("Available rooms:\n")
// L429: Starts a loop controlled by `_, rn := range roomNames`, repeating until the loop condition or range is exhausted.
	for _, rn := range roomNames {
// L430: Creates `count` from the result of `s.GetRoomClientCount`, capturing fresh state for the rest of this scope.
		count := s.GetRoomClientCount(rn)
// L431: Calls `s.operatorSend` here for its side effects or returned value in the surrounding control flow.
		s.operatorSend(fmt.Sprintf("  %s (%d clients)\n", rn, count))
// L432: Closes the current block and returns control to the surrounding scope.
	}
// L433: Closes the current block and returns control to the surrounding scope.
}
```

