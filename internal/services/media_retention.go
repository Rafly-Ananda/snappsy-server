package services

import "time"

func retentionDeadline(days int, now time.Time) *time.Time {
	if days <= 0 {
		return nil
	}
	deadline := now.AddDate(0, 0, days)
	return &deadline
}
