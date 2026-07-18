package domain

import "time"

type Event struct {
	ID           string
	Type         string
	Source       string
	Subject      string
	PartitionKey string
	Time         time.Time
	Data         map[string]any
}
