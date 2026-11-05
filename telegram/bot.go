package telegram

import (
	"crypto/subtle"
	"errors"
	"evsys/entity"
	"evsys/internal"
	"evsys/utility"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strings"
)

const (
	// updatesLongPoll is the timeout, in seconds, this bot asks Telegram to hold a getUpdates
	// request open for.
	updatesLongPoll = 60
	// apiTimeout bounds a single call to the Telegram API. The library defaults to an
	// http.Client with no timeout at all, which lets one wedged connection park a send for as
	// long as the network keeps the socket open.
	//
	// It must stay above updatesLongPoll: the same client serves the long-poll for updates, and a
	// timeout below that interval would cut every poll short and turn the bot into a reconnect
	// loop. TestApiTimeoutOutlastsTheLongPoll guards that.
	apiTimeout = 90 * time.Second
	// deliverySlots bounds how many sends may be in flight at once. Telegram being unreachable
	// must not cost more than this many goroutines, and an operator alert is worth nothing by the
	// time a wedged queue drains, so sends over the limit are dropped and logged rather than
	// queued behind a stall.
	deliverySlots = 8
	// subscriptionsRetry is how often a failed load of the subscriptions is tried again. Until one
	// succeeds nobody is known to alert, so this bounds how long the bot stays silent after the
	// database comes back.
	subscriptionsRetry = 30 * time.Second
)

// TgBot implements EventHandler
type TgBot struct {
	api      *tgbotapi.BotAPI
	database internal.Database
	// subscriptions is read by the event pump while the updates pump adds and removes entries, so
	// every access goes through the accessors below. An unsynchronised map here was not merely
	// racy: a /start arriving while an event was being fanned out is a concurrent map read and
	// write, which is fatal for the whole process, not just for the bot.
	subscriptions map[int]entity.UserSubscription
	mutex         sync.RWMutex
	// subscriptionsMissing is set while the load at startup keeps failing, and undelivered counts
	// the events that went past the subscribers nobody could read. Both are guarded by mutex.
	subscriptionsMissing bool
	undelivered          int
	// retryEvery is subscriptionsRetry, a field so tests need not wait for it.
	retryEvery time.Duration
	event      chan MessageContent
	send       chan MessageContent
	stop       chan struct{}
	// slots hands out the delivery permits counted by deliverySlots.
	slots chan struct{}
	// inviteCode is what /start must carry to subscribe. Empty closes registration.
	inviteCode string
	// deliver performs one send. It is a field so tests can drive the pumps without the Telegram
	// API; production always uses sendMessage.
	deliver func(id int64, text string)
}

type MessageContent struct {
	ChatID int64
	Text   string
}

func NewBot(apiKey, inviteCode string) (*TgBot, error) {
	tgBot := &TgBot{
		inviteCode:    inviteCode,
		subscriptions: make(map[int]entity.UserSubscription),
		event:         make(chan MessageContent, 100),
		send:          make(chan MessageContent, 100),
		stop:          make(chan struct{}),
		slots:         make(chan struct{}, deliverySlots),
		retryEvery:    subscriptionsRetry,
	}
	tgBot.deliver = tgBot.sendMessage
	api, err := tgbotapi.NewBotAPIWithClient(apiKey, &http.Client{Timeout: apiTimeout})
	if err != nil {
		return nil, err
	}
	tgBot.api = api
	return tgBot, nil
}

// mergeLoaded adds what the database returned and ends the outage, reporting how many events it
// cost. It merges rather than replaces: a late load must not forget someone who registered while it
// was failing.
func (b *TgBot) mergeLoaded(subscriptions []entity.UserSubscription) int {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for _, subscription := range subscriptions {
		b.subscriptions[subscription.UserID] = subscription
	}
	undelivered := b.undelivered
	b.subscriptionsMissing = false
	b.undelivered = 0
	return undelivered
}

func (b *TgBot) markSubscriptionsMissing() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.subscriptionsMissing = true
}

