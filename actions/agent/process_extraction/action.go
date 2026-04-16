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
	"time"

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
	{Name: "confirmations_processed", Type: core.ConnectionTypeInteger, Label: "Confirmations processed"},
	{Name: "identities_merged", Type: core.ConnectionTypeInteger, Label: "Identities merged"},
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
	Recurrence  string  `json:"recurrence,omitempty"` // 'daily' | 'weekly' | 'monthly' | 'every Monday' | etc.
}

type extractionConfirm struct {
	PendingActionID string `json:"pending_action_id"`
	Resolution      string `json:"resolution"` // 'confirmed' | 'declined' | 'task_completed'
	Evidence        string `json:"evidence"`
	TaskTitle       string `json:"task_title,omitempty"` // For task_completed: which task
}

// extractionResult summarises what the action did, returned as the
// node's output and surfaced in the execution detail view.
type extractionResult struct {
	MemoriesWritten         int      `json:"memories_written"`
	MemoriesFlagged         int      `json:"memories_flagged"`
	MemoriesDiscarded       int      `json:"memories_discarded"`
	MemoriesSuperseded      int      `json:"memories_superseded"`
	MemoriesDeduplicated    int      `json:"memories_deduplicated"`
	PendingActionsWritten   int      `json:"pending_actions_written"`
	CommitmentsWritten      int      `json:"commitments_written"`
	ConfirmationsProcessed  int      `json:"confirmations_processed"`
	IdentitiesMerged        int      `json:"identities_merged"`
	Errors                  []string `json:"errors,omitempty"`
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
	processConfirmations(flow, ctx, agentID, agentUserID, payload.Confirmations, &result)

	return map[string]interface{}{
		"memories_written":         result.MemoriesWritten,
		"memories_flagged":         result.MemoriesFlagged,
		"memories_discarded":       result.MemoriesDiscarded,
		"memories_superseded":      result.MemoriesSuperseded,
		"memories_deduplicated":    result.MemoriesDeduplicated,
		"pending_actions_written":  result.PendingActionsWritten,
		"commitments_written":      result.CommitmentsWritten,
		"confirmations_processed":  result.ConfirmationsProcessed,
		"identities_merged":        result.IdentitiesMerged,
		"errors":                   result.Errors,
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

		// Phase 7: temporal decay — auto-set valid_until based on type.
		switch mem.Type {
		case "task":
			validUntil := time.Now().Add(7 * 24 * time.Hour)
			body["valid_until"] = validUntil.Format(time.RFC3339)
		case "session_summary":
			validUntil := time.Now().Add(30 * 24 * time.Hour)
			body["valid_until"] = validUntil.Format(time.RFC3339)
		}

		resp, err := postJSONWithResponse(flow, ctx, fmt.Sprintf("/api/v1/internal/agent/%s/memory", agentID), body, http.StatusCreated)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("memory[%d]: %v", i, err))
			continue
		}

		// Extract the new memory ID for hygiene checks.
		newMemoryID := ""
		if resp != nil {
			if id, ok := resp["id"].(string); ok {
				newMemoryID = id
			}
		}

		// 0.5–0.8 → flagged (stored but counted separately for the
		// profile UI to surface for review). >=0.8 → normal store.
		if mem.Confidence < confidenceStoreThreshold {
			result.MemoriesFlagged++
		} else {
			result.MemoriesWritten++
		}

		// Phase 7: hygiene checks — find contradictions and duplicates.
		// Only run if we have a memory ID and the memory has an embedding
		// (the backfill goroutine may not have embedded it yet, but the
		// API endpoint handles missing embeddings gracefully).
		if newMemoryID != "" && agentUserID != "" {
			runHygieneCheck(flow, ctx, agentID, agentUserID, newMemoryID, mem, result)
		}
	}

	// Phase 7: enforce pin limit after all memories are written.
	if agentUserID != "" {
		_ = postJSON(flow, ctx,
			fmt.Sprintf("/api/v1/internal/agent/%s/memory/enforce-pin-limit", agentID),
			map[string]interface{}{"agent_user_id": agentUserID},
			http.StatusOK)
	}
}

