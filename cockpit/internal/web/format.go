package web

import (
	"fmt"
	"time"
)

func humanBytes(value uint64) string {
	const unit = uint64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor := unit
	exponent := 0
	for remaining := value / unit; remaining >= unit && exponent < 5; remaining /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}

func humanDuration(seconds uint64) string {
	duration := time.Duration(seconds) * time.Second
	if duration >= 24*time.Hour {
		return fmt.Sprintf("%dd %dh", int(duration/(24*time.Hour)), int(duration/time.Hour)%24)
	}
	return duration.Round(time.Minute).String()
}

func humanTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Format("2006-01-02 15:04:05")
}
