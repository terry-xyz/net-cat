# Net-Cat

A group chat server that runs in your terminal. Anyone on the same network can connect and chat in real time — no browser, no account, no app to install.

```
Welcome to TCP-Chat!
         _nnnn_
        dGGGGMMb
       @p~qp~~qMb
       M|@||@) M|
       @,----.JM|
      JS^\__/  qKL
     dZP        qKRb
    dZP          qKKb
   fZP            SMMb
   HZM            MMMM
   FqM            MMMM
 __| ".        |\dS"qML
 |    `.       | `' \Zq
_)      \.___.,|     .'
\____   )MMMMMP|   .'
     `-'       `--'
```

## What You Need

- **Go** (version 1.25 or newer) — this is the programming language the server is written in.
  - Download it from [go.dev/dl](https://go.dev/dl/).
  - Pick the installer for your operating system (Windows, macOS, or Linux) and follow the installation steps.
  - After installing, open a terminal and type `go version`. If you see a version number, you're good.

That's the only thing you need to install. Everything else is included.

## Getting the Project

If you received this as a `.zip` file, extract it to any folder.

If you want to get it from GitHub, open a terminal and run:

```bash
git clone <repository-url>
cd net-cat
```

## Building the Server

Open a terminal, navigate to the project folder, and run:

```bash
go build -o TCPChat
```

This creates a file called `TCPChat` (or `TCPChat.exe` on Windows) — that's your chat server.

## Starting the Server

Run the server with:

```bash
./TCPChat
```

You'll see a message like:

```
Listening on port :8989
```

The server is now running and waiting for people to connect.

### Using a Different Port

By default the server uses port `8989`. To use a different port:

```bash
./TCPChat 3000
```

This starts the server on port `3000` instead. Valid ports are `1` through `65535`.

## Connecting to the Chat

Anyone on the same network can join. They do **not** need Go installed — just a terminal.

### On Linux or macOS

```bash
nc localhost 8989
```

### On Windows

```bash
nc localhost 8989
```

Replace `localhost` with the server's IP address if connecting from a different computer on the network (e.g., `nc 192.168.1.50 8989`).

### What Happens When You Connect

1. You see the welcome banner with the penguin ASCII art.
2. You're asked to enter a name. Pick something unique — no spaces allowed, max 32 characters.
3. You choose a chat room to join. Press Enter to join the default room (`general`), or type a room name to join or create a different room.
4. Once you're in, everything you type is sent to everyone in your room.
5. If the room is full (10 people max per room), you'll be asked if you want to wait in a queue. When a spot opens, you're automatically let in.

## Chatting

Just type your message and press **Enter**. Everyone in the chat will see it with your name and a timestamp:

```
[2026-02-24 14:30:05][Alice]: Hello everyone!
```

## Commands

Type these instead of a regular message. All commands start with `/`.

### Commands Everyone Can Use

| Command | What It Does |
|---|---|
| `/list` | Shows everyone in your current room and how long they've been idle |
| `/rooms` | Lists all available rooms with client counts, marks your current room |
| `/switch <room>` | Switch to another room (creates it if it doesn't exist yet) |
| `/create <room>` | Create and switch to a new room |
| `/whisper <name> <message>` | Sends a private message only that person can see (works across rooms) |
| `/name <newname>` | Changes your display name |
| `/help` | Shows all commands available to you |
| `/quit` | Disconnects you from the chat |

### Admin Commands

These only work if the server operator has promoted you to admin.

| Command | What It Does |
|---|---|
| `/kick <name>` | Removes someone from your room (they can rejoin after 5 minutes) |
| `/ban <name>` | Permanently removes someone (they cannot rejoin until the server restarts) |
| `/mute <name>` | Prevents someone from sending messages |
| `/unmute <name>` | Allows a muted person to send messages again |
| `/announce <message>` | Sends a highlighted announcement to everyone |

## Running the Server (Operator Guide)

The person running the server (the operator) has extra powers. While the server is running, you can type commands directly into the server's terminal.

### Operator Commands

You have access to all admin commands listed above, plus:

| Command | What It Does |
|---|---|
| `/promote <name>` | Makes someone an admin (only the operator can do this) |
| `/demote <name>` | Removes someone's admin status |
| `/kick <IP>` | Kicks a queued user by their IP address |
| `/ban <IP>` | Bans a queued user by their IP address |

### Shutting Down the Server

Press **Ctrl+C** in the server terminal. The server will:

1. Notify all connected users that it's shutting down.
2. Wait up to 5 seconds for everyone to disconnect.
3. Close all remaining connections.
4. Save the logs and exit.

## Logs

The server automatically saves all chat activity to daily log files in a `logs/` folder:

```
logs/chat_2026-02-24.log
```

A new log file is created each day. Log entries include `@roomname` tags to track which room each event occurred in. If you restart the server on the same day, the previous messages are loaded back into the correct rooms so new users see room-specific chat history.

## Admin Persistence

When the operator promotes someone to admin, their name is saved in an `admins.json` file. If that person disconnects and later reconnects, they automatically get their admin powers back.

## Troubleshooting

**"Address already in use" when starting the server**
Another program is using that port. Either stop the other program, or start the server on a different port: `./TCPChat 4000`

**Can't connect from another computer**
Make sure both computers are on the same network. Use the server computer's local IP address (find it with `ipconfig` on Windows or `ifconfig` / `ip addr` on Linux/macOS) instead of `localhost`. Also check that the firewall isn't blocking the port.

**"go: command not found" when building**
Go isn't installed or isn't in your system PATH. Reinstall Go from [go.dev/dl](https://go.dev/dl/) and make sure to follow the PATH setup instructions for your operating system.

**Connection drops unexpectedly**
The server checks for unresponsive connections every 10 seconds. If your network is unstable, you may get disconnected. Simply reconnect and pick your name again.

## Quick Start Summary

```bash
# 1. Build
go build -o TCPChat

# 2. Start the server
./TCPChat

# 3. Connect from another terminal
nc localhost 8989
```
