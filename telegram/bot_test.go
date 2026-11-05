package telegram

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"evsys/entity"
	"evsys/internal"

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

	// Start loading from the database
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			bot.mergeLoaded([]entity.UserSubscription{
				{UserID: 9000, SubscriptionType: "status"},
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
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

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

// fakeSubscriptions is the part of the database the bot's registration touches. The embedded nil
// interface panics on anything else, which keeps these tests honest about what they exercise.
type fakeSubscriptions struct {
	internal.Database
	added, deleted []int
	addErr         error
}

func (f *fakeSubscriptions) AddSubscription(s *entity.UserSubscription) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added = append(f.added, s.UserID)
	return nil
}

func (f *fakeSubscriptions) DeleteSubscription(s *entity.UserSubscription) error {
	f.deleted = append(f.deleted, s.UserID)
	return nil
}

func command(text string, from *tgbotapi.User, chatType string) *tgbotapi.Message {
	name := strings.Fields(text)[0]
	return &tgbotapi.Message{
		Text:     text,
		From:     from,
		Chat:     &tgbotapi.Chat{ID: 77, Type: chatType},
		Entities: &[]tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len(name)}},
	}
}

func TestRegistration(t *testing.T) {
	person := &tgbotapi.User{ID: 42, UserName: "op_erator"}
	tests := []struct {
		name       string
		invite     string
		message    *tgbotapi.Message
		subscribed bool
		replies    int
	}{
		{"valid invite", "s3cret", command("/start s3cret", person, "private"), true, 1},
		{"no code", "s3cret", command("/start", person, "private"), false, 0},
		{"wrong code", "s3cret", command("/start nope", person, "private"), false, 0},
		{"closed registration refuses the empty code", "", command("/start", person, "private"), false, 0},
		{"closed registration refuses any code", "", command("/start s3cret", person, "private"), false, 0},
		{"group chat", "s3cret", command("/start s3cret", person, "group"), false, 0},
		{"bot account", "s3cret", command("/start s3cret", &tgbotapi.User{ID: 42, UserName: "x", IsBot: true}, "private"), false, 0},
		{"no sender", "s3cret", command("/start s3cret", nil, "private"), false, 0},
		{"no username", "s3cret", command("/start s3cret", &tgbotapi.User{ID: 42}, "private"), true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot := testBot(func(int64, string) {})
			bot.inviteCode = tt.invite
			db := &fakeSubscriptions{}
			bot.database = db

			bot.handleCommand(tt.message)

			if got := bot.isSubscriber(42); got != tt.subscribed {
				t.Errorf("subscribed = %v, want %v", got, tt.subscribed)
			}
			if tt.subscribed && len(db.added) != 1 {
				t.Errorf("saved %d subscriptions, want 1", len(db.added))
			}
			if len(bot.send) != tt.replies {
				t.Errorf("%d replies, want %d", len(bot.send), tt.replies)
			}
		})
	}
}

func TestStrangersGetNoAnswerToCommands(t *testing.T) {
	stranger := &tgbotapi.User{ID: 42, UserName: "stranger"}
	for _, text := range []string{"/status", "/test", "/stop"} {
		bot := testBot(func(int64, string) {}, 1)
		bot.handleCommand(command(text, stranger, "private"))
		if len(bot.send) != 0 {
			t.Errorf("%s answered a non-subscriber", text)
		}
		if bot.subscriptionCount() != 1 {
			t.Errorf("%s changed the subscribers", text)
		}
	}
}

func TestSubscriberCommands(t *testing.T) {
	bot := testBot(func(int64, string) {}, 42)
	db := &fakeSubscriptions{}
	bot.database = db
	subscriber := &tgbotapi.User{ID: 42, UserName: "op"}

	bot.handleCommand(command("/test", subscriber, "private"))
	bot.handleCommand(command("/start", subscriber, "private"))
	if len(bot.send) != 2 {
		t.Fatalf("%d replies, want 2", len(bot.send))
	}
	if len(db.added) != 0 {
		t.Error("/start saved an existing subscriber again")
	}

	bot.handleCommand(command("/stop", subscriber, "private"))
	if bot.isSubscriber(42) || len(db.deleted) != 1 {
		t.Errorf("/stop: subscribed %v, deleted %v", bot.isSubscriber(42), db.deleted)
	}
}

// A failed save must not leave the user receiving alerts until the next restart with no record of it.
func TestFailedSaveDoesNotSubscribe(t *testing.T) {
	bot := testBot(func(int64, string) {})
	bot.inviteCode = "s3cret"
	bot.database = &fakeSubscriptions{addErr: errors.New("mongodb://user:pass@host refused")}

	bot.handleCommand(command("/start s3cret", &tgbotapi.User{ID: 42, UserName: "op"}, "private"))

	if bot.isSubscriber(42) {
		t.Error("subscribed despite the failed save")
	}
	reply := <-bot.send
	if strings.Contains(reply.Text, "mongodb") {
		t.Errorf("internal error reached the user: %q", reply.Text)
	}
}