// countIfSubscriptionsMissing records an event that could not reach the subscribers nobody could read.
func (b *TgBot) countIfSubscriptionsMissing() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.subscriptionsMissing {
		b.undelivered++
		log.Printf("bot: subscriptions are not loaded, a notification did not reach them (%d so far)", b.undelivered)
	}
}

func (b *TgBot) addSubscription(subscription entity.UserSubscription) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.subscriptions[subscription.UserID] = subscription
}

func (b *TgBot) removeSubscription(userID int) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	delete(b.subscriptions, userID)
}

// subscribers returns a snapshot of the current subscribers. A copy, deliberately: the caller sends
// network requests while walking it, and holding the lock across those would block every /start and
// /stop for as long as Telegram takes to answer.
func (b *TgBot) subscribers() []entity.UserSubscription {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	list := make([]entity.UserSubscription, 0, len(b.subscriptions))
	for _, subscription := range b.subscriptions {
		list = append(list, subscription)
	}
	return list
}

func (b *TgBot) isSubscriber(userID int) bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	_, ok := b.subscriptions[userID]
	return ok
}

func (b *TgBot) subscriptionCount() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return len(b.subscriptions)
}

// SetDatabase attach database service
func (b *TgBot) SetDatabase(database internal.Database) {
	b.database = database
}

func (b *TgBot) Start() {
	b.loadSubscriptions()
	go b.sendPump()
	go b.eventPump()
	go b.updatesPump()
}

// loadSubscriptions reads the subscribers, and keeps trying in the background if that fails. The bot
// has no other record of who to alert, so giving up here means silence until someone restarts the
// service, and silence looks exactly like a network with nothing to report.
func (b *TgBot) loadSubscriptions() {
	if b.database == nil {
		return
	}
	if b.tryLoadSubscriptions() {
		return
	}
	b.markSubscriptionsMissing()
	go b.retryLoadSubscriptions()
}

func (b *TgBot) tryLoadSubscriptions() bool {
	subscriptions, err := b.database.GetSubscriptions()
	if err != nil {
		log.Printf("bot: error getting subscriptions, nobody receives alerts until this succeeds: %v", err)
		return false
	}
	undelivered := b.mergeLoaded(subscriptions)
	log.Printf("bot: loaded %d subscriptions", len(subscriptions))
	if undelivered > 0 {
		b.announceUndelivered(undelivered)
	}
	return true
}

func (b *TgBot) retryLoadSubscriptions() {
	ticker := time.NewTicker(b.retryEvery)
	defer ticker.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-ticker.C:
			if b.tryLoadSubscriptions() {
				return
			}
		}
	}
}

// announceUndelivered tells subscribers that alerts were missed: what was lost is not resent, so they
// must know to look.
func (b *TgBot) announceUndelivered(count int) {
	text := sanitize(fmt.Sprintf("Subscriptions could not be read at startup, %d notifications were not delivered. Check /status.", count))
	select {
	case b.event <- MessageContent{Text: text}:
	case <-b.stop:
	}
}

// Start listening for updates
func (b *TgBot) updatesPump() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = updatesLongPoll
	updates, err := b.api.GetUpdatesChan(u)
	if err != nil {
		log.Printf("bot: error getting updates: %v", err)
		return
	}
	for update := range updates {
		if update.Message == nil || !update.Message.IsCommand() {
			continue
		}
		b.handleCommand(update.Message)
	}
}

