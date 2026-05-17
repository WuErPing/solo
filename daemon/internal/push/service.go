package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const defaultExpoPushURL = "https://exp.host/--/api/v2/push/send"
const maxBatchSize = 100
const defaultMaxRetries = 3
const defaultRetryDelay = 1 * time.Second

// Pusher sends push notifications to mobile devices.
type Pusher interface {
	Send(tokens []string, payload NotificationPayload) error
}

// ExpoPushService sends notifications via Expo Push API with retry support.
type ExpoPushService struct {
	baseURL    string
	tokenStore TokenStore
	client     *http.Client
	logger     *slog.Logger
	MaxRetries int
	RetryDelay time.Duration
}

// expoPushMessage represents the request body for Expo Push API.
type expoPushMessage struct {
	To    string           `json:"to"`
	Title string           `json:"title"`
	Body  string           `json:"body"`
	Data  NotificationData `json:"data"`
	Sound string           `json:"sound,omitempty"`
}

// expoPushTicket represents the response from Expo Push API.
type expoPushTicket struct {
	Status  string             `json:"status"`
	ID      string             `json:"id,omitempty"`
	Message string             `json:"message,omitempty"`
	Details *expoErrorDetails  `json:"details,omitempty"`
}

type expoErrorDetails struct {
	Error string `json:"error"`
}

// NewExpoPushService creates a new Expo push service.
// If baseURL is empty, uses the default Expo API URL.
func NewExpoPushService(baseURL string, tokenStore TokenStore, logger *slog.Logger) *ExpoPushService {
	if baseURL == "" {
		baseURL = defaultExpoPushURL
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ExpoPushService{
		baseURL:    baseURL,
		tokenStore: tokenStore,
		client:     &http.Client{},
		logger:     logger,
		MaxRetries: defaultMaxRetries,
		RetryDelay: defaultRetryDelay,
	}
}

// Send sends push notifications to the given tokens.
func (s *ExpoPushService) Send(tokens []string, payload NotificationPayload) error {
	if len(tokens) == 0 {
		return nil
	}

	// Prepare messages
	messages := make([]expoPushMessage, len(tokens))
	for i, token := range tokens {
		messages[i] = expoPushMessage{
			To:    token,
			Title: payload.Title,
			Body:  payload.Body,
			Data:  payload.Data,
			Sound: "default",
		}
	}

	// Batch and send
	var firstErr error
	for i := 0; i < len(messages); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(messages) {
			end = len(messages)
		}
		batch := messages[i:end]
		if err := s.sendBatch(batch); err != nil {
			s.logger.Error("failed to send push batch", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (s *ExpoPushService) sendBatch(messages []expoPushMessage) error {
	body, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < s.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := s.RetryDelay * time.Duration(1<<(attempt-1))
			s.logger.Info("retrying push batch", "attempt", attempt, "delay", delay)
			time.Sleep(delay)
		}

		req, err := http.NewRequest("POST", s.baseURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("send request: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			err := s.handleResponse(resp, messages)
			resp.Body.Close()
			return err
		}

		// Non-retryable: 4xx except 429
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			s.logger.Error("expo API client error", "status", resp.StatusCode)
			resp.Body.Close()
			return fmt.Errorf("expo API returned status %d", resp.StatusCode)
		}

		// Retryable: 429 and 5xx
		resp.Body.Close()
		lastErr = fmt.Errorf("expo API returned status %d", resp.StatusCode)
		s.logger.Warn("expo API retryable error", "status", resp.StatusCode, "attempt", attempt)
	}

	return fmt.Errorf("expo push failed after %d attempts: %w", s.MaxRetries, lastErr)
}

func (s *ExpoPushService) handleResponse(resp *http.Response, messages []expoPushMessage) error {
	var result struct {
		Data []expoPushTicket `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	// Handle tickets and clean up invalid tokens
	for i, ticket := range result.Data {
		if ticket.Status == "error" && ticket.Details != nil {
			if ticket.Details.Error == "DeviceNotRegistered" || ticket.Details.Error == "InvalidCredentials" {
				if s.tokenStore != nil && i < len(messages) {
					s.tokenStore.Remove(messages[i].To)
				}
			}
		}
	}

	return nil
}
