package testutil

import "github.com/seanmeyer/opportunity-hunter/core"

// FakeNotifier records all notification actions.
type FakeNotifier struct {
	Actions []core.NotifyAction
	Errors  []string
	Err     error // error to return from ExecuteActions
}

// ExecuteActions records the actions.
func (n *FakeNotifier) ExecuteActions(actions []core.NotifyAction) error {
	n.Actions = append(n.Actions, actions...)
	return n.Err
}

// PostError records error messages.
func (n *FakeNotifier) PostError(message string) error {
	n.Errors = append(n.Errors, message)
	return n.Err
}
