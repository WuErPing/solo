package base

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// EventTranslator converts raw bytes into events.
type EventTranslator interface {
	// Translate converts raw data into events.
	// Returns (events, isTerminal, error).
	Translate(raw []byte, timestamp time.Time) ([]interface{}, bool, error)
}

// TerminalEventDetector checks if an event is a terminal event.
type TerminalEventDetector interface {
	// IsTerminal returns (result, isTerminal, err) for terminal events.
	IsTerminal(evt interface{}) (*AgentRunResult, bool, error)
}

// AgentRunResult is the result of a blocking agent run.
type AgentRunResult struct {
	SessionID string
	FinalText string
	Usage     interface{}
	Canceled  bool
}

// EventPump reads from a stream and translates events.
type EventPump struct {
	logger     *slog.Logger
	dispatcher EventDispatcher
	provider   string
}

// NewEventPump creates a new event pump.
func NewEventPump(logger *slog.Logger, dispatcher EventDispatcher) *EventPump {
	return &EventPump{
		logger:     logger,
		dispatcher: dispatcher,
	}
}

// SetProvider sets the provider name for fallback events (turn_canceled, turn_failed, error).
func (p *EventPump) SetProvider(provider string) {
	p.provider = provider
}

// RunBlocking reads events until terminal state and returns the result.
func (p *EventPump) RunBlocking(
	ctx context.Context,
	reader io.Reader,
	translator EventTranslator,
	detector TerminalEventDetector,
) (*AgentRunResult, error) {
	result, err := p.pump(ctx, reader, translator, detector)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RunBackground starts pumping events in the background.
func (p *EventPump) RunBackground(
	ctx context.Context,
	reader io.Reader,
	translator EventTranslator,
	detector TerminalEventDetector,
) {
	go func() {
		_, _ = p.pump(ctx, reader, translator, detector)
	}()
}

func (p *EventPump) pump(
	ctx context.Context,
	reader io.Reader,
	translator EventTranslator,
	detector TerminalEventDetector,
) (*AgentRunResult, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	// Unblock scanner.Scan() when ctx is cancelled.
	// scanner.Scan() calls reader.Read() which ignores context — the ctx.Done()
	// check inside the for-loop body is only reached after Scan() returns, so a
	// hanging process that never writes to stdout keeps the pump blocked forever.
	// Closing the reader from a goroutine forces Read() to return with an error,
	// which makes Scan() return false and allows the post-loop ctx check to fire.
	// pumpDone prevents the goroutine from closing an already-clean reader after
	// pump returns from natural EOF.
	pumpDone := make(chan struct{})
	defer close(pumpDone)
	if rc, ok := reader.(io.Closer); ok {
		go func() {
			select {
			case <-ctx.Done():
				_ = rc.Close()
			case <-pumpDone:
			}
		}()
	}

	var result *AgentRunResult
	var resultErr error
	terminalReached := false

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			p.emitEvent(map[string]interface{}{
				"type":     "turn_canceled",
				"provider": p.provider,
				"reason":   "context_cancelled",
			})
			return &AgentRunResult{Canceled: true}, ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		events, isTerminal, err := translator.Translate(line, time.Now())
		if err != nil {
			p.logger.Warn("event translation failed", "error", err, "line", string(line))
			p.emitEvent(map[string]interface{}{
				"type":     "error",
				"provider": p.provider,
				"error":    err.Error(),
			})
			continue
		}

		for _, evt := range events {
			p.emitEvent(evt)

			if detector != nil {
				if r, isTerm, err := detector.IsTerminal(evt); isTerm {
					result = r
					resultErr = err
					terminalReached = true
				}
			}
		}

		if isTerminal {
			break
		}
		if terminalReached {
			break
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		p.logger.Warn("scanner error", "error", err)
	}

	// If the context was cancelled (either via rc.Close() above or between
	// scan iterations), emit turn_canceled rather than turn_failed.
	if ctx.Err() != nil {
		p.emitEvent(map[string]interface{}{
			"type":     "turn_canceled",
			"provider": p.provider,
			"reason":   "context_cancelled",
		})
		return &AgentRunResult{Canceled: true}, ctx.Err()
	}

	// If stream ended without terminal state, emit turn_failed. With the
	// context still alive this means the provider process died mid-turn, so
	// wrap with ErrProviderCrashed for upstream crash recovery.
	if !terminalReached {
		p.logger.Warn("event stream ended before terminal state")
		p.emitEvent(map[string]interface{}{
			"type":     "turn_failed",
			"provider": p.provider,
			"error":    "event stream ended before reaching terminal state",
		})
		return &AgentRunResult{}, fmt.Errorf("%w: event stream ended before terminal state", ErrProviderCrashed)
	}

	return result, resultErr
}

func (p *EventPump) emitEvent(evt interface{}) {
	if p.dispatcher != nil {
		p.dispatcher.Emit(evt)
	}
}

// --- JSON Event Translator Helper ---

// JSONEventTranslator translates JSON lines into events.
type JSONEventTranslator struct {
	logger *slog.Logger
}

// NewJSONEventTranslator creates a new JSON event translator.
func NewJSONEventTranslator(logger *slog.Logger) *JSONEventTranslator {
	return &JSONEventTranslator{logger: logger}
}

// ParseJSONLine parses a single JSON line into a raw map.
func (t *JSONEventTranslator) ParseJSONLine(line []byte) (map[string]json.RawMessage, error) {
	var event map[string]json.RawMessage
	if err := json.Unmarshal(line, &event); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return event, nil
}