func TestIsForbidden(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{tgbotapi.Error{Message: "Forbidden: bot was blocked by the user"}, true},
		{tgbotapi.Error{Message: "Forbidden: user is deactivated"}, true},
		{tgbotapi.Error{Message: "Bad Request: can't parse entities"}, false},
		{errors.New("Forbidden: not from the API"), false},
	}
	for _, tt := range tests {
		if got := isForbidden(tt.err); got != tt.want {
			t.Errorf("isForbidden(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestSanitizeEscapesEveryReservedCharacter(t *testing.T) {
	for _, char := range "_*[]()~`>#+-=|{}.!\\" {
		if got := sanitize(string(char)); got != "\\"+string(char) {
			t.Errorf("sanitize(%q) = %q", char, got)
		}
	}
}

// failingLoad is a database whose subscriptions become readable after a number of failed attempts.
type failingLoad struct {
	internal.Database
	mutex    sync.Mutex
	failures int
	calls    int
	result   []entity.UserSubscription
}

func (f *failingLoad) GetSubscriptions() ([]entity.UserSubscription, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.calls++
	if f.failures > 0 {
		f.failures--
		return nil, errors.New("server selection timeout")
	}
	return f.result, nil
}

func (f *failingLoad) callCount() int {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.calls
}

func (f *failingLoad) setFailures(n int) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.failures = n
}

// TestFailedLoadIsRetried is the regression for the silent bot: one failed read at startup used to
// leave it with no subscribers until the service was restarted.
func TestFailedLoadIsRetried(t *testing.T) {
	rec := newRecorder(false)
	bot := testBot(rec.deliver)
	bot.retryEvery = time.Millisecond
	db := &failingLoad{failures: 3, result: []entity.UserSubscription{{UserID: 1}, {UserID: 2}}}
	bot.database = db
	t.Cleanup(func() { close(bot.stop) })

	bot.loadSubscriptions()

	deadline := time.Now().Add(2 * time.Second)
	for bot.subscriptionCount() != 2 {
		if time.Now().After(deadline) {
			t.Fatalf("subscriptions were not loaded after %d attempts", db.callCount())
		}
		time.Sleep(time.Millisecond)
	}
	if calls := db.callCount(); calls != 4 {
		t.Errorf("attempts = %d, want 4", calls)
	}
}

// Nothing is retried when the first read works, and no notice goes out.
func TestSuccessfulLoadIsNotRetried(t *testing.T) {
	rec := newRecorder(false)
	bot := testBot(rec.deliver)
	bot.retryEvery = time.Millisecond
	db := &failingLoad{result: []entity.UserSubscription{{UserID: 1}}}
	bot.database = db
	t.Cleanup(func() { close(bot.stop) })

	bot.loadSubscriptions()
	time.Sleep(20 * time.Millisecond)

	if calls := db.callCount(); calls != 1 {
		t.Errorf("attempts = %d, want 1", calls)
	}
	if len(bot.event) != 0 {
		t.Errorf("%d events queued, want none", len(bot.event))
	}
}

// Events during the outage cannot reach anyone, so once the load succeeds the subscribers are told.
func TestSubscribersAreToldAboutMissedEvents(t *testing.T) {
	rec := newRecorder(false)
	bot := testBot(rec.deliver)
	bot.retryEvery = time.Hour // held back until the event below has been counted
	db := &failingLoad{failures: 1, result: []entity.UserSubscription{{UserID: 7}}}
	bot.database = db
	t.Cleanup(func() { close(bot.stop) })

	bot.loadSubscriptions()
	bot.countIfSubscriptionsMissing()
	bot.countIfSubscriptionsMissing()

	if !bot.tryLoadSubscriptions() {
		t.Fatal("second load failed")
	}
	select {
	case event := <-bot.event:
		if !strings.Contains(event.Text, "2 notifications") {
			t.Errorf("notice = %q, want the count of missed events", event.Text)
		}
	default:
		t.Fatal("no notice about the missed events")
	}

	bot.countIfSubscriptionsMissing()
	if len(bot.event) != 0 || bot.undelivered != 0 {
		t.Error("events after the load must not be counted as missed")
	}
}

// Registration works during the outage, and a late load must not forget who registered.
func TestLateLoadKeepsNewSubscribers(t *testing.T) {
	rec := newRecorder(false)
	bot := testBot(rec.deliver)
	bot.retryEvery = time.Hour
	bot.database = &failingLoad{failures: 1, result: []entity.UserSubscription{{UserID: 1}}}
	t.Cleanup(func() { close(bot.stop) })

	bot.loadSubscriptions()
	bot.addSubscription(entity.UserSubscription{UserID: 2})
	if !bot.tryLoadSubscriptions() {
		t.Fatal("second load failed")
	}

	if !bot.isSubscriber(1) || !bot.isSubscriber(2) {
		t.Errorf("subscribers = %v, want both 1 and 2", bot.subscribers())
	}
}

func TestStopEndsTheRetries(t *testing.T) {
	bot := testBot(newRecorder(false).deliver)
	bot.retryEvery = time.Millisecond
	db := &failingLoad{failures: 1 << 30}
	bot.database = db

	bot.loadSubscriptions()
	deadline := time.Now().Add(2 * time.Second)
	for db.callCount() < 3 {
		if time.Now().After(deadline) {
			t.Fatal("no retries happened")
		}
		time.Sleep(time.Millisecond)
	}
	close(bot.stop)
	time.Sleep(20 * time.Millisecond) // a retry already past the select may still finish
	before := db.callCount()
	time.Sleep(50 * time.Millisecond)
	if after := db.callCount(); after != before {
		t.Errorf("attempts went from %d to %d after Stop", before, after)
	}
}

func TestSplitMessage(t *testing.T) {
	t.Run("short text is left alone", func(t *testing.T) {
		text := "*PE00001*: Online\n\nend\n"
		if got := splitMessage(text, 100); len(got) != 1 || got[0] != text {
			t.Errorf("got %q", got)
		}
	})
	t.Run("prefers a blank line to a line end", func(t *testing.T) {
		got := splitMessage("aaa\nbbb\n\nccc\nddd", 12)
		if len(got) != 2 || got[0] != "aaa\nbbb" || got[1] != "ccc\nddd" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("falls back to a line end", func(t *testing.T) {
		got := splitMessage("aaa\nbbb\nccc", 8)
		if len(got) != 2 || got[0] != "aaa\nbbb" || got[1] != "ccc" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("cuts a line longer than the limit", func(t *testing.T) {
		got := splitMessage(strings.Repeat("x", 25), 10)
		if strings.Join(got, "") != strings.Repeat("x", 25) || len(got) != 3 {
			t.Errorf("got %q", got)
		}
	})
	t.Run("counts characters, not bytes", func(t *testing.T) {
		text := strings.Repeat("я", 10)
		if got := splitMessage(text, 10); len(got) != 1 {
			t.Errorf("got %d parts", len(got))
		}
		for _, part := range splitMessage(strings.Repeat("я", 25), 10) {
			if !utf8.ValidString(part) || utf8.RuneCountInString(part) > 10 {
				t.Errorf("bad part %q", part)
			}
		}
	})
	t.Run("keeps an escape sequence whole", func(t *testing.T) {
		for _, part := range splitMessage(strings.Repeat("\\.", 30), 11) {
			if (len(part)-len(strings.TrimRight(part, "\\")))%2 != 0 {
				t.Errorf("part ends inside an escape: %q", part)
			}
		}
	})
	t.Run("no empty parts", func(t *testing.T) {
		for _, part := range splitMessage("aaa\n\n\n\n\nbbb\n\n\n", 5) {
			if strings.TrimSpace(part) == "" {
				t.Errorf("empty part in %q", part)
			}
		}
	})
}

// statusDatabase is the part of the database /status reads.
type statusDatabase struct {
	internal.Database
	status []entity.ChargePointStatus
}

func (s *statusDatabase) GetLastStatus() ([]entity.ChargePointStatus, error) {
	return s.status, nil
}

// A status too long for one message must arrive whole, with every charge point next to its connectors.
func TestLongStatusFitsTelegramLimit(t *testing.T) {
	var status []entity.ChargePointStatus
	for i := 0; i < 80; i++ {
		status = append(status, entity.ChargePointStatus{
			ChargePointID: fmt.Sprintf("PE%05d", i),
			IsOnline:      true,
			EventTime:     time.Now(),
			Connectors: []entity.ConnectorStatus{
				{ConnectorID: 1, Status: "Faulted", StatusTime: time.Now(), Info: "Socket has an error. Charge is not possible"},
				{ConnectorID: 2, Status: "Available", StatusTime: time.Now()},
			},
		})
	}
	bot := testBot(func(int64, string) {}, 1)
	bot.database = &statusDatabase{status: status}

	chunks := splitMessage(bot.composeStatusMessage(), messageChunk)
	if len(chunks) < 2 {
		t.Fatalf("%d parts, the test needs a status that does not fit in one", len(chunks))
	}
	for _, chunk := range chunks {
		if n := utf8.RuneCountInString(chunk); n > 4096 {
			t.Errorf("part of %d characters", n)
		}
	}
	joined := strings.Join(chunks, "\n\n")
	for _, s := range status {
		if !strings.Contains(joined, "*"+s.ChargePointID+"*") {
			t.Errorf("%s is missing", s.ChargePointID)
		}
	}
	for _, chunk := range chunks[:len(chunks)-1] {
		if !strings.Contains(chunk[strings.LastIndex(chunk, "*PE"):], "Connector 2") {
			t.Errorf("a part ends in the middle of a charge point: ...%q", chunk[len(chunk)-60:])
		}
	}
	if !strings.Contains(chunks[len(chunks)-1], "Active subscriptions: 1") {
		t.Error("the closing line is lost")
	}
}

// fakeTelegram answers API requests in place of the network.
type fakeTelegram struct {
	mutex  sync.Mutex
	texts  []string
	answer func(attempt int) (string, error)
}

func (f *fakeTelegram) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := req.ParseForm(); err != nil {
		return nil, err
	}
	f.mutex.Lock()
	f.texts = append(f.texts, req.PostForm.Get("text"))
	attempt := len(f.texts)
	f.mutex.Unlock()
	body, err := f.answer(attempt)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

const telegramOK = `{"ok":true,"result":{"message_id":1}}`

func botWithFakeTelegram(answer func(attempt int) (string, error), subscribers ...int) (*TgBot, *fakeTelegram) {
	fake := &fakeTelegram{answer: answer}
	bot := testBot(nil, subscribers...)
	bot.api = &tgbotapi.BotAPI{Token: "test", Client: &http.Client{Transport: fake}}
	return bot, fake
}

// longText is numbered lines long enough for three parts.
func longText() string {
	var lines []string
	for i := 0; i < 1000; i++ {
		lines = append(lines, "line "+strconv.Itoa(i)+strings.Repeat("x", 5))
	}
	return strings.Join(lines, "\n")
}

func TestPartsAreSentInOrder(t *testing.T) {
	bot, fake := botWithFakeTelegram(func(int) (string, error) { return telegramOK, nil })
	bot.sendMessage(1, longText())

	if len(fake.texts) < 2 {
		t.Fatalf("%d requests, want several", len(fake.texts))
	}
	if got := strings.Join(fake.texts, "\n"); got != longText() {
		t.Error("parts do not add up to the original text in order")
	}
}

func TestShortMessageIsOneRequest(t *testing.T) {
	bot, fake := botWithFakeTelegram(func(int) (string, error) { return telegramOK, nil })
	bot.sendMessage(1, "*PE00001*: Online")
	if len(fake.texts) != 1 || fake.texts[0] != "*PE00001*: Online" {
		t.Errorf("requests = %q", fake.texts)
	}
}

// A user who blocked the bot is removed at the first part, and the rest are not tried.
func TestForbiddenStopsTheRemainingParts(t *testing.T) {
	bot, fake := botWithFakeTelegram(func(int) (string, error) {
		return `{"ok":false,"description":"Forbidden: bot was blocked by the user"}`, nil
	}, 1)
	bot.sendMessage(1, longText())

	if len(fake.texts) != 1 {
		t.Errorf("%d requests, want 1", len(fake.texts))
	}
	if bot.isSubscriber(1) {
		t.Error("the subscription was not removed")
	}
}

// A transport failure holds a delivery slot for up to apiTimeout, so it must not repeat per part.
func TestTransportErrorStopsTheRemainingParts(t *testing.T) {
	bot, fake := botWithFakeTelegram(func(int) (string, error) { return "", timeoutError{} })
	bot.sendMessage(1, longText())
	if len(fake.texts) != 1 {
		t.Errorf("%d requests, want 1", len(fake.texts))
	}
}

// Rejected markup costs only that part: it goes out plain, within the limit, and the rest follow.
func TestRejectedMarkupIsResentAndTheRestFollow(t *testing.T) {
	bot, fake := botWithFakeTelegram(func(attempt int) (string, error) {
		if attempt == 1 {
			return `{"ok":false,"description":"Bad Request: can't parse entities"}`, nil
		}
		return telegramOK, nil
	})
	parts := splitMessage(longText(), messageChunk)
	bot.sendMessage(1, longText())

	if want := len(parts) + 1; len(fake.texts) != want {
		t.Fatalf("%d requests, want %d", len(fake.texts), want)
	}
	if !strings.HasPrefix(fake.texts[1], "This message caused an error") {
		t.Errorf("second request = %.40q", fake.texts[1])
	}
	if n := utf8.RuneCountInString(fake.texts[1]); n > 4096 {
		t.Errorf("resent part has %d characters", n)
	}
}
