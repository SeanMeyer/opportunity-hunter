package core

import "fmt"

// ActionType defines the kind of Discord notification action.
type ActionType string

const (
	CreateThread ActionType = "create_thread"
	PostToThread ActionType = "post_to_thread"
	PostMessage  ActionType = "post_message"
)

// ReminderType distinguishes different reminder triggers.
type ReminderType string

// NotifyFormatter formats picks and reminders into notification actions.
type NotifyFormatter interface {
	FormatPicks(ctx NotifyContext) []NotifyAction
	FormatReminder(opp Opportunity, pick Pick, reminderType ReminderType) []NotifyAction
}

// NotifyContext carries everything a formatter needs.
type NotifyContext struct {
	Evaluations   []Evaluation
	Picks         []Pick
	Opportunities []Opportunity
	Synthesis     string // from Briefer, empty if not implemented
}

// NotifyAction is a single notification operation.
type NotifyAction struct {
	Type       ActionType
	ThreadName string // for CreateThread
	ThreadRef  string // for PostToThread: references an earlier CreateThread by ThreadName
	Message    NotifyMessage
	Ping       bool
}

// NotifyMessage is the content of a notification.
type NotifyMessage struct {
	Content string
	Embeds  []Embed
}

// Embed is a Discord embed.
type Embed struct {
	Title       string
	Description string
	Color       int
	Fields      []EmbedField
}

// EmbedField is a field within an embed.
type EmbedField struct {
	Name   string
	Value  string
	Inline bool
}

// ValidateActions checks that notification actions form a valid sequence.
func ValidateActions(actions []NotifyAction) error {
	threads := make(map[string]bool)
	for i, a := range actions {
		switch a.Type {
		case CreateThread:
			if a.ThreadName == "" {
				return fmt.Errorf("action %d: CreateThread requires ThreadName", i)
			}
			threads[a.ThreadName] = true
		case PostToThread:
			if a.ThreadRef == "" {
				return fmt.Errorf("action %d: PostToThread requires ThreadRef", i)
			}
			if !threads[a.ThreadRef] {
				return fmt.Errorf("action %d: PostToThread ThreadRef %q not found in preceding CreateThread actions", i, a.ThreadRef)
			}
		case PostMessage:
			if a.ThreadName != "" || a.ThreadRef != "" {
				return fmt.Errorf("action %d: PostMessage should not have thread fields", i)
			}
		}
	}
	return nil
}
