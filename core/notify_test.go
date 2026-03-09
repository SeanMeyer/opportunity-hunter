package core_test

import (
	"testing"

	"github.com/seanmeyer/opportunity-hunter/core"
)

func TestValidateActions_ValidSequence(t *testing.T) {
	actions := []core.NotifyAction{
		{Type: core.CreateThread, ThreadName: "PNW Cascades — Jan 15"},
		{Type: core.PostToThread, ThreadRef: "PNW Cascades — Jan 15"},
		{Type: core.PostToThread, ThreadRef: "PNW Cascades — Jan 15"},
	}
	if err := core.ValidateActions(actions); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateActions_PostMessageOnly(t *testing.T) {
	actions := []core.NotifyAction{
		{Type: core.PostMessage, Message: core.NotifyMessage{Content: "hello"}},
	}
	if err := core.ValidateActions(actions); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateActions_MissingThreadRef(t *testing.T) {
	actions := []core.NotifyAction{
		{Type: core.PostToThread, ThreadRef: "nonexistent"},
	}
	if err := core.ValidateActions(actions); err == nil {
		t.Fatal("expected error for missing ThreadRef")
	}
}

func TestValidateActions_CreateThreadEmptyName(t *testing.T) {
	actions := []core.NotifyAction{
		{Type: core.CreateThread, ThreadName: ""},
	}
	if err := core.ValidateActions(actions); err == nil {
		t.Fatal("expected error for empty ThreadName")
	}
}

func TestValidateActions_PostMessageWithThreadFields(t *testing.T) {
	actions := []core.NotifyAction{
		{Type: core.PostMessage, ThreadName: "oops"},
	}
	if err := core.ValidateActions(actions); err == nil {
		t.Fatal("expected error for PostMessage with thread fields")
	}
}
