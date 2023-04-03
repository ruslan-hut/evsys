package telegram

import (
	"evsys/internal"
	"fmt"
	"log"
)
import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

// TgBot implements EventHandler
type TgBot struct {
	api           *tgbotapi.BotAPI
	subscriptions map[int]UserSubscription
	event         chan string
}

type UserSubscription struct {
	UserID           int
	SubscriptionType string
}

func NewBot(apiKey string) (*TgBot, error) {
	tgBot := &TgBot{
		subscriptions: make(map[int]UserSubscription),
		event:         make(chan string, 100),
	}
	api, err := tgbotapi.NewBotAPI(apiKey)
	if err != nil {
		return nil, err
	}
	tgBot.api = api
	go tgBot.EventPump()
	go tgBot.Start()
	return tgBot, nil
}

// Start listening for updates
func (b *TgBot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := b.api.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
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
			b.subscriptions[update.Message.From.ID] = UserSubscription{UserID: update.Message.From.ID, SubscriptionType: ""}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You are now subscribed to updates!")
			_, err := b.api.Send(msg)
			if err != nil {
				log.Printf("bot: error sending message: %v", err)
			}
		case "stop":

		case "status":

		}
	}
}

// EventPump sending events to all subscribers
func (b *TgBot) EventPump() {
	for {
		if event, ok := <-b.event; ok {
			for _, subscription := range b.subscriptions {
				msg := tgbotapi.NewMessage(int64(subscription.UserID), event)
				_, err := b.api.Send(msg)
				if err != nil {
					log.Printf("bot: error sending message: %v", err)
				}
			}
		}
	}
}

func (b *TgBot) OnStatusNotification(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: Connector %v: `%v`", event.ChargePointId, event.ConnectorId, event.Status)
	b.event <- msg
}

func (b *TgBot) OnTransactionStart(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: Connector %v: `%v`\n", event.ChargePointId, event.ConnectorId, event.Status)
	msg += fmt.Sprintf("Transaction ID: %v START\n", event.TransactionId)
	msg += fmt.Sprintf("User: %v\n", event.Username)
	msg += fmt.Sprintf("ID Tag: %v\n", event.IdTag)
	b.event <- msg
}

func (b *TgBot) OnTransactionStop(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: Connector %v: `%v`\n", event.ChargePointId, event.ConnectorId, event.Status)
	msg += fmt.Sprintf("Transaction ID: %v STOP\n", event.TransactionId)
	msg += fmt.Sprintf("User: %v\n", event.Username)
	msg += fmt.Sprintf("ID Tag: %v\n", event.IdTag)
	b.event <- msg
}

func (b *TgBot) OnAuthorize(event *internal.EventMessage) {
	msg := fmt.Sprintf("*%v*: user: `%v`\n", event.ChargePointId, event.IdTag)
	msg += fmt.Sprintf("Auth status: %v\n", event.Status)
	b.event <- msg
}
