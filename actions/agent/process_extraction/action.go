// Package process_extraction is the executor action that consumes the
// structured JSON output of the extraction System Flow's AI node and
// fans it out into the agent_memory, agent_pending_action, and
// agent_commitment tables via the Phase 2a internal CRUD endpoints.
//
// This action is the fan-out step of the extraction pipeline:
//
//	trigger → ai/anthropic → agent/process_extraction
//	                ↓
//	        {memories, proposed_actions, commitments}
//	                ↓
//	     N × POST /internal/agent/:id/memory
//	     N × POST /internal/agent/:id/pending-action
//	     N × POST /internal/agent/:id/commitment
//
// The confidence threshold logic (>=0.8 store, 0.5–0.8 flag, <0.5
// discard) lives here rather than in the API: the API accepts whatever
// the action writes, and the action is the single control point where
// admins customising the extraction flow can tune behaviour.
//
// Partial success is the rule: if one memory write fails, the action
// continues with the rest and reports aggregate counts. A single bad
// memory must not abort the whole extraction. Errors are logged into
// the returned `errors` array so admins can see them in the execution
// detail view without the flow itself failing.
//
// See plans/agent_memory.md §"The extraction pipeline" for the full
// output schema and design rationale.
package process_extraction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Process Extraction Output"
	Description  = "Parse a structured extraction JSON payload and write memories, pending actions, and commitments"
	Website      = "https://www.flomation.co"
	Icon         = "brain"
	Date         = "05/04/2026"
	Type         = core.ActionTypeAction
)

// Confidence thresholds per plans/agent_memory.md §"Concurrency and
// write safety". Memories below the discard threshold are dropped
// silently; memories between discard and store get stored but flagged
// for review (currently implemented as storing with pinned=false and
// letting the Phase 6 profile UI surface them); memories at or above
// the store threshold are written normally.
const (
	confidenceDiscardThreshold = 0.5
	confidenceStoreThreshold   = 0.8
)

var Inputs = [...]core.Connection{
	{
		Name:        "agent_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent ID",
		Placeholder: "${flow.agent_id}",
		Required:    true,
	},
	{
		Name:        "extraction_json",
		Type:        core.ConnectionTypeString,
		Label:       "Extraction JSON",
		Placeholder: "${node.ai.response}",
		Required:    true,
	},
	{
		Name:        "agent_user_id",
		Type:        core.ConnectionTypeString,
		Label:       "Agent User ID",
		Placeholder: "${trigger.agent_user_id}",
		Required:    false,
	},
	{
		Name:        "conversation_id",
		Type:        core.ConnectionTypeString,
		Label:       "Conversation ID",
		Placeholder: "${trigger.conversation_id}",
		Required:    false,
	},
	{
		Name:        "source_message_id",
		Type:        core.ConnectionTypeString,
		Label:       "Source Message ID",
		Placeholder: "${trigger.message_id}",
		Required:    false,
	},
}

var Outputs = [...]core.Connection{
	{Name: "memories_written", Type: core.ConnectionTypeInteger, Label: "Memories written"},
	{Name: "memories_flagged", Type: core.ConnectionTypeInteger, Label: "Memories flagged (0.5–0.8 confidence)"},
	{Name: "memories_discarded", Type: core.ConnectionTypeInteger, Label: "Memories discarded (<0.5 confidence)"},
	{Name: "pending_actions_written", Type: core.ConnectionTypeInteger, Label: "Pending actions written"},
	{Name: "commitments_written", Type: core.ConnectionTypeInteger, Label: "Commitments written"},
	{Name: "errors", Type: core.ConnectionTypeObject, Label: "Per-record errors"},
}

// extractionPayload mirrors the JSON schema from
// plans/agent_memory.md §"The extraction pipeline". The extraction flow's
// AI action produces this shape; if the model drifts or returns malformed
// JSON we return a structured error rather than exploding.
type extractionPayload struct {
	Memories        []extractionMemory  `json:"memories"`
	ProposedActions []extractionAction  `json:"proposed_actions"`
	Commitments     []extractionCommit  `json:"commitments"`
	Confirmations   []extractionConfirm `json:"confirmations"` // Phase 5 consumer
}

type extractionMemory struct {
	Type       string  `json:"type"` // 'preference' | 'feedback' | 'fact' | 'relationship' | 'task' | 'session_summary'
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Confidence float64 `json:"confidence"`
	Pinned     bool    `json:"pinned,omitempty"` // optional — 'preference' and 'feedback' are auto-pinned in the post-processing step below
}

