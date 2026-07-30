package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"evsys/internal/config"
	"evsys/ocpp"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

/*
Tests for the WebSocket keepalive. They run over a real socket on both ends, because what is under
test is the interaction between our ping ticker, our read deadline and a peer that may or may not
answer - none of which exists above the connection.

The windows come from fields on Server rather than the constants, so these run on a millisecond
timescale. The behaviour is the same as production; only the clock differs.
*/

// eventLogger records FeatureEvent and Warn lines. Both are written from the connection's pumps, so
// every field is guarded.
type eventLogger struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLogger) FeatureEvent(feature, id, text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, feature+" "+id+" "+text)
}
func (l *eventLogger) RawDataEvent(_, _ string) {}
func (l *eventLogger) Debug(m string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, m)
}
func (l *eventLogger) Error(_ string, _ error) {}
func (l *eventLogger) Warn(m string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, m)
}
func (l *eventLogger) has(substr string) bool {
	return l.count(substr) > 0
}

func (l *eventLogger) count(substr string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, e := range l.events {
		if strings.Contains(e, substr) {
			n++
		}
	}
	return n
}

// statusRecorder captures the online status the pool reports for each charge point. The pool calls
// it from its own goroutine, hence the mutex.
type statusRecorder struct {
	mu       sync.Mutex
	statuses []bool
}

func (r *statusRecorder) OnOnlineStatusChanged(_ string, isOnline bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statuses = append(r.statuses, isOnline)
}

func (r *statusRecorder) wentOffline() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.statuses {
		if !s {
			return true
		}
	}
	return false
}

// keepaliveServer starts a websocket server with the keepalive windows shortened to the given
// values, and returns it with the URL a client should dial.
func keepaliveServer(t *testing.T, pingPeriod, pongWait, silenceWait time.Duration) (*Server, *eventLogger, *statusRecorder, string) {
	t.Helper()

	logger := &eventLogger{}
	watchdog := &statusRecorder{}

	s := NewServer(&config.Config{}, logger)
	s.SetWatchdog(watchdog)
	s.SetMessageHandler(func(_ ocpp.WebSocket, _ []byte) error { return nil })
	s.pingPeriod = pingPeriod
	s.pongWait = pongWait
	s.silenceWait = silenceWait

	router := httprouter.New()
	s.Register(router)
	ts := httptest.NewServer(router)
	t.Cleanup(func() {
		ts.Close()
		s.pool.Stop()
	})

	return s, logger, watchdog, "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/CP1"
}

// dial connects as a charge point would. pongs decides whether this peer answers pings, which is
// what distinguishes an OCPP stack that implements pongs from one that does not.
func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Sec-WebSocket-Protocol": []string{"ocpp1.6"}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// readUntilClosed drains the connection so control frames are processed, and reports when the
// server closes it. A client that is not reading never sees a ping and so never pongs.
func readUntilClosed(conn *websocket.Conn) <-chan struct{} {
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	return closed
}

func registered(s *Server, id string) bool {
	s.pool.mutex.Lock()
	defer s.pool.mutex.Unlock()
	_, ok := s.pool.clients[id]
	return ok
}

func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}

// TestKeepalivePingsIdleConnection pins the ping half. Without it nothing traverses an idle
// connection, which is how the NAT mappings in front of this fleet were expiring unnoticed - and
// the read deadline would be a plain timer rather than a liveness check.
func TestKeepalivePingsIdleConnection(t *testing.T) {
	_, _, _, url := keepaliveServer(t, 20*time.Millisecond, 2*time.Second, 4*time.Second)
	conn := dial(t, url)

	var mu sync.Mutex
	pings := 0
	conn.SetPingHandler(func(data string) error {
		mu.Lock()
		pings++
		mu.Unlock()
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})
	readUntilClosed(conn)

	waitFor(t, "the server to ping an idle connection at least twice", 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return pings >= 2
	})
}

