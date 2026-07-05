// Package await implements the Human-in-the-Loop node. On its first pass it
// presents a message with N options to a human over one or more channels
// (by invoking the author's referenced Send Message nodes), registers the
// request with the API, and suspends the execution. When the human responds
// — a Slack/Telegram button or a tokenised web link handled by Launch — the
// API resumes the execution, injecting the chosen option into the checkpoint.
// The node then re-executes and routes down the handle for that option
// ("option_<value>"), or down "timeout" if the request expired first.
package await

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
	Name         = "Human Approval"
	Description  = "Ask a human to choose an option, then branch on their answer. Suspends until a reply or timeout."
	Website      = "https://www.flomation.co"
	Icon         = "user+check"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAwait
)

var Inputs = [...]core.Connection{
	{
		Name:        "message",
		Type:        core.ConnectionTypeText,
		Label:       "Message",
		Placeholder: "Approve the deployment to production?",
		Required:    true,
	},
	{
		Name:        "options",
		Type:        core.ConnectionTypeKeyValueArray,
		Label:       "Options",
		Placeholder: "Button label → value (drives the output handles)",
		Required:    true,
	},
	{
		Name:        "timeout",
		Type:        core.ConnectionTypeString,
		Label:       "Timeout",
		Placeholder: "24h (blank = 24h)",
	},
	{
		Name:        "web_base_url",
		Type:        core.ConnectionTypeString,
		Label:       "Response Link Base URL",
		Placeholder: "Public Launch URL for click-link fallback (optional)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "matched_case", Type: core.ConnectionTypeString, Label: "Chosen Handle"},
	{Name: "answered_option", Type: core.ConnectionTypeString, Label: "Chosen Option"},
	{Name: "answered_by", Type: core.ConnectionTypeString, Label: "Answered By"},
	{Name: "outcome", Type: core.ConnectionTypeString, Label: "Outcome"},
}

// defaultTimeout is used when the author leaves the timeout blank.
const defaultTimeout = 24 * time.Hour

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	// --- Resume pass: a human answered (or the request timed out) ---
	if flow.IsResumedNode(node.ID) {
		return resume(flow, node)
	}

	// --- First pass: register, deliver, suspend ---
	message := optStr("message", inputs)
	if message == "" {
		return failFirstPass("message is required"), fmt.Errorf("message is required")
	}

	opts := parseOptions(inputs)
	if len(opts) == 0 {
		return failFirstPass("at least one option is required"), fmt.Errorf("options are required")
	}

	timeout := defaultTimeout
	if ts := optStr("timeout", inputs); ts != "" {
		if d, err := parseDuration(ts); err == nil {
			timeout = d
		}
	}
	resumeAt := time.Now().UTC().Add(timeout)

	ctx := flow.GetContext()
	if ctx == nil || ctx.APIURL == "" {
		return failFirstPass("no execution context / API URL"), fmt.Errorf("await: no API URL in context")
	}

	// Register the request with the API. It upserts on (execution_id, node_id)
	// so a runner retry before the suspend persists reuses the same row, and
	// mints a per-option token for the web click-link fallback.
	registered, err := createRequest(flow, ctx, node, message, opts, resumeAt, optStr("web_base_url", inputs))
	if err != nil {
		return failFirstPass(fmt.Sprintf("could not register request: %v", err)),
			fmt.Errorf("await: register request: %w", err)
	}

	// Deliver over each referenced Send Message node. First response wins;
	// delivery failures on one channel don't abort the others.
	delivered, deliverErrs := deliver(flow, node, message, registered)

	// Remember the request id so the resume pass can verify the injected
	// answer belongs to this node (survives in the checkpoint variables).
	flow.SetVariable(requestVarKey(node.ID), registered.RequestID)

	flow.Suspend(&core.SuspendInfo{
		NodeID:            node.ID,
		Reason:            "await_human",
		ResumeAt:          &resumeAt,
		ResumeTriggerType: "hitl_response",
		ResumeTriggerMatch: map[string]interface{}{
			"request_id": registered.RequestID,
		},
	})

	summary := fmt.Sprintf("Awaiting human response (%d option(s)) — delivered to %d channel(s)", len(opts), delivered)
	if len(deliverErrs) > 0 {
		summary += fmt.Sprintf("; %d delivery error(s): %s", len(deliverErrs), strings.Join(deliverErrs, "; "))
	}
	return map[string]interface{}{
		"tool_result":  summary,
		"matched_case": "",
		"outcome":      "awaiting",
	}, core.ErrSuspended
}

// resume reads the injected answer and produces the routing handle.
func resume(flow *core.Flow, node *core.Node) (map[string]interface{}, error) {
	rd := flow.GetResumeData()
	awaitData, _ := rd["await"].(map[string]interface{})
	if awaitData == nil {
		// No answer payload — treat as a timeout so the flow still progresses
		// down a defined path rather than dead-ending.
		return map[string]interface{}{
			"tool_result":  "Resumed without a response payload — routing to timeout",
			"matched_case": "timeout",
			"outcome":      "timeout",
		}, nil
	}

	// Guard: the injected answer must belong to this node's request.
	if want, ok := flow.GetVariable(requestVarKey(node.ID)); ok {
		if got, _ := awaitData["request_id"].(string); got != "" && fmt.Sprintf("%v", want) != got {
			return map[string]interface{}{
				"tool_result":  "Resumed with a mismatched request id — routing to timeout",
				"matched_case": "timeout",
				"outcome":      "timeout",
			}, nil
		}
	}

	outcome, _ := awaitData["outcome"].(string)
	if outcome == "timeout" {
		return map[string]interface{}{
			"tool_result":  "No response before timeout",
			"matched_case": "timeout",
			"outcome":      "timeout",
		}, nil
	}

	optionValue, _ := awaitData["option_value"].(string)
	answeredBy, _ := awaitData["answered_by"].(string)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Human chose %q", optionValue),
		"matched_case":    "option_" + optionValue,
		"answered_option": optionValue,
		"answered_by":     answeredBy,
		"outcome":         "answered",
	}, nil
}

