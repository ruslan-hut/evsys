package internal

type StatusHandler interface {
	OnOnlineStatusChanged(id string, isOnline bool)
}
