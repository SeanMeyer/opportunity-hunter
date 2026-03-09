package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Discord posts messages to Discord webhooks with thread support and retry.
type Discord struct {
	webhookURL string
	client     *http.Client
	maxRetries int
}

// NewDiscord creates a new Discord notification client.
func NewDiscord(webhookURL string) *Discord {
	return &Discord{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 30 * time.Second},
		maxRetries: 3,
	}
}

// discordPayload is the Discord webhook JSON body.
type discordPayload struct {
	Content    string          `json:"content,omitempty"`
	ThreadName string          `json:"thread_name,omitempty"`
	Embeds     []discordEmbed  `json:"embeds,omitempty"`
}

type discordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type threadResponse struct {
	ChannelID string `json:"channel_id"`
}

// PostMessage sends a standalone message.
func (d *Discord) PostMessage(ctx context.Context, payload discordPayload) error {
	_, err := d.postWithRetry(ctx, d.webhookURL, payload)
	return err
}

// PostThread creates a new thread and returns the thread ID.
func (d *Discord) PostThread(ctx context.Context, payload discordPayload) (string, error) {
	body, err := d.postWithRetry(ctx, d.webhookURL+"?wait=true", payload)
	if err != nil {
		return "", err
	}
	var resp threadResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse thread response: %w", err)
	}
	return resp.ChannelID, nil
}

// PostToThread sends a message to an existing thread.
func (d *Discord) PostToThread(ctx context.Context, threadID string, payload discordPayload) error {
	url := d.webhookURL + "?thread_id=" + threadID
	_, err := d.postWithRetry(ctx, url, payload)
	return err
}

// PostError sends an error message to the webhook.
func (d *Discord) PostError(ctx context.Context, message string) error {
	payload := discordPayload{Content: "**Error:** " + message}
	_, err := d.postWithRetry(ctx, d.webhookURL, payload)
	return err
}

func (d *Discord) postWithRetry(ctx context.Context, url string, payload discordPayload) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var lastErr error
	for attempt := range d.maxRetries + 1 {
		body, retry, postErr := d.doPost(ctx, url, data)
		if postErr == nil {
			return body, nil
		}
		lastErr = postErr
		if !retry {
			break
		}
		delay := time.Duration(attempt+1) * time.Second
		slog.Warn("discord retry", "attempt", attempt+1, "err", postErr, "delay", delay)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

func (d *Discord) doPost(ctx context.Context, url string, data []byte) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, true, err // network error is retryable
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return body, false, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if secs, parseErr := strconv.ParseFloat(retryAfter, 64); parseErr == nil {
				select {
				case <-ctx.Done():
					return nil, false, ctx.Err()
				case <-time.After(time.Duration(secs * float64(time.Second))):
				}
			}
		}
		return nil, true, fmt.Errorf("discord rate limited (429)")
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("discord server error %d", resp.StatusCode)
	default:
		return nil, false, fmt.Errorf("discord client error %d: %s", resp.StatusCode, string(body))
	}
}
