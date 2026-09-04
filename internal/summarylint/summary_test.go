// Package summarylint holds the rules a human-facing action Summary must keep.
//
// Summary and Description serve different readers. Description is written for
// the model choosing a tool: long, prose-like, and often carrying explicit
// instructions ("Use this when the conversation hints at scheduling…").
// Summary is what a person browsing the Add Node menu reads instead.
//
// The failure this guards against is drift — a Summary written by copying the
// Description, which puts prompt engineering back in front of users while
// looking like the job was done.
package summarylint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maxSummaryRunes keeps a summary inside the menu's two-line clamp at the
// panel's width. Past this it is truncated, which is worse than a shorter
// sentence would have been.
const maxSummaryRunes = 80

// aiVoice are phrases that only make sense when addressed to a model. Their
// presence means the text was written for, or copied from, the Description.
var aiVoice = []string{
	"use this when", "use when", "use it when", "call this when", "call when",
	"useful when", "the ai ", "the agent should", "pass the", "prefer this",
	"tool_result", "returns the", "emits ",
}

type action struct {
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

func loadManifest(t *testing.T) map[string]action {
	t.Helper()
	// Walk up to the module root, so the test works from anywhere.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, "internal", "assets", "manifest", "manifest.json")
		if b, err := os.ReadFile(p); err == nil {
			var m map[string]action
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatalf("manifest is not readable: %v", err)
			}
			return m
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find internal/assets/manifest/manifest.json — run go generate ./actions/...")
	return nil
}

func TestSummariesAreShortEnoughToRead(t *testing.T) {
	for id, a := range loadManifest(t) {
		if a.Summary == "" {
			continue
		}
		if n := len([]rune(a.Summary)); n > maxSummaryRunes {
			t.Errorf("%s: summary is %d characters, over %d — it will be truncated in the menu\n  %q",
				id, n, maxSummaryRunes, a.Summary)
		}
	}
}

// TestSummariesAreNotPastedPromptText catches the cheap wrong way to finish
// this catalogue: pasting the Description into the Summary.
//
// An identical string is only a problem when the Description is the sort a
// person should not have been shown — long, or written at the model. Plenty of
// descriptions are already short plain English ("Upload a file to Google
// Drive"), and repeating one is the honest answer: it pins the menu copy so a
// later rewrite of the Description for the AI cannot quietly change what users
// read.
func TestSummariesAreNotPastedPromptText(t *testing.T) {
	for id, a := range loadManifest(t) {
		if a.Summary == "" {
			continue
		}
		if !strings.EqualFold(strings.TrimRight(a.Summary, "."), strings.TrimRight(a.Description, ".")) {
			continue
		}
		if n := len([]rune(a.Description)); n > maxSummaryRunes {
			t.Errorf("%s: summary is a copy of a %d-character description — write one for a person", id, n)
		}
		lower := strings.ToLower(a.Description)
		for _, phrase := range aiVoice {
			if strings.Contains(lower, phrase) {
				t.Errorf("%s: summary is a copy of a description that addresses the model (%q)", id, strings.TrimSpace(phrase))
				break
			}
		}
	}
}

func TestSummariesDoNotAddressTheModel(t *testing.T) {
	for id, a := range loadManifest(t) {
		if a.Summary == "" {
			continue
		}
		lower := strings.ToLower(a.Summary)
		for _, phrase := range aiVoice {
			if strings.Contains(lower, phrase) {
				t.Errorf("%s: summary contains %q, which is written for the AI rather than the reader\n  %q",
					id, strings.TrimSpace(phrase), a.Summary)
			}
		}
	}
}

// TestSummariesStartWithAVerb is a light style check: the menu reads as a list
// of things you can do, so "Send an email" beats "An action that sends email".
func TestSummariesStartWithAVerb(t *testing.T) {
	badOpeners := []string{"a ", "an ", "the ", "this ", "action ", "triggers ", "trigger "}
	for id, a := range loadManifest(t) {
		if a.Summary == "" {
			continue
		}
		lower := strings.ToLower(a.Summary)
		for _, opener := range badOpeners {
			if strings.HasPrefix(lower, opener) {
				t.Errorf("%s: summary opens with %q — lead with the verb\n  %q", id, strings.TrimSpace(opener), a.Summary)
			}
		}
	}
}