// --- request registration ---

type registeredRequest struct {
	RequestID  string   `json:"request_id"`
	WebBaseURL string   `json:"web_base_url"`
	Options    []Option `json:"options"`
}

func createRequest(flow *core.Flow, ctx *core.ExecutionContext, node *core.Node, message string, opts []Option, expiresAt time.Time, webBaseURL string) (*registeredRequest, error) {
	payload := map[string]interface{}{
		"execution_id": ctx.ExecutionID,
		"flo_id":       ctx.FlowID,
		"node_id":      node.ID,
		"message":      message,
		"options":      opts,
		"expires_at":   expiresAt.Format(time.RFC3339),
		"web_base_url": webBaseURL,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/internal/hitl/request", ctx.APIURL)
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if ctx.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.Token)
	}

	resp, err := ctx.InternalClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(b))
	}

	var out registeredRequest
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.RequestID == "" {
		return nil, fmt.Errorf("API returned an empty request id")
	}
	if webBaseURL != "" && out.WebBaseURL == "" {
		out.WebBaseURL = webBaseURL
	}
	return &out, nil
}

// --- delivery ---

// deliver renders the interactive payload for each Send Message node wired to
// the Await node's delivery handle and invokes it in-process. Returns the count
// delivered and any per-channel error strings (non-fatal). Delivery nodes are
// reached via the delivery handle — the author simply drops a Send node onto
// the canvas and wires it to the Await node, exactly like AI tools.
func deliver(flow *core.Flow, node *core.Node, message string, req *registeredRequest) (int, []string) {
	var delivered int
	var errs []string

	for _, target := range flow.FindTargetByHandle(node.ID, core.DeliveryHandle) {
		if target == nil || target.Data == nil {
			continue
		}
		channel := inferChannel(target.Data.Label, target.Type)
		extra := injectionFor(channel, message, req)
		if _, err := flow.InvokeNode(target, extra); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", target.ID, err))
			continue
		}
		delivered++
	}
	return delivered, errs
}

// injectionFor maps a channel to the Send node inputs to override.
func injectionFor(channel, message string, req *registeredRequest) map[string]interface{} {
	switch channel {
	case "slack":
		return map[string]interface{}{
			"blocks":  string(RenderSlackBlocks(message, req.RequestID, req.Options)),
			"message": message,
		}
	case "telegram":
		return map[string]interface{}{
			"reply_markup": string(RenderTelegramKeyboard(req.Options)),
			"message":      message,
		}
	case "email":
		return map[string]interface{}{"body": RenderWebLinks(message, req.WebBaseURL, req.Options)}
	case "discord":
		return map[string]interface{}{"content": RenderWebLinks(message, req.WebBaseURL, req.Options)}
	default: // sms / twilio / any other channel — plain text with links
		return map[string]interface{}{"message": RenderWebLinks(message, req.WebBaseURL, req.Options)}
	}
}

func inferChannel(label, nodeType string) string {
	s := strings.ToLower(label + " " + nodeType)
	switch {
	case strings.Contains(s, "slack"):
		return "slack"
	case strings.Contains(s, "telegram"):
		return "telegram"
	case strings.Contains(s, "email"):
		return "email"
	case strings.Contains(s, "discord"):
		return "discord"
	case strings.Contains(s, "twilio"), strings.Contains(s, "sms"):
		return "sms"
	default:
		return ""
	}
}

// --- input helpers ---

func parseOptions(inputs []*core.Connection) []Option {
	c := core.FindConnection("options", inputs)
	if c == nil {
		return nil
	}
	var opts []Option
	for _, kv := range c.KeyValuePairs() {
		label := strings.TrimSpace(kv.Key)
		value := strings.TrimSpace(kv.Value)
		if value == "" {
			value = slug(label)
		}
		if label == "" && value == "" {
			continue
		}
		if label == "" {
			label = value
		}
		opts = append(opts, Option{Value: value, Label: label})
	}
	return opts
}

// slug normalises a label into a handle-safe option value.
func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('_')
		}
	}
	return b.String()
}

func failFirstPass(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":  msg,
		"matched_case": "",
		"outcome":      "error",
	}
}

func requestVarKey(nodeID string) string { return "__await_" + nodeID + "_request_id" }

func optStr(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

// parseDuration supports Go durations plus days (d) and weeks (w).
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "d"), "%d", &days); err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	if strings.HasSuffix(s, "w") {
		var weeks int
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "w"), "%d", &weeks); err == nil && weeks > 0 {
			return time.Duration(weeks) * 7 * 24 * time.Hour, nil
		}
	}
	return 0, fmt.Errorf("unsupported duration: %s", s)
}
