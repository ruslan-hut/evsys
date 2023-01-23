package types

import "time"

// DateTime wraps a time.Time struct, allowing for improved dateTime JSON compatibility.
type DateTime struct {
	time.Time
}

// NewDateTime Creates a new DateTime struct, embedding a time.Time struct.
func NewDateTime(time time.Time) *DateTime {
	return &DateTime{Time: time}
}