// handleCommand answers one command. Alerts carry user names, RFID tags and the state of the network,
// so everything but /stop is for subscribers, and only an invitation makes a subscriber. Anyone else
// gets no reply: the bot's name is public, and an answer only tells a scanner that it found a live bot.
func (b *TgBot) handleCommand(message *tgbotapi.Message) {
	if message.From == nil || message.From.IsBot || message.Chat == nil || !message.Chat.IsPrivate() {
		return
	}
	userID := message.From.ID
	chatID := message.Chat.ID
	switch message.Command() {
	case "start":
		if b.isSubscriber(userID) {
			b.send <- MessageContent{ChatID: chatID, Text: "You are already subscribed"}
			return
		}
		if !b.validInvite(message.CommandArguments()) {
			log.Printf("bot: registration refused for user %v", userID)
			return
		}
		b.subscribe(message.From, chatID)
	case "stop":
		if !b.isSubscriber(userID) {
			return
		}
		b.unsubscribe(userID)
		b.send <- MessageContent{ChatID: chatID, Text: "Your subscription has been removed"}
	case "test":
		if !b.isSubscriber(userID) {
			return
		}
		msg := fmt.Sprintf("*%v*: Connector %v: `%v`", "ChargePointId", 1, "Status")
		b.send <- MessageContent{ChatID: chatID, Text: msg}
	case "status":
		if !b.isSubscriber(userID) {
			return
		}
		b.send <- MessageContent{ChatID: chatID, Text: b.composeStatusMessage()}
	}
}

// validInvite reports whether code opens registration. An empty invite code closes it.
func (b *TgBot) validInvite(code string) bool {
	if b.inviteCode == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(code), []byte(b.inviteCode)) == 1
}

// subscribe saves the subscription first, so a failed save cannot leave a user receiving alerts until
// the next restart without being recorded anywhere.
func (b *TgBot) subscribe(user *tgbotapi.User, chatID int64) {
	// the store refuses an empty name, and a Telegram username is optional
	name := user.UserName
	if name == "" {
		name = fmt.Sprintf("id%d", user.ID)
	}
	subscription := entity.UserSubscription{UserID: user.ID, User: name, SubscriptionType: "status"}
	if b.database != nil {
		if err := b.database.AddSubscription(&subscription); err != nil {
			log.Printf("bot: error adding subscription for user %v: %v", user.ID, err)
			b.send <- MessageContent{ChatID: chatID, Text: "Subscription failed, please contact the administrator"}
			return
		}
	}
	b.addSubscription(subscription)
	log.Printf("bot: user %v (%v) subscribed", user.ID, name)
	b.send <- MessageContent{ChatID: chatID, Text: fmt.Sprintf("Hello *%v*, you are now subscribed to updates", sanitize(name))}
}

func (b *TgBot) unsubscribe(userID int) {
	b.removeSubscription(userID)
	if b.database != nil {
		if err := b.database.DeleteSubscription(&entity.UserSubscription{UserID: userID}); err != nil {
			log.Printf("bot: error deleting subscription for user %v: %v", userID, err)
		}
	}
	log.Printf("bot: user %v unsubscribed", userID)
}

// eventPump sending events to all subscribers
func (b *TgBot) eventPump() {
	for {
		select {
		case <-b.stop:
			return
		case event, ok := <-b.event:
			if !ok {
				return
			}
			// dispatched rather than sent from here: this pump is the only path alerts take, so a
			// send that blocks on it delays every later alert - which is how a fault notification
			// arrives after the shift that needed it, or never
			b.countIfSubscriptionsMissing()
			for _, subscription := range b.subscribers() {
				b.dispatch(int64(subscription.UserID), event.Text)
			}
		}
	}
}

// sendPump sending messages to users
func (b *TgBot) sendPump() {
	for {
		select {
		case <-b.stop:
			return
		case event, ok := <-b.send:
			if !ok {
				return
			}
			b.dispatch(event.ChatID, event.Text)
		}
	}
}

// dispatch delivers one message without blocking the caller, and gives up if the permitted number
// of sends is already in flight. Dropping is the lesser evil: the alternative is a queue that grows
// while Telegram is unreachable and then delivers a burst of stale alerts, and a drop leaves a line
// in the log where a stalled queue leaves nothing at all.
func (b *TgBot) dispatch(id int64, text string) {
	select {
	case b.slots <- struct{}{}:
	default:
		log.Printf("bot: dropped a notification for %v, %d sends already in flight", id, deliverySlots)
		return
	}
	go func() {
		defer func() { <-b.slots }()
		b.deliver(id, text)
	}()
}

