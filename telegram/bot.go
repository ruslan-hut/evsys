package telegram

import (
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
	event         chan MessageContent
	send          chan MessageContent
	stop          chan struct{}
	// slots hands out the delivery permits counted by deliverySlots.
	slots chan struct{}
	// deliver performs one send. It is a field so tests can drive the pumps without the Telegram
	// API; production always uses sendMessage.
	deliver func(id int64, text string)
}

type MessageContent struct {
	ChatID int64
	Text   string
}

func NewBot(apiKey string) (*TgBot, error) {
	tgBot := &TgBot{
		subscriptions: make(map[int]entity.UserSubscription),
		event:         make(chan MessageContent, 100),
		send:          make(chan MessageContent, 100),
		stop:          make(chan struct{}),
		slots:         make(chan struct{}, deliverySlots),
	}
	tgBot.deliver = tgBot.sendMessage
	api, err := tgbotapi.NewBotAPIWithClient(apiKey, &http.Client{Timeout: apiTimeout})
	if err != nil {
		return nil, err
	}
	tgBot.api = api
	return tgBot, nil
}

// setSubscriptions replaces the whole set, as loading from the database does.
func (b *TgBot) setSubscriptions(subscriptions map[int]entity.UserSubscription) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.subscriptions = subscriptions
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
	loaded := make(map[int]entity.UserSubscription)
	if b.database != nil {
		subscriptions, err := b.database.GetSubscriptions()
		if err != nil {
			log.Printf("bot: error getting subscriptions: %v", err)
		} else {
			for _, subscription := range subscriptions {
				loaded[subscription.UserID] = subscription
			}
		}
	}
	b.setSubscriptions(loaded)
	go b.sendPump()
	go b.eventPump()
	go b.updatesPump()
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
		if update.Message == nil {
			continue
		}
		if !update.Message.IsCommand() {
			continue
		}
		switch update.Message.Command() {
		case "start":
			subscription := entity.UserSubscription{
				UserID:           update.Message.From.ID,
				User:             update.Message.From.UserName,
				SubscriptionType: "status",
			}
			b.addSubscription(subscription)
			msg := fmt.Sprintf("Hello *%v*, you are now subscribed to updates", update.Message.From.UserName)
			if b.database != nil {
				err := b.database.AddSubscription(&subscription)
				if err != nil {
					log.Printf("bot: error adding subscription: %v", err)
					msg = fmt.Sprintf("Error adding subscription:\n `%v`", err)
				}
			}
			b.send <- MessageContent{ChatID: update.Message.Chat.ID, Text: msg}
		case "stop":
			b.removeSubscription(update.Message.From.ID)
			if b.database != nil {
				err := b.database.DeleteSubscription(&entity.UserSubscription{UserID: update.Message.From.ID})
				if err != nil {
					log.Printf("bot: error deleting subscription: %v", err)
				}
			}
			b.send <- MessageContent{ChatID: update.Message.Chat.ID, Text: "Your subscription has been removed"}
		case "test":
			msg := fmt.Sprintf("*%v*: Connector %v: `%v`", "ChargePointId", 1, "Status")
			b.send <- MessageContent{ChatID: update.Message.Chat.ID, Text: msg}
		case "status":
			msg := b.composeStatusMessage()
			b.send <- MessageContent{ChatID: update.Message.Chat.ID, Text: msg}
		}
	}
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
		if isTransportError(err) {
			return
		}
		// maybe error was while parsing, so we can send a message about this error
		msg = tgbotapi.NewMessage(id, fmt.Sprintf("Error: %v", err))
		_, err = b.api.Send(msg)
		if err != nil {
			log.Printf("bot: error sending message: %v", err)
		}
	}
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
	msg += fmt.Sprintf("ID Tag: %v\n", event.IdTag)
	if event.Info != "" {
		msg += fmt.Sprintf("%v\n", sanitize(event.Info))
	}
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnTransactionStop(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: Connector %v: `%v`\n", event.ChargePointId, event.ConnectorId, event.Status)
	msg += fmt.Sprintf("Transaction ID: %v STOP\n", event.TransactionId)
	msg += fmt.Sprintf("User: %v\n", sanitize(event.Username))
	msg += fmt.Sprintf("ID Tag: %v\n", event.IdTag)
	msg += fmt.Sprintf("Info: %v\n", sanitize(event.Info))
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnTransactionEvent(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: Connector %v: `%v`\n", event.ChargePointId, event.ConnectorId, event.Status)
	msg += fmt.Sprintf("Transaction ID: %v ACTIVE\n", event.TransactionId)
	msg += fmt.Sprintf("User: %v\n", sanitize(event.Username))
	msg += fmt.Sprintf("ID Tag: %v\n", event.IdTag)
	msg += fmt.Sprintf("Info: %v\n", sanitize(event.Info))
	b.event <- MessageContent{Text: msg}
}

func (b *TgBot) OnAuthorize(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: user: `%v`\n", event.ChargePointId, event.IdTag)
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
		msg += fmt.Sprintf("ID Tag: %v\n", event.IdTag)
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
			msg += fmt.Sprintf("Error getting last status:\n `%v`", err)
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
	reservedChars := "\\`*_{}[]()#+-.!|"

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
