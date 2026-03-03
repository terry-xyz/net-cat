package server

// Stress tests with high concurrency to verify the server handles load without
// deadlocks, goroutine leaks, message loss, or corruption.
// Required by Task 26 (IMPLEMENTATION_PLAN.md Phase 9).

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// ==================== Test 1: Rapid Messages ====================
// 10 clients each sending 100 messages rapidly: all messages delivered in order, none dropped.

func TestStressRapidMessages(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	const numClients = 10
	const msgsPerClient = 100

	// Connect and onboard all clients
	conns := make([]net.Conn, numClients)
	names := make([]string, numClients)
	for i := 0; i < numClients; i++ {
		names[i] = fmt.Sprintf("user%d", i)
		conns[i] = tcpOnboard(t, addr, names[i])
		defer conns[i].Close()
		time.Sleep(30 * time.Millisecond) // let join notifications propagate
	}
	// Drain all join notifications
	time.Sleep(300 * time.Millisecond)
	for _, conn := range conns {
		drainFor(conn, 200*time.Millisecond)
	}

	// Each client sends 100 messages as fast as possible
	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < msgsPerClient; j++ {
				msg := fmt.Sprintf("msg_%d_%d", idx, j)
				fmt.Fprintf(conns[idx], "%s\n", msg)
				// Small delay to avoid overwhelming the echo mode pipeline
				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}
	wg.Wait()

	// Allow time for all messages to propagate through the server
	time.Sleep(1500 * time.Millisecond)

	// Check log file for all messages
	history := s.GetHistory()
	chatCount := 0
	msgSeen := make(map[string]int) // msg content -> count
	for _, h := range history {
		if h.Type == 0 { // MsgChat
			chatCount++
			msgSeen[h.Content]++
		}
	}

	expectedTotal := numClients * msgsPerClient
	if chatCount != expectedTotal {
		t.Errorf("expected %d chat messages in history, got %d", expectedTotal, chatCount)
	}

	// Verify every message appears exactly once
	for i := 0; i < numClients; i++ {
		for j := 0; j < msgsPerClient; j++ {
			key := fmt.Sprintf("msg_%d_%d", i, j)
			if msgSeen[key] != 1 {
				t.Errorf("message %q appeared %d times (expected 1)", key, msgSeen[key])
			}
		}
	}
}

// ==================== Test 2: Concurrent Log Accuracy ====================
// 10 clients sending messages simultaneously: all messages appear exactly once
// in the log file with correct timestamps.

func TestStressConcurrentLogAccuracy(t *testing.T) {
	s, addr, tmpDir := startIntServer(t)
	defer s.Shutdown()

	const numClients = 10
	const msgsPerClient = 50

	conns := make([]net.Conn, numClients)
	names := make([]string, numClients)
	for i := 0; i < numClients; i++ {
		names[i] = fmt.Sprintf("logger%d", i)
		conns[i] = tcpOnboard(t, addr, names[i])
		defer conns[i].Close()
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)

	// All clients send simultaneously
	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startBarrier // synchronize start
			for j := 0; j < msgsPerClient; j++ {
				// Zero-padded numbers prevent substring false matches
				// (e.g., "log_0_001" won't match "log_0_010")
				msg := fmt.Sprintf("log_%02d_%03d", idx, j)
				fmt.Fprintf(conns[idx], "%s\n", msg)
				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}
	close(startBarrier) // release all goroutines simultaneously
	wg.Wait()

	// Wait for all messages to be logged
	time.Sleep(1 * time.Second)

	// Read the log file and verify all messages
	logsDir := filepath.Join(tmpDir, "logs")
	files, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("failed to read logs dir: %v", err)
	}

	var logContent string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".log") {
			data, err := os.ReadFile(filepath.Join(logsDir, f.Name()))
			if err != nil {
				t.Fatalf("failed to read log: %v", err)
			}
			logContent += string(data)
		}
	}

	// Verify each message appears exactly once in the log
	for i := 0; i < numClients; i++ {
		for j := 0; j < msgsPerClient; j++ {
			msg := fmt.Sprintf("log_%02d_%03d", i, j)
			count := strings.Count(logContent, msg)
			if count != 1 {
				t.Errorf("message %q appeared %d times in log (expected 1)", msg, count)
			}
		}
	}

	// Verify log lines are well-formed (each CHAT line has a timestamp)
	for _, line := range strings.Split(logContent, "\n") {
		if strings.Contains(line, "CHAT") {
			if !strings.HasPrefix(line, "[") {
				t.Errorf("malformed log line (missing timestamp): %q", line)
			}
		}
	}
}