// sendMessage common routine to send a message via bot API
func (b *TgBot) sendMessage(id int64, text string) {
	msg := tgbotapi.NewMessage(id, text)
	msg.ParseMode = "MarkdownV2"
	_, err := b.api.Send(msg)
	if err == nil {
		return
	}
	// Forbidden is final: the user blocked the bot or no longer exists, and every later alert would
	// fail the same way.
	if isForbidden(err) {
		log.Printf("bot: sending message to %v: %v", id, err)
		if b.isSubscriber(int(id)) {
			b.unsubscribe(int(id))
		}
		return
	}
	// The retries below exist for one failure: Telegram refusing the markup. A transport failure
	// tells us nothing about the text, and retrying it only multiplies a timeout by the number of
	// attempts - three wedged requests instead of one, all of them holding a delivery slot.
	if isTransportError(err) {
		log.Printf("bot: sending message to %v: %v", id, err)
		return
	}
	safeMsg := tgbotapi.NewMessage(id, fmt.Sprintf("This message caused an error:\n%v", removeMarkup(text)))
	_, err = b.api.Send(safeMsg)
	if err != nil {
		log.Printf("bot: error sending unmarkuped message: %v", err)
	}
}

// isForbidden reports whether Telegram refused delivery to this user for good.
func isForbidden(err error) bool {
	var apiErr tgbotapi.Error
	return errors.As(err, &apiErr) && strings.HasPrefix(apiErr.Message, "Forbidden:")
}

// isTransportError reports whether the send failed before Telegram had an opinion about it - a
// timeout, a refused connection, a DNS failure. Errors from the API itself, markup complaints
// among them, arrive as tgbotapi.Error and are not transport errors.
func isTransportError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr)
}