// runHygieneCheck calls the API's check-hygiene endpoint to find
// contradictions and duplicates, then resolves them.
func runHygieneCheck(
	flow *core.Flow, ctx *core.ExecutionContext,
	agentID, agentUserID, newMemoryID string,
	mem extractionMemory, result *extractionResult,
) {
	// Try to fetch the new memory's embedding for similarity-based checks.
	// If no embedding yet (backfill hasn't run), use a zero-length slice —
	// the API-side title-based contradiction check doesn't need one.
	var embeddingSlice []float32
	memData, err := getJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/memory/%s", newMemoryID))
	if err == nil && memData != nil {
		if embeddingRaw, ok := memData["embedding"]; ok && embeddingRaw != nil {
			if arr, ok := embeddingRaw.([]interface{}); ok {
				for _, f := range arr {
					if n, ok := f.(float64); ok {
						embeddingSlice = append(embeddingSlice, float32(n))
					}
				}
			}
		}
	}

	// Call check-hygiene endpoint.
	hygieneResp, err := postJSONWithResponse(flow, ctx,
		fmt.Sprintf("/api/v1/internal/agent/%s/memory/check-hygiene", agentID),
		map[string]interface{}{
			"agent_user_id": agentUserID,
			"memory_type":   mem.Type,
			"memory_id":     newMemoryID,
			"title":         mem.Title,
			"body":          mem.Body,
			"embedding":     embeddingSlice,
		}, http.StatusOK)
	if err != nil || hygieneResp == nil {
		return
	}

	// Process duplicates (merge).
	if dupes, ok := hygieneResp["duplicates"].([]interface{}); ok {
		for _, d := range dupes {
			if dm, ok := d.(map[string]interface{}); ok {
				if dupeID, ok := dm["id"].(string); ok && dupeID != newMemoryID {
					_ = postJSON(flow, ctx,
						fmt.Sprintf("/api/v1/internal/agent/%s/memory/merge", agentID),
						map[string]interface{}{
							"duplicate_id": dupeID,
							"canonical_id": newMemoryID,
						}, http.StatusNoContent)
					result.MemoriesDeduplicated++
				}
			}
		}
	}

	// Process contradictions (supersede).
	if contras, ok := hygieneResp["contradictions"].([]interface{}); ok {
		for _, c := range contras {
			if cm, ok := c.(map[string]interface{}); ok {
				if oldID, ok := cm["id"].(string); ok && oldID != newMemoryID {
					_ = postJSON(flow, ctx,
						fmt.Sprintf("/api/v1/internal/agent/%s/memory/supersede", agentID),
						map[string]interface{}{
							"old_id": oldID,
							"new_id": newMemoryID,
						}, http.StatusNoContent)
					result.MemoriesSuperseded++
				}
			}
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

		// Deduplicate: skip if an open pending action of the same type
		// already exists for this user. Prevents duplicate identity_link
		// records when extraction fires on both inbound and assistant turns.
		if pa.Type == "identity_link" {
			existing, _ := getJSON(flow, ctx, fmt.Sprintf(
				"/api/v1/internal/agent/%s/pending-action/match?agent_user_id=%s&type=identity_link",
				agentID, agentUserID))
			if existing != nil {
				if eid, ok := existing["id"].(string); ok && eid != "" {
					// Already have an open identity_link for this user — skip.
					continue
				}
			}
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
		} else if c.DueIn != "" {
			// Phase 3: resolve relative durations ("30 minutes", "1 hour",
			// "2 hours") into absolute timestamps so the commitment poller
			// can select them via due_at <= NOW(). Only simple durations
			// are handled; complex expressions ("tomorrow at 9am") should
			// be emitted as due_at by the AI action directly.
			if resolved := resolveDueIn(c.DueIn); resolved != "" {
				body["due_at"] = resolved
			}
		}

		if c.Recurrence != "" {
			body["recurrence"] = c.Recurrence
		}

		if err := postJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/agent/%s/commitment", agentID), body, http.StatusCreated); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("commitment[%d]: %v", i, err))
			continue
		}
		result.CommitmentsWritten++
	}
}