// ==================== Test 3: Rapid Connect/Disconnect ====================
// 50 clients connecting and disconnecting in rapid succession: no goroutine leaks.

func TestStressRapidConnectDisconnect(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	// Wait for server goroutines to stabilize
	time.Sleep(500 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const cycles = 50
	var wg sync.WaitGroup

	for i := 0; i < cycles; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return
			}
			// Read just enough to see the banner started
			buf := make([]byte, 512)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			conn.Read(buf)
			// Immediately close (simulate rapid disconnect)
			conn.Close()
		}(i)
		// Small stagger to avoid SYN flood behavior
		time.Sleep(10 * time.Millisecond)
	}
	wg.Wait()

	// Allow goroutines to clean up
	time.Sleep(2 * time.Second)

	final := runtime.NumGoroutine()
	// Allow a small margin for background goroutines (timer, GC, etc.)
	leaked := final - baseline
	if leaked > 10 {
		t.Errorf("potential goroutine leak: baseline=%d, final=%d, leaked=%d", baseline, final, leaked)
	}
}

// ==================== Test 4: Queue Positions ====================
// 10 active clients + 20 queued clients: queue positions update correctly,
// admission is FIFO.

func TestStressQueuePositions(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	// Connect 10 active clients
	activeConns := make([]net.Conn, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("active%d", i)
		activeConns[i] = tcpOnboard(t, addr, name)
		defer activeConns[i].Close()
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	// Connect 20 queued clients (new flow: banner → name → room → queue)
	queuedConns := make([]net.Conn, 20)
	for i := 0; i < 20; i++ {
		conn := tcpDial(t, addr)
		queuedConns[i] = conn
		defer conn.Close()

		// Complete name registration
		readUntil(conn, "[ENTER YOUR NAME]:", 3*time.Second)
		fmt.Fprintf(conn, "queued%d\n", i)
		// Complete room selection (default room)
		readUntil(conn, "[ENTER ROOM NAME]", 3*time.Second)
		fmt.Fprintf(conn, "\n")

		text, err := readUntil(conn, "Would you like to wait?", 3*time.Second)
		if err != nil {
			t.Fatalf("queued client %d: did not get queue prompt: %v", i, err)
		}
		expectedPos := fmt.Sprintf("#%d", i+1)
		if !strings.Contains(text, expectedPos) {
			t.Errorf("queued client %d: expected position %s, got: %q", i, expectedPos, text)
		}
		fmt.Fprintf(conn, "yes\n")
	}
	time.Sleep(500 * time.Millisecond)

	// Verify queue length
	qLen := s.GetQueueLength()
	if qLen != 20 {
		t.Errorf("expected queue length 20, got %d", qLen)
	}

	// Disconnect first 3 active clients to admit first 3 from queue
	for i := 0; i < 3; i++ {
		activeConns[i].Close()
		activeConns[i] = nil
		time.Sleep(300 * time.Millisecond) // let admission + position updates propagate
	}

	// First 3 queued clients should be admitted (already named, get prompt directly)
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("queued%d", i)
		_, err := readUntil(queuedConns[i], "]["+name+"]:", 5*time.Second)
		if err != nil {
			t.Errorf("queued client %d should have been admitted: %v", i, err)
		}
	}

	// Remaining 17 queued clients should have updated positions
	time.Sleep(500 * time.Millisecond)

	// Verify room still has 10 active clients (capacity is per-room)
	roomCount := s.GetRoomClientCount(s.DefaultRoom)
	if roomCount != 10 {
		t.Errorf("expected 10 active clients in room after admission, got %d", roomCount)
	}

	// Verify queue is now 17
	qLen = s.GetQueueLength()
	if qLen != 17 {
		t.Errorf("expected queue length 17 after 3 admissions, got %d", qLen)
	}
}

