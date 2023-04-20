package utility

import (
	"fmt"
	"github.com/google/uuid"
	"strconv"
)

// ToInt converts a string to an integer
func ToInt(s string) int {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return int(f)
}

// IntToString converts an integer to a string like 1234 to 1.2
func IntToString(i int) string {
	if i < 100 {
		return "0.0"
	}
	firstPart := i / 1000
	secondPart := (i % 1000) / 100
	return strconv.Itoa(firstPart) + "." + strconv.Itoa(secondPart)
}

func NewUUID() string {
	return uuid.New().String()
}
