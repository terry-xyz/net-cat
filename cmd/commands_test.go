package cmd

import "testing"

func TestParseCommandNonCommand(t *testing.T) {
	tests := []string{"hello", "", "not a /command", "hello /world"}
	for _, input := range tests {
		_, _, isCmd := ParseCommand(input)
		if isCmd {
			t.Errorf("ParseCommand(%q) isCommand=true, want false", input)
		}
	}
}

func TestParseCommandLoneSlash(t *testing.T) {
	name, args, isCmd := ParseCommand("/")
	if !isCmd {
		t.Fatal("/ should be a command")
	}
	if name != "" {
		t.Errorf("name=%q, want empty", name)
	}
	if args != "" {
		t.Errorf("args=%q, want empty", args)
	}
}

func TestParseCommandSimple(t *testing.T) {
	name, args, isCmd := ParseCommand("/list")
	if !isCmd || name != "list" || args != "" {
		t.Errorf("got name=%q args=%q isCmd=%v", name, args, isCmd)
	}
}

func TestParseCommandWithArgs(t *testing.T) {
	name, args, isCmd := ParseCommand("/kick alice")
	if !isCmd || name != "kick" || args != "alice" {
		t.Errorf("got name=%q args=%q", name, args)
	}
}

func TestParseCommandExcessWhitespace(t *testing.T) {
	name, args, isCmd := ParseCommand("/whisper    bob    hello")
	if !isCmd || name != "whisper" {
		t.Fatalf("name=%q", name)
	}
	if args != "bob    hello" {
		t.Errorf("args=%q, want %q", args, "bob    hello")
	}
}

func TestParseCommandCaseSensitive(t *testing.T) {
	// /LIST should be treated as a command but not match any known command
	name, _, isCmd := ParseCommand("/LIST")
	if !isCmd {
		t.Fatal("should be a command")
	}
	if name != "LIST" {
		t.Errorf("name=%q", name)
	}
	if _, exists := Commands[name]; exists {
		t.Error("/LIST should not be a recognized command")
	}
}

func TestAllCommandsRegistered(t *testing.T) {
	expected := []string{"list", "quit", "name", "whisper", "help",
		"kick", "ban", "mute", "unmute", "announce",
		"promote", "demote"}
	for _, name := range expected {
		if _, ok := Commands[name]; !ok {
			t.Errorf("command %q not in registry", name)
		}
	}
}

func TestUserCommandCount(t *testing.T) {
	count := 0
	for _, def := range Commands {
		if def.MinPriv == PrivUser {
			count++
		}
	}
	if count != 8 {
		t.Errorf("user-level commands: got %d, want 8", count)
	}
}

func TestPrivilegeLevels(t *testing.T) {
	if GetPrivilegeLevel(false, false) != PrivUser {
		t.Error("non-admin non-operator should be user")
	}
	if GetPrivilegeLevel(true, false) != PrivAdmin {
		t.Error("admin should be admin level")
	}
	if GetPrivilegeLevel(false, true) != PrivOperator {
		t.Error("operator should be operator level")
	}
	if GetPrivilegeLevel(true, true) != PrivOperator {
		t.Error("admin+operator should be operator level")
	}
}

func TestPartialMatchNotRecognized(t *testing.T) {
	name, _, _ := ParseCommand("/lis")
	if _, exists := Commands[name]; exists {
		t.Error("/lis should not match /list (no partial matching)")
	}
}