type extractionAction struct {
	Type       string                 `json:"type"` // 'identity_link' | 'forget_memory' | 'correct_memory' | ...
	Evidence   string                 `json:"evidence"`
	Confidence float64                `json:"confidence"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

type extractionCommit struct {
	Kind        string  `json:"kind"` // 'followup' | 'reminder' | 'monitor' | 'chase'
	Description string  `json:"description"`
	TriggerType string  `json:"trigger_type"` // 'time_elapsed' | 'absolute_time' | 'condition' | 'user_prompt'
	DueIn       string  `json:"due_in,omitempty"`
	DueAt       string  `json:"due_at,omitempty"` // ISO-8601; Phase 3 consumer
	Evidence    string  `json:"evidence"`
	Confidence  float64 `json:"confidence"`
	MadeBy      string  `json:"made_by,omitempty"` // 'assistant' | 'user'
}

type extractionConfirm struct {
	PendingActionID string `json:"pending_action_id"`
	Resolution      string `json:"resolution"` // 'confirmed' | 'declined'
	Evidence        string `json:"evidence"`
}

// extractionResult summarises what the action did, returned as the
// node's output and surfaced in the execution detail view.
type extractionResult struct {
	MemoriesWritten       int      `json:"memories_written"`
	MemoriesFlagged       int      `json:"memories_flagged"`
	MemoriesDiscarded     int      `json:"memories_discarded"`
	PendingActionsWritten int      `json:"pending_actions_written"`
	CommitmentsWritten    int      `json:"commitments_written"`
	Errors                []string `json:"errors,omitempty"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	agentID, err := requiredString("agent_id", inputs)
	if err != nil {
		return nil, err
	}
	rawJSON, err := requiredString("extraction_json", inputs)
	if err != nil {
		return nil, err
	}

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return nil, fmt.Errorf("execution context with API URL is required")
	}

	// The extraction_json input often arrives wrapped in markdown code
	// fences (AI models love those). Strip them defensively before
	// unmarshalling so flow authors don't have to pre-process.
	cleaned := stripCodeFence(rawJSON)

	var payload extractionPayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		// Malformed JSON is a hard failure — the extraction pass
		// produced nothing usable, so we can't partial-succeed around
		// it. Return an error so the execution is marked failed and
		// the admin sees it in the detail view.
		return nil, fmt.Errorf("failed to parse extraction JSON: %w", err)
	}

	agentUserID := optionalString("agent_user_id", inputs)
	conversationID := optionalString("conversation_id", inputs)
	sourceMessageID := optionalString("source_message_id", inputs)

	result := extractionResult{}

	processMemories(flow, ctx, agentID, agentUserID, conversationID, sourceMessageID, payload.Memories, &result)
	processPendingActions(flow, ctx, agentID, agentUserID, conversationID, sourceMessageID, payload.ProposedActions, &result)
	processCommitments(flow, ctx, agentID, agentUserID, conversationID, sourceMessageID, payload.Commitments, &result)

	return map[string]interface{}{
		"memories_written":        result.MemoriesWritten,
		"memories_flagged":        result.MemoriesFlagged,
		"memories_discarded":      result.MemoriesDiscarded,
		"pending_actions_written": result.PendingActionsWritten,
		"commitments_written":     result.CommitmentsWritten,
		"errors":                  result.Errors,
	}, nil
}

// --- memories ---

func processMemories(
	flow *core.Flow, ctx *core.ExecutionContext,
	agentID, agentUserID, conversationID, sourceMessageID string,
	memories []extractionMemory, result *extractionResult,
) {
	for i, mem := range memories {
		// Discard below the floor — the plan's "<0.5 discarded" rule.
		// Nothing is written, nothing is logged as an error. Silent
		// discard is the intended behaviour for low-confidence noise.
		if mem.Confidence < confidenceDiscardThreshold {
			result.MemoriesDiscarded++
			continue
		}

		// Auto-pin preference/feedback types per plans/agent_memory.md
		// memory-type table — these are the "always included" types.
		pinned := mem.Pinned
		if mem.Type == "preference" || mem.Type == "feedback" {
			pinned = true
		}

		scope := "user"
		body := map[string]interface{}{
			"scope":       scope,
			"memory_type": mem.Type,
			"title":       mem.Title,
			"body":        mem.Body,
			"confidence":  mem.Confidence,
			"pinned":      pinned,
		}
		if agentUserID != "" {
			body["agent_user_id"] = agentUserID
		} else {
			// No user_id → the memory is agent-global rather than
			// scoped to a specific person. The extraction flow can't
			// always resolve a user (e.g. first-contact webhook), so
			// this is the graceful degradation path.
			body["scope"] = "global"
		}
		if conversationID != "" {
			body["source_conversation"] = conversationID
		}
		if sourceMessageID != "" {
			body["source_message"] = sourceMessageID
		}

		if err := postJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/agent/%s/memory", agentID), body, http.StatusCreated); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("memory[%d]: %v", i, err))
			continue
		}

		// 0.5–0.8 → flagged (stored but counted separately for the
		// profile UI to surface for review). >=0.8 → normal store.
		if mem.Confidence < confidenceStoreThreshold {
			result.MemoriesFlagged++
		} else {
			result.MemoriesWritten++
		}
	}
}

