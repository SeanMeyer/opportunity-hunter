package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// Client wraps a Discord webhook and processes notification actions.
type Client struct {
	discord *Discord
}

// NewClient creates a notification client for the given webhook URL.
func NewClient(webhookURL string) *Client {
	return &Client{discord: NewDiscord(webhookURL)}
}

// ExecuteActions processes notification actions in order, tracking thread IDs.
// Returns a map of threadName → discordThreadID for newly created threads.
func (c *Client) ExecuteActions(ctx context.Context, actions []core.NotifyAction) (map[string]string, error) {
	if err := core.ValidateActions(actions); err != nil {
		return nil, fmt.Errorf("invalid actions: %w", err)
	}

	threads := make(map[string]string) // ThreadName -> Discord thread ID

	for i, action := range actions {
		payload := toDiscordPayload(action.Message)
		if action.Ping {
			payload.Content = "@here " + payload.Content
		}

		switch action.Type {
		case core.CreateThread:
			payload.ThreadName = action.ThreadName
			threadID, err := c.discord.PostThread(ctx, payload)
			if err != nil {
				return threads, fmt.Errorf("action %d (CreateThread %q): %w", i, action.ThreadName, err)
			}
			threads[action.ThreadName] = threadID
			slog.Info("created thread", "name", action.ThreadName, "id", threadID)

		case core.PostToThread:
			threadID := action.ThreadID
			if threadID == "" {
				var ok bool
				threadID, ok = threads[action.ThreadRef]
				if !ok {
					return threads, fmt.Errorf("action %d: thread %q not found", i, action.ThreadRef)
				}
			}
			if err := c.discord.PostToThread(ctx, threadID, payload); err != nil {
				return threads, fmt.Errorf("action %d (PostToThread %q): %w", i, action.ThreadRef, err)
			}

		case core.PostMessage:
			if err := c.discord.PostMessage(ctx, payload); err != nil {
				return threads, fmt.Errorf("action %d (PostMessage): %w", i, err)
			}
		}

		// Brief delay between Discord messages to avoid rate limits.
		if i < len(actions)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return threads, nil
}

// PostError sends an error message.
func (c *Client) PostError(ctx context.Context, message string) error {
	return c.discord.PostError(ctx, message)
}

func toDiscordPayload(msg core.NotifyMessage) discordPayload {
	payload := discordPayload{Content: msg.Content}
	for _, e := range msg.Embeds {
		embed := discordEmbed{
			Title:       e.Title,
			Description: e.Description,
			Color:       e.Color,
		}
		for _, f := range e.Fields {
			embed.Fields = append(embed.Fields, discordEmbedField{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
		payload.Embeds = append(payload.Embeds, embed)
	}
	return payload
}
