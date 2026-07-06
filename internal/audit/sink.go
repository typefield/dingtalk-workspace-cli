package audit

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Sink interface {
	Emit(event *Event) error
	Close() error
}

type NopSink struct{}

func (NopSink) Emit(*Event) error { return nil }
func (NopSink) Close() error      { return nil }

type FileSink struct {
	mu        sync.Mutex
	writer    *DateRotatingWriter
	chain     *Chain
	forwarder *HTTPForwarder
}

func NewFileSink(writer *DateRotatingWriter, chain *Chain, forwarder *HTTPForwarder) *FileSink {
	return &FileSink{
		writer:    writer,
		chain:     chain,
		forwarder: forwarder,
	}
}

func (s *FileSink) Emit(evt *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	body, err := marshalWithoutHash(evt)
	if err != nil {
		return fmt.Errorf("audit: marshal event: %w", err)
	}

	prevHash, hash := s.chain.Seal(body)
	evt.PrevHash = prevHash
	evt.Hash = hash

	line, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("audit: marshal final event: %w", err)
	}
	line = append(line, '\n')

	if _, err := s.writer.Write(line); err != nil {
		return fmt.Errorf("audit: write event: %w", err)
	}

	if s.forwarder != nil {
		s.forwarder.Forward(*evt)
	}

	return nil
}

func (s *FileSink) Close() error {
	return s.writer.Close()
}

func marshalWithoutHash(evt *Event) ([]byte, error) {
	saved := Event{
		Timestamp:     evt.Timestamp,
		ExecutionID:   evt.ExecutionID,
		AgentID:       evt.AgentID,
		Actor:         evt.Actor,
		Product:       evt.Product,
		Command:       evt.Command,
		Endpoint:      evt.Endpoint,
		ParamsSummary: evt.ParamsSummary,
		Result:        evt.Result,
		ErrCategory:   evt.ErrCategory,
		ErrReason:     evt.ErrReason,
		DurationMs:    evt.DurationMs,
		CLIVersion:    evt.CLIVersion,
		OS:            evt.OS,
		Arch:          evt.Arch,
	}
	return json.Marshal(saved)
}
