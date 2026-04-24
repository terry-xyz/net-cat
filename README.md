# net-cat

`net-cat` is a terminal-based TCP chat server written in Go. It supports multi-room chat, private messaging, moderation commands, daily log files, chat-history recovery, and an operator/admin model for managing active rooms.

## Requirements

- Go 1.25 or newer
- A terminal client such as `nc`, or `bash` with `/dev/tcp` support for manual testing

## Quick Start

Clone the GitHub repository:

```bash
git clone https://github.com/terry-xyz/net-cat.git
cd net-cat
```

Build the server:

```bash
go build -o TCPChat
```

Start the server on the default port (`8989`):

```bash
./TCPChat
```

Start the server on a custom port:

```bash
./TCPChat 3000
```

Connect from another terminal:

```bash
nc localhost 8989
```

If `nc` is unavailable, use bash:

```bash
exec 3<>/dev/tcp/localhost/8989; cat <&3 & cat >&3
```

Change `localhost` with your IPv4 to connect on different machine (must be connected on the same network as the server)

## Usage

On connect, the server prompts for:

1. A username
2. A room name, or empty input to join the default room (`general`)

Each room supports up to 10 active clients. Additional clients can wait in a FIFO queue until a slot becomes available.

Messages are timestamped and broadcast only within the current room. Private messages work across rooms with `/whisper`.

## Commands

User commands:

| Command | Description |
| --- | --- |
| `/list` | List clients in the current room with idle times |
| `/rooms` | List rooms and client counts |
| `/stats` | Show active users, room count, and uptime |
| `/switch <room>` | Switch to another room |
| `/create <room>` | Create and switch to a new room |
| `/name <newname>` | Change your display name |
| `/whisper <name> <message>` | Send a private message |
| `/help` | Show available commands |
| `/quit` | Disconnect |

Admin commands:

| Command | Description |
| --- | --- |
| `/kick <name>` | Remove a user from chat |
| `/ban <name>` | Ban a user for the current server session |
| `/mute <name>` | Mute a user |
| `/unmute <name>` | Unmute a user |
| `/announce <message>` | Broadcast an announcement |

Operator-only commands from the server terminal:

| Command | Description |
| --- | --- |
| `/promote <name>` | Promote a user to admin |
| `/demote <name>` | Remove admin privileges |
| `/kick <ip>` | Kick a queued user by IP |
| `/ban <ip>` | Ban a queued user by IP |

## Operational Notes

- Logs are written to `logs/chat_YYYY-MM-DD.log`.
- Same-day history is recovered on restart and restored per room.
- Promoted admins are persisted in `admins.json`.
- `Ctrl+C` triggers graceful shutdown with a 5-second disconnect window before forced close.

## Development

Run the full test suite:

```bash
go test ./...
```

Useful commands:

```bash
go test -v ./...
go test -cover ./...
gofmt -w .
```

Project layout:

- `main.go`: process entrypoint and signal handling
- `server/`: room management, commands, moderation, recovery, and operator flow
- `client/`: connection state and terminal I/O behavior
- `cmd/`: shared command registry and parsing
- `logger/`: daily log handling
- `models/`: shared message types

## Documentation

- [CONTRIBUTING.md](CONTRIBUTING.md)
- [CHANGELOG.md](CHANGELOG.md)
- [LICENSE](LICENSE)
