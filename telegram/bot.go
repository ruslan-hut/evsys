package telegram

import (
	"evsys/internal"
	"evsys/models"
	"evsys/utility"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strings"
)

// TgBot implements EventHandler
type TgBot struct {
	api           *tgbotapi.BotAPI
	database      internal.Database
	subscriptions map[int]models.UserSubscription
	event         chan MessageContent
	send          chan MessageContent
}

type MessageContent struct {
	ChatID int64
	Text   string
}

func NewBot(apiKey string) (*TgBot, error) {
	tgBot := &TgBot{
		subscriptions: make(map[int]models.UserSubscription),
		event:         make(chan MessageContent, 100),
		send:          make(chan MessageContent, 100),
	}
	api, err := tgbotapi.NewBotAPI(apiKey)
	if err != nil {
		return nil, err
	}
	tgBot.api = api
	return tgBot, nil
}

// SetDatabase attach database service
func (b *TgBot) SetDatabase(database internal.Database) {
	b.database = database
}

func (b *TgBot) Start() {
	b.subscriptions = make(map[int]models.UserSubscription)
	if b.database != nil {
		subscriptions, err := b.database.GetSubscriptions()
		if err != nil {
			log.Printf("bot: error getting subscriptions: %v", err)
		} else {
			for _, subscription := range subscriptions {
				b.subscriptions[subscription.UserID] = subscription
			}
		}
	}
	go b.sendPump()
	go b.eventPump()
	go b.updatesPump()
}

// Start listening for updates
func (b *TgBot) updatesPump() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
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
			subscription := models.UserSubscription{
				UserID:           update.Message.From.ID,
				User:             update.Message.From.UserName,
				SubscriptionType: "status",
			}
			b.subscriptions[update.Message.From.ID] = subscription
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
			delete(b.subscriptions, update.Message.From.ID)
			if b.database != nil {
				err := b.database.DeleteSubscription(&models.UserSubscription{UserID: update.Message.From.ID})
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
		if event, ok := <-b.event; ok {
			for _, subscription := range b.subscriptions {
				b.sendMessage(int64(subscription.UserID), event.Text)
			}
		}
	}
}

// sendPump sending messages to users
func (b *TgBot) sendPump() {
	for {
		if event, ok := <-b.send; ok {
			b.sendMessage(event.ChatID, event.Text)
		}
	}
}

// sendMessage common routine to send a message via bot API
func (b *TgBot) sendMessage(id int64, text string) {
	msg := tgbotapi.NewMessage(id, text)
	msg.ParseMode = "MarkdownV2"
	_, err := b.api.Send(msg)
	if err != nil {
		safeMsg := tgbotapi.NewMessage(id, fmt.Sprintf("This message caused an error:\n%v", removeMarkup(text)))
		_, err = b.api.Send(safeMsg)
		if err != nil {
			log.Printf("bot: error sending unmarkuped message: %v", err)
			// maybe error was while parsing, so we can send a message about this error
			msg = tgbotapi.NewMessage(id, fmt.Sprintf("Error: %v", err))
			_, err = b.api.Send(msg)
			if err != nil {
				log.Printf("bot: error sending message: %v", err)
			}
		}
	}
}

func (b *TgBot) OnStatusNotification(event *internal.EventMessage) {
	// only send notifications about Faulted status
	if event.Status != "Faulted" {
		return
	}
	var msg string
	if event.ConnectorId == 0 {
		msg = fmt.Sprintf("*%v*: `%v`", event.ChargePointId, event.Status)
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
	msg += fmt.Sprintf("Active subscriptions: %v", len(b.subscriptions))
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
