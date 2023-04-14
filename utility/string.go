package utility

import (
	"fmt"
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
