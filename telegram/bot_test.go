package telegram

import (
	"bytes"
	"errors"
	"log"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"evsys/entity"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// testBot builds a bot with the Telegram API left out. Everything under test here happens before a
// request is made, so deliver stands in for the API and no token or network is needed.
func testBot(deliver func(id int64, text string), subscribers ...int) *TgBot {
	subscriptions := make(map[int]entity.UserSubscription, len(subscribers))
	for _, id := range subscribers {
		subscriptions[id] = entity.UserSubscription{UserID: id, SubscriptionType: "status"}
	}
	return &TgBot{
		subscriptions: subscriptions,
		event:         make(chan MessageContent, 100),
		send:          make(chan MessageContent, 100),
		stop:          make(chan struct{}),
		slots:         make(chan struct{}, deliverySlots),
		deliver:       deliver,
	}
}

// recorder counts deliveries and can hold each one open, standing in for a Telegram request that
// has connected and is waiting on a reply that never comes.
type recorder struct {
	mu       sync.Mutex
	ids      []int64
	entered  chan struct{}
	release  chan struct{}
	blocking bool
}

func newRecorder(blocking bool) *recorder {
	return &recorder{
		entered:  make(chan struct{}, 64),
		release:  make(chan struct{}),
		blocking: blocking,
	}
}

func (r *recorder) deliver(id int64, _ string) {
	r.mu.Lock()
	r.ids = append(r.ids, id)
	r.mu.Unlock()
	r.entered <- struct{}{}
	if r.blocking {
		<-r.release
	}
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ids)
}

func (r *recorder) delivered() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.ids...)
}

// waitForEntries waits until n deliveries have started.
func (r *recorder) waitForEntries(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-r.entered:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of %d deliveries started", i, n)
		}
	}
}

/*
TestSubscriptionsAreSafeUnderConcurrency is the regression for the crash. The subscription map was
read by the event pump while the updates pump added and removed entries, with nothing between them:
a /start arriving as an alert fanned out is a concurrent map read and write, which Go answers with a
fatal error that takes down the central system - every charge point dropped because one operator
subscribed at the wrong moment.

Run under -race. Without the accessors the detector reports the write against the range loop in
subscribers, and the test can also simply crash, which is the point.
*/
func TestSubscriptionsAreSafeUnderConcurrency(t *testing.T) {
	bot := testBot(func(int64, string) {})

	var wg sync.WaitGroup
	const rounds = 300

	// the updates pump: subscriptions coming and going
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			bot.addSubscription(entity.UserSubscription{UserID: i, SubscriptionType: "status"})
			if i%3 == 0 {
				bot.removeSubscription(i - 1)
			}
		}
	}()

	// the event pump: walking the subscribers to fan an alert out
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			for _, subscription := range bot.subscribers() {
				_ = subscription.UserID
			}
		}
	}()

	// the /status command, which reports the count
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			_ = bot.subscriptionCount()
		}
	}()

	// Start reloading the whole set from the database
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			bot.setSubscriptions(map[int]entity.UserSubscription{
				9000: {UserID: 9000, SubscriptionType: "status"},
			})
		}
	}()

	wg.Wait()

	if bot.subscriptionCount() != len(bot.subscribers()) {
		t.Error("count and snapshot disagree about the subscriber set")
	}
}

// TestEventReachesEverySubscriber pins the fan-out itself: the point of the pump is that one alert
// goes to everyone who asked for alerts.
func TestEventReachesEverySubscriber(t *testing.T) {
	rec := newRecorder(false)
	bot := testBot(rec.deliver, 1, 2, 3)
	go bot.eventPump()
	t.Cleanup(func() { close(bot.stop) })

	bot.event <- MessageContent{Text: "PE00004 Faulted"}
	rec.waitForEntries(t, 3)

	seen := map[int64]bool{}
	for _, id := range rec.delivered() {
		seen[id] = true
	}
	for _, want := range []int64{1, 2, 3} {
		if !seen[want] {
			t.Errorf("subscriber %d was not notified", want)
		}
	}
}

