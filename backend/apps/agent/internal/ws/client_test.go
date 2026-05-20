package ws

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestRunSessionExitsImmediatelyAfterReadLoop is a regression test for the
// 65–71 s probe-result lag we hit in dev. runSession used to block on
// <-hbDone until the heartbeat goroutine noticed the dead connection on its
// next ping tick (up to wstimeouts.PingInterval = 54 s). The fix is to cancel
// a session-scoped context as soon as readLoop returns so the hb goroutine
// wakes up immediately.
//
// We don't simulate the entire dial-and-reconnect cycle here — we just call
// runSession directly with a stub conn, close the conn from outside, and
// assert runSession returns within ~1 s instead of waiting out the 54 s
// PingInterval.
func TestRunSessionExitsImmediatelyAfterReadLoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		// Read once so the handshake completes, then forcibly close from the
		// server side — same as what a dying gateway would do.
		go func() {
			_, _, _ = conn.ReadMessage()
		}()
		// Wait briefly so the client's initial heartbeat lands before we yank
		// the conn (otherwise the client may not even open its hb goroutine).
		time.Sleep(50 * time.Millisecond)
		_ = conn.Close()
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	c := New(wsURL, "secret", "nd_test",
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Manually dial and inject the conn the way Run would have. We're not
	// driving Run() here because we want to assert *runSession* returns fast
	// independent of the outer reconnect loop.
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.runSession(context.Background())
	}()

	// The server closes the conn 50 ms after upgrade. Before the fix
	// runSession would still sit on <-hbDone until the 54 s ping tick.
	// Allow generous slack for slow CI but well under the regression window.
	select {
	case <-done:
		// pass
	case <-time.After(5 * time.Second):
		t.Fatalf("runSession did not exit within 5 s — hb goroutine still " +
			"holding the session open past the dead connection (regression " +
			"of the 65 s probe-result lag fix)")
	}
}

// TestRunSessionClosesConnOnExit asserts the second half of the same fix:
// when runSession returns it should have closed c.conn so the next Send
// fails fast instead of writing into a doomed socket and blocking up to
// wsWriteDeadline (10 s).
func TestRunSessionClosesConnOnExit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go func() { _, _, _ = conn.ReadMessage() }()
		time.Sleep(50 * time.Millisecond)
		_ = conn.Close()
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	c := New(wsURL, "secret", "nd_test",
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	var wg sync.WaitGroup
	wg.Go(func() {
		c.runSession(context.Background())
	})
	wg.Wait()

	// runSession must call closeConn before returning, leaving c.conn == nil.
	c.mu.Lock()
	got := c.conn
	c.mu.Unlock()
	if got != nil {
		t.Fatalf("expected c.conn == nil after runSession exit, got non-nil")
	}
}
