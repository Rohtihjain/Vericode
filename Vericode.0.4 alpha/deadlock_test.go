package main

import (
	"strings"
	"testing"
	"time"
)

// runSourceTimeout runs a program on another goroutine so a missed deadlock
// fails the test instead of hanging the suite.
func runSourceTimeout(t *testing.T, input string, d time.Duration) (string, error, bool) {
	t.Helper()
	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := runSource(t, input)
		ch <- result{out, err}
	}()
	select {
	case r := <-ch:
		return r.out, r.err, false
	case <-time.After(d):
		return "", nil, true
	}
}

func requireDeadlock(t *testing.T, input string, wants ...string) string {
	t.Helper()
	start := time.Now()
	_, err, hung := runSourceTimeout(t, input, 3*time.Second)
	if hung {
		t.Fatalf("deadlock was not detected; program still running after 3s\ninput:\n%s", input)
	}
	if err == nil {
		t.Fatalf("expected a deadlock error, got none\ninput:\n%s", input)
	}
	msg := err.Error()
	if !strings.Contains(msg, "deadlock") {
		t.Fatalf("error is not a deadlock: %s", msg)
	}
	for _, want := range wants {
		if !strings.Contains(msg, want) {
			t.Fatalf("deadlock report missing %q:\n%s", want, msg)
		}
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("deadlock detection took too long: %v", elapsed)
	}
	return msg
}

func TestDeadlockRecvWithNoSender(t *testing.T) {
	msg := requireDeadlock(t, `let ch = chan(); print recv(ch);`, "main: waiting on recv()")
	if !strings.Contains(msg, "no timers are pending") {
		t.Fatalf("report should explain the cause:\n%s", msg)
	}
}

func TestDeadlockSendWithNoReceiver(t *testing.T) {
	requireDeadlock(t, `send(chan(), 1);`, "main: waiting on send()")
}

func TestDeadlockAwaitingBlockedTask(t *testing.T) {
	msg := requireDeadlock(t, `
let ch = chan();
let worker = spawn func() { recv(ch); }();
await worker;
`, "main: waiting on await()", "task 'worker'", "recv()")
	if !strings.Contains(msg, "spawned at") {
		t.Fatalf("report should name where the blocked task came from:\n%s", msg)
	}
}

func TestDeadlockUnawaitedTask(t *testing.T) {
	requireDeadlock(t, `
let ch = chan();
let t = spawn func() { recv(ch); }();
print "main done";
`, "deadlock", "task 't'")
}

func TestDeadlockErrorIsLocated(t *testing.T) {
	msg := requireDeadlock(t, `let ch = chan(); recv(ch);`)
	if !strings.Contains(msg, "<input>:1:") || !strings.Contains(msg, "-->") {
		t.Fatalf("deadlock should carry a source location:\n%s", msg)
	}
}

func TestNoDeadlockBufferedChannel(t *testing.T) {
	out := mustRun(t, `
let ch = chan(2);
send(ch, 1);
send(ch, 2);
print recv(ch) + recv(ch);
`)
	if strings.TrimSpace(out) != "3" {
		t.Fatalf("got %q, want 3", out)
	}
}

func TestNoDeadlockWhileSleeping(t *testing.T) {
	// The send happens after sleep resumes; the pending timer must stop the
	// detector from declaring a deadlock while both VMs are blocked.
	out := mustRun(t, `
let ch = chan();
let t = spawn func() { sleep(80); send(ch, 7); }();
print recv(ch);
await t;
`)
	if strings.TrimSpace(out) != "7" {
		t.Fatalf("got %q, want 7", out)
	}
}

func TestNoDeadlockProducerConsumer(t *testing.T) {
	out := mustRun(t, `
let ch = chan(2);
let prod = spawn func() {
  loop i in range(0, 20) { send(ch, i); }
}();
let total = 0;
loop 20 times { total += recv(ch); }
await prod;
print total;
`)
	if strings.TrimSpace(out) != "190" {
		t.Fatalf("got %q, want 190", out)
	}
}

func TestDeadlockInsideParallelWorker(t *testing.T) {
	requireDeadlock(t, `
let never = chan();
pmap([1, 2, 3], func(x) { return recv(never); }, 2);
`, "deadlock", "parallel worker")
}