// --- confirmations (Phase 5) ---

func processConfirmations(
	flow *core.Flow, ctx *core.ExecutionContext,
	agentID, agentUserID string,
	confirmations []extractionConfirm, result *extractionResult,
) {
	if agentUserID == "" || len(confirmations) == 0 {
		return
	}

	for i, conf := range confirmations {
		_ = i // used in error messages below
		// Phase 7: handle task_completed confirmations by superseding
		// the matching task memory.
		if conf.Resolution == "task_completed" {
			handleTaskCompleted(flow, ctx, agentID, agentUserID, conf, result)
			continue
		}

		// Resolve the pending action — either by explicit ID or by
		// matching on user + type (identity_link is the primary use case).
		paID := conf.PendingActionID

		if paID == "" {
			// Try to find a matching pending action by type. The extraction
			// pipeline may not know the ID, so we search for the most recent
			// open pending action of type "identity_link" for this user.
			pa, err := getJSON(flow, ctx, fmt.Sprintf(
				"/api/v1/internal/agent/%s/pending-action/match?agent_user_id=%s&type=identity_link",
				agentID, agentUserID))
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("confirmation[%d]: no matching pending action: %v", i, err))
				continue
			}
			if id, ok := pa["id"].(string); ok {
				paID = id
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("confirmation[%d]: no matching pending action found", i))
				continue
			}
		}

		if conf.Resolution == "declined" {
			// User declined — mark the pending action as declined.
			if err := patchJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/pending-action/%s", paID),
				map[string]interface{}{"status": "declined"}); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("confirmation[%d]: failed to decline: %v", i, err))
			}
			result.ConfirmationsProcessed++
			continue
		}

		if conf.Resolution == "confirmed" {
			// Fetch the pending action to check its current status and payload.
			pa, err := getJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/pending-action/%s", paID))
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("confirmation[%d]: failed to fetch pending action: %v", i, err))
				continue
			}

			currentStatus, _ := pa["status"].(string)
			paType, _ := pa["type"].(string)

			if paType == "identity_link" {
				if currentStatus == "awaiting_confirmation" {
					// First side confirmed. Move to awaiting-other-side.
					if err := patchJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/pending-action/%s", paID),
						map[string]interface{}{"status": "confirmed_here_awaiting_other_side"}); err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("confirmation[%d]: failed to update status: %v", i, err))
						continue
					}

					// Proactively dispatch verification to the other channel.
					// The API looks up the target identity, checks channel
					// privacy, creates a target-side PA, and forwards to
					// Launch to fire the orchestrator on the target channel.
					sourceChannel := ""
					if sc, ok := pa["source_channel"].(string); ok {
						sourceChannel = sc
					}
					_ = postJSON(flow, ctx,
						fmt.Sprintf("/api/v1/internal/agent/%s/identity/request-verification", agentID),
						map[string]interface{}{
							"pending_action_id":  paID,
							"source_user_id":     agentUserID,
							"source_channel_type": sourceChannel,
						}, http.StatusOK)

					result.ConfirmationsProcessed++
				} else if currentStatus == "confirmed_here_awaiting_other_side" {
					// Both sides confirmed! Execute the merge.
					payload, _ := pa["payload"].(map[string]interface{})
					sourceUserID, _ := payload["source_user_id"].(string)
					targetUserID, _ := payload["target_user_id"].(string)

					if sourceUserID == "" || targetUserID == "" {
						// Fall back to merging current user into the
						// identity linked user. The claiming side's
						// agent_user_id is the source; the target is
						// the identity they claimed to also be.
						sourceUserID = agentUserID
						if tgt, ok := payload["target_user_id"].(string); ok && tgt != "" {
							targetUserID = tgt
						}
					}

					if sourceUserID != "" && targetUserID != "" && sourceUserID != targetUserID {
						if err := postJSON(flow, ctx,
							fmt.Sprintf("/api/v1/internal/agent/%s/identity/merge", agentID),
							map[string]interface{}{
								"source_user_id": sourceUserID,
								"target_user_id": targetUserID,
							}, http.StatusNoContent); err != nil {
							result.Errors = append(result.Errors, fmt.Sprintf("confirmation[%d]: merge failed: %v", i, err))
							continue
						}
						result.IdentitiesMerged++
					}

					// Mark the pending action as executed.
					_ = patchJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/pending-action/%s", paID),
						map[string]interface{}{"status": "executed"})

					result.ConfirmationsProcessed++
				}
			} else {
				// Non-identity-link confirmations (forget_memory, correct_memory, etc.)
				// Mark as executed — the specific side effects are handled
				// by dedicated actions in future phases.
				if err := patchJSON(flow, ctx, fmt.Sprintf("/api/v1/internal/pending-action/%s", paID),
					map[string]interface{}{"status": "executed"}); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("confirmation[%d]: failed to execute: %v", i, err))
					continue
				}
				result.ConfirmationsProcessed++
			}
		}
	}
}