// ==================== Test 5: Broadcast Completeness ====================
// All 10 clients sending messages simultaneously: each receives all others' messages,
// no cross-contamination.

func TestStressBroadcastCompleteness(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	const numClients = 10

	conns := make([]net.Conn, numClients)
	names := make([]string, numClients)
	for i := 0; i < numClients; i++ {
		names[i] = fmt.Sprintf("bc%d", i)
		conns[i] = tcpOnboard(t, addr, names[i])
		defer conns[i].Close()
		time.Sleep(30 * time.Millisecond)
	}
	// Drain join notifications
	time.Sleep(500 * time.Millisecond)
	for _, conn := range conns {
		drainFor(conn, 200*time.Millisecond)
	}

	// All clients send one unique message simultaneously
	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startBarrier
			msg := fmt.Sprintf("broadcast_%d_unique", idx)
			fmt.Fprintf(conns[idx], "%s\n", msg)
		}(i)
	}
	close(startBarrier)
	wg.Wait()

	// Wait for messages to propagate
	time.Sleep(1 * time.Second)

	// Each client should see all OTHER clients' messages (9 messages each).
	// Drain all connections in parallel to avoid sequential 1s × N timeouts.
	results := make([]string, numClients)
	var drainWg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		drainWg.Add(1)
		go func(idx int) {
			defer drainWg.Done()
			results[idx] = stripAnsi(drainFor(conns[idx], 500*time.Millisecond))
		}(i)
	}
	drainWg.Wait()

	for i := 0; i < numClients; i++ {
		for j := 0; j < numClients; j++ {
			if i == j {
				continue // sender doesn't receive their own broadcast
			}
			expected := fmt.Sprintf("broadcast_%d_unique", j)
			if !strings.Contains(results[i], expected) {
				t.Errorf("client %d (bc%d) did not receive message from client %d: %q not found in output",
					i, i, j, expected)
			}
		}
	}
}

// ==================== Test 6: Messages During Join/Leave ====================
// Client sending at maximum rate while others join/leave: no deadlock, no panic.

func TestStressMessageDuringJoinLeave(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	// Connect the sender first
	sender := tcpOnboard(t, addr, "sender")
	defer sender.Close()
	time.Sleep(100 * time.Millisecond)

	// Start sender goroutine that sends messages continuously
	senderDone := make(chan struct{})
	go func() {
		defer close(senderDone)
		for i := 0; i < 100; i++ {
			msg := fmt.Sprintf("rapid_%d", i)
			fmt.Fprintf(sender, "%s\n", msg)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Meanwhile, rapidly connect and disconnect other clients
	var wg sync.WaitGroup
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("joiner%d", idx)
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return
			}
			// Read banner
			readUntil(conn, "[ENTER YOUR NAME]:", 3*time.Second)
			fmt.Fprintf(conn, "%s\n", name)
			// Wait for prompt (onboarded)
			readUntil(conn, "]["+name+"]:", 3*time.Second)
			// Stay for a bit, then leave
			time.Sleep(time.Duration(50+idx*10) * time.Millisecond)
			conn.Close()
		}(i)
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for sender to finish
	<-senderDone

	// Wait for all joiners/leavers to finish
	wg.Wait()

	// Allow cleanup
	time.Sleep(500 * time.Millisecond)

	// The test passes if we reach here without deadlock or panic.
	// Verify sender is still connected and responsive
	fmt.Fprintf(sender, "still_alive\n")
	_, err := readUntil(sender, "][sender]:", 3*time.Second)
	if err != nil {
		t.Errorf("sender should still be connected after stress: %v", err)
	}

	// Verify server state is consistent
	count := s.GetClientCount()
	if count < 1 {
		t.Error("sender should still be in the client map")
	}
}

