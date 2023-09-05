package utility

import (
	"fmt"
	"math"
	"time"
)

func TimeAgo(t time.Time) string {
	duration := time.Since(t).Round(time.Minute)
	minutes := int(math.Abs(duration.Minutes()))
	if minutes == 0 {
		return "just now"
	} else if minutes == 1 {
		return "1 minute"
	} else if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	} else if minutes < 120 {
		return "1 hour"
	} else if minutes < 1440 {
		return fmt.Sprintf("%d hours", minutes/60)
	} else if minutes < 2880 {
		return "1 day"
	} else {
		return fmt.Sprintf("%d days", minutes/1440)
	}
}