// --- pending actions ---

func processPendingActions(
	flow *core.Flow, ctx *core.ExecutionContext,
	agentID, agentUserID, conversationID, sourceMessageID string,
	actions []extractionAction, result *extractionResult,
) {
	if agentUserID == "" {
		// Pending actions require a user_id (the schema enforces
		// NOT NULL). Skip all of them if the extraction flow couldn't
		// resolve one — they can't be created, and flagging this as a
		// per-record error would be misleading since it's a missing
		// upstream input, not a per-action failure.
		return
	}
	for i, pa := range actions {
		if pa.Confidence < confidenceDiscardThreshold {
			continue
		}

		payload := pa.Payload
		if payload == nil {
			payload = map[string]interface{}{}
		}

		body := map[string]interface{}{
			"agent_user_id": agentUserID,
			"type":          pa.Type,
			"evidence":      pa.Evidence,
			"payload":       payload,
		}
		if conversationID != "" {
			body["source_conversation"] = conversationID
		}
		if sourceMessageID != "" {
			body["source_message"] = sourceMessageID
		}

		if err := postJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/agent/%s/pending-action", agentID), body, http.StatusCreated); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("pending_action[%d]: %v", i, err))
			continue
		}
		result.PendingActionsWritten++
	}
}

// --- commitments ---

func processCommitments(
	flow *core.Flow, ctx *core.ExecutionContext,
	agentID, agentUserID, conversationID, sourceMessageID string,
	commits []extractionCommit, result *extractionResult,
) {
	for i, c := range commits {
		if c.Confidence < confidenceDiscardThreshold {
			continue
		}

		madeBy := c.MadeBy
		if madeBy == "" {
			madeBy = "assistant" // most commitments come from assistant turns
		}

		body := map[string]interface{}{
			"kind":         c.Kind,
			"description":  c.Description,
			"trigger_type": c.TriggerType,
			"made_by":      madeBy,
		}
		if agentUserID != "" {
			body["agent_user_id"] = agentUserID
		}
		if conversationID != "" {
			body["conversation_id"] = conversationID
			body["source_conversation"] = conversationID
		}
		if sourceMessageID != "" {
			body["source_message"] = sourceMessageID
		}
		if c.DueAt != "" {
			body["due_at"] = c.DueAt
		}
		// Phase 2d does NOT resolve "in 30 minutes" style due_in strings
		// into absolute timestamps here. The Phase 3 commitment poller
		// or a future date-math action will handle that. If due_at is
		// unset, the commitment is stored with NULL due_at and will
		// only fire when a future phase backfills it.

		if err := postJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/agent/%s/commitment", agentID), body, http.StatusCreated); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("commitment[%d]: %v", i, err))
			continue
		}
		result.CommitmentsWritten++
	}
}

// --- shared HTTP helper ---

// postJSON posts a JSON body to the given API path (relative — the
// ctx.APIURL base is prepended) and returns an error if the response
// status doesn't match expectStatus. Body content on a non-matching
// status is included in the error for debuggability.
func postJSON(flow *core.Flow, ctx *core.ExecutionContext, path string, body map[string]interface{}, expectStatus int) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, ctx.APIURL+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != expectStatus {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// stripCodeFence unwraps a ```json … ``` or ``` … ``` block if present.
// AI models frequently return structured output inside a fenced code
// block even when asked for raw JSON; defending against that upfront
// means flow authors don't have to shim it.
func stripCodeFence(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	// Remove the opening fence line (``` or ```json etc).
	if idx := strings.Index(trimmed, "\n"); idx != -1 {
		trimmed = trimmed[idx+1:]
	} else {
		return trimmed // just a lone ``` — let JSON parser fail with a useful error
	}
	// Remove the closing fence if present.
	trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	return strings.TrimSpace(trimmed)
}

// --- input helpers ---

func requiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}

func optionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}
