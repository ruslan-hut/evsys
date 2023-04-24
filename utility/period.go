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
		return "1 minute ago"
	} else if minutes < 60 {
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if minutes < 120 {
		return "1 hour ago"
	} else if minutes < 1440 {
		return fmt.Sprintf("%d hours ago", minutes/60)
	} else if minutes < 2880 {
		return "1 day ago"
	} else {
		return fmt.Sprintf("%d days ago", minutes/1440)
	}
}
