package cloudevent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SpecVersion     = "1.0"
	JSONContentType = "application/json"
)

var ErrInvalidEnvelope = errors.New("invalid CloudEvents envelope")

type Envelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            time.Time       `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

func Encode(
	id, source, eventType string,
	eventTime time.Time,
	data []byte,
) ([]byte, error) {
	envelope := Envelope{
		SpecVersion:     SpecVersion,
		ID:              strings.TrimSpace(id),
		Source:          strings.TrimSpace(source),
		Type:            strings.TrimSpace(eventType),
		Time:            eventTime,
		DataContentType: JSONContentType,
		Data:            data,
	}
	if err := envelope.Validate(); err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(&envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal CloudEvents envelope: %w", err)
	}

	return encoded, nil
}

func Decode(data []byte) (*Envelope, error) {
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("%w: decode JSON: %w", ErrInvalidEnvelope, err)
	}
	if err := envelope.Validate(); err != nil {
		return nil, err
	}

	return &envelope, nil
}

func (e *Envelope) Validate() error {
	switch {
	case e.SpecVersion != SpecVersion:
		return fmt.Errorf("%w: unsupported specversion", ErrInvalidEnvelope)
	case strings.TrimSpace(e.ID) == "":
		return fmt.Errorf("%w: id is required", ErrInvalidEnvelope)
	case strings.TrimSpace(e.Source) == "":
		return fmt.Errorf("%w: source is required", ErrInvalidEnvelope)
	case strings.TrimSpace(e.Type) == "":
		return fmt.Errorf("%w: type is required", ErrInvalidEnvelope)
	case e.Time.IsZero():
		return fmt.Errorf("%w: time is required", ErrInvalidEnvelope)
	case e.DataContentType != JSONContentType:
		return fmt.Errorf("%w: unsupported datacontenttype", ErrInvalidEnvelope)
	case len(e.Data) == 0 || string(e.Data) == "null":
		return fmt.Errorf("%w: data is required", ErrInvalidEnvelope)
	default:
		return nil
	}
}