// handleTaskCompleted searches for active memories matching the
// confirmation's task_title and supersedes them. This covers both
// task-type memories AND related fact memories (e.g. "clinic contact
// methods" when the user cancels "forget about the clinic research").
//
// Matching strategy:
//  1. Title substring match (fast, covers exact and partial matches)
//  2. Keyword overlap (catches semantically related facts)
func handleTaskCompleted(
	flow *core.Flow, ctx *core.ExecutionContext,
	agentID, agentUserID string,
	conf extractionConfirm, result *extractionResult,
) {
	if conf.TaskTitle == "" {
		return
	}

	endpoint := fmt.Sprintf(
		"/api/v1/internal/agent/%s/memory?agent_user_id=%s&limit=50",
		agentID, agentUserID)

	rawResp, err := getRaw(flow, ctx, endpoint)
	if err != nil || rawResp == nil {
		return
	}

	var memories []struct {
		ID         string `json:"id"`
		MemoryType string `json:"memory_type"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(rawResp, &memories); err != nil {
		return
	}

	taskTitle := strings.ToLower(conf.TaskTitle)
	// Extract keywords (3+ chars) from the task title for fuzzy matching.
	keywords := extractKeywords(taskTitle)

	superseded := 0
	for _, mem := range memories {
		if mem.Status != "active" {
			continue
		}

		memTitle := strings.ToLower(mem.Title)
		memBody := strings.ToLower(mem.Body)
		matched := false

		// 1. Direct title match (tasks and facts).
		if strings.Contains(memTitle, taskTitle) || strings.Contains(taskTitle, memTitle) {
			matched = true
		}

		// 2. Keyword overlap — if 2+ keywords from the task title appear
		//    in the memory's title or body, it's related.
		if !matched && len(keywords) >= 2 {
			hits := 0
			for _, kw := range keywords {
				if strings.Contains(memTitle, kw) || strings.Contains(memBody, kw) {
					hits++
				}
			}
			if hits >= 2 || (len(keywords) <= 2 && hits >= 1) {
				matched = true
			}
		}

		if matched {
			_ = postJSON(flow, ctx,
				fmt.Sprintf("/api/v1/internal/agent/%s/memory/supersede", agentID),
				map[string]interface{}{
					"old_id": mem.ID,
					"new_id": mem.ID,
				}, http.StatusNoContent)
			superseded++
		}
	}

	if superseded > 0 {
		result.ConfirmationsProcessed++
	}
}

// extractKeywords splits text into words of 3+ characters, filtering
// out common stop words.
func extractKeywords(text string) []string {
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true,
		"that": true, "this": true, "from": true, "about": true,
		"will": true, "have": true, "been": true, "they": true,
		"their": true, "your": true, "what": true, "when": true,
		"where": true, "which": true, "who": true, "how": true,
		"are": true, "was": true, "were": true, "has": true,
		"had": true, "can": true, "not": true, "all": true,
	}
	words := strings.Fields(text)
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:\"'()-")
		if len(w) >= 3 && !stopWords[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

// getRaw performs a GET request and returns the raw response body.
func getRaw(flow *core.Flow, ctx *core.ExecutionContext, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, ctx.APIURL+path, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// getJSON performs a GET request and returns the parsed JSON body as a map.
func getJSON(flow *core.Flow, ctx *core.ExecutionContext, path string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, ctx.APIURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return result, nil
}

// patchJSON performs a PATCH request with a JSON body.
func patchJSON(flow *core.Flow, ctx *core.ExecutionContext, path string, body map[string]interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPatch, ctx.APIURL+path, bytes.NewReader(raw))
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

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
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

// postJSONWithResponse is like postJSON but returns the decoded response body.
func postJSONWithResponse(flow *core.Flow, ctx *core.ExecutionContext, path string, body map[string]interface{}, expectStatus int) (map[string]interface{}, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, ctx.APIURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != expectStatus {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil // Non-JSON response is ok for some endpoints
	}
	return result, nil
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

// resolveDueIn parses a human-readable time expression into an ISO-8601
// absolute timestamp. Handles three families of input:
//
//  1. Go durations: "30m", "1h", "2h30m"
//  2. Natural relative: "30 minutes", "2 hours", "1 day", "3 weeks"
//  3. Named relative: "tomorrow", "tomorrow at 9am", "tomorrow at 14:30",
//     "next monday", "next tuesday at 10am", "in an hour"
//
// Returns empty string for genuinely unparseable inputs — the commitment
// will be stored without a due_at and surfaced in the admin UI for
// manual resolution.
func resolveDueIn(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}

	now := time.Now()

	// --- family 1: Go duration ("30m", "1h", "2h30m") ---
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(d).UTC().Format(time.RFC3339)
	}

	// --- family 3: named relative patterns (check before numeric) ---

	// "in an hour", "in a minute", "in a day"
	if strings.HasPrefix(s, "in a ") || strings.HasPrefix(s, "in an ") {
		rest := strings.TrimPrefix(strings.TrimPrefix(s, "in an "), "in a ")
		rest = strings.TrimSpace(rest)
		switch {
		case strings.HasPrefix(rest, "hour"):
			return now.Add(time.Hour).UTC().Format(time.RFC3339)
		case strings.HasPrefix(rest, "minute"):
			return now.Add(time.Minute).UTC().Format(time.RFC3339)
		case strings.HasPrefix(rest, "day"):
			return now.Add(24 * time.Hour).UTC().Format(time.RFC3339)
		case strings.HasPrefix(rest, "week"):
			return now.Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)
		}
	}

	// "tomorrow" or "tomorrow at HH:MM" / "tomorrow at Ham" / "tomorrow at H:MMam"
	if strings.HasPrefix(s, "tomorrow") {
		tomorrow := now.AddDate(0, 0, 1)
		if t := extractTimeOfDay(s, tomorrow); !t.IsZero() {
			return t.UTC().Format(time.RFC3339)
		}
		// Bare "tomorrow" → same time tomorrow
		return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(),
			now.Hour(), now.Minute(), 0, 0, now.Location()).UTC().Format(time.RFC3339)
	}

	// "next monday", "next tuesday at 10am", etc.
	if strings.HasPrefix(s, "next ") {
		rest := strings.TrimPrefix(s, "next ")
		if target := resolveNextWeekday(rest, now); !target.IsZero() {
			return target.UTC().Format(time.RFC3339)
		}
	}

	// --- family 2: "<number> <unit>" ("30 minutes", "2 hours") ---
	// Also handles "in 30 minutes" by stripping the leading "in ".
	numeric := strings.TrimPrefix(s, "in ")
	numeric = strings.TrimSpace(numeric)
	parts := strings.Fields(numeric)
	if len(parts) >= 2 {
		var n float64
		if _, err := fmt.Sscanf(parts[0], "%f", &n); err == nil && n > 0 {
			unit := strings.TrimSuffix(parts[1], "s")
			var d time.Duration
			switch unit {
			case "second":
				d = time.Duration(n) * time.Second
			case "minute":
				d = time.Duration(n) * time.Minute
			case "hour":
				d = time.Duration(n) * time.Hour
			case "day":
				d = time.Duration(n) * 24 * time.Hour
			case "week":
				d = time.Duration(n) * 7 * 24 * time.Hour
			case "month":
				return now.AddDate(0, int(n), 0).UTC().Format(time.RFC3339)
			}
			if d > 0 {
				return now.Add(d).UTC().Format(time.RFC3339)
			}
		}
	}

	return ""
}

// extractTimeOfDay looks for "at HH:MM", "at Ham", "at H:MMam", "at HH:MMpm"
// patterns inside s and returns the given day with that time set. Returns
// zero time if no pattern is found.
func extractTimeOfDay(s string, day time.Time) time.Time {
	atIdx := strings.Index(s, "at ")
	if atIdx == -1 {
		return time.Time{}
	}
	timeStr := strings.TrimSpace(s[atIdx+3:])

	// Try 24-hour formats first
	for _, layout := range []string{"15:04", "15"} {
		if t, err := time.Parse(layout, timeStr); err == nil {
			return time.Date(day.Year(), day.Month(), day.Day(),
				t.Hour(), t.Minute(), 0, 0, day.Location())
		}
	}

	// Try 12-hour formats: "9am", "9:30am", "9 am", "9:30 am"
	timeStr = strings.ReplaceAll(timeStr, " ", "") // normalise "9 am" → "9am"
	for _, layout := range []string{"3:04pm", "3pm", "3:04am", "3am"} {
		// time.Parse uses Go's reference time; "pm"/"am" are handled natively
		if t, err := time.Parse(layout, timeStr); err == nil {
			return time.Date(day.Year(), day.Month(), day.Day(),
				t.Hour(), t.Minute(), 0, 0, day.Location())
		}
	}

	return time.Time{}
}

// resolveNextWeekday parses "monday", "tuesday at 10am", etc. and returns
// the next occurrence of that weekday (always in the future, 1-7 days out).
func resolveNextWeekday(s string, now time.Time) time.Time {
	weekdays := map[string]time.Weekday{
		"monday": time.Monday, "tuesday": time.Tuesday,
		"wednesday": time.Wednesday, "thursday": time.Thursday,
		"friday": time.Friday, "saturday": time.Saturday,
		"sunday": time.Sunday,
	}

	// Extract the weekday name (first word).
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return time.Time{}
	}
	target, ok := weekdays[parts[0]]
	if !ok {
		return time.Time{}
	}

	// Find the next occurrence.
	daysAhead := int(target-now.Weekday()+7) % 7
	if daysAhead == 0 {
		daysAhead = 7 // "next monday" on a monday = 7 days out
	}
	nextDay := now.AddDate(0, 0, daysAhead)

	// Check for "at HH:MM" suffix.
	if t := extractTimeOfDay(s, nextDay); !t.IsZero() {
		return t
	}

	// Default to 9am on that day — a reasonable default for "next monday"
	// that's better than midnight.
	return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(),
		9, 0, 0, 0, now.Location())
}