func (b *TgBot) OnStatusNotification(event *internal.EventMessage) {
	// only send notifications about Faulted status
	if event.Status != "Faulted" {
		return
	}
	var msg string
	if event.ConnectorId == 0 {
		msg = fmt.Sprintf("*%v*: `%v`\n", event.ChargePointId, event.Status)
	} else {
		msg = fmt.Sprintf("*%v*: Connector %v: `%v`\n", event.ChargePointId, event.ConnectorId, event.Status)
		if event.TransactionId >= 0 {
			msg += fmt.Sprintf("Transaction ID: %v\n", event.TransactionId)
		}
	}
	if event.Info != "" {
		msg += fmt.Sprintf("%v\n", sanitize(event.Info))
	}
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnTransactionStart(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: Connector %v: `%v`\n", event.ChargePointId, event.ConnectorId, event.Status)
	msg += fmt.Sprintf("Transaction ID: %v START\n", event.TransactionId)
	msg += fmt.Sprintf("User: %v\n", sanitize(event.Username))
	msg += fmt.Sprintf("ID Tag: %v\n", sanitize(event.IdTag))
	if event.Info != "" {
		msg += fmt.Sprintf("%v\n", sanitize(event.Info))
	}
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnTransactionStop(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: Connector %v: `%v`\n", event.ChargePointId, event.ConnectorId, event.Status)
	msg += fmt.Sprintf("Transaction ID: %v STOP\n", event.TransactionId)
	msg += fmt.Sprintf("User: %v\n", sanitize(event.Username))
	msg += fmt.Sprintf("ID Tag: %v\n", sanitize(event.IdTag))
	msg += fmt.Sprintf("Info: %v\n", sanitize(event.Info))
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnTransactionEvent(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: Connector %v: `%v`\n", event.ChargePointId, event.ConnectorId, event.Status)
	msg += fmt.Sprintf("Transaction ID: %v ACTIVE\n", event.TransactionId)
	msg += fmt.Sprintf("User: %v\n", sanitize(event.Username))
	msg += fmt.Sprintf("ID Tag: %v\n", sanitize(event.IdTag))
	msg += fmt.Sprintf("Info: %v\n", sanitize(event.Info))
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnAuthorize(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: user: `%v`\n", sanitize(event.ChargePointId), event.IdTag)
	msg += fmt.Sprintf("Auth status: %v\n", event.Status)
	if event.Username != "" {
		msg += fmt.Sprintf("User: %v\n", sanitize(event.Username))
	}
	if event.Info != "" {
		msg += fmt.Sprintf("%v\n", sanitize(event.Info))
	}
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnAlert(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*:", event.ChargePointId)
	if event.ConnectorId > 0 {
		msg += fmt.Sprintf(" Connector: %v", event.ConnectorId)
	}
	msg += " `ALERT`\n"
	if event.TransactionId > 0 {
		msg += fmt.Sprintf("Transaction ID: %v\n", event.TransactionId)
	}
	if event.Username != "" {
		msg += fmt.Sprintf("User: %v\n", sanitize(event.Username))
	}
	if event.IdTag != "" {
		msg += fmt.Sprintf("ID Tag: %v\n", sanitize(event.IdTag))
	}
	msg += fmt.Sprintf("%v", sanitize(event.Info))
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnInfo(event *internal.EventMessage) {
	msg := fmt.Sprintf("%v", sanitize(event.Info))
	b.event <- MessageContent{Text: msg}
}

// compose status message
func (b *TgBot) composeStatusMessage() string {
	msg := "Status info:\n"
	msg += "\n"
	if b.database != nil {
		status, err := b.database.GetLastStatus()
		if err != nil {
			log.Printf("bot: error getting last status: %v", err)
			msg += "Status is not available right now\n"
		} else {
			for _, s := range status {
				msg += fmt.Sprintf("*%v*: ", s.ChargePointID)
				if s.IsOnline {
					msg += "Online"
				} else {
					msg += "*OFFLINE*"
				}
				eventTime := utility.TimeAgo(s.EventTime)
				msg += fmt.Sprintf(" %v\n", sanitize(eventTime))
				for _, c := range s.Connectors {
					statusTime := utility.TimeAgo(c.StatusTime)
					msg += fmt.Sprintf("Connector %v: `%v` %v\n", c.ConnectorID, c.Status, sanitize(statusTime))
					if c.TransactionId > 0 {
						msg += fmt.Sprintf("Transaction: %v\n", c.TransactionId)
					}
					if c.Info != "" && c.Status != "Available" {
						msg += fmt.Sprintf("%v\n", sanitize(c.Info))
					}
				}
				msg += "\n"
			}
		}
	}
	msg += "\n"
	msg += fmt.Sprintf("Active subscriptions: %v", b.subscriptionCount())
	return msg
}

func removeMarkup(input string) string {
	reservedChars := "\\`*_|"

	sanitized := ""
	for _, char := range input {
		if !strings.ContainsRune(reservedChars, char) {
			sanitized += string(char)
		}
	}

	return sanitized
}

func sanitize(input string) string {
	// Define a list of reserved characters that need to be escaped
	// every character MarkdownV2 reserves: https://core.telegram.org/bots/api#markdownv2-style
	reservedChars := "\\`*_{}[]()~>#+-=.!|"

	// Loop through each character in the input string
	sanitized := ""
	for _, char := range input {
		// Check if the character is reserved
		if strings.ContainsRune(reservedChars, char) {
			// Escape the character with a backslash
			sanitized += "\\" + string(char)
		} else {
			// Add the character to the sanitized string
			sanitized += string(char)
		}
	}

	return sanitized
}

func (b *TgBot) Stop() {
	log.Println("stopping telegram bot...")
	close(b.stop)
	b.api.StopReceivingUpdates()
}