// TestConnectionSilentAfterPongIsClosed is the case the whole change exists for: a peer that has
// gone away without closing the socket. It answers one ping, proving it speaks pongs, then stops.
func TestConnectionSilentAfterPongIsClosed(t *testing.T) {
	s, logger, watchdog, url := keepaliveServer(t, 20*time.Millisecond, 150*time.Millisecond, 10*time.Second)
	conn := dial(t, url)

	var once sync.Once
	conn.SetPingHandler(func(data string) error {
		// exactly one pong, then silence: enough to earn the short window and then outstay it
		var err error
		once.Do(func() {
			err = conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
		})
		return err
	})
	closed := readUntilClosed(conn)

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("the server never closed a connection that stopped answering")
	}

	waitFor(t, "the connection to be unregistered", time.Second, func() bool { return !registered(s, "CP1") })
	waitFor(t, "the charge point to be reported offline", time.Second, watchdog.wentOffline)

	if !logger.has("no answer to keepalive pings") {
		t.Error("a connection closed by our own deadline should say so, to be greppable apart from a transport failure")
	}
}

// TestConnectionThatNeverPongsGetsTheLongWindow guards the risk the short window introduces. Not
// every OCPP stack answers pings, and for one that does not, being quiet for two minutes is normal:
// it may have nothing to say until its next heartbeat. Closing those on the short window would turn
// a keepalive into a fleet-wide disconnect loop.
func TestConnectionThatNeverPongsGetsTheLongWindow(t *testing.T) {
	pongWait := 100 * time.Millisecond
	s, _, _, url := keepaliveServer(t, 20*time.Millisecond, pongWait, 1200*time.Millisecond)
	conn := dial(t, url)

	// a peer that reads pings and ignores them, as a stack without pong support does
	conn.SetPingHandler(func(string) error { return nil })
	closed := readUntilClosed(conn)

	select {
	case <-closed:
		t.Fatal("a connection that never pongs was closed on the short window")
	case <-time.After(4 * pongWait):
	}
	if !registered(s, "CP1") {
		t.Fatal("connection unregistered while still inside its long window")
	}

	// bounded all the same: silence that outlasts the long window is still silence
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("the long window never expired")
	}
}

// TestIncomingMessageRefreshesTheDeadline pins that traffic, not just pongs, counts as liveness. A
// charge point mid-transaction sends meter values every 15 seconds and may never pong; if only
// pongs refreshed the deadline, the keepalive would disconnect the busiest chargers.
func TestIncomingMessageRefreshesTheDeadline(t *testing.T) {
	window := 200 * time.Millisecond
	s, _, _, url := keepaliveServer(t, time.Hour, window, window)
	conn := dial(t, url)

	conn.SetPingHandler(func(string) error { return nil })
	closed := readUntilClosed(conn)

	// keep talking for several windows; a refreshed deadline never fires
	deadline := time.Now().Add(5 * window)
	for time.Now().Before(deadline) {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`[2,"1","Heartbeat",{}]`)); err != nil {
			t.Fatalf("write: %v", err)
		}
		time.Sleep(window / 4)
	}
	select {
	case <-closed:
		t.Fatal("a connection sending messages was closed by the read deadline")
	default:
	}
	if !registered(s, "CP1") {
		t.Fatal("a connection sending messages was unregistered")
	}

	// and once it stops talking, the window applies as normal
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("the read deadline never fired after the messages stopped")
	}
}

// TestWritePumpExitsWhenSendChannelCloses covers the exit the ping ticker made reachable. The pool
// closes a client's send channel when its buffer overflows or when the server shuts down; the pump
// used to break out of the select instead of returning, so it spun on the closed channel forever,
// burning a core and writing close frames until the process died.
//
// It has to assert on the pump's own exit. Unregistration is not evidence: the peer answers our
// close frame with its own, which ends the read pump and unregisters the connection whether or not
// the write pump is still spinning behind it.
func TestWritePumpExitsWhenSendChannelCloses(t *testing.T) {
	s, logger, watchdog, url := keepaliveServer(t, time.Hour, time.Hour, time.Hour)
	conn := dial(t, url)
	readUntilClosed(conn)

	waitFor(t, "the connection to register", time.Second, func() bool { return registered(s, "CP1") })

	s.pool.mutex.Lock()
	client := s.pool.clients["CP1"]
	s.pool.mutex.Unlock()
	close(client.send)

	waitFor(t, "the write pump to stop", 2*time.Second, func() bool { return logger.has("write pump stopped") })
	waitFor(t, "the connection to be unregistered", 2*time.Second, func() bool { return !registered(s, "CP1") })
	waitFor(t, "the charge point to be reported offline", time.Second, watchdog.wentOffline)

	// a pump that exited once cannot exit again; more than one means it went round the loop
	if n := logger.count("write pump stopped"); n != 1 {
		t.Errorf("write pump stopped %d times, want exactly 1", n)
	}
}
