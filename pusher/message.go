package pusher

type Channel string
type Event string

const (
	SystemLog Channel = "sys_log"
	Call      Event   = "call_event"
)

type Message struct {
	Channel Channel
	Event   Event
	Text    string
}