/*
TestEventPumpIsNotBlockedByASlowSend is the regression for the stall. The pump used to call the API
synchronously, and the library's client had no timeout, so one connection that hung held up every
later alert behind it - the failure mode where a charge point faults, the notification is queued
behind a wedged request, and nothing reaches the operator at all.
*/
func TestEventPumpIsNotBlockedByASlowSend(t *testing.T) {
	rec := newRecorder(true)
	bot := testBot(rec.deliver, 1)
	go bot.eventPump()
	t.Cleanup(func() {
		close(rec.release)
		close(bot.stop)
	})

	// the first alert wedges mid-send; the two behind it must still get out
	bot.event <- MessageContent{Text: "first"}
	bot.event <- MessageContent{Text: "second"}
	bot.event <- MessageContent{Text: "third"}

	rec.waitForEntries(t, 3)
}

/*
TestDispatchDropsWhenSlotsAreExhausted guards the other half of that fix. Not blocking is only safe
if it cannot spawn goroutines without limit: an unreachable Telegram would otherwise accumulate one
per alert per subscriber for as long as the outage lasts. Over the limit a send is dropped, and the
drop is logged - a stalled queue leaves nothing behind at all.
*/
func TestDispatchDropsWhenSlotsAreExhausted(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(nil) })

	rec := newRecorder(true)
	bot := testBot(rec.deliver)
	t.Cleanup(func() { close(rec.release) })

	// occupy every slot, then ask for one more
	for i := 0; i < deliverySlots; i++ {
		bot.dispatch(int64(i), "wedged")
	}
	rec.waitForEntries(t, deliverySlots)
	bot.dispatch(999, "over the limit")

	// the dropped send must not have started, now or later
	time.Sleep(50 * time.Millisecond)
	if got := rec.count(); got != deliverySlots {
		t.Errorf("%d deliveries started, want %d: the send over the limit was not dropped", got, deliverySlots)
	}
	if !bytes.Contains(logs.Bytes(), []byte("dropped a notification for 999")) {
		t.Errorf("a dropped notification must be logged, got %q", logs.String())
	}

	// and a slot freed by a finished send is usable again. Releasing the send only unblocks it;
	// the slot comes back when its goroutine returns, so wait for that rather than assume it.
	rec.release <- struct{}{}
	deadline := time.Now().Add(2 * time.Second)
	for len(bot.slots) >= deliverySlots && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	bot.dispatch(1000, "after a slot frees")
	rec.waitForEntries(t, 1)
}

// TestApiTimeoutOutlastsTheLongPoll pins the trap in bounding the API client. The same client serves
// the getUpdates long poll, so a timeout at or below that interval would cut every poll short and
// leave the bot reconnecting instead of receiving commands.
func TestApiTimeoutOutlastsTheLongPoll(t *testing.T) {
	longPoll := time.Duration(updatesLongPoll) * time.Second
	if apiTimeout <= longPoll {
		t.Fatalf("apiTimeout %s must exceed the %s long poll, or getUpdates is cut short every time",
			apiTimeout, longPoll)
	}
}

// TestIsTransportError pins which failures skip the retry. Getting this backwards either retries a
// timeout three times, multiplying the stall the timeout was added to bound, or stops retrying the
// markup failure the retry exists for.
func TestIsTransportError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "client timeout",
			err:  &url.Error{Op: "Post", URL: "https://api.telegram.org", Err: timeoutError{}},
			want: true,
		},
		{
			name: "connection refused",
			err:  &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			want: true,
		},
		{
			name: "telegram rejecting the markup",
			err:  tgbotapi.Error{Message: "Bad Request: can't parse entities"},
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("something else"),
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isTransportError(test.err); got != test.want {
				t.Errorf("isTransportError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

// timeoutError is a net.Error that reports a timeout, as http.Client's own deadline error does.
type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout awaiting response headers" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