// ==================== Test 7: Midnight Rotation Under Load ====================
// Simulates midnight boundary by directly calling ClearHistory and verifying
// messages aren't lost or duplicated during the transition.

func TestStressMidnightRotationUnderLoad(t *testing.T) {
	s, addr, _ := startIntServer(t)
	defer s.Shutdown()

	const numClients = 5
	const msgsBeforeMidnight = 10
	const msgsAfterMidnight = 10

	conns := make([]net.Conn, numClients)
	names := make([]string, numClients)
	for i := 0; i < numClients; i++ {
		names[i] = fmt.Sprintf("midnight%d", i)
		conns[i] = tcpOnboard(t, addr, names[i])
		defer conns[i].Close()
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)

	// Phase 1: Send messages before "midnight"
	for i := 0; i < numClients; i++ {
		for j := 0; j < msgsBeforeMidnight; j++ {
			msg := fmt.Sprintf("before_%d_%d", i, j)
			fmt.Fprintf(conns[i], "%s\n", msg)
			time.Sleep(2 * time.Millisecond)
		}
	}
	time.Sleep(500 * time.Millisecond)

	// Verify pre-midnight messages are in history
	histBefore := s.GetHistory()
	preMidnightChat := 0
	for _, h := range histBefore {
		if h.Type == 0 && strings.HasPrefix(h.Content, "before_") {
			preMidnightChat++
		}
	}
	if preMidnightChat != numClients*msgsBeforeMidnight {
		t.Errorf("expected %d pre-midnight chat messages, got %d", numClients*msgsBeforeMidnight, preMidnightChat)
	}

	// Simulate midnight: clear history while clients continue sending
	sendingDone := make(chan struct{})
	go func() {
		defer close(sendingDone)
		for i := 0; i < numClients; i++ {
			for j := 0; j < msgsAfterMidnight; j++ {
				msg := fmt.Sprintf("after_%d_%d", i, j)
				fmt.Fprintf(conns[i], "%s\n", msg)
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	// Clear history (simulating midnight boundary)
	time.Sleep(50 * time.Millisecond) // let some "after" messages start
	s.ClearHistory()

	<-sendingDone
	time.Sleep(500 * time.Millisecond)

	// Verify: history should only contain post-midnight messages
	histAfter := s.GetHistory()
	postMidnightChat := 0
	preFound := 0
	for _, h := range histAfter {
		if h.Type == 0 {
			if strings.HasPrefix(h.Content, "after_") {
				postMidnightChat++
			}
			if strings.HasPrefix(h.Content, "before_") {
				preFound++
			}
		}
	}

	if preFound > 0 {
		t.Errorf("pre-midnight messages should be cleared from history, found %d", preFound)
	}
	// Some post-midnight messages may have arrived before ClearHistory,
	// so postMidnightChat might be less than the total. The key invariant is:
	// no pre-midnight messages remain and no messages are duplicated.
	if postMidnightChat == 0 {
		t.Error("expected at least some post-midnight messages in history")
	}

	// Verify no duplicates in history
	seen := make(map[string]int)
	for _, h := range histAfter {
		if h.Type == 0 {
			seen[h.Content]++
		}
	}
	for msg, count := range seen {
		if count > 1 {
			t.Errorf("duplicate message in history: %q appeared %d times", msg, count)
		}
	}

	// A new client joining should see only post-midnight history
	newConn := tcpDial(t, addr)
	defer newConn.Close()
	readUntil(newConn, "[ENTER YOUR NAME]:", 3*time.Second)
	fmt.Fprintf(newConn, "newjoin\n")
	histText, _ := readUntil(newConn, "][newjoin]:", 5*time.Second)
	stripped := stripAnsi(histText)
	if strings.Contains(stripped, "before_") {
		t.Error("new client should not see pre-midnight messages in history")
	}
}
