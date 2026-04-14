package cmd

import "strings"

// PrivilegeLevel controls who may invoke a command.
type PrivilegeLevel int

const (
	PrivUser     PrivilegeLevel = iota // any connected client
	PrivAdmin                          // promoted admin or server operator
	PrivOperator                       // server operator terminal only
)

// CommandDef describes a registered command.
type CommandDef struct {
	Name        string
	MinPriv     PrivilegeLevel
	Usage       string
	Description string
}

// Commands is the canonical command registry.
var Commands = map[string]CommandDef{
	"list":     {Name: "list", MinPriv: PrivUser, Usage: "/list", Description: "List connected clients with idle times"},
	"quit":     {Name: "quit", MinPriv: PrivUser, Usage: "/quit", Description: "Disconnect from chat"},
	"name":     {Name: "name", MinPriv: PrivUser, Usage: "/name <newname>", Description: "Change your display name"},
	"whisper":  {Name: "whisper", MinPriv: PrivUser, Usage: "/whisper <name> <message>", Description: "Send a private message"},
	"help":     {Name: "help", MinPriv: PrivUser, Usage: "/help", Description: "Show available commands"},
	"rooms":    {Name: "rooms", MinPriv: PrivUser, Usage: "/rooms", Description: "List available rooms with client counts"},
	"stats":    {Name: "stats", MinPriv: PrivUser, Usage: "/stats", Description: "Show server statistics"},
	"switch":   {Name: "switch", MinPriv: PrivUser, Usage: "/switch <room>", Description: "Switch to another room"},
	"create":   {Name: "create", MinPriv: PrivUser, Usage: "/create <room>", Description: "Create and switch to a new room"},
	"kick":     {Name: "kick", MinPriv: PrivAdmin, Usage: "/kick <name>", Description: "Kick a user from chat"},
	"ban":      {Name: "ban", MinPriv: PrivAdmin, Usage: "/ban <name>", Description: "Ban a user from chat"},
	"mute":     {Name: "mute", MinPriv: PrivAdmin, Usage: "/mute <name>", Description: "Mute a user"},
	"unmute":   {Name: "unmute", MinPriv: PrivAdmin, Usage: "/unmute <name>", Description: "Unmute a user"},
	"announce": {Name: "announce", MinPriv: PrivAdmin, Usage: "/announce <message>", Description: "Broadcast an announcement"},
	"promote":  {Name: "promote", MinPriv: PrivOperator, Usage: "/promote <name>", Description: "Promote a user to admin"},
	"demote":   {Name: "demote", MinPriv: PrivOperator, Usage: "/demote <name>", Description: "Demote an admin"},
}

// CommandOrder defines the display order used by /help.
var CommandOrder = []string{
	"list", "rooms", "stats", "switch", "create", "quit", "name", "whisper", "help",
	"kick", "ban", "mute", "unmute", "announce",
	"promote", "demote",
}

// ParseCommand splits a /-prefixed input into command name and trimmed arguments.
// Returns isCommand=false for non-command input.
// ParseCommand splits slash-prefixed input into a command name and trimmed argument string.
func ParseCommand(input string) (name string, args string, isCommand bool) {
	if len(input) == 0 || input[0] != '/' {
		return "", "", false
	}
	rest := input[1:]
	if len(rest) == 0 {
		return "", "", true // lone "/"
	}
	idx := strings.IndexByte(rest, ' ')
	if idx < 0 {
		return rest, "", true
	}
	return rest[:idx], strings.TrimSpace(rest[idx+1:]), true
}

// GetPrivilegeLevel maps boolean flags to the corresponding level.
func GetPrivilegeLevel(isAdmin, isOperator bool) PrivilegeLevel {
	if isOperator {
		return PrivOperator
	}
	if isAdmin {
		return PrivAdmin
	}
	return PrivUser
}
