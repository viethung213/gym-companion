package consumer

import (
	"encoding/json"
	"time"
)

// CloudEvent standard envelope for Kafka message consumption.
type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

// ResolveLocalDate formats timestamp into YYYY-MM-DD in the user's primary locale (Asia/Ho_Chi_Minh, UTC+7).
func ResolveLocalDate(t time.Time) string {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	return t.In(loc).Format("2006-01-02")
}
