package core

import (
	"bytes"
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"flomation.app/automate/executor/internal/assets"
	"flomation.app/automate/executor/internal/environment"
	log "github.com/sirupsen/logrus"
)

// manifestDescriptions is a lazy-loaded lookup from action label
// (e.g. "tools/calendar_read") to the manifest description string.
// Used by injectToolDefinitions to produce richer tool descriptions.
var manifestDescriptions map[string]string

func getManifestDescriptions() map[string]string {
	if manifestDescriptions != nil {
		return manifestDescriptions
	}
	manifestDescriptions = make(map[string]string)
	data, err := assets.Manifest.ReadFile("manifest/manifest.json")
	if err != nil {
		return manifestDescriptions
	}
	var manifest map[string]struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifestDescriptions
	}
	for key, entry := range manifest {
		if entry.Description != "" {
			manifestDescriptions[key] = entry.Description
		}
	}
	return manifestDescriptions
}

// credentialMetaLinks maps actionID -> inputName -> credential-metadata key, for
// every input declaring FromCredentialMeta.
var credentialMetaLinks map[string]map[string]string

// getCredentialMetaLinks reads the links from the EMBEDDED MANIFEST rather than
// from the saved node, and that distinction is the whole point.
//
// A node's inputs are snapshotted into the flow when it is added to the canvas,
// so a flow built before this feature existed carries inputs with no
// from_credential_meta at all. Reading the manifest means the auto-fill starts
// working for those flows the moment the executor ships, with no re-save and no
// migration. Mirrors getManifestDescriptions, which exists for the same reason.
func getCredentialMetaLinks() map[string]map[string]string {
	if credentialMetaLinks != nil {
		return credentialMetaLinks
	}
	credentialMetaLinks = make(map[string]map[string]string)
	data, err := assets.Manifest.ReadFile("manifest/manifest.json")
	if err != nil {
		return credentialMetaLinks
	}
	var manifest map[string]struct {
		Inputs []struct {
			Name               string `json:"name"`
			FromCredentialMeta string `json:"from_credential_meta"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return credentialMetaLinks
	}
	for actionID, entry := range manifest {
		for _, in := range entry.Inputs {
			if in.FromCredentialMeta == "" {
				continue
			}
			if credentialMetaLinks[actionID] == nil {
				credentialMetaLinks[actionID] = make(map[string]string)
			}
			credentialMetaLinks[actionID][in.Name] = in.FromCredentialMeta
		}
	}
	return credentialMetaLinks
}

// credentialRefPattern matches a whole-value ${credentials.NAME} reference — and
// deliberately NOT ${credentials.NAME.key}, which is already resolved to a
// metadata value and is not the connection itself. Credential names are
// sanitised to [A-Za-z0-9_-] (no dots), so the absence of a dot is a reliable
// discriminator.
var credentialRefPattern = regexp.MustCompile(`^\$\{credentials\.([A-Za-z0-9_-]+)\}$`)

// autofillFromCredential fills blank inputs that declare FromCredentialMeta,
// using the credential bound to a sibling input on the same node.
//
// It rewrites the value to a ${credentials.NAME.key} reference rather than
// fetching anything: substitution already knows how to resolve those, so this
// only writes the reference the operator would otherwise have had to know
// existed. Returns the inputs unchanged when the action declares no links, when
// the operator has typed something, or when no credential is bound — the last
// being the bring-your-own-token path, which must keep working.
func autofillFromCredential(actionID string, inputs []*Connection) {
	links := getCredentialMetaLinks()[actionID]
	if len(links) == 0 {
		return
	}

	// The credential bound anywhere on this node. Actions have exactly one
	// connection input in practice, so the first match is the right one.
	credName := ""
	for _, in := range inputs {
		if in == nil {
			continue
		}
		s, ok := in.Value.(string)
		if !ok {
			continue
		}
		if m := credentialRefPattern.FindStringSubmatch(strings.TrimSpace(s)); m != nil {
			credName = m[1]
			break
		}
	}
	if credName == "" {
		return
	}

	for i := range inputs {
		if inputs[i] == nil {
			continue
		}
		key, linked := links[inputs[i].Name]
		if !linked {
			continue
		}
		// Anything the operator typed wins. A blank string counts as untouched,
		// matching how the auto-wire above treats an empty input.
		if s, ok := inputs[i].Value.(string); ok && strings.TrimSpace(s) != "" {
			continue
		} else if !ok && inputs[i].Value != nil {
			continue
		}
		inputs[i].Value = "${credentials." + credName + "." + key + "}"
		log.WithFields(log.Fields{
			"action": actionID, "input": inputs[i].Name, "credential": credName,
		}).Debug("filled a blank input from the connected credential's metadata")
	}
}

const (
	TriggerTypeManual = "trigger/manual"
)

const (
	ActionTypeTrigger     = 1
	ActionTypeAction      = 2
	ActionTypeOutput      = 3
	ActionTypeConditional = 4
	ActionTypeLoop        = 5
	ActionTypeSwitch      = 6
	// ActionTypeAwait is the Human-in-the-Loop node. Like Switch it exposes
	// multiple output handles (one per option, plus "timeout") and routes to
	// the matched handle — but it also suspends the flow while awaiting a
	// human response, then resumes down the handle for the chosen option.
	ActionTypeAwait = 7
)

const (
	// ToolRequestsKey is the special output key that AI actions set when
	// the model returns tool_use (Anthropic) or tool_calls (OpenAI).
	// The engine detects this in executeNodeChildren and enters the
	// tool loop — executing the tools subgraph and feeding results back.
	ToolRequestsKey = "__tool_requests"

	// ToolConversationStateKey carries the full messages array across
	// tool rounds so the AI API sees the complete conversation including
	// all tool_use and tool_result blocks.
	ToolConversationStateKey = "__conversation_state"

	// ToolResultsKey is set by the engine after executing the tools
	// subgraph. The AI action reads it on re-invocation to build
	// tool_result content blocks for the API.
	ToolResultsKey = "__tool_results"

	// IntermediateTextKey carries any text the AI emitted alongside
	// tool_use blocks. The engine fires Response handle children with
	// this text mid-loop so the user sees progress messages like
	// "Checking your calendar..." while tools execute.
	IntermediateTextKey = "__intermediate_text"

	// ToolExchangesKey accumulates completed tool exchanges across all
	// rounds of the tool loop. Each entry is a map with tool_use_id,
	// name, input, result, and is_error. The AI actions read this on
	// their final (non-tool_use) invocation to record the exchanges
	// in the conversation history via RecordToolExchange.
	ToolExchangesKey = "__tool_exchanges"

	// ToolsHandle is the source handle ID for the tools subgraph edge.
	ToolsHandle = "tools"

	// DeliveryHandle is the source handle ID the Human-in-the-Loop node uses to
	// reach the Send Message nodes it fans a request out to. Like the tools
	// handle, its children are invoked in-process, not walked as normal
	// downstream nodes.
	DeliveryHandle = "delivery"

	// MaxToolRoundsDefault is the safety cap on tool calling rounds.
	MaxToolRoundsDefault = 25

	// StreamSentencesKey flags that the AI action is streaming. The
	// actual channel is stored as a flow variable (not in outputs)
	// to avoid JSON serialisation issues.
	StreamSentencesKey = "__stream_sentences"

	// StreamToolRequestsKey is the flow variable where the streaming
	// goroutine stores any tool_use requests found during SSE parsing.
	StreamToolRequestsKey = "__stream_tool_requests"

	// StreamFullTextKey stores accumulated full text from streaming.
	StreamFullTextKey = "__stream_full_text"

	// StreamStopReasonKey stores the stop_reason from a streaming response.
	StreamStopReasonKey = "__stream_stop_reason"

	// StreamUsageKey stores token usage from a streaming response.
	StreamUsageKey = "__stream_usage"

	// SubFlowNameKey is the output key set by subflow/invoke actions.
	// The engine detects this in executeNodeChildren and dispatches
	// to the matching subflow/begin node.
	SubFlowNameKey = "__subflow_name"

	// MaxSubFlowDepth caps recursion for sub-flow invocations.
	MaxSubFlowDepth = 10
)

// ToolRequest represents a single tool call from an AI model.
// Anthropic: content block with type="tool_use".
// OpenAI: tool_calls array entry with type="function".
type ToolRequest struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ToolResult carries the output of a tool execution back to the AI.
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

var (
	ErrNoStartNode = errors.New("no start node specified")
	ErrInvalidNode = errors.New("invalid node")
)

const (
	ConnectionTypeString        = "string"
	ConnectionTypeObject        = "object"
	ConnectionTypeInteger       = "integer"
	ConnectionTypeBoolean       = "boolean"
	ConnectionTypeText          = "text"
	ConnectionTypeKeyValueArray = "key_value_array"
	ConnectionTypeDateTime      = "datetime"
	ConnectionTypeMultiSelect   = "multi_select"
	// ConnectionTypeRows renders a structured rows-of-cells widget
	// in the editor: a + button adds rows, and each row carries a
	// + button to add columns. Each cell is a VariableInput so it
	// can hold literal text or a ${...} reference (or both).
	// On the wire the value remains a JSON 2D string —
	// `[["A","B"],["C","D"]]` — for compatibility with existing
	// action parsers like google/sheets/append and
	// microsoft/excel/append_rows that already json.Unmarshal the
	// input into [][]interface{}.
	ConnectionTypeRows = "rows"

	// ConnectionTypeCredential / ConnectionTypeSecret are picker-only
	// input types in the editor. They render a CredentialProperty
	// constrained to the environment's managed credentials and
	// secrets respectively — no raw text entry — so a user can't
	// accidentally paste a literal token into a saved flow.
	//
	// The semantic difference:
	//
	//   * Credential — only managed credentials (${credentials.X}).
	//     OAuth-refreshed providers like Google, Microsoft, Slack.
	//     Use for fields where the platform owns the token lifecycle.
	//
	//   * Secret — managed credentials AND environment secrets
	//     (${secrets.X}). A managed credential satisfies a secret
	//     slot because both resolve to a token at run-time; the
	//     reverse is not true. Use for bot tokens, API keys, and
	//     any long-lived literal the user pastes into the environment.
	ConnectionTypeCredential = "credential"
	ConnectionTypeSecret     = "secret"

	// ConnectionTypeCode is for free-text inputs that hold script
	// source — Python, JavaScript, anything we add later. The
	// editor renders this with a monospace font and (in future)
	// can layer syntax highlighting / line numbers on top, none
	// of which would make sense for the generic Text type.
	ConnectionTypeCode = "code"

	// ConnectionTypeMoney is a decimal monetary amount entered in MAJOR
	// units (e.g. £12.34), rendered by the editor as a currency-prefixed
	// money field. The stored/substituted value is a plain decimal string;
	// actions convert it to the currency's smallest unit (pence, cents…)
	// at execution time via a currency-aware helper. Kept string-typed so
	// ${...} substitution flows through untouched.
	ConnectionTypeMoney = "money"

	// ConnectionTypeFieldSourceMap maps a set of named fields to the part of an
	// HTTP request each is sourced from (path/query/header/body). The editor
	// renders it as rows of [field name] + [source dropdown]; the stored value is
	// a JSON OBJECT string ({"id":"path","limit":"query"}). Used by the Web
	// Trigger's request-field mapping. The dropdown choices come from the input's
	// Options; the value is an object (not the key_value_array's array form).
	ConnectionTypeFieldSourceMap = "field_source_map"

	// ConnectionTypeComboBox is a string field that carries Options as
	// *suggestions* rather than a closed set. The editor renders it as a
	// text input with a dropdown of the suggested values: the operator can
	// pick one or type their own. Use it where a helpful shortlist exists but
	// the field is genuinely open-ended — the embedding Model, for instance,
	// where the common models are worth surfacing but any model name is valid.
	// On the wire it behaves exactly like a string, so ${...} substitution and
	// every existing string reader flow through untouched.
	ConnectionTypeComboBox = "combobox"

	// ConnectionTypeFile marks an input that holds a file REFERENCE (a
	// flo:blob: token, or flo:file:/base64), not a value the operator types.
	// The editor renders it as an upload widget: the file is uploaded to the
	// blob store (POST /api/v1/asset) and the returned flo:blob: token is
	// stored as the input value. On the wire it is a plain string token, so
	// ${...} substitution and every file-consuming action (which already
	// resolves it via ResolveToLocalFile) flow through untouched — no action
	// or engine change is needed to accept an uploaded asset.
	ConnectionTypeFile = "file"

	// ConnectionTypeColour is a colour value. The editor renders it as a swatch
	// that opens a colour-wheel + hex/RGB picker; the stored value is a hex
	// string (e.g. "#00aa9c"). It stays string-typed on the wire so ${...}
	// substitution and named colours (white, flomation-teal, …) flow through
	// untouched — actions read it exactly like a string and parse the colour
	// themselves (see graphics_common.parseColour / image colour parsing).
	ConnectionTypeColour = "colour"
)

type Action func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error)

// substitutionString renders an arbitrary value for inline ${...} substitution.
// Strings interpolate bare (no surrounding quotes); slices/structs/maps are
// JSON-encoded so downstream actions can re-parse them. Numbers and booleans
// render identically under either form. This replaces an earlier fmt.Sprintf
// "%v" path that produced Go syntax for complex types (e.g. "[{Head north
// 500 60}]") — unparseable JSON and not useful as a string either.
func substitutionString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

// jsonEscapeInner escapes a string for safe insertion INSIDE a JSON string
// literal — i.e. it returns what would sit between the quotes, with the
// quotes stripped. Double quotes, backslashes, newlines, tabs and control
// characters are all escaped, so a value spliced into a `"${x}"` position
// in a hand-authored JSON template can never break the surrounding JSON.
// HTML escaping is disabled so &, < and > pass through as themselves,
// keeping the on-the-wire value identical to what the user entered.
func jsonEscapeInner(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return s
	}
	// Encoder emits the quoted string plus a trailing newline.
	out := strings.TrimRight(buf.String(), "\n")
	if len(out) >= 2 && out[0] == '"' && out[len(out)-1] == '"' {
		return out[1 : len(out)-1]
	}
	return s
}

// replaceToken substitutes every ${token} occurrence in *val with
// replacement. When jsonCtx is true (the input is a JSON container such as
// a "rows" 2D array), the replacement is JSON-escaped first so variable
// values containing quotes, backslashes, newlines or entire JSON blobs
// can't break the surrounding JSON that downstream actions parse. For
// non-JSON inputs the escape is skipped, preserving verbatim substitution.
func replaceToken(val *string, token string, jsonCtx bool, replacement string) {
	if jsonCtx {
		replacement = jsonEscapeInner(replacement)
	}
	*val = strings.ReplaceAll(*val, "${"+token+"}", replacement)
}

type Edge struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle,omitempty"`
	TargetHandle string `json:"targetHandle,omitempty"`
}

type ConnectionOption struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Group optionally places this option under a labelled, collapsible section
	// in a multi-select input (e.g. grouping webhook events by resource, like
	// Jira's own webhook page). Options sharing a Group render together; an empty
	// Group renders ungrouped, so this is backward-compatible.
	Group string `json:"group,omitempty"`
}

// VisibleWhen controls conditional visibility of an input based on
// the value of another input. The input is only shown (and validated)
// when the referenced field has one of the specified values.
type VisibleWhen struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

type Connection struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	Value       interface{}        `json:"value"`
	Label       string             `json:"label"`
	Placeholder string             `json:"placeholder"`
	Required    bool               `json:"required,omitempty"`
	Options     []ConnectionOption `json:"options,omitempty"`
	Visible     *VisibleWhen       `json:"visible_when,omitempty"`

	// FromCredentialMeta names a key on the connected credential's metadata that
	// fills this input when the operator leaves it blank.
	//
	// It exists because several providers capture a value at OAuth time that the
	// operator then has to retype by hand: Salesforce's instance_url, QuickBooks'
	// realm_id, Xero's tenant_id. Retyping is not merely tedious, it is a trap —
	// the Salesforce field's placeholder invites pasting the URL from the browser
	// address bar, which is the LIGHTNING host, not the API one, and fails in a
	// way that reads as a broken integration.
	//
	// When set, and this input is blank, and a sibling input on the same node
	// holds a ${credentials.NAME} reference, the value becomes
	// ${credentials.NAME.<FromCredentialMeta>} — which the ordinary substitution
	// machinery then resolves. Nothing new fetches anything; this only writes the
	// reference the operator would otherwise have had to know to write.
	//
	// An input carrying this must NOT be Required: the whole point is that it can
	// be left empty. It stays visible and typeable for the bring-your-own-token
	// path, where there is no credential to read from.
	FromCredentialMeta string `json:"from_credential_meta,omitempty"`
}

func (c *Connection) String() *string {
	if c == nil {
		return nil
	}

	if c.Type == ConnectionTypeString || c.Type == ConnectionTypeText ||
		c.Type == ConnectionTypeDateTime || c.Type == ConnectionTypeMultiSelect ||
		c.Type == ConnectionTypeRows || c.Type == ConnectionTypeMoney ||
		c.Type == ConnectionTypeComboBox || c.Type == ConnectionTypeFile ||
		c.Type == ConnectionTypeColour {
		if v, ok := c.Value.(string); ok {
			return &v
		}
		// Non-string value landed in a string/text-typed input — this happens
		// when a whole-value ${parent.X} reference targets an upstream output
		// that is a slice/map/struct (e.g. ${parent.steps} → []RouteStep).
		// JSON-encode so downstream actions can json.Unmarshal the result.
		// Returning nil here would mask the wire-up — the action sees an
		// empty input and silently skips its job.
		if c.Value == nil {
			empty := ""
			return &empty
		}
		if b, err := json.Marshal(c.Value); err == nil {
			s := string(b)
			return &s
		}
		return nil
	} else if c.Type == ConnectionTypeBoolean {
		_, err := strconv.ParseBool(fmt.Sprintf("%v", c.Value))
		if err != nil {
			return nil
		}
	} else if c.Type == ConnectionTypeInteger {
		_, err := strconv.ParseInt(fmt.Sprintf("%v", c.Value), 10, 64)
		if err != nil {
			return nil
		}
	}

	v := fmt.Sprintf("%v", c.Value)
	return &v
}

// Number reads an integer-typed input.
//
// The value's dynamic type depends on how it got here: a literal the editor
// stored is a float64 after JSON decoding, an auto-wired parent output may be an
// int or int64, and a field carrying a ${...} reference has been rewritten to a
// string by the substitution pass.
//
// The previous implementation ended its type-assertion chain with an unchecked
// c.Value.(string), which panicked — taking the whole flow run with it — on any
// integer input whose value was nil (an input present but never filled in) or a
// bool. The type switch below returns nil for those instead, which is what every
// caller already treats as "unset".
func (c *Connection) Number() *int64 {
	if c == nil {
		return nil
	}

	if c.Type != ConnectionTypeInteger {
		return nil
	}

	switch v := c.Value.(type) {
	case int64:
		return &v
	case int:
		val := int64(v)
		return &val
	case float64:
		val := int64(v)
		return &val
	case string:
		val, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return nil
		}
		return &val
	default:
		return nil
	}
}

// Boolean reads a boolean-typed input.
//
// A checkbox the user ticked arrives as a Go bool. A checkbox bound to a
// variable does not: the editor's BooleanProperty stores the reference as a
// string ("${var.approved}"), and ExecuteNode's substitution pass rewrites every
// ${...} into its resolved text before an action sees it — so the value lands
// here as the string "true". Asserting c.Value.(bool) would read that as unset
// and silently ignore the variable the flow author bound.
//
// So a string is parsed, the way String() already parses a boolean-typed value
// with strconv.ParseBool and Number() already falls back to parsing a string.
// The bool fast path stays exact.
//
// Anything unparseable — including the empty string an unresolved ${var.missing}
// substitutes to — returns nil, i.e. "unset". Callers that gate a destructive
// action on this therefore fail closed on a typo'd variable name.
func (c *Connection) Boolean() *bool {
	if c == nil {
		return nil
	}

	if c.Type != ConnectionTypeBoolean {
		return nil
	}

	if v, ok := c.Value.(bool); ok {
		return &v
	}
	if c.Value == nil {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", c.Value))) {
	case "yes", "on":
		v := true
		return &v
	case "no", "off":
		v := false
		return &v
	}

	v, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprintf("%v", c.Value)))
	if err != nil {
		return nil
	}
	return &v
}

// KeyValuePair represents a single key-value pair from a key_value_array input.
type KeyValuePair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// KeyValuePairs parses a key_value_array connection value into a slice of KeyValuePair.
func (c *Connection) KeyValuePairs() []KeyValuePair {
	if c == nil || c.Value == nil {
		return nil
	}

	// Value is stored as a JSON string or already-parsed array
	switch v := c.Value.(type) {
	case string:
		var pairs []KeyValuePair
		if err := json.Unmarshal([]byte(v), &pairs); err != nil {
			return nil
		}
		return pairs
	case []interface{}:
		var pairs []KeyValuePair
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				pairs = append(pairs, KeyValuePair{
					Key:   fmt.Sprintf("%v", m["key"]),
					Value: fmt.Sprintf("%v", m["value"]),
				})
			}
		}
		return pairs
	default:
		return nil
	}
}

func FindConnection(name string, connections []*Connection) *Connection {
	for _, c := range connections {
		if c.Name == name {
			return c
		}
	}

	return nil
}

type NodeConfig struct {
	ID      string        `json:"id"`
	Name    *string       `json:"name"`
	Type    int64         `json:"type"`
	Plugin  string        `json:"plugin"`
	Inputs  []*Connection `json:"inputs"`
	Outputs []*Connection `json:"outputs"`
}

type NodeResult struct {
	Inputs  []*Connection `json:"inputs"`
	Outputs []*Connection `json:"outputs"`
}

type NodeData struct {
	ID      string     `json:"id"`
	Label   string     `json:"label"`
	Config  NodeConfig `json:"config"`
	Results NodeResult `json:"results"`
}

type Node struct {
	ID   string    `json:"id"`
	Type string    `json:"type"`
	Data *NodeData `json:"data"`
}

// ExecutionIdentity is a single declared channel identity for the
// executing user, snapshot at execution start. Returned as part of
// ${flow.identities} and surfaced to AI nodes via the system prompt.
type ExecutionIdentity struct {
	ChannelType string `json:"channel_type"`
	ExternalID  string `json:"external_id"`
	DisplayName string `json:"display_name,omitempty"`
}

// ExecutionContext holds read-only metadata about the current execution,
// exposed to actions via ${flow.xxx} variable substitution.
type ExecutionContext struct {
	FlowID         string `json:"flow_id"`
	ExecutionID    string `json:"execution_id"`
	Sequence       int64  `json:"sequence"`
	AuthorID       string `json:"author_id"`
	OrganisationID string `json:"organisation_id"`
	RunnerID       string `json:"runner_id"`
	StartTime      string `json:"start_time"`
	TriggerType    string `json:"trigger_type"`
	AuthorEmail    string `json:"author_email"`
	TriggererEmail string `json:"triggerer_email"`
	APIURL         string `json:"api_url,omitempty"`
	Token          string `json:"token,omitempty"`
	SystemPrompt   string `json:"system_prompt,omitempty"`
	// AgentID is set when this execution is running as part of an agent
	// orchestrator flow. When present, AI actions use it to automatically
	// record their response as a direction=outbound agent_message so the
	// next turn's conversation_history includes assistant replies. This is
	// what prevents the conversation-loop bug where the model sees only
	// consecutive user turns and tries to answer them all at once.
	AgentID string `json:"agent_id,omitempty"`
	// AgentUserID is the canonical AgentUser this turn belongs to, as
	// resolved by Launch from the inbound message's stable external id
	// (e.g. Slack U-id, Telegram numeric sender id). Phase 2 semantic
	// memory retrieval uses this to scope memory lookups per-user, so
	// preferences persist across channels and conversations. Empty when
	// the execution is not running in an agent context.
	AgentUserID string `json:"agent_user_id,omitempty"`
	// PlanTaskID identifies the plan_task this execution is progressing
	// when the orchestrator was dispatched via the Plan Task Trigger
	// (Agent Planning M1.5). Exposed as ${flow.plan_task_id} so the
	// plan/block AI tool can transition the correct task without the
	// model having to track the UUID across turns. Empty for any
	// non-plan-task execution.
	PlanTaskID string `json:"plan_task_id,omitempty"`
	// UserID is the platform user_id resolved by the API's inbound
	// pipeline — either a declared owner via user_identity, or an
	// anonymous stub user keyed per-(org, channel, external_id). It
	// exposes the executing user to flows via ${flow.user_id} so
	// orchestrator logic can reason about who triggered the run.
	UserID string `json:"user_id,omitempty"`
	// UserVariables is the executing user's extended profile snapshot,
	// loaded at execution bootstrap from the API's
	// /internal/user/:id/variables endpoint. Exposed to flows as
	// ${user.X} (e.g. ${user.first_name}, ${user.full_address}).
	// Keys: id, name, email, salutation, first_name, last_name,
	// job_title, address_line_1, address_line_2, city, region,
	// postcode, country, full_name, full_address.
	UserVariables map[string]string `json:"user_variables,omitempty"`
	// Identities is the executing user's declared channel-identity set
	// within OrganisationID, snapshot at execution start. Exposed to
	// flows as ${flow.identities} (JSON-encoded). AI nodes also see
	// these in the system prompt so they can address users by their
	// other channel handles without an explicit fetch action.
	Identities []ExecutionIdentity `json:"identities,omitempty"`
	// ConversationID is the current open conversation scoped to
	// (agent, user, channel, thread). AI actions auto-record their
	// outbound turn into this conversation so the sequence column
	// stays contiguous and future conversation_history fetches see
	// assistant replies interleaved with user turns.
	ConversationID string `json:"conversation_id,omitempty"`
	// TriggerSource distinguishes reactive ("channel") from proactive
	// ("commitment") executions. Used to skip extraction on commitment-
	// triggered turns to prevent duplicate commitment creation.
	TriggerSource string `json:"trigger_source,omitempty"`
	// ChannelType identifies the source channel: "slack", "telegram",
	// "webhook", "commitment". Available as ${flow.channel_type} for
	// Switch/conditional branching regardless of node parentage.
	ChannelType string `json:"channel_type,omitempty"`
	// ChannelID is the target channel/chat for message delivery.
	// Slack channel ID, Telegram chat ID, etc. Available as
	// ${flow.channel_id} so send actions work regardless of node position.
	ChannelID string `json:"channel_id,omitempty"`
	// ThreadID is the thread/reply identifier (Slack thread_ts).
	ThreadID string `json:"thread_id,omitempty"`
	// PageAccessToken is the Facebook Page access token for Messenger
	// replies. Available as ${flow.page_access_token} so send actions
	// work regardless of node position in the flow.
	PageAccessToken string `json:"page_access_token,omitempty"`
	// Role is "user" or "assistant" for extraction flows. Used by
	// process_extraction to gate commitment writes to assistant turns only.
	Role string `json:"role,omitempty"`
	// SystemFlow is true when this execution is running a system flow
	// (e.g. extraction). System flow executions must not cascade into
	// further extraction to prevent infinite loops.
	SystemFlow bool `json:"system_flow,omitempty"`
	// TLS fields for mTLS client configuration. When set, the executor
	// builds a dedicated HTTP client for internal API calls.
	TLSCACert string `json:"tls_ca_cert,omitempty"`
	TLSCert   string `json:"tls_cert,omitempty"`
	TLSKey    string `json:"tls_key,omitempty"`
	// APIClient is the mTLS-configured HTTP client for internal API calls.
	// Nil when mTLS is not configured — actions should use InternalClient().
	APIClient *http.Client `json:"-"`
}

// InternalClient returns the mTLS client for internal API calls if
// configured, otherwise falls back to http.DefaultClient.
func (ctx *ExecutionContext) InternalClient() *http.Client {
	if ctx != nil && ctx.APIClient != nil {
		return ctx.APIClient
	}
	return http.DefaultClient
}

// GetContext returns the full execution context.
func (f *Flow) GetContext() *ExecutionContext {
	return f.context
}

// Blobs returns the per-execution BlobStore, lazily creating it the
// first time a caller needs to off-load a large tool output. After
// M1 (file_attachments plan) the store is API-backed: tokens are
// generated and resolved against the API's blob_object endpoints
// over the mTLS InternalClient. The store inherits the execution's
// organisation_id and execution_id so server-side rows are scoped
// correctly without any per-call wiring.
func (f *Flow) Blobs() *BlobStore {
	f.blobsMu.Lock()
	defer f.blobsMu.Unlock()
	if f.blobs == nil {
		ctx := f.context
		var (
			client      = http.DefaultClient
			apiURL      string
			orgID       string
			ownerID     string
			executionID string
		)
		if ctx != nil {
			client = ctx.InternalClient()
			apiURL = ctx.APIURL
			executionID = ctx.ExecutionID
			// Scope falls back from organisation to flow author so
			// personal-mode flows can still off-load large tool
			// outputs. Empty-string discriminator picks the right
			// header inside the BlobStore.
			if ctx.OrganisationID != "" {
				orgID = ctx.OrganisationID
			} else {
				ownerID = ctx.AuthorID
			}
		}
		f.blobs = NewBlobStore(client, apiURL, orgID, ownerID, executionID)
	}
	return f.blobs
}

// Get returns the value of a named execution context field.
func (ctx *ExecutionContext) Get(name string) string {
	switch name {
	case "flow_id":
		return ctx.FlowID
	case "execution_id":
		return ctx.ExecutionID
	case "sequence":
		return fmt.Sprintf("%d", ctx.Sequence)
	case "author_id":
		return ctx.AuthorID
	case "organisation_id":
		return ctx.OrganisationID
	case "runner_id":
		return ctx.RunnerID
	case "start_time":
		return ctx.StartTime
	case "trigger_type":
		return ctx.TriggerType
	case "author_email":
		return ctx.AuthorEmail
	case "triggerer_email":
		return ctx.TriggererEmail
	case "system_prompt":
		return ctx.SystemPrompt
	case "agent_id":
		return ctx.AgentID
	case "agent_user_id":
		return ctx.AgentUserID
	case "plan_task_id":
		return ctx.PlanTaskID
	case "user_id":
		return ctx.UserID
	case "identities":
		// JSON-encode the slice so downstream actions can either parse it
		// or splice it into prompts as-is. Empty slice serialises to "[]"
		// which is safe in both flows.
		if len(ctx.Identities) == 0 {
			return "[]"
		}
		b, err := json.Marshal(ctx.Identities)
		if err != nil {
			return "[]"
		}
		return string(b)
	case "conversation_id":
		return ctx.ConversationID
	case "trigger_source":
		return ctx.TriggerSource
	case "channel_type":
		return ctx.ChannelType
	case "channel_id":
		return ctx.ChannelID
	case "thread_id":
		return ctx.ThreadID
	case "page_access_token":
		return ctx.PageAccessToken
	case "role":
		return ctx.Role
	default:
		return ""
	}
}

type Flow struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`

	nodeResults          map[string]map[string]interface{}
	nodeExecutionResults map[string]*ExecutionNodeResult
	outputs              map[string]interface{}
	variables            map[string]interface{}
	entryNodeID          string
	reachableNodes       map[string]bool
	inErrorChain         bool
	hadError             bool
	suspended            bool
	suspendInfo          *SuspendInfo
	resumed              bool                   // true when restored from a checkpoint
	resumedSuspendNodeID string                 // the node that caused the original suspend
	resumeData           map[string]interface{} // data injected on resume (e.g. a human's choice)
	traversedNodes       map[string]bool        // tracks which nodes have had children traversed
	executing            map[string]bool        // nodes whose action is mid-execution — re-entrancy guard for diamonds
	context              *ExecutionContext
	ctx                  gocontext.Context
	cancel               gocontext.CancelFunc

	// actions and env are stashed at Execute time so an action can invoke
	// another node in-process (used by the Human-in-the-Loop node to fan a
	// request out over its referenced Send Message nodes). The Action
	// signature does not carry these, so they live on the Flow instead.
	actions map[string]Action
	env     *environment.Environment

	// blobs is the per-execution off-loading store for large tool
	// outputs. Lazily initialised by Blobs() so flows that never
	// touch the AI tool loop pay no filesystem cost.
	blobs   *BlobStore
	blobsMu sync.Mutex
}

// ErrCancelled is returned when a flow execution is cancelled.
var ErrCancelled = errors.New("execution cancelled")

// ErrSuspended is returned when a flow execution is suspended (paused).
// The executor should serialise a checkpoint and exit.
var ErrSuspended = errors.New("execution suspended")

// SuspendInfo holds metadata about why the execution was suspended
// and how it should be resumed.
type SuspendInfo struct {
	NodeID             string                 `json:"suspend_node_id"`
	Reason             string                 `json:"suspend_reason"`
	ResumeAt           *time.Time             `json:"resume_at,omitempty"`
	ResumeTriggerType  string                 `json:"resume_trigger_type,omitempty"`
	ResumeTriggerMatch map[string]interface{} `json:"resume_trigger_match,omitempty"`
}

// Checkpoint holds all serialisable runtime state needed to resume
// a suspended execution on a different runner.
type Checkpoint struct {
	NodeResults          map[string]map[string]interface{} `json:"node_results"`
	Variables            map[string]interface{}            `json:"variables,omitempty"`
	Outputs              map[string]interface{}            `json:"outputs,omitempty"`
	NodeExecutionResults map[string]*ExecutionNodeResult   `json:"node_execution_results,omitempty"`
	EntryNodeID          string                            `json:"entry_node_id"`
	ReachableNodes       map[string]bool                   `json:"reachable_nodes,omitempty"`
	InErrorChain         bool                              `json:"in_error_chain"`
	HadError             bool                              `json:"had_error"`
	SuspendInfo          *SuspendInfo                      `json:"suspend_info,omitempty"`
	// ResumeData carries values injected at resume time (e.g. the option a
	// human chose). The API patches this field onto the stored checkpoint
	// JSONB before re-queuing, so it reaches the executor untouched by the
	// runner. Restored into Flow.resumeData and read via GetResumeData().
	ResumeData map[string]interface{} `json:"resume_data,omitempty"`
}

// Suspend marks the execution as suspended with the given metadata.
// Called by suspend/pause/wait actions.
func (f *Flow) Suspend(info *SuspendInfo) {
	f.suspended = true
	f.suspendInfo = info
}

// IsSuspended returns true if the execution has been suspended.
func (f *Flow) IsSuspended() bool { return f.suspended }

// GetSuspendInfo returns the suspend metadata.
func (f *Flow) GetSuspendInfo() *SuspendInfo { return f.suspendInfo }

// CreateCheckpoint serialises the current runtime state for later resume.
func (f *Flow) CreateCheckpoint() *Checkpoint {
	return &Checkpoint{
		NodeResults:          f.nodeResults,
		Variables:            filterSerialisableVariables(f.variables),
		Outputs:              f.outputs,
		NodeExecutionResults: f.nodeExecutionResults,
		EntryNodeID:          f.entryNodeID,
		ReachableNodes:       f.reachableNodes,
		InErrorChain:         f.inErrorChain,
		HadError:             f.hadError,
		SuspendInfo:          f.suspendInfo,
		ResumeData:           f.resumeData,
	}
}

// RestoreCheckpoint restores runtime state from a serialised checkpoint.
func (f *Flow) RestoreCheckpoint(cp *Checkpoint) {
	if cp.NodeResults != nil {
		f.nodeResults = cp.NodeResults
	}
	if cp.Variables != nil {
		f.variables = cp.Variables
	}
	if cp.Outputs != nil {
		f.outputs = cp.Outputs
	}
	if cp.NodeExecutionResults != nil {
		f.nodeExecutionResults = cp.NodeExecutionResults
	}
	f.entryNodeID = cp.EntryNodeID
	if cp.ReachableNodes != nil {
		f.reachableNodes = cp.ReachableNodes
	}
	f.inErrorChain = cp.InErrorChain
	f.hadError = cp.HadError
	if cp.ResumeData != nil {
		f.resumeData = cp.ResumeData
	}
	// Clear suspend state — we are resuming
	f.suspended = false
	f.suspendInfo = nil
	f.resumed = true

	// Remove the suspend node from cache so it re-executes on resume.
	// Its children were never traversed (ErrSuspended interrupted), so
	// ExecuteNode must run again for that node to reach Phase 2.
	if cp.SuspendInfo != nil && cp.SuspendInfo.NodeID != "" {
		f.resumedSuspendNodeID = cp.SuspendInfo.NodeID
		delete(f.nodeResults, cp.SuspendInfo.NodeID)
	}

	// Diagnostic: one structured log line capturing what was actually
	// restored. Lets operators ground-truth resume behaviour from runner
	// logs — the alternative is silent failure where the checkpoint is
	// empty and the flow re-executes from scratch.
	resumeNodeID := ""
	resumeReason := ""
	if cp.SuspendInfo != nil {
		resumeNodeID = cp.SuspendInfo.NodeID
		resumeReason = cp.SuspendInfo.Reason
	}
	log.WithFields(log.Fields{
		"resume_node_id":     resumeNodeID,
		"resume_reason":      resumeReason,
		"cached_node_count":  len(f.nodeResults),
		"reachable_count":    len(f.reachableNodes),
		"node_results_count": len(f.nodeExecutionResults),
		"in_error_chain":     f.inErrorChain,
		"had_error":          f.hadError,
	}).Info("checkpoint restore complete")
}

// IsResumed returns true if this execution was restored from a checkpoint.
func (f *Flow) IsResumed() bool { return f.resumed }

// IsResumedNode returns true if this node was the one that caused the suspend
// and is now being re-executed after resume.
func (f *Flow) IsResumedNode(nodeID string) bool {
	return f.resumed && f.resumedSuspendNodeID == nodeID
}

// GetResumeData returns the data injected when the execution was resumed
// (nil on a fresh run). The Human-in-the-Loop node reads the chosen option
// and outcome from here on its resume pass.
func (f *Flow) GetResumeData() map[string]interface{} { return f.resumeData }

// SetResumeData overrides the injected resume data. Primarily for tests.
func (f *Flow) SetResumeData(m map[string]interface{}) { f.resumeData = m }

// InvokeNode runs another node's action in-process and returns its outputs.
// It is used by the Human-in-the-Loop node to deliver a request through its
// referenced Send Message nodes: the caller passes rendered values (blocks,
// reply_markup, message text) in extra, which are injected into the target
// node's matching inputs before delivery and restored afterwards so the
// node's saved configuration is left untouched.
//
// The target node is resolved and executed through executeNodeActionOnly so
// it goes through the same variable substitution (secrets, ${flow.X}, …),
// event emission, and result recording as any other node — the delivery
// therefore appears in the execution inspector like a normal send.
func (f *Flow) InvokeNode(node *Node, extra map[string]interface{}) (map[string]interface{}, error) {
	if node == nil || node.Data == nil {
		return nil, ErrInvalidNode
	}

	// Snapshot the target inputs we are about to override, then restore them
	// on return — mirrors the tool-input reset used by the AI tool loop so a
	// second delivery (or a checkpoint) never sees injected values.
	type saved struct {
		conn      *Connection
		val       interface{}
		transient bool // true if we appended this input and must strip it
	}
	var snapshot []saved
	setInput := func(name string, value interface{}) {
		for _, c := range node.Data.Config.Inputs {
			if c != nil && c.Name == name {
				snapshot = append(snapshot, saved{conn: c, val: c.Value})
				c.Value = value
				return
			}
		}
		// Input not declared on the node — append a transient one and record
		// it so we can strip it again afterwards.
		nc := &Connection{Name: name, Type: ConnectionTypeText, Value: value}
		node.Data.Config.Inputs = append(node.Data.Config.Inputs, nc)
		snapshot = append(snapshot, saved{conn: nc, transient: true})
	}
	for k, v := range extra {
		setInput(k, v)
	}
	defer func() {
		for i := len(snapshot) - 1; i >= 0; i-- {
			s := snapshot[i]
			if s.transient {
				for idx, c := range node.Data.Config.Inputs {
					if c == s.conn {
						node.Data.Config.Inputs = append(node.Data.Config.Inputs[:idx], node.Data.Config.Inputs[idx+1:]...)
						break
					}
				}
				continue
			}
			s.conn.Value = s.val
		}
	}()

	// Ensure the delivery node re-runs even if a stale cache entry exists.
	delete(f.nodeResults, node.ID)

	// Prevent parent-resolution recursion. The target is typically wired as a
	// child of the node invoking it (e.g. the Human-in-the-Loop node delivering
	// through its Send Message children via the delivery handle). That node is
	// mid-execution and not yet cached, so executeNodeActionOnly's parent walk
	// would re-enter it. Temporarily seed any not-yet-cached parent with an
	// empty result (delivery nodes read their own config, not parent outputs),
	// then restore. Mirrors how the AI tool loop invokes tools only after the
	// AI node is cached.
	var seeded []string
	for _, p := range f.FindSource(node.ID) {
		if p == nil {
			continue
		}
		if _, cached := f.nodeResults[p.ID]; !cached {
			f.nodeResults[p.ID] = map[string]interface{}{}
			if f.traversedNodes == nil {
				f.traversedNodes = make(map[string]bool)
			}
			f.traversedNodes[p.ID] = true
			seeded = append(seeded, p.ID)
		}
	}
	defer func() {
		for _, id := range seeded {
			delete(f.nodeResults, id)
			delete(f.traversedNodes, id)
		}
	}()

	return f.executeNodeActionOnly(f.actions, node, f.env)
}

// filterSerialisableVariables removes non-JSON-serialisable values
// (channels, functions) from the variables map for checkpointing.
func filterSerialisableVariables(vars map[string]interface{}) map[string]interface{} {
	if vars == nil {
		return nil
	}
	filtered := make(map[string]interface{}, len(vars))
	for k, v := range vars {
		// Skip channels and functions
		switch v.(type) {
		case chan string, chan []byte, chan interface{}:
			continue
		default:
			filtered[k] = v
		}
	}
	return filtered
}

// GetNodeExecutionResults returns the per-node execution results map.
func (f *Flow) GetNodeExecutionResults() map[string]*ExecutionNodeResult {
	return f.nodeExecutionResults
}

// HadError returns true if the flow encountered an error during execution,
// even if it was handled by an On Error chain.
func (f *Flow) HadError() bool {
	return f.hadError
}

// SetContext attaches execution metadata for ${flow.xxx} variable resolution.
func (f *Flow) SetContext(ctx *ExecutionContext) {
	f.context = ctx
}

// SetCancelContext attaches a cancellable Go context to the flow.
// The flow checks this context between node executions and aborts if cancelled.
func (f *Flow) SetCancelContext(ctx gocontext.Context, cancel gocontext.CancelFunc) {
	f.ctx = ctx
	f.cancel = cancel
}

// Cancel cancels the flow execution. Safe to call multiple times.
func (f *Flow) Cancel() {
	if f.cancel != nil {
		f.cancel()
	}
}

// Context returns the flow's Go context, or context.Background() if none set.
func (f *Flow) GoContext() gocontext.Context {
	if f.ctx != nil {
		return f.ctx
	}
	return gocontext.Background()
}

// ExecutionNodeResult captures per-node execution metadata including
// resolved inputs, outputs, status, duration, and any error message.
type ExecutionNodeResult struct {
	ID       string                 `json:"id"`
	Action   string                 `json:"action"`
	Label    string                 `json:"label"`
	Status   string                 `json:"status"`
	Inputs   map[string]interface{} `json:"inputs,omitempty"`
	Outputs  map[string]interface{} `json:"outputs,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration int64                  `json:"duration_ms"`
}

type ExecutionResult struct {
	ID              string                          `json:"id"`
	FlowID          string                          `json:"flow_id"`
	Status          int64                           `json:"status"`
	Duration        int64                           `json:"duration"`
	BillingDuration int64                           `json:"billing_duration"`
	Outputs         map[string]interface{}          `json:"outputs"`
	NodeResults     map[string]*ExecutionNodeResult `json:"node_results,omitempty"`
	Checkpoint      *Checkpoint                     `json:"checkpoint,omitempty"`
}

func Load(path *string) (*Flow, error) {
	if path == nil {
		return nil, nil
	}

	b, err := os.ReadFile(*path)
	if err != nil {
		return nil, err
	}

	var f Flow
	if err = json.Unmarshal(b, &f); err != nil {
		return nil, err
	}

	f.nodeResults = make(map[string]map[string]interface{})
	f.nodeExecutionResults = make(map[string]*ExecutionNodeResult)
	f.outputs = make(map[string]interface{})

	return &f, nil
}

// reservedTriggerDataKeys enumerates trigger-data keys that must NOT be
// injected as inputs on a trigger node. These are values that users compose
// into text on downstream actions — e.g. an AI action's System Prompt field
// typically contains "${flow.system_prompt}\n\n<extra directives>". They
// are surfaced to the flow via the ExecutionContext (accessed as
// ${flow.xxx}) instead of via the trigger-node-echo relay path.
//
// If we injected such a key onto the trigger node, the trigger's action
// would re-emit it as an output and the auto-wire-by-name mechanism in
// executeNodeActionOnly would clobber the user-composed text with the bare
// value. The explicit-wins precedence rule in executeNodeActionOnly already
// guards against that clobber, but filtering here is defence in depth and
// keeps the trigger node's input/output surface clean.
//
// Note: keys that users reference whole (e.g. ${conversation_history} for
// an array input on an AI action) are deliberately NOT reserved. They rely
// on the trigger echoing them through as outputs so that auto-wire or the
// whole-value-reference substitution path can pick them up downstream.
var reservedTriggerDataKeys = map[string]bool{
	"system_prompt":        true,
	"__node_id":            true,
	"__triggering_user_id": true,
}

// InjectTriggerData merges trigger invocation data into the first trigger
// node's inputs, making dynamic event data available to the flow.
func (f *Flow) InjectTriggerData(data map[string]interface{}) {
	// Highest priority: match by explicit __node_id (set by trigger sync)
	nodeID, _ := data["__node_id"].(string)
	channelType, _ := data["channel_type"].(string)

	var targetNode *Node

	for _, n := range f.Nodes {
		if n == nil || n.Data == nil {
			continue
		}

		isTrigger := strings.HasPrefix(n.Type, "trigger/") ||
			(n.Data != nil && strings.HasPrefix(n.Data.Label, "trigger/"))

		if !isTrigger {
			continue
		}

		// Exact node ID match — highest priority
		if nodeID != "" && n.ID == nodeID {
			targetNode = n
			break
		}

		// Match channel type to trigger label (e.g. "slack" matches "trigger/slack")
		if channelType != "" {
			label := n.Data.Label
			if label == "trigger/"+channelType {
				targetNode = n
				break
			}
			// Sub-type fallback: telegram_voice → trigger/telegram
			if strings.Contains(channelType, "_") {
				baseType := channelType[:strings.LastIndex(channelType, "_")]
				if label == "trigger/"+baseType {
					targetNode = n
					break
				}
			}
		}

		// Record first trigger as fallback
		if targetNode == nil {
			targetNode = n
		}
	}

	if targetNode == nil {
		return
	}

	n := targetNode

	// Add each data field as an input connection on the trigger node.
	//
	// Reserved keys are agent-orchestration values that are already surfaced
	// to the flow via the ExecutionContext (e.g. ${flow.system_prompt}) or
	// are intended to be bound by downstream actions via explicit variable
	// references. We deliberately skip injecting them as trigger-node inputs
	// because trigger actions tend to re-emit all of their inputs as outputs,
	// which would auto-wire them into same-named inputs on downstream nodes
	// and silently override user-authored values (e.g. an AI action's
	// system_prompt field).
	for k, v := range data {
		if reservedTriggerDataKeys[k] {
			continue
		}
		found := false
		for _, input := range n.Data.Config.Inputs {
			if input.Name == k {
				input.Value = v
				found = true
				break
			}
		}
		if !found {
			n.Data.Config.Inputs = append(n.Data.Config.Inputs, &Connection{
				Name:  k,
				Type:  ConnectionTypeString,
				Value: v,
			})
		}
	}

	log.WithFields(log.Fields{
		"node_id": n.ID,
		"label":   n.Data.Label,
		"keys":    len(data),
	}).Info("injected trigger data into trigger node")
}

func (f *Flow) Execute(actions map[string]Action, entry *string, environment *environment.Environment) (map[string]interface{}, error) {
	// Stash the registry and environment so actions can invoke other nodes
	// in-process (see InvokeNode — used by the Human-in-the-Loop node).
	f.actions = actions
	f.env = environment

	var start *Node

	if entry != nil {
		start = f.FindNode(*entry)
		if start == nil {
			// A stale/mismatched entry node id (e.g. from a trigger record whose
			// __node_id points at a since-removed node) must not hard-fail the
			// run. Fall back to the trigger scan below rather than erroring.
			log.WithField("entry", *entry).Warn("entry node not found; falling back to trigger scan")
		}
	}

	if start == nil {
		for _, n := range f.Nodes {
			if n == nil {
				continue
			}

			if strings.HasPrefix(n.Type, "trigger/") {
				start = n
				break
			}
		}
	}

	if start == nil {
		return nil, ErrNoStartNode
	}

	f.entryNodeID = start.ID
	f.reachableNodes = f.computeReachable(start.ID)

	_, err := f.ExecuteNode(actions, start, environment)
	if errors.Is(err, ErrSuspended) {
		// Execution suspended — return current outputs with suspend error
		// so the caller can serialise the checkpoint.
		return f.outputs, ErrSuspended
	}
	if err != nil && !f.inErrorChain {
		// Look for an On Error trigger node and execute its chain
		onErrorHandled := f.executeOnErrorChain(actions, err, environment)
		if !onErrorHandled {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	// Return outputs set explicitly via SetOutput (by Output-type nodes)
	return f.outputs, nil
}

// executeOnErrorChain finds a "error/on_error" node and executes its
// downstream chain with error context. Returns true if an error handler
// was found and executed successfully.
func (f *Flow) executeOnErrorChain(actions map[string]Action, flowErr error, environment *environment.Environment) bool {
	var onErrorNode *Node
	for _, n := range f.Nodes {
		if n == nil {
			continue
		}
		if n.Type == "error/on_error" || (n.Data != nil && n.Data.Label == "error/on_error") {
			onErrorNode = n
			break
		}
	}

	if onErrorNode == nil {
		return false
	}

	// Mark that this execution encountered an error, even though it is
	// being handled. The overall execution status should remain "failed".
	f.hadError = true

	// Find the node that caused the error from execution results
	var failedNodeID, failedNodeLabel, failedNodeType string
	for _, result := range f.nodeExecutionResults {
		if result != nil && result.Status == "failed" {
			failedNodeID = result.ID
			failedNodeLabel = result.Label
			failedNodeType = result.Action
			break
		}
	}

	// Prevent re-entrancy: errors in the error chain must not trigger it again
	f.inErrorChain = true

	// Add the On Error node, its descendants, AND their parents to the
	// reachable set. Parents matter because error chain nodes may have
	// upstream dependencies (e.g. Format Date → Slack) that aren't
	// reachable from the entry trigger via forward BFS.
	if f.reachableNodes != nil {
		errorReachable := f.computeReachable(onErrorNode.ID)
		for id := range errorReachable {
			f.reachableNodes[id] = true
		}
		// Also add parents of error-chain nodes so parent resolution works.
		for id := range errorReachable {
			for _, e := range f.Edges {
				if e != nil && e.Target == id && !f.reachableNodes[e.Source] {
					f.reachableNodes[e.Source] = true
				}
			}
		}
	}

	// Inject error context as the On Error node's cached result
	errorOutputs := map[string]interface{}{
		"error_message":    flowErr.Error(),
		"error_node_id":    failedNodeID,
		"error_node_label": failedNodeLabel,
		"error_node_type":  failedNodeType,
	}
	f.nodeResults[onErrorNode.ID] = errorOutputs
	f.emitNodeEvent(onErrorNode.ID, onErrorNode.Type, onErrorNode.Data.Label, "success", 0, "")
	f.recordNodeResult(onErrorNode, "success", nil, errorOutputs, 0, "")

	// Mark the On Error node as traversed at injection time. We execute
	// its children directly via the loop below, so the "default" Phase 2
	// child traversal in ExecuteNode must not also fire. Without this,
	// when a child's parent-resolution calls ExecuteNode(onErrorNode),
	// the cached-but-not-traversed branch (flow.go ~1088) re-enters
	// executeNodeChildren(onErrorNode) and re-fires every child — for
	// the test that fails as a double-execution of log-error-1, the
	// second pass losing the error_message context.
	if f.traversedNodes == nil {
		f.traversedNodes = make(map[string]bool)
	}
	f.traversedNodes[onErrorNode.ID] = true

	// Execute children of the On Error node
	children := f.FindTarget(onErrorNode.ID)
	for _, c := range children {
		if c.Type != "" && strings.HasPrefix(c.Type, "trigger/") {
			continue
		}
		_, childErr := f.ExecuteNode(actions, c, environment)
		if childErr != nil {
			log.WithFields(log.Fields{
				"error": childErr,
			}).Error("Error in On Error handler chain")
			return false
		}
	}

	return true
}

func (f *Flow) ExecuteNode(actions map[string]Action, node *Node, environment *environment.Environment) (map[string]interface{}, error) {
	if node == nil || node.Data == nil {
		return nil, ErrInvalidNode
	}

	// If already fully processed (action + children), return cached result.
	// This prevents re-entrant child traversal when a child's parent resolution
	// calls ExecuteNode on this node again.
	//
	// On resume, nodeResults contains cached action outputs from the checkpoint
	// but the traversedNodes set is empty — so Phase 1 returns cached results
	// while Phase 2 (child traversal) still runs, walking the graph until it
	// reaches the uncached suspend node.
	if v, exists := f.nodeResults[node.ID]; exists {
		if f.traversedNodes[node.ID] {
			if f.resumed {
				log.WithFields(log.Fields{
					"node_id":   node.ID,
					"node_type": node.Type,
					"phase":     "cached-and-traversed",
				}).Info("resume cache hit — skipping node entirely")
			}
			return v, nil
		}
		if f.resumed {
			log.WithFields(log.Fields{
				"node_id":   node.ID,
				"node_type": node.Type,
				"phase":     "cached-not-traversed",
			}).Info("resume cache hit — walking children only, action skipped")
		}
		// Action cached but children not yet traversed — proceed to Phase 2.
		// EXCEPT for Loop nodes: re-entering executeNodeChildren for a loop
		// would re-run every iteration. That happens when a loop body's
		// parent-resolution call (loop body child → parent inputs → its
		// parent IS the loop node) walks back to the loop node before
		// traversedNodes has been set by the legitimate outer caller's
		// Pass-2 traversal — Pass 1 only runs the action, never marks
		// traversed. The fix: mark traversed and return cached without
		// recursing. The outer caller's Pass 2 still runs the loop once,
		// correctly, via direct executeNodeChildren.
		if f.traversedNodes == nil {
			f.traversedNodes = make(map[string]bool)
		}
		f.traversedNodes[node.ID] = true
		if node.Data != nil && node.Data.Config.Type == ActionTypeLoop {
			return v, nil
		}
		return f.executeNodeChildren(actions, node, v, environment)
	}

	// Phase 1: execute this node's own action (resolve parents, substitute vars, run action, cache)
	outputs, err := f.executeNodeActionOnly(actions, node, environment)
	if err != nil {
		return nil, err
	}

	// Mark as traversed and proceed to Phase 2
	if f.traversedNodes == nil {
		f.traversedNodes = make(map[string]bool)
	}
	f.traversedNodes[node.ID] = true

	// Phase 2: determine and execute children using breadth-first ordering
	return f.executeNodeChildren(actions, node, outputs, environment)
}

// executeNodeActionOnly resolves parent inputs, performs variable substitution,
// executes the node's action, and caches the result. It does NOT traverse
// children — that is handled separately to enable breadth-first ordering.
func (f *Flow) executeNodeActionOnly(actions map[string]Action, node *Node, environment *environment.Environment) (map[string]interface{}, error) {
	var err error

	if node == nil || node.Data == nil {
		return nil, ErrInvalidNode
	}

	// Check for cancellation or suspension before executing
	if f.ctx != nil {
		select {
		case <-f.ctx.Done():
			f.emitNodeEvent(node.ID, node.Type, node.Data.Label, "cancelled", 0, ErrCancelled.Error())
			f.recordNodeResult(node, "cancelled", nil, nil, 0, ErrCancelled.Error())
			return nil, ErrCancelled
		default:
		}
	}
	if f.suspended {
		return nil, ErrSuspended
	}

	if v, exists := f.nodeResults[node.ID]; exists {
		log.WithFields(log.Fields{
			"id":    node.ID,
			"label": node.Data.Label,
		}).Info("Node already executed, returning cached result")
		return v, nil
	}

	// Re-entrancy guard for diamonds: if this node's action is already mid-
	// execution higher up the stack, a parent/scoped pull has walked forward back
	// into it. Do NOT run it a second time — return the (not-yet-cached) empty
	// state; the outer frame will finish it. Without this, resolving a parent
	// that also has this node as a child re-runs the node (double-run).
	if f.executing == nil {
		f.executing = make(map[string]bool)
	}
	if f.executing[node.ID] {
		return nil, nil
	}
	f.executing[node.ID] = true
	defer delete(f.executing, node.ID)

	log.WithFields(log.Fields{
		"id":     node.ID,
		"action": node.Type,
		"label":  node.Data.Label,
	}).Info("executing node")

	var results map[string]interface{}
	parentResults := make(map[string]interface{})
	parents := f.FindSource(node.ID)
	for _, p := range parents {
		if p == nil {
			continue
		}

		// Skip non-entry trigger parents — only the entry trigger should be executed
		isTriggerParent := strings.HasPrefix(p.Type, "trigger/") ||
			(p.Data != nil && strings.HasPrefix(p.Data.Label, "trigger/"))
		if isTriggerParent && p.ID != f.entryNodeID {
			continue
		}

		// Skip parents that are not reachable from the entry trigger.
		// This prevents executing nodes on unrelated trigger paths
		// (e.g. a Switch node only reachable from a different trigger).
		if f.reachableNodes != nil && !f.reachableNodes[p.ID] {
			log.WithFields(log.Fields{
				"node":   node.ID,
				"parent": p.ID,
				"action": p.Data.Label,
			}).Debug("skipping parent not reachable from entry trigger")
			continue
		}

		// Skip parents on unmatched Switch/Conditional branches.
		// If a parent was reached via a Switch edge whose case wasn't
		// matched at runtime, it should not be executed.
		if f.isOnUnmatchedBranch(actions, p.ID, environment) {
			log.WithFields(log.Fields{
				"node":   node.ID,
				"parent": p.ID,
				"action": p.Data.Label,
			}).Debug("skipping parent on unmatched conditional branch")
			continue
		}

		// Resolve the parent via ExecuteNode (action + its children). Traversing
		// children matters for a node reached ONLY as a parent — e.g. a rootless
		// subgraph pulled in as an input provider: its matched-branch children
		// (like an Authorize-Ingress after a Create-Security-Group) must still
		// run. The in-progress guard in executeNodeActionOnly prevents the one
		// dangerous case — a child that is the very node we're resolving for
		// (diamond) — from being re-run.
		results, err = f.ExecuteNode(actions, p, environment)
		if err != nil {
			return nil, err
		}

		for k, v := range results {
			// Non-empty values take precedence: if we already have a non-empty
			// value for this key, don't overwrite it with an empty one.
			// This prevents a switch pass-through of content="" from clobbering
			// a data_rename's content="actual text" when both are parents.
			if existing, has := parentResults[k]; has {
				if isEmpty(v) && !isEmpty(existing) {
					continue
				}
			}
			parentResults[k] = v

			// Also store scoped version keyed by node ID: ${nodeId.key}
			// This allows disambiguating when multiple parents share
			// the same output key. The editor shows the node name in
			// the autocomplete but inserts the ID-based reference.
			parentResults[p.ID+"."+k] = v
		}
	}

	// Log parent result keys for debugging multi-parent resolution
	if len(parents) > 1 {
		var flatKeys []string
		for k := range parentResults {
			if !strings.Contains(k, ".") {
				flatKeys = append(flatKeys, k)
			}
		}
		log.WithFields(log.Fields{
			"node_id":   node.ID,
			"node_type": node.Data.Label,
			"parents":   len(parents),
			"flat_keys": flatKeys,
		}).Info("multi-parent result merge")
	}

	action, exists := actions[node.Type]
	if !exists {
		// Fall back to Data.Label for action lookup (ReactFlow stores component type in node.Type)
		action, exists = actions[node.Data.Label]
	}
	if !exists {
		log.WithFields(log.Fields{
			"type":  node.Type,
			"label": node.Data.Label,
		}).Debug("Unknown node action")
		return nil, ErrInvalidNode
	}

	// Auto-generate tool definitions for AI nodes from their Tools handle
	// children. Must happen before building the configuration slice so
	// the generated tool_definitions input is included.
	if node.Data != nil && strings.HasPrefix(node.Data.Label, "ai/") {
		toolsChildren := f.FindTargetByHandle(node.ID, ToolsHandle)
		if len(toolsChildren) > 0 {
			f.injectToolDefinitions(node, toolsChildren, actions)
		}
	}

	// Fill any blank input that declares FromCredentialMeta from the credential
	// bound elsewhere on this node, BEFORE the loop below reads the values —
	// substitution then resolves the reference like any other.
	if node.Data != nil {
		actionID := node.Type
		if _, known := getCredentialMetaLinks()[actionID]; !known {
			actionID = node.Data.Label
		}
		autofillFromCredential(actionID, node.Data.Config.Inputs)
	}

	var configuration []*Connection
	for _, v := range node.Data.Config.Inputs {
		value := v.Value

		// Auto-wire parent outputs by matching input name, but only when this
		// input has NOT been explicitly set on the node. Explicit values
		// (non-nil and, for strings, non-empty) always take precedence over
		// auto-wired parent outputs — otherwise a parent that happens to emit
		// an output with the same name as a downstream input would silently
		// clobber the user-authored value. A blank/unset input still falls
		// back to the matching parent output to preserve the convenience of
		// implicit wiring where the user has not supplied a value.
		hasExplicitValue := value != nil
		if s, ok := value.(string); ok && s == "" {
			hasExplicitValue = false
		}
		if !hasExplicitValue {
			if parentVal, exists := results[v.Name]; exists {
				value = parentVal
			}
		}

		// Determine whether we need to run string-based variable substitution
		// on this input. If the (possibly auto-wired) value is a string, we
		// substitute directly. For non-string types, we only stringify-and-
		// substitute when the raw value actually contains a ${...} reference
		// (e.g. "${env.X}" in an integer field) — otherwise the typed value
		// flows through untouched. Non-string pass-through is critical for
		// array/object inputs like conversation_history on AI actions.
		var val *string
		if s, ok := value.(string); ok {
			val = &s
		} else if value != nil {
			raw := fmt.Sprintf("%v", value)
			if strings.Contains(raw, "${") {
				val = &raw
			}
		}

		// Whole-value reference: when an input's entire value is a single
		// ${name} reference and the referenced value is a non-string (e.g.
		// an array of conversation history messages), preserve the raw
		// typed value instead of stringifying it. This is required for AI
		// action inputs that accept arrays/objects.
		if val != nil {
			trimmed := strings.TrimSpace(*val)
			wholeRef := regexp.MustCompile(`^\$\{([^{}]+)\}$`)
			if sub := wholeRef.FindStringSubmatch(trimmed); sub != nil {
				name := sub[1]
				// Only intercept plain parent-result references.
				if !strings.HasPrefix(name, "env.") && !strings.HasPrefix(name, "flow.") &&
					!strings.HasPrefix(name, "var.") && !strings.HasPrefix(name, "secrets.") &&
					!strings.HasPrefix(name, "secret.") && !strings.HasPrefix(name, "credentials.") {
					// Check direct parent results first
					if res, exists := parentResults[name]; exists {
						if _, isStr := res.(string); !isStr {
							configuration = append(configuration, &Connection{
								Name:  v.Name,
								Type:  v.Type,
								Value: res,
							})
							continue
						}
					}
					// Fall back to global node results for scoped references
					// (e.g. ${nodeId.key} where nodeId is an ancestor, not a
					// direct parent — separated by Switch/For loop). Also
					// supports path-bearing references like
					// ${nodeId.body.items[0]} — ParseReference splits the
					// ID, child key, and path; ResolvePath walks the
					// child value to surface the deep target as a typed
					// Go value to downstream actions that want it.
					if dotIdx := strings.IndexByte(name, '.'); dotIdx > 0 {
						var scopeNodeID, scopeKey string
						var path []string
						if ref, ok := ParseReference(name); ok {
							scopeNodeID = ref.Root
							scopeKey = ref.Child
							path = ref.Path
						} else {
							scopeNodeID = name[:dotIdx]
							scopeKey = name[dotIdx+1:]
						}
						if nr, ok := f.nodeResults[scopeNodeID]; ok {
							if res, ok := nr[scopeKey]; ok {
								target := res
								if len(path) > 0 {
									if walked, perr := ResolvePath(res, path); perr == nil {
										target = walked
									} else {
										// Path-resolve failed — fall through to
										// the string-substitution regex path so
										// the failure is logged there with the
										// canonical empty-string replacement.
										target = nil
									}
								}
								if target != nil {
									if _, isStr := target.(string); !isStr {
										configuration = append(configuration, &Connection{
											Name:  v.Name,
											Type:  v.Type,
											Value: target,
										})
										continue
									}
								}
							}
						}
					}
				}
			}
		}

		if val != nil {
			// When the input is a JSON container (a "rows" 2D array,
			// hand-authored as `[["${name}","${phone}"]]` and later
			// json.Unmarshal'd by actions like google/sheets/append),
			// substituted values must be JSON-string-escaped. Otherwise a
			// variable holding a double quote, backslash, newline or a
			// whole pasted JSON blob (common in free-text form answers)
			// breaks the surrounding JSON and the action fails to parse
			// its input. Escaping a plain value is a no-op, so ordinary
			// string inputs are unaffected.
			jsonCtx := v.Type == ConnectionTypeRows

			r := regexp.MustCompile(`\${[^{}]*}`)
			matches := r.FindAllString(*val, -1)

			for _, m := range matches {
				m = strings.TrimPrefix(m, "${")
				m = strings.TrimSuffix(m, "}")

				if strings.HasPrefix(m, "env.") {
					if environment == nil {
						log.WithFields(log.Fields{
							"name": m,
						}).Warn("No environment configured for property substitution")
						continue
					}
					name := strings.TrimPrefix(m, "env.")
					p, err := environment.GetProperty(name)
					if err != nil {
						log.WithFields(log.Fields{
							"error": err,
						}).Error("Unable to get Property")
						continue
					}

					if p == nil || p.Value == nil {
						log.WithFields(log.Fields{
							"name": name,
						}).Warn("Missing property")
						continue
					}

					replaceToken(val, m, jsonCtx, *p.Value)
				} else if strings.HasPrefix(m, "flow.") {
					name := strings.TrimPrefix(m, "flow.")
					if f.context != nil {
						contextVal := f.context.Get(name)
						// Always substitute — empty string is a valid
						// resolved value (e.g. thread_id when there's no
						// thread). Leaving the literal ${flow.xxx} in place
						// causes downstream actions to receive it as text.
						replaceToken(val, m, jsonCtx, contextVal)
					} else {
						log.WithFields(log.Fields{
							"name": name,
						}).Warn("no execution context for flow variable substitution")
					}
				} else if strings.HasPrefix(m, "user.") {
					// ${user.X} — extended-profile namespace. Populated at
					// execution-context bootstrap from the API's internal
					// user-variables endpoint. Empty/missing fields resolve
					// to "" rather than leaving the literal placeholder,
					// matching the ${flow.X} semantic.
					name := strings.TrimPrefix(m, "user.")
					var userVal string
					if f.context != nil && f.context.UserVariables != nil {
						userVal = f.context.UserVariables[name]
					}
					replaceToken(val, m, jsonCtx, userVal)
				} else if strings.HasPrefix(m, "var.") {
					// ${var.X} — Set-Variable-node-backed flow-scoped
					// variables. The stored value can be any interface{}
					// (the Set Variable action accepts JSON / typed
					// outputs), so we route through ParseReference +
					// ResolvePath to support ${var.X.field[0].subfield}
					// drilling into structured variable values.
					//
					// An UNRESOLVED reference — unknown variable, nil
					// variables map, or a failed path walk — resolves to
					// the EMPTY STRING, never the literal ${var.X}. Leaving
					// the literal in place silently corrupts downstream
					// inputs (e.g. a While loop that compares
					// "${var.started}" to "false" and so never matches).
					// Mirrors ${user.X}, which also defaults to empty when
					// absent.
					name := strings.TrimPrefix(m, "var.")
					resolved := ""
					if ref, ok := ParseReference(m); ok && len(ref.Path) > 0 {
						// Path-bearing reference. Root="var",
						// Child=first segment of variable name,
						// Path=rest. The variables map is keyed by
						// the original short name though, so we
						// look up the unprefixed child first.
						if f.variables != nil {
							if varVal, ok := f.variables[ref.Child]; ok {
								if walked, err := ResolvePath(varVal, ref.Path); err != nil {
									log.WithFields(log.Fields{
										"name":  m,
										"error": err,
									}).Warn("path resolution failed on ${var.X}")
								} else {
									resolved = substitutionString(walked)
								}
							} else {
								log.WithFields(log.Fields{
									"name": ref.Child,
								}).Warn("unknown flow variable")
							}
						}
					} else if f.variables != nil {
						if varVal, ok := f.variables[name]; ok {
							resolved = substitutionString(varVal)
						} else {
							log.WithFields(log.Fields{
								"name": name,
							}).Warn("unknown flow variable")
						}
					}
					replaceToken(val, m, jsonCtx, resolved)
				} else if strings.HasPrefix(m, "credentials.") {
					if environment == nil {
						log.WithFields(log.Fields{
							"name": m,
						}).Warn("No environment configured for credential substitution")
						continue
					}
					// ${credentials.NAME} resolves the OAuth access token;
					// ${credentials.NAME.KEY} resolves a metadata value captured
					// at connect time (QuickBooks realm_id / Xero tenant_id).
					// Credential names are sanitised to [A-Za-z0-9_-] (no dots),
					// so a dot after the name always delimits the metadata key.
					rest := strings.TrimPrefix(m, "credentials.")
					if dot := strings.IndexByte(rest, '.'); dot >= 0 {
						name := rest[:dot]
						key := rest[dot+1:]
						metaVal, err := environment.GetCredentialMeta(name, key)
						if err != nil {
							log.WithFields(log.Fields{
								"error": err,
								"name":  name,
								"key":   key,
							}).Error("unable to get credential metadata")
							continue
						}
						if metaVal == nil {
							log.WithFields(log.Fields{
								"name": name,
								"key":  key,
							}).Warn("missing credential metadata")
							continue
						}
						*val = strings.ReplaceAll(*val, "${"+m+"}", *metaVal)
						continue
					}
					name := rest
					token, err := environment.GetCredential(name)
					if err != nil {
						log.WithFields(log.Fields{
							"error": err,
							"name":  name,
						}).Error("unable to get credential")
						continue
					}
					if token == nil {
						log.WithFields(log.Fields{
							"name": name,
						}).Warn("missing credential")
						continue
					}
					replaceToken(val, m, jsonCtx, *token)
				} else if strings.HasPrefix(m, "secrets.") || strings.HasPrefix(m, "secret.") {
					if environment == nil {
						log.WithFields(log.Fields{
							"name": m,
						}).Warn("No environment configured for secret substitution")
						continue
					}
					name := strings.TrimPrefix(m, "secrets.")
					name = strings.TrimPrefix(name, "secret.")
					p, err := environment.GetSecret(name)
					if err != nil {
						log.WithFields(log.Fields{
							"error": err,
						}).Error("unable to get Secret")
						continue
					}

					if p == nil || p.Value == nil {
						log.WithFields(log.Fields{
							"name": name,
						}).Warn("missing secret")
						continue
					}

					replaceToken(val, m, jsonCtx, *p.Value)
				} else {
					// Parent-output reference. Two forms:
					//   ${nodeId.key}                      — top-level output
					//   ${nodeId.key.subfield[0].sub2}     — drill into a
					//                                        structured output
					// Path support: parse the reference up front. If a
					// path is present, we MUST go through the
					// node-results cache + ResolvePath (the
					// parentResults map is keyed by full ${ref}, which
					// won't have a path-bearing key). For path-free
					// references the existing fast path still wins.
					ref, refOk := ParseReference(m)
					pathPresent := refOk && len(ref.Path) > 0

					if !pathPresent {
						if res, exists := parentResults[m]; exists {
							replaceToken(val, m, jsonCtx, substitutionString(res))
							continue
						}
					}

					if dotIdx := strings.IndexByte(m, '.'); dotIdx > 0 {
						// Scoped node reference: ${nodeId.key[.path...]} — look up
						// in the global node results cache. This handles cases
						// where the referenced node is an ancestor but not a
						// direct parent (e.g. separated by a Switch or For loop).
						//
						// Use ParseReference's tokenisation (when it
						// succeeded) to pick out the node ID + first
						// child key correctly even when the reference
						// contains paths or bracket indices.
						var scopeNodeID, scopeKey string
						var path []string
						if refOk {
							scopeNodeID = ref.Root
							scopeKey = ref.Child
							path = ref.Path
						} else {
							scopeNodeID = m[:dotIdx]
							scopeKey = m[dotIdx+1:]
						}

						// If the node hasn't been executed yet (e.g. sibling
						// in a loop body), execute it now to populate results.
						//
						// Triggers are an exception: they're entry points,
						// not dependencies. Only the FIRING trigger should
						// run in any given execution; executing a different
						// trigger as a side-effect of variable resolution
						// produces empty outputs (its real data never
						// arrives) AND falsely marks it as triggered in the
						// execution viewer. Worse, the missing output then
						// leaks the trigger's node ID into downstream AI
						// prompts as a literal `${<uuid>.key}` reference,
						// which Anthropic models have been observed to
						// strip-hyphens-from and use as a fake blob handle
						// (see execution d402887d). Skip them here so
						// neither symptom appears.
						if _, ok := f.nodeResults[scopeNodeID]; !ok {
							if scopeNode := f.FindNode(scopeNodeID); scopeNode != nil {
								if scopeNode.Data != nil && scopeNode.Data.Config.Type == ActionTypeTrigger {
									log.WithFields(log.Fields{
										"node_id": scopeNodeID,
										"key":     scopeKey,
									}).Debug("skipping scoped dependency execution for trigger node — only the firing trigger should run")
								} else if f.isOnUnmatchedBranch(actions, scopeNodeID, environment) {
									// The referenced node sits on a conditional
									// branch that wasn't taken (e.g. a Create
									// Security Group under an If whose OTHER branch
									// matched). Resolving ${nodeId.key} must not run
									// an action on a disabled branch — leave the
									// reference unresolved so it falls through to the
									// empty-string replacement below.
									log.WithFields(log.Fields{
										"node_id": scopeNodeID,
										"key":     scopeKey,
									}).Debug("skipping scoped dependency on unmatched branch — resolving to empty")
								} else {
									log.WithFields(log.Fields{
										"node_id": scopeNodeID,
										"key":     scopeKey,
									}).Info("executing scoped dependency node")
									// ExecuteNode (action + children) so a matched-
									// branch child of the referenced node still runs
									// (e.g. Authorize-Ingress after Create-Security-
									// Group). The in-progress guard prevents re-running
									// the node we're substituting for if it happens to
									// be a downstream child (diamond double-run).
									if _, err := f.ExecuteNode(actions, scopeNode, environment); err != nil {
										log.WithFields(log.Fields{
											"node_id": scopeNodeID,
											"error":   err,
										}).Warn("failed to execute scoped dependency node")
									}
								}
							}
						}

						// Always remove the ${...} placeholder. If the
						// lookup succeeds we replace with the resolved
						// value; if it fails we replace with empty string
						// so the literal `${nodeId.key}` text never reaches
						// downstream actions or AI prompts. The literal
						// leftover is what previously let the AI extract a
						// UUID from an unresolved reference and construct a
						// fake blob handle from it; an empty replacement
						// removes that surface area entirely. The miss is
						// still logged so flow authors can spot the
						// misconfiguration.
						replacement := ""
						if nr, ok := f.nodeResults[scopeNodeID]; ok {
							if res, ok := nr[scopeKey]; ok {
								if len(path) > 0 {
									// Path-bearing reference — walk
									// into the resolved value. JSON-
									// string roots are auto-parsed by
									// ResolvePath, so this transparently
									// supports Web/HTTP response_body
									// references like
									// ${web.response_body.user.id}.
									walked, perr := ResolvePath(res, path)
									if perr != nil {
										log.WithFields(log.Fields{
											"output":  m,
											"node_id": scopeNodeID,
											"key":     scopeKey,
											"path":    path,
											"error":   perr,
										}).Warn("path resolution failed on scoped output")
									} else {
										replacement = substitutionString(walked)
									}
								} else {
									replacement = substitutionString(res)
								}
							} else {
								log.WithFields(log.Fields{
									"output":  m,
									"node_id": scopeNodeID,
									"key":     scopeKey,
								}).Warn("scoped output key not found in node results")
							}
						} else {
							log.WithFields(log.Fields{
								"output":  m,
								"node_id": scopeNodeID,
							}).Warn("scoped node not found in results cache")
						}
						replaceToken(val, m, jsonCtx, replacement)
					} else {
						log.WithFields(log.Fields{
							"output": m,
						}).Warn("substitution upstream output does not exist")
						// Bare, unscoped reference (no namespace, no dot) that
						// matched no parent output — e.g. a form field that was
						// conditionally hidden and so never submitted. Replace
						// with empty string rather than leaving the literal
						// ${field_name} in place, matching the scoped-reference
						// branch above and the ${flow.X}/${user.X}/${var.X}
						// misses. A leaked ${...} literal reaches downstream
						// actions and AI prompts as text (see the no-literal
						// guarantee in flow_scoped_dep_test.go).
						replaceToken(val, m, jsonCtx, "")
					}
				}
			}

			value = *val
		}

		configuration = append(configuration, &Connection{
			Name:  v.Name,
			Type:  v.Type,
			Value: value,
		})
	}

	// Emit running event
	f.emitNodeEvent(node.ID, node.Type, node.Data.Label, "running", 0, "")

	startTime := time.Now()
	outputs, err := action(f, node, configuration)
	durationMs := time.Since(startTime).Milliseconds()

	// Build obfuscated input map for streaming and recording
	inputMap := f.buildObfuscatedInputMap(node, configuration)

	if errors.Is(err, ErrSuspended) {
		// Record the suspend node as suspended (not failed) and cache its outputs
		f.emitNodeEvent(node.ID, node.Type, node.Data.Label, "suspended", durationMs, "", inputMap, outputs)
		f.recordNodeResult(node, "suspended", configuration, outputs, durationMs, "")
		if outputs != nil {
			f.nodeResults[node.ID] = outputs
		}
		return outputs, ErrSuspended
	}

	if err != nil {
		f.emitNodeEvent(node.ID, node.Type, node.Data.Label, "failed", durationMs, err.Error(), inputMap, outputs)
		f.recordNodeResult(node, "failed", configuration, outputs, durationMs, err.Error())

		log.WithFields(log.Fields{
			"error": err,
		}).Error("Error processing Action")
		return nil, err
	}

	// Check for soft failures: the action returned nil error but set
	// success=false in its outputs (common in tool actions that handle
	// errors gracefully). Mark the node as failed so the UI shows it
	// correctly, but still return the outputs (not a Go error) so the
	// tool loop can feed the error back to the AI for recovery.
	if successVal, ok := outputs["success"]; ok {
		if isFalse, ok := successVal.(bool); ok && !isFalse {
			errMsg := ""
			if e, ok := outputs["error"].(string); ok {
				errMsg = e
			}
			f.emitNodeEvent(node.ID, node.Type, node.Data.Label, "failed", durationMs, errMsg, inputMap, outputs)
			f.recordNodeResult(node, "failed", configuration, outputs, durationMs, errMsg)
			f.nodeResults[node.ID] = outputs
			return outputs, nil
		}
	}

	// For conditional/switch/loop nodes, merge parent outputs into this
	// node's outputs so they pass through to child branches. Without this,
	// a Send action after a Switch can't see the AI response or trigger
	// data from upstream — it only sees the Switch's own outputs ({result,
	// matched_case}). The merge uses child-wins semantics: the conditional's
	// own outputs take precedence over inherited parent outputs.
	if node.Data != nil && (node.Data.Config.Type == ActionTypeConditional ||
		node.Data.Config.Type == ActionTypeSwitch ||
		node.Data.Config.Type == ActionTypeLoop ||
		node.Data.Config.Type == ActionTypeAwait) {
		for k, v := range parentResults {
			// Skip scoped parent keys (nodeId.outputName) — only pass through
			// flat keys. Scoped keys are rebuilt per-node in the parent loop
			// and should not cascade through pass-through nodes.
			if strings.Contains(k, ".") {
				continue
			}
			if _, exists := outputs[k]; !exists {
				outputs[k] = v
			}
		}
	}

	// For trigger nodes, merge injected trigger data (stored as Config.Inputs
	// by InjectTriggerData) into the action's outputs. Without this, downstream
	// nodes cannot reference trigger data via ${nodeID.key} because the trigger
	// action's Execute only returns its own built-in outputs (e.g. {quote, start}).
	// The merge uses input-wins-not semantics: if the action already produced an
	// output with the same name as an injected input, the action's output takes
	// precedence. This ensures built-in trigger outputs are stable while injected
	// data fills the gaps.
	if node.Data != nil && node.Data.Config.Type == ActionTypeTrigger {
		for _, input := range node.Data.Config.Inputs {
			if _, exists := outputs[input.Name]; !exists && input.Value != nil {
				if s, ok := input.Value.(string); ok && s != "" {
					outputs[input.Name] = s
				} else if input.Value != nil {
					outputs[input.Name] = input.Value
				}
			}
		}
	}

	// Strip platform-internal tags (e.g. [LINK_OFFER:...]) from the response
	// output before caching. These tags are instructions from the system prompt
	// that must not be delivered to the user.
	if resp, ok := outputs["response"].(string); ok {
		if start := strings.Index(resp, "[LINK_OFFER:"); start >= 0 {
			if end := strings.Index(resp[start:], "]"); end >= 0 {
				tag := resp[start+len("[LINK_OFFER:") : start+end]
				// Emit a platform event so the API can create the pending action
				fmt.Fprintf(os.Stdout, "__LINK_OFFER__:%s\n", tag)
			}
		}
		outputs["response"] = stripPlatformTags(resp)
	}

	f.emitNodeEvent(node.ID, node.Type, node.Data.Label, "success", durationMs, "", inputMap, outputs)
	f.recordNodeResult(node, "success", configuration, outputs, durationMs, "")

	f.nodeResults[node.ID] = outputs
	return outputs, nil
}

// executeNodeChildren determines and executes the children of a node using
// breadth-first ordering: all sibling actions execute before any subtrees.
func (f *Flow) executeNodeChildren(actions map[string]Action, node *Node, outputs map[string]interface{}, environment *environment.Environment) (map[string]interface{}, error) {
	combinedResults := make(map[string]interface{})

	// subflow/end terminates the sub-flow's traversal. Its outputs become
	// the sub-flow's return value (see executeSubFlow's collection at
	// flow.go:2918-2929), so any nodes wired downstream of End must not
	// execute. Without this guard, edges from End leak into the main
	// graph traversal and silently run "after-end" nodes.
	if node.Data != nil && (node.Data.Label == "subflow/end" || node.Type == "subflow/end") {
		return combinedResults, nil
	}

	var children []*Node
	if node.Data.Config.Type == ActionTypeConditional {
		result, ok := outputs["result"].(bool)
		if !ok {
			return nil, fmt.Errorf("conditional node %s did not return a boolean result", node.ID)
		}

		if result {
			children = f.FindTargetByHandle(node.ID, "true-branch")
		} else {
			children = f.FindTargetByHandle(node.ID, "false-branch")
		}
	} else if node.Data.Config.Type == ActionTypeSwitch {
		// Switch: route to the matched case handle, or default
		branch, _ := outputs["matched_case"].(string)
		if branch != "" {
			children = f.FindTargetByHandle(node.ID, branch)
		}
		// Fall back to default handle if no match or no children on matched handle
		if len(children) == 0 {
			children = f.FindTargetByHandle(node.ID, "default")
		}
	} else if node.Data.Config.Type == ActionTypeAwait {
		// Human-in-the-Loop: route to the handle for the chosen option
		// ("option_<value>") or "timeout". Emitted under matched_case for
		// symmetry with Switch. No default fallback — an unrouted outcome
		// simply ends that branch. On the first (suspending) pass matched_case
		// is empty and there are no children to run.
		branch, _ := outputs["matched_case"].(string)
		if branch != "" {
			children = f.FindTargetByHandle(node.ID, branch)
		}
	} else if node.Data.Config.Type == ActionTypeLoop {
		// Loop execution: iterate body children, then execute output children
		loopChildren := f.FindTargetByHandle(node.ID, "loop")
		outputChildren := f.FindTargetByHandle(node.ID, "output")

		maxIter := int64(1000)
		if mi, ok := outputs["max_iterations"].(int64); ok && mi > 0 {
			maxIter = mi
		}

		var iteration int64
		var loopAborted bool
		for iteration = 0; iteration < maxIter; iteration++ {
			// Re-evaluate the loop condition by re-executing the loop node's action
			// On first iteration, use the already-computed outputs
			if iteration > 0 {
				// Clear the loop node's own cached result so it re-evaluates
				delete(f.nodeResults, node.ID)
				// Clear loop body results so children re-execute, but do NOT
				// clear the loop node's parents — they provide inputs like
				// count/array that must persist across iterations.
				f.clearSubgraphResults(node.ID, "loop")

				// Re-resolve inputs from parents and re-execute via the full
				// resolution path. Using executeNodeActionOnly ensures auto-wired
				// parent outputs (e.g. count from a list action) are properly
				// resolved on each iteration.
				reOutputs, err := f.executeNodeActionOnly(actions, node, environment)
				if err != nil {
					// For voice calls, re-evaluation errors (closed WebSocket)
					// mean the call ended — exit the loop cleanly.
					ctx := f.GetContext()
					if ctx != nil && ctx.ChannelType == "twilio_voice" {
						loopAborted = true
						break
					}
					return nil, err
				}
				outputs = reOutputs
			}

			// Set loop context variables and update outputs
			f.SetVariable("loop.index", iteration)
			f.SetVariable("loop.iteration", iteration+1)
			outputs["current_index"] = iteration
			outputs["iterations"] = iteration
			f.nodeResults[node.ID] = outputs

			result, ok := outputs["result"].(bool)
			if !ok || !result {
				break
			}

			// Execute loop body
			for _, c := range loopChildren {
				if c.Type != "" && strings.HasPrefix(c.Type, "trigger/") {
					continue
				}
				_, err := f.ExecuteNode(actions, c, environment)
				if err != nil {
					// For voice calls, subgraph errors during call hangup
					// should terminate the loop cleanly.
					ctx := f.GetContext()
					if ctx != nil && ctx.ChannelType == "twilio_voice" {
						log.WithFields(log.Fields{
							"error": err,
							"node":  c.ID,
						}).Info("voice loop subgraph error (likely call hangup) — exiting loop")
						loopAborted = true
						break
					}
					// Store final iteration count before returning error
					outputs["iterations"] = iteration + 1
					f.nodeResults[node.ID] = outputs
					return nil, err
				}
			}

			if loopAborted {
				break
			}

			// Clear loop body results for next iteration
			f.clearSubgraphResults(node.ID, "loop")
		}

		// Store final iteration count
		outputs["iterations"] = iteration
		f.nodeResults[node.ID] = outputs

		// Execute output-handle children (post-loop).
		// Skip if the loop was aborted by a voice hangup.
		if loopAborted {
			children = nil
		} else {
			children = outputChildren
		}
	} else if _, hasToolReqs := outputs[ToolRequestsKey]; hasToolReqs {
		// AI Tool Use loop: the AI action returned tool requests instead
		// of a final response. Execute the tools subgraph for each request,
		// collect results, re-invoke the AI action with results, and repeat
		// until the model produces a text response (stop_reason != tool_use).
		toolsChildren := f.FindTargetByHandle(node.ID, ToolsHandle)
		outputChildren := f.FindTarget(node.ID)
		// Filter: output children are those NOT connected via the tools handle
		toolsChildIDs := make(map[string]bool)
		for _, tc := range toolsChildren {
			toolsChildIDs[tc.ID] = true
		}
		var nonToolChildren []*Node
		for _, oc := range outputChildren {
			if !toolsChildIDs[oc.ID] {
				nonToolChildren = append(nonToolChildren, oc)
			}
		}

		if len(toolsChildren) == 0 {
			// No tools wired — just pass through to normal children
			children = nonToolChildren
		} else {
			maxRounds := MaxToolRoundsDefault

			// Resolve Response handle children once for intermediate messages.
			// These are the nodes wired to the "output" (Response) handle —
			// they get fired mid-loop for intermediate text AND at the end
			// for the final response.
			responseChildren := f.FindTargetByHandle(node.ID, "output")

			// Snapshot tool node inputs before the loop so we can reset
			// them between calls. Without this, the first call's injected
			// values persist and block subsequent calls from receiving
			// different parameter values from the AI.
			toolInputSnapshots := make(map[string][]interface{})
			for _, c := range toolsChildren {
				if c.Data == nil {
					continue
				}
				var snap []interface{}
				for _, inp := range c.Data.Config.Inputs {
					snap = append(snap, inp.Value)
				}
				toolInputSnapshots[c.ID] = snap
			}

			for round := 0; round < maxRounds; round++ {
				requests, ok := outputs[ToolRequestsKey].([]ToolRequest)
				if !ok || len(requests) == 0 {
					break
				}

				// If the AI emitted text alongside tool calls (e.g.
				// "Checking your calendar..."), fire the Response handle
				// children now so the user sees the message immediately.
				if iText, ok := outputs[IntermediateTextKey].(string); ok && iText != "" {
					// Skip intermediate text dispatch for voice calls —
					// playing partial audio mid-tool-loop sounds broken.
					ctx := f.GetContext()
					isVoice := ctx != nil && (ctx.ChannelType == "twilio_voice")

					if !isVoice {
						// The AI emitted text alongside tool calls. Fire the
						// Response handle subgraph so the user sees the message
						// while tools execute.
						f.clearSubgraphResults(node.ID, "output")
						if f.nodeResults[node.ID] == nil {
							f.nodeResults[node.ID] = make(map[string]interface{})
						}
						f.nodeResults[node.ID]["response"] = iText
						for _, rc := range responseChildren {
							if _, err := f.ExecuteNode(actions, rc, environment); err != nil {
								log.WithFields(log.Fields{
									"error": err,
									"node":  rc.ID,
								}).Warn("intermediate message dispatch failed")
							}
						}
						// Clear the subgraph again so the final response (or
						// next intermediate) can re-execute it cleanly.
						f.clearSubgraphResults(node.ID, "output")
					}
				}

				var results []ToolResult
				for _, req := range requests {
					// Refresh the typing indicator between tool calls so the
					// user sees continuous feedback during multi-tool turns.
					// Only fires for Telegram (other channels don't support it).
					f.sendTypingIndicator()

					// Set tool context variables so the tools subgraph
					// can read ${var.tool.name}, ${var.tool.id}, etc.
					f.SetVariable("tool.name", req.Name)
					f.SetVariable("tool.id", req.ID)
					if inputJSON, err := json.Marshal(req.Input); err == nil {
						f.SetVariable("tool.input", string(inputJSON))
					}
					for k, v := range req.Input {
						f.SetVariable("tool.input."+k, substitutionString(v))
					}

					// Clear tools subgraph results for this execution
					f.clearSubgraphResults(node.ID, ToolsHandle)

					// Route to the matching tool child by name. Each tool
					// action wired to the Tools handle is matched by its
					// data.label (action type name) against the requested
					// tool name. No Switch node needed — the engine routes
					// internally.
					var toolOutput string
					var toolErr bool
					var matchedTool *Node
					for _, c := range toolsChildren {
						if c.Data == nil {
							continue
						}
						label := c.Data.Label

						// Exact label match (tools/email_send == tools/email_send)
						if label == "tools/"+req.Name || label == req.Name {
							matchedTool = c
							break
						}

						// Sanitised match: the AI receives tool names with /
						// replaced by _ (via sanitiseToolName), so reverse that
						// by sanitising the label and comparing.
						sanitised := sanitiseToolName(label)
						if sanitised == req.Name {
							matchedTool = c
							break
						}
						// Also try stripping tools/ prefix before sanitising.
						if strings.HasPrefix(label, "tools/") {
							if sanitiseToolName(strings.TrimPrefix(label, "tools/")) == req.Name {
								matchedTool = c
								break
							}
						}

						// Config.Name fallback.
						if c.Data.Config.Name != nil && *c.Data.Config.Name == req.Name {
							matchedTool = c
							break
						}
					}

					if matchedTool == nil {
						toolOutput = fmt.Sprintf("Unknown tool: %s. Available tools: ", req.Name)
						for i, c := range toolsChildren {
							if i > 0 {
								toolOutput += ", "
							}
							toolOutput += c.Data.Label
						}
						toolErr = true
					} else {
						// Reset the matched tool's inputs to their original
						// snapshotted values before injecting new parameters.
						// This prevents the previous call's values from
						// persisting and blocking the current call's values.
						if snap, ok := toolInputSnapshots[matchedTool.ID]; ok {
							for i, inp := range matchedTool.Data.Config.Inputs {
								if i < len(snap) {
									inp.Value = snap[i]
								}
							}
						}

						// Detokenise any blob references in the LLM's
						// arguments. The model passes flo:blob:<handle>
						// references verbatim when carrying data
						// between tool calls; we resolve them back to
						// the real values here so actions stay
						// completely unaware of off-loading.
						//
						// Use f.Blobs() (lazy-init getter) rather than
						// the raw f.blobs field — otherwise the AI's
						// first tool call with a blob token sees a nil
						// store (the store is only instantiated on the
						// first TokeniseLargeOutputs call further
						// down), the resolution silently no-ops, and
						// the raw token reaches the action's base64
						// decoder. f.Blobs() always returns a usable
						// store; the failure modes from here on are
						// genuine "handle not found" / "API down".
						//
						// On any resolution failure we short-circuit
						// the whole tool call with a clear error: the
						// raw token never reaches the action (which
						// would only emit a base64-decode error the AI
						// can't act on). The error string names the
						// offending field so the AI can correct the
						// next tool call — either with a real token
						// from this turn's manifest, or by calling the
						// producing tool again.
						if detoked, derr := DetokeniseInputs(req.Input, f.Blobs()); derr == nil {
							req.Input = detoked
						} else {
							log.WithFields(log.Fields{
								"tool":  req.Name,
								"error": derr,
							}).Warn("blob token resolution failed; surfacing as tool error")
							toolOutput = fmt.Sprintf(
								"Unable to resolve blob reference passed to %s: %v\n\n"+
									"The flo:blob:<handle> token you passed is unknown to the executor — "+
									"either the handle was hallucinated, or the handle came from a prior "+
									"execution and is no longer available. Pass the verbatim token from "+
									"THIS turn's tool result manifest, or invoke the producing tool again "+
									"to generate a fresh one.",
								req.Name, derr)
							toolErr = true
							// Don't run the action — it would only emit
							// a confusing base64-decode error on the raw
							// token. The toolErr+toolOutput pair we just
							// set propagates back to the AI's next turn
							// via the normal result-collection path
							// below.
						}

						// Inject tool input as the matched node's input values.
						// We must also clear the cached result for this node
						// so executeNodeActionOnly re-reads the updated inputs.
						delete(f.nodeResults, matchedTool.ID)

						for _, inp := range matchedTool.Data.Config.Inputs {
							// Never allow the AI to override pre-configured
							// input values set by the flow author. These are
							// filtered out of the tool schema entirely, so
							// the AI shouldn't pass them — but some models
							// hallucinate parameters and pass empty strings,
							// wiping out legitimate pre-set values.
							//
							// This now also protects ${...} variable references.
							// The variable will be resolved during normal input
							// processing; the AI's value would overwrite the
							// reference before resolution could occur.
							if inp.Value != nil {
								if s, ok := inp.Value.(string); ok && s != "" {
									continue
								}
							}
							if v, exists := req.Input[inp.Name]; exists {
								inp.Value = v
								log.WithFields(log.Fields{
									"tool":  req.Name,
									"input": inp.Name,
									"value": fmt.Sprintf("%v", v),
								}).Debug("injected tool input")
							}
						}
						// Also add any inputs that don't have a matching
						// connection definition
						for k, v := range req.Input {
							found := false
							for _, inp := range matchedTool.Data.Config.Inputs {
								if inp.Name == k {
									found = true
									break
								}
							}
							if !found {
								matchedTool.Data.Config.Inputs = append(
									matchedTool.Data.Config.Inputs,
									&Connection{Name: k, Type: ConnectionTypeString, Value: v},
								)
							}
						}

						// toolErr can be set by the detokenisation
						// short-circuit above. In that case we've
						// already populated toolOutput with an
						// actionable error for the AI and must NOT
						// run the action with the bad inputs.
						if !toolErr {
							_, err := f.ExecuteNode(actions, matchedTool, environment)
							if err != nil {
								toolOutput = fmt.Sprintf("Tool execution error: %v", err)
								toolErr = true
							} else {
								// Cross-conversation relay recording: if the
								// tool just dispatched a messaging action
								// (send-slack, send-telegram, ...) record
								// the outbound against the recipient's
								// conversation so future inbound from them
								// surfaces the prior message in history.
								// Lives in flow_record_outbound.go; pure
								// post-send bookkeeping, never blocks.
								f.recordOutboundRelay(matchedTool)
							}
						}
					}

					// Collect the tool result from the executed node
					if toolOutput == "" && matchedTool != nil {
						if r, exists := f.nodeResults[matchedTool.ID]; exists && r != nil {
							for _, key := range []string{"tool_result", "result", "response", "output"} {
								if v, ok := r[key]; ok {
									if s, ok := v.(string); ok && s != "" {
										toolOutput = s
										break
									}
									if v != nil {
										if b, err := json.Marshal(v); err == nil {
											toolOutput = string(b)
											break
										}
									}
								}
							}
							// Fall back to JSON of all outputs
							if toolOutput == "" {
								if b, err := json.Marshal(r); err == nil {
									toolOutput = string(b)
								}
							}
						}
					}
					if toolOutput == "" {
						toolOutput = "Tool produced no output"
						toolErr = true
					}

					// Off-load any large outputs to the BlobStore and
					// append a manifest of references to the
					// LLM-visible content. The model sees compact
					// tokens it can pass verbatim into subsequent
					// tool calls; the action's actual output map in
					// f.nodeResults is left intact so the editor's
					// inspector, the Download button, and graph-wired
					// downstream nodes all keep seeing real values.
					if !toolErr && matchedTool != nil {
						if r, exists := f.nodeResults[matchedTool.ID]; exists && r != nil {
							manifest, failures := TokeniseLargeOutputs(r, f.Blobs())
							if len(manifest) > 0 || len(failures) > 0 {
								toolOutput += FormatTokenManifest(manifest, failures)
								log.WithFields(log.Fields{
									"tool":          req.Name,
									"tokenised":     len(manifest),
									"tokenise_fail": len(failures),
								}).Info("blob off-load result for tool outputs")
							}
						}
					}

					results = append(results, ToolResult{
						ToolUseID: req.ID,
						Content:   toolOutput,
						IsError:   toolErr,
					})
				}

				// Accumulate completed tool exchanges so the AI action
				// can record them in the conversation history after the
				// final response. This persists across rounds.
				var accumulated []map[string]interface{}
				if prev, ok := f.GetVariable(ToolExchangesKey); ok && prev != nil {
					if arr, ok := prev.([]map[string]interface{}); ok {
						accumulated = arr
					}
				}
				for i, req := range requests {
					exchange := map[string]interface{}{
						"tool_use_id": req.ID,
						"name":        req.Name,
						"input":       req.Input,
						"result":      results[i].Content,
						"is_error":    results[i].IsError,
					}
					accumulated = append(accumulated, exchange)
				}
				f.SetVariable(ToolExchangesKey, accumulated)

				// Store results and conversation state for the AI action's
				// next invocation, then re-execute the AI node.
				f.SetVariable(ToolResultsKey, results)
				if convState, exists := outputs[ToolConversationStateKey]; exists {
					f.SetVariable(ToolConversationStateKey, convState)
				}

				// Clear AI node's cached result so it re-executes
				delete(f.nodeResults, node.ID)

				reOutputs, err := f.executeNodeActionOnly(actions, node, environment)
				if err != nil {
					return nil, err
				}
				outputs = reOutputs

				// Clean up tool variables for next round
				f.SetVariable(ToolResultsKey, nil)
				f.SetVariable(ToolConversationStateKey, nil)

				// If the re-invocation used streaming, drain the channel
				// and extract results (text plays via TTS, tool requests
				// are picked up by the next loop iteration).
				if _, isStreaming := outputs[StreamSentencesKey]; isStreaming {
					f.drainStreamingChannel(actions, node, outputs, environment, nil)
					delete(outputs, StreamSentencesKey)
					f.nodeResults[node.ID] = outputs
				}

				// Check if the model is done (no more tool requests)
				if _, hasMore := outputs[ToolRequestsKey]; !hasMore {
					break
				}
			}

			// Update the node's cached result with the final outputs
			f.nodeResults[node.ID] = outputs

			// Execute non-tool children (the main response path)
			children = nonToolChildren
		}

		// After tool loop (or if no tools): check should_respond for
		// the no_response handle routing
		if sr, ok := outputs["should_respond"]; ok {
			if shouldRespond, ok := sr.(bool); ok && !shouldRespond {
				noRespChildren := f.FindTargetByHandle(node.ID, "no_response")
				if len(noRespChildren) > 0 {
					children = noRespChildren
				} else {
					// No handle wired — just skip all children (silent drop)
					children = nil
				}
			}
		}
	} else if sr, ok := outputs["should_respond"]; ok {
		// AI node with should_respond output — route appropriately
		if shouldRespond, ok := sr.(bool); ok && !shouldRespond {
			noRespChildren := f.FindTargetByHandle(node.ID, "no_response")
			if len(noRespChildren) > 0 {
				children = noRespChildren
			} else {
				children = nil
			}
		} else {
			// should_respond=true: route to "output" handle children
			// (the Response handle), falling back to default handle
			children = f.FindTargetByHandle(node.ID, "output")
			if len(children) == 0 {
				children = f.FindTargetByDefaultHandle(node.ID)
			}
		}
	}

	// Streaming sentence consumer: drain the sentence channel, fire
	// Response children per sentence, then check for tool requests.
	if flag, ok := outputs[StreamSentencesKey]; ok && flag != nil {
		f.drainStreamingChannel(actions, node, outputs, environment, children)
		delete(outputs, StreamSentencesKey)
		f.nodeResults[node.ID] = outputs

		// If the stream produced tool requests, re-enter executeNodeChildren
		// which will hit the tool loop with the updated outputs.
		if _, hasToolReqs := outputs[ToolRequestsKey]; hasToolReqs {
			toolsChildren := f.FindTargetByHandle(node.ID, ToolsHandle)
			if len(toolsChildren) > 0 {
				return f.executeNodeChildren(actions, node, outputs, environment)
			}
		}

		// No tools — streaming text was the final response.
		children = nil
	}

	if node.Data != nil && (node.Data.Label == "subflow/invoke" || node.Type == "subflow/invoke") {
		// Sub-flow invocation: dispatch to the matching Begin Sub-Flow,
		// execute its chain, and merge the results into this node's outputs.
		if sfName, ok := outputs[SubFlowNameKey].(string); ok && sfName != "" {
			// Merge parent outputs into the invoke outputs so that
			// upstream variables (e.g. current_index from a Loop) are
			// available inside the sub-flow via ${variable} substitution.
			invokePayload := make(map[string]interface{})
			for _, p := range f.FindSource(node.ID) {
				if pResult, exists := f.nodeResults[p.ID]; exists {
					for k, v := range pResult {
						invokePayload[k] = v
					}
				}
			}
			for k, v := range outputs {
				invokePayload[k] = v
			}
			sfOutputs, err := f.executeSubFlow(actions, sfName, invokePayload, environment)
			if err != nil {
				return nil, err
			}
			for k, v := range sfOutputs {
				if k == "tool_result" || k == SubFlowNameKey {
					continue
				}
				outputs[k] = v
			}
			outputs["tool_result"] = fmt.Sprintf("Sub-flow '%s' completed", sfName)
			outputs["success"] = true
			outputs["error"] = ""
			f.nodeResults[node.ID] = outputs
		}
		children = f.FindTarget(node.ID)
	}

	// Default: if no special handler above set children, use all children.
	// This covers regular action nodes without should_respond, tool
	// requests, loops, or subflows. Nodes that DID have a handler
	// (should_respond, streaming, etc.) already set children — even to
	// nil — and we must not overwrite that.
	// We detect "was handled" by checking if should_respond exists in
	// outputs, since only AI nodes have it.
	if children == nil {
		if _, hasShouldRespond := outputs["should_respond"]; !hasShouldRespond {
			children = f.FindTarget(node.ID)
		}
	}

	// AI "Finished" handle: fire once after the entire AI turn completes
	// (after streaming, tool loops, etc.). Used for post-response actions
	// like Add to Conversation that should run once per turn, not per
	// sentence or per tool round.
	if _, hasShouldRespond := outputs["should_respond"]; hasShouldRespond {
		finishedChildren := f.FindTargetByHandle(node.ID, "no_response")
		for _, fc := range finishedChildren {
			if _, err := f.ExecuteNode(actions, fc, environment); err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"node":  fc.ID,
				}).Warn("AI finished handle execution failed")
			}
		}
	}

	// Filter out trigger nodes and Begin Sub-Flow nodes from children
	var validChildren []*Node
	for _, c := range children {
		if c.Type != "" && strings.HasPrefix(c.Type, "trigger/") {
			continue
		}
		if c.Data != nil && (c.Data.Label == "subflow/begin" || c.Type == "subflow/begin") {
			continue
		}
		// Skip a child that is currently mid-execution higher up the stack
		// (diamond). It is being handled by its own outer frame, which will
		// traverse its children; touching it here would run its action with a
		// not-yet-ready parent set and prematurely walk its subtree.
		if f.executing[c.ID] {
			continue
		}
		validChildren = append(validChildren, c)
	}

	// Breadth-first: execute all siblings' actions before traversing any subtrees.
	// Pass 1: run each sibling's action only (no child traversal)
	for _, c := range validChildren {
		outputs, err := f.executeNodeActionOnly(actions, c, environment)
		if err != nil {
			return nil, err
		}
		// Mark traversed AFTER running the action. Without this, any
		// grandchild's parent-resolution walk back to c would hit
		// ExecuteNode's "cached but not traversed" branch and call
		// executeNodeChildren(c) AGAIN — re-running the whole subtree
		// (catastrophic for Loop nodes: re-runs every iteration; bad
		// even for regular nodes: double-fires downstream sends, slacks,
		// etc.). Pass 2 below traverses c's children directly via
		// executeNodeChildren — bypassing ExecuteNode entirely — so
		// the traversedNodes mark doesn't block legitimate traversal.
		// Resume semantics are preserved: the start-node on resume is
		// invoked via ExecuteNode (not via Pass 1), so it still hits
		// the cached-but-not-traversed branch and walks correctly.
		if f.traversedNodes == nil {
			f.traversedNodes = make(map[string]bool)
		}
		f.traversedNodes[c.ID] = true
		// Include this child's own outputs in the combined results
		for k, v := range outputs {
			combinedResults[k] = v
		}
	}

	// Pass 2: now traverse each sibling's children (parent results are cached from pass 1)
	for _, c := range validChildren {
		childOutputs := f.nodeResults[c.ID]
		childResults, err := f.executeNodeChildren(actions, c, childOutputs, environment)
		if err != nil {
			return combinedResults, err
		}
		for k, v := range childResults {
			combinedResults[k] = v
		}
	}

	return combinedResults, nil
}

// clearSubgraphResults removes cached results for all nodes reachable from
// drainStreamingChannel reads all sentences from the streaming channel,
// fires Response children per sentence, then extracts tool requests and
// metadata from flow variables set by the streaming goroutine.
func (f *Flow) drainStreamingChannel(actions map[string]Action, node *Node, outputs map[string]interface{}, env *environment.Environment, children []*Node) {
	streamVar, hasVar := f.GetVariable(StreamSentencesKey)
	if !hasVar {
		return
	}
	ch, ok := streamVar.(chan string)
	if !ok || ch == nil {
		return
	}

	responseChildren := children
	if responseChildren == nil {
		responseChildren = f.FindTargetByHandle(node.ID, "output")
		if len(responseChildren) == 0 {
			responseChildren = f.FindTargetByDefaultHandle(node.ID)
		}
	}

	for sentence := range ch {
		if sentence == "" {
			continue
		}
		f.clearSubgraphResults(node.ID, "output")
		if f.nodeResults[node.ID] == nil {
			f.nodeResults[node.ID] = make(map[string]interface{})
		}
		f.nodeResults[node.ID]["response"] = sentence
		for _, rc := range responseChildren {
			if rc.Data != nil && rc.Data.Config.Type == ActionTypeLoop {
				continue
			}
			if _, err := f.ExecuteNode(actions, rc, env); err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"node":  rc.ID,
				}).Warn("streaming sentence dispatch failed")
			}
		}
		f.clearSubgraphResults(node.ID, "output")
	}

	f.SetVariable(StreamSentencesKey, nil)

	if fullText, ok := f.GetVariable(StreamFullTextKey); ok {
		if s, ok := fullText.(string); ok && s != "" {
			outputs["response"] = s
		}
		f.SetVariable(StreamFullTextKey, nil)
	}
	if sr, ok := f.GetVariable(StreamStopReasonKey); ok {
		if s, ok := sr.(string); ok {
			outputs["stop_reason"] = s
		}
		f.SetVariable(StreamStopReasonKey, nil)
	}
	if usage, ok := f.GetVariable(StreamUsageKey); ok {
		if u, ok := usage.(map[string]int64); ok {
			outputs["input_tokens"] = u["input_tokens"]
			outputs["output_tokens"] = u["output_tokens"]
		}
		f.SetVariable(StreamUsageKey, nil)
	}
	if m, ok := f.GetVariable("__stream_model"); ok {
		if s, ok := m.(string); ok && s != "" {
			outputs["model"] = s
		}
		f.SetVariable("__stream_model", nil)
	}
	if toolReqs, ok := f.GetVariable(StreamToolRequestsKey); ok {
		if reqs, ok := toolReqs.([]ToolRequest); ok && len(reqs) > 0 {
			outputs[ToolRequestsKey] = reqs
		}
		f.SetVariable(StreamToolRequestsKey, nil)
	}
}

// the given source via the specified handle. This allows loop body nodes to
// be re-executed on each iteration.
func (f *Flow) clearSubgraphResults(source string, handle string) {
	var targets []*Node
	if handle != "" {
		targets = f.FindTargetByHandle(source, handle)
	} else {
		targets = f.FindTarget(source)
	}

	for _, t := range targets {
		if _, exists := f.nodeResults[t.ID]; exists {
			delete(f.nodeResults, t.ID)
			// Recurse into children of this node
			f.clearSubgraphResults(t.ID, "")
		}
	}
}

// computeReachable performs a forward BFS from the given start node,
// returning the set of all node IDs reachable by following edges. This is
// used to restrict parent resolution so that nodes on unrelated trigger
// paths are not executed.
func (f *Flow) computeReachable(startID string) map[string]bool {
	// Forward BFS following edges from a start node.
	forward := func(start string) map[string]bool {
		seen := map[string]bool{start: true}
		q := []string{start}
		for len(q) > 0 {
			c := q[0]
			q = q[1:]
			for _, e := range f.Edges {
				if e == nil {
					continue
				}
				if e.Source == c && !seen[e.Target] {
					seen[e.Target] = true
					q = append(q, e.Target)
				}
			}
		}
		return seen
	}

	reachable := forward(startID)

	// Union of every trigger's forward reach. Used to distinguish an independent
	// trigger path (which must NOT be pulled in — that was the original purpose
	// of this function, preventing cross-trigger contamination) from a rootless
	// input-provider subgraph (which MUST run, since a reachable node depends on
	// its output).
	triggerForward := map[string]bool{}
	for _, n := range f.Nodes {
		if n == nil || n.Data == nil {
			continue
		}
		if strings.HasPrefix(n.Type, "trigger/") || strings.HasPrefix(n.Data.Label, "trigger/") {
			for id := range forward(n.ID) {
				triggerForward[id] = true
			}
		}
	}

	// Backward closure: pull in ancestors (input providers) of reachable nodes,
	// but ONLY rootless ones — nodes not forward-reachable from ANY trigger. This
	// includes a lookup/constant subgraph wired only as a parent of a reachable
	// action (e.g. describe -> if -> object_get feeding Run Instances) while
	// still excluding another trigger's path. Whether a pulled-in ancestor
	// actually executes is still gated by the unmatched-branch check, so an
	// ancestor sitting on a disabled conditional branch is correctly skipped.
	queue := make([]string, 0, len(reachable))
	for id := range reachable {
		queue = append(queue, id)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, e := range f.Edges {
			if e == nil {
				continue
			}
			if e.Target == current && !reachable[e.Source] && !triggerForward[e.Source] {
				reachable[e.Source] = true
				queue = append(queue, e.Source)
			}
		}
	}

	return reachable
}

func (f *Flow) FindSource(target string) []*Node {
	results := make([]*Node, 0)

	for _, e := range f.Edges {
		if e == nil {
			continue
		}

		if e.Target == target {
			n := f.FindNode(e.Source)
			if n != nil {
				results = append(results, n)
			}
		}
	}

	return results
}

func (f *Flow) FindTargetByHandle(source string, handle string) []*Node {
	results := make([]*Node, 0)

	for _, e := range f.Edges {
		if e == nil {
			continue
		}

		if e.Source == source && e.SourceHandle == handle {
			n := f.FindNode(e.Target)
			if n != nil {
				results = append(results, n)
			}
		}
	}

	return results
}

// FindTargetByDefaultHandle returns child nodes connected via the default
// (empty/unnamed) source handle only. Used by AI nodes to route to the
// Response path without accidentally including Tools or Finished children.
func (f *Flow) FindTargetByDefaultHandle(source string) []*Node {
	results := make([]*Node, 0)
	for _, e := range f.Edges {
		if e == nil {
			continue
		}
		if e.Source == source && e.SourceHandle == "" {
			n := f.FindNode(e.Target)
			if n != nil {
				results = append(results, n)
			}
		}
	}
	return results
}

func (f *Flow) FindTarget(source string) []*Node {
	results := make([]*Node, 0)

	for _, e := range f.Edges {
		if e == nil {
			continue
		}

		if e.Source == source {
			n := f.FindNode(e.Target)
			if n != nil {
				results = append(results, n)
			}
		}
	}

	return results
}

func (f *Flow) FindNode(id string) *Node {
	for _, n := range f.Nodes {
		if n.ID == id {
			return n
		}
	}

	return nil
}

func (f *Flow) GetNodeResult(nodeID string) map[string]interface{} {
	if f.nodeResults == nil {
		return nil
	}
	return f.nodeResults[nodeID]
}

// GetAllNodeResults returns all cached node results. Used by actions
// that need to find a value (like session_id) from any upstream node.
func (f *Flow) GetAllNodeResults() map[string]map[string]interface{} {
	return f.nodeResults
}

// SetNodeResultForTest allows tests to pre-populate cached node results.
func (f *Flow) SetNodeResultForTest(nodeID string, result map[string]interface{}) {
	if f.nodeResults == nil {
		f.nodeResults = make(map[string]map[string]interface{})
	}
	f.nodeResults[nodeID] = result
}

func (f *Flow) SetOutput(name string, value interface{}) {
	if _, exists := f.outputs[name]; exists {
		log.WithFields(log.Fields{
			"value": name,
		}).Warn("overwriting already set output value")
	}

	if f.outputs == nil {
		f.outputs = make(map[string]interface{})
	}

	f.outputs[name] = value
}

func (f *Flow) GetOutput(name string) interface{} {
	return f.outputs[name]
}

func (f *Flow) GetOutputs() map[string]interface{} {
	return f.outputs
}

func (f *Flow) SetVariable(name string, value interface{}) {
	if f.variables == nil {
		f.variables = make(map[string]interface{})
	}
	f.variables[name] = value
}

func (f *Flow) GetVariable(name string) (interface{}, bool) {
	if f.variables == nil {
		return nil, false
	}
	v, ok := f.variables[name]
	return v, ok
}

func (f *Flow) GetVariables() map[string]interface{} {
	return f.variables
}

// sendTypingIndicator fires a typing indicator to the current channel.
// Only works for Telegram (other channels silently skip). Uses the API's
// channel-action endpoint. Fire-and-forget — never blocks the tool loop.
func (f *Flow) sendTypingIndicator() {
	ctx := f.GetContext()
	if ctx == nil || ctx.APIURL == "" || ctx.AgentID == "" {
		return
	}
	if ctx.ChannelType != "telegram" {
		return
	}
	if ctx.ChannelID == "" {
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"channel_type": ctx.ChannelType,
		"action":       "typing",
		"chat_id":      ctx.ChannelID,
	})

	endpoint := fmt.Sprintf("%s/api/v1/internal/agent/%s/channel-action",
		ctx.APIURL, ctx.AgentID)

	go func() {
		// Fire-and-forget: don't wait for response. Use a transport
		// that doesn't follow redirects and a tiny timeout just to
		// establish the connection. We don't care about the result.
		transport := &http.Transport{
			DisableKeepAlives: true,
		}
		client := &http.Client{Timeout: 1 * time.Second, Transport: transport}
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload)) // #nosec G107
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return // Connection failed — silently ignore
		}
		resp.Body.Close()
	}()
}

// injectToolDefinitions auto-generates Anthropic/OpenAI tool definitions
// from the nodes wired to the AI node's Tools handle. Each tool child
// becomes a tool definition with its name, description, and input schema
// derived from the node's configured inputs. This replaces the manual
// JSON tool_definitions input.
func (f *Flow) injectToolDefinitions(aiNode *Node, toolNodes []*Node, actions map[string]Action) {
	// Credential input names to exclude from the tool's input schema —
	// these are configured on the node, not provided by the AI.
	credentialInputs := map[string]bool{
		"api_key": true, "bot_token": true, "signing_secret": true,
		"token": true, "password": true, "secret": true,
		"secret_key": true, "access_key": true,
		"agent_id": true, "channel_type": true,
		"user_token": true,
	}

	// Map Flomation connection types to JSON Schema types
	typeMap := map[string]string{
		ConnectionTypeString:      "string",
		ConnectionTypeText:        "string",
		ConnectionTypeInteger:     "integer",
		ConnectionTypeBoolean:     "boolean",
		ConnectionTypeObject:      "object",
		ConnectionTypeDateTime:    "string",
		ConnectionTypeMultiSelect: "string",
		ConnectionTypeComboBox:    "string",
		ConnectionTypeColour:      "string",
	}

	var tools []map[string]interface{}
	seenToolNames := make(map[string]bool)

	// Agent Planning M3.5 — tools forbidden in plan-task mode. The
	// AI Prompt action runs inside a plan task when the orchestrator
	// was dispatched by the Plan Task Trigger (channel_type='plan_task').
	// In that mode, the AI must NOT be able to recursively spawn or
	// cancel plans — those calls cause the orchestrator-loop runaway
	// (plan creates plan creates plan...) we saw in M3 testing. The
	// AI should complete the task via set_output or escape via
	// plan/block; it should never need plan/create or plan/cancel.
	planTaskMode := f.context != nil && f.context.ChannelType == "plan_task"
	planTaskForbidden := map[string]bool{
		"plan_create": true,
		"plan_cancel": true,
		// M4: plan/start would let a plan task transition another
		// agent's draft to active — recursion vector. The AI in
		// plan-task mode should never need this; the parent plan
		// is already active by definition.
		"plan_start": true,
		// M5: plan/revise would let a plan task mutate its parent
		// plan's task graph (add tasks, change dependencies). A
		// task progressing the parent shouldn't be authoring it.
		// Filter to keep the boundary clean.
		"plan_revise": true,
	}

	for _, toolNode := range toolNodes {
		if toolNode.Data == nil {
			continue
		}

		// Derive tool name: strip "tools/" prefix from label and sanitise
		// to match Anthropic's tool name pattern ^[a-zA-Z0-9_-]{1,128}$
		toolName := toolNode.Data.Label
		if strings.HasPrefix(toolName, "tools/") {
			toolName = strings.TrimPrefix(toolName, "tools/")
		}
		toolName = sanitiseToolName(toolName)

		// Skip duplicate tool names — Anthropic requires unique names
		if seenToolNames[toolName] {
			continue
		}

		// M3.5 plan-task tool filter (runs after dedup so the skip-
		// log shows the user-friendly sanitised name).
		if planTaskMode && planTaskForbidden[toolName] {
			log.WithFields(log.Fields{
				"tool":         toolName,
				"channel_type": f.context.ChannelType,
			}).Info("filtered tool — forbidden in plan-task mode")
			continue
		}
		seenToolNames[toolName] = true

		// Description: prefer manifest description (truncated to save tokens),
		// fall back to config.Name, then the tool name itself.
		description := toolName
		if desc, ok := getManifestDescriptions()[toolNode.Data.Label]; ok && desc != "" {
			description = desc
			// Truncate long descriptions to reduce prompt token usage.
			// The AI can infer parameter usage from the schema.
			if len(description) > 120 {
				// Cut at last space before limit to avoid mid-word truncation.
				cut := 120
				if idx := strings.LastIndex(description[:cut], " "); idx > 60 {
					cut = idx
				}
				description = description[:cut]
			}
		} else if toolNode.Data.Config.Name != nil && *toolNode.Data.Config.Name != "" {
			description = *toolNode.Data.Config.Name
		}

		// Build input_schema from the tool's inputs
		properties := make(map[string]interface{})
		var required []string
		for _, inp := range toolNode.Data.Config.Inputs {
			if credentialInputs[inp.Name] {
				continue
			}
			// Skip inputs that have a value already set (configured
			// by the flow author, not provided by the AI). This includes
			// ${...} variable references which resolve at execution time.
			if inp.Value != nil {
				if s, ok := inp.Value.(string); ok && s != "" {
					continue
				}
			}

			propType := typeMap[inp.Type]
			if propType == "" {
				propType = "string"
			}

			prop := map[string]interface{}{
				"type": propType,
			}
			if inp.Label != "" {
				prop["description"] = inp.Label
			} else if inp.Placeholder != "" {
				prop["description"] = inp.Placeholder
			}

			// Map Options to JSON Schema enum values so the AI knows
			// what valid inputs are (e.g. "events", "availability").
			if len(inp.Options) > 0 {
				var enumVals []string
				var enumDescs []string
				for _, opt := range inp.Options {
					enumVals = append(enumVals, opt.Value)
					if opt.Name != "" && opt.Name != opt.Value {
						enumDescs = append(enumDescs, fmt.Sprintf("%s (%s)", opt.Value, opt.Name))
					}
				}
				prop["enum"] = enumVals
				if len(enumDescs) > 0 {
					prop["description"] = fmt.Sprintf("%v. Options: %s",
						prop["description"], strings.Join(enumDescs, ", "))
				}
			}

			properties[inp.Name] = prop
			if inp.Required {
				required = append(required, inp.Name)
			}
		}

		tool := map[string]interface{}{
			"name":        toolName,
			"description": description,
			"input_schema": map[string]interface{}{
				"type":       "object",
				"properties": properties,
			},
		}
		if len(required) > 0 {
			tool["input_schema"].(map[string]interface{})["required"] = required
		}

		tools = append(tools, tool)
	}

	if len(tools) == 0 {
		return
	}

	// Inject as the tool_definitions input, overwriting any manual JSON
	toolJSON, err := json.Marshal(tools)
	if err != nil {
		return
	}

	// Find or create the tool_definitions input on the AI node
	found := false
	for _, inp := range aiNode.Data.Config.Inputs {
		if inp.Name == "tool_definitions" {
			inp.Value = string(toolJSON)
			found = true
			break
		}
	}
	if !found {
		aiNode.Data.Config.Inputs = append(aiNode.Data.Config.Inputs, &Connection{
			Name:  "tool_definitions",
			Type:  ConnectionTypeText,
			Value: string(toolJSON),
		})
	}

	log.WithFields(log.Fields{
		"ai_node":    aiNode.ID,
		"tool_count": len(tools),
		"tools_json": string(toolJSON),
	}).Info("auto-generated tool definitions from graph")
}

// sanitiseToolName replaces characters that don't match the Anthropic/OpenAI
// tool name pattern ^[a-zA-Z0-9_-]{1,128}$ with underscores, and truncates
// to 128 characters. Spaces become underscores, consecutive underscores
// are collapsed, and leading/trailing underscores are trimmed.
func sanitiseToolName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	// Collapse consecutive underscores
	result := b.String()
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	result = strings.Trim(result, "_")
	if len(result) > 128 {
		result = result[:128]
	}
	if result == "" {
		result = "tool"
	}
	return result
}

// collectToolResult walks the cached results of nodes reachable from the
// tools handle of the given parent node, looking for a "tool_result",
// "result", or "response" output key. Returns the first non-empty value
// found, or empty string if none. This is how the engine extracts the
// tool action's output after executing the tools subgraph.
func (f *Flow) collectToolResult(parentID string) string {
	// Walk all cached results looking for tool output keys
	toolResultKeys := []string{"tool_result", "result", "response", "output"}
	for _, results := range f.nodeResults {
		if results == nil {
			continue
		}
		for _, key := range toolResultKeys {
			if v, ok := results[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
				// Non-string: marshal to JSON
				if v != nil {
					if b, err := json.Marshal(v); err == nil && string(b) != "null" {
						return string(b)
					}
				}
			}
		}
	}
	return ""
}

// secretPattern matches ${secrets.X} and ${secret.X} variable references.
// isEmpty checks if a value is considered empty for parent merge purposes.
func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	return false
}

var secretPattern = regexp.MustCompile(`\$\{secrets?\.`)

// executeSubFlow finds a Begin Sub-Flow node by name, executes its chain,
// and returns the outputs from the End Sub-Flow node (or the last node).
func (f *Flow) executeSubFlow(actions map[string]Action, name string, invokeOutputs map[string]interface{}, environment *environment.Environment) (map[string]interface{}, error) {
	var beginNode *Node
	for _, n := range f.Nodes {
		if n == nil || n.Data == nil {
			continue
		}
		if n.Data.Label != "subflow/begin" && n.Type != "subflow/begin" {
			continue
		}
		for _, inp := range n.Data.Config.Inputs {
			if inp.Name == "name" {
				if s, ok := inp.Value.(string); ok && s == name {
					beginNode = n
				}
				break
			}
		}
		if beginNode != nil {
			break
		}
	}

	if beginNode == nil {
		return nil, fmt.Errorf("sub-flow '%s' not found — add a Begin Sub-Flow node with this name", name)
	}

	// Recursion guard.
	depthKey := "subflow.depth." + name
	depth := int64(0)
	if v, ok := f.GetVariable(depthKey); ok {
		if d, ok := v.(int64); ok {
			depth = d
		}
	}
	if depth >= MaxSubFlowDepth {
		return nil, fmt.Errorf("sub-flow '%s' exceeded maximum recursion depth (%d)", name, MaxSubFlowDepth)
	}
	f.SetVariable(depthKey, depth+1)
	defer f.SetVariable(depthKey, depth)

	// Inject invoke parameters into the Begin node's inputs.
	// First update any existing inputs that match, then add new dynamic
	// inputs for invoke outputs not already defined on the Begin node.
	seen := make(map[string]bool)
	for _, inp := range beginNode.Data.Config.Inputs {
		seen[inp.Name] = true
		if inp.Name == "name" {
			continue
		}
		if v, exists := invokeOutputs[inp.Name]; exists {
			inp.Value = v
		}
	}
	for k, v := range invokeOutputs {
		if seen[k] {
			continue
		}
		beginNode.Data.Config.Inputs = append(beginNode.Data.Config.Inputs, &Connection{
			Name:  k,
			Type:  ConnectionTypeString,
			Value: v,
		})
	}

	// Mark all sub-flow nodes as reachable so parent resolution
	// doesn't skip them (the reachability BFS from the entry trigger
	// never reaches sub-flow nodes since they're dispatched programmatically).
	if f.reachableNodes != nil {
		f.reachableNodes[beginNode.ID] = true
		queue := []*Node{beginNode}
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			for _, child := range f.FindTarget(curr.ID) {
				if !f.reachableNodes[child.ID] {
					f.reachableNodes[child.ID] = true
					queue = append(queue, child)
				}
			}
		}
	}

	// Clear cached results for the sub-flow subgraph.
	delete(f.nodeResults, beginNode.ID)
	delete(f.nodeExecutionResults, beginNode.ID)
	f.clearSubgraphResults(beginNode.ID, "")

	log.WithFields(log.Fields{
		"name":  name,
		"begin": beginNode.ID,
		"depth": depth + 1,
	}).Info("executing sub-flow")

	if _, err := f.ExecuteNode(actions, beginNode, environment); err != nil {
		return nil, fmt.Errorf("sub-flow '%s' failed: %w", name, err)
	}

	// Collect results: prefer End Sub-Flow node outputs if one was reached.
	for _, n := range f.Nodes {
		if n == nil || n.Data == nil {
			continue
		}
		if n.Data.Label != "subflow/end" && n.Type != "subflow/end" {
			continue
		}
		if result, exists := f.nodeResults[n.ID]; exists {
			return result, nil
		}
	}

	// No End node — return the Begin node's outputs as fallback.
	if result, exists := f.nodeResults[beginNode.ID]; exists {
		return result, nil
	}

	return map[string]interface{}{}, nil
}

// isOnUnmatchedBranch checks whether a node sits on an unmatched
// Switch or Conditional branch. It walks up through the node's ancestors
// looking for edges from Switch/Conditional nodes whose source handle
// doesn't match the runtime-selected case.
func (f *Flow) isOnUnmatchedBranch(actions map[string]Action, nodeID string, environment *environment.Environment) bool {
	return f.checkUnmatchedBranch(actions, nodeID, make(map[string]bool), environment)
}

func (f *Flow) checkUnmatchedBranch(actions map[string]Action, nodeID string, visited map[string]bool, environment *environment.Environment) bool {
	if visited[nodeID] {
		return false
	}
	visited[nodeID] = true

	// Group edges by source node to handle cases where a node has
	// multiple edges from the same Switch (e.g. both case_1 and default).
	// The node is only on an unmatched branch if ALL edges from a given
	// Switch/Conditional source are unmatched.
	type edgeInfo struct {
		handle     string
		sourceNode *Node
		sourceType int64
	}
	edgesBySource := make(map[string][]edgeInfo)

	for _, e := range f.Edges {
		if e == nil || e.Target != nodeID {
			continue
		}
		sourceNode := f.FindNode(e.Source)
		if sourceNode == nil || sourceNode.Data == nil {
			continue
		}
		edgesBySource[e.Source] = append(edgesBySource[e.Source], edgeInfo{
			handle:     e.SourceHandle,
			sourceNode: sourceNode,
			sourceType: sourceNode.Data.Config.Type,
		})
	}

	// A node is only on an unmatched branch if ALL of its source paths
	// are unmatched. If any single source provides a valid (matched) path,
	// the node is reachable and should not be skipped.
	//
	// Example: AI node has edges from Switch/case_1 (matched) AND from
	// data_rename (on unmatched case_0 path). The AI is reachable via
	// case_1, so it must NOT be marked as unmatched.
	if len(edgesBySource) == 0 {
		return false
	}

	allSourcesUnmatched := true
	for _, edges := range edgesBySource {
		if len(edges) == 0 {
			continue
		}
		sourceNode := edges[0].sourceNode
		sourceType := edges[0].sourceType

		thisSourceUnmatched := false

		// The branch decision lives in the gating node's cached result. When a
		// node is reached via a PARALLEL path before its gating conditional has
		// run (e.g. C wired below both a matched sibling and an as-yet-unrun
		// If → A chain), that result is missing — and without it we would wrongly
		// treat the unmatched parent as matched and execute it. Evaluate the
		// gating node's action on demand to obtain the routing decision. This is
		// safe: routing nodes are pure evaluations, executeNodeActionOnly caches
		// the result (so the later forward traversal reuses it rather than
		// re-running), and it resolves only the gating node's own inputs — never
		// its downstream children.
		isRoutingNode := sourceType == ActionTypeConditional ||
			sourceType == ActionTypeSwitch || sourceType == ActionTypeAwait
		if isRoutingNode {
			if _, ok := f.nodeResults[sourceNode.ID]; !ok && sourceNode.ID != nodeID && actions != nil {
				if _, err := f.executeNodeActionOnly(actions, sourceNode, environment); err != nil {
					log.WithFields(log.Fields{
						"gating_node": sourceNode.ID,
						"for_node":    nodeID,
						"error":       err,
					}).Debug("could not evaluate gating node on demand for branch check")
				}
			}
		}

		if sourceType == ActionTypeConditional {
			if cached, ok := f.nodeResults[sourceNode.ID]; ok {
				result, _ := cached["result"].(bool)
				allEdgesUnmatched := true
				for _, ei := range edges {
					if (result && ei.handle == "true-branch") || (!result && ei.handle == "false-branch") || ei.handle == "" {
						allEdgesUnmatched = false
						break
					}
				}
				if allEdgesUnmatched {
					thisSourceUnmatched = true
				}
			}
		}

		if sourceType == ActionTypeSwitch {
			if cached, ok := f.nodeResults[sourceNode.ID]; ok {
				matchedCase, _ := cached["matched_case"].(string)
				if matchedCase != "" {
					allEdgesUnmatched := true
					for _, ei := range edges {
						if ei.handle == "" || ei.handle == matchedCase || ei.handle == "default" {
							allEdgesUnmatched = false
							break
						}
					}
					if allEdgesUnmatched {
						thisSourceUnmatched = true
					}
				}
			}
		}

		if sourceType == ActionTypeAwait {
			// Human-in-the-Loop routes to the chosen "option_<value>" handle or
			// "timeout" (emitted under matched_case, like Switch). Unlike Switch
			// there is no "default" handle, so it is not exempted here.
			if cached, ok := f.nodeResults[sourceNode.ID]; ok {
				matchedCase, _ := cached["matched_case"].(string)
				if matchedCase != "" {
					allEdgesUnmatched := true
					for _, ei := range edges {
						if ei.handle == "" || ei.handle == matchedCase {
							allEdgesUnmatched = false
							break
						}
					}
					if allEdgesUnmatched {
						thisSourceUnmatched = true
					}
				}
			}
		}

		// Recurse up through all parent types — a node is on an unmatched
		// branch if its ancestor chain leads through an unmatched branch,
		// even through intermediate regular actions (e.g. STT between a
		// Switch and data_rename).
		if !thisSourceUnmatched && f.checkUnmatchedBranch(actions, sourceNode.ID, visited, environment) {
			thisSourceUnmatched = true
		}

		if !thisSourceUnmatched {
			// At least one source is on a valid (matched) path — the node
			// is reachable. No need to check remaining sources.
			allSourcesUnmatched = false
			break
		}
	}

	return allSourcesUnmatched
}

// platformTagPattern matches [UPPERCASE_TAG:...] patterns that the AI may
// emit as platform instructions. These must be stripped before delivery.
var platformTagPattern = regexp.MustCompile(`\[(?:LINK_OFFER|LINK_CONFIRM|LINK_[A-Z_]+)[^\]]*\]`)

// stripPlatformTags removes platform-internal tags like [LINK_OFFER:channel:id]
// from AI responses. These tags are instructions from the system prompt that
// must not be delivered to the end user.
func stripPlatformTags(s string) string {
	return strings.TrimSpace(platformTagPattern.ReplaceAllString(s, ""))
}

// emitNodeEvent writes a __NODE__: prefixed JSON line to stdout for the
// runner and API SSE infrastructure to consume. When inputs/outputs are
// provided (on completion), they are included so the editor can show them
// in real time without waiting for the full execution to finish.
func (f *Flow) emitNodeEvent(id, action, label, status string, durationMs int64, errMsg string, inputs ...map[string]interface{}) {
	evt := map[string]interface{}{
		"id":     id,
		"action": action,
		"label":  label,
		"status": status,
	}
	if durationMs > 0 {
		evt["duration_ms"] = durationMs
	}
	if errMsg != "" {
		evt["error"] = errMsg
	}
	// inputs[0] = resolved inputs, inputs[1] = outputs (variadic to keep "running" calls simple)
	// Large string values (e.g. base64 audio from elevenlabs/text_to_speech) are
	// truncated for the streamed event only — the full, untruncated data is
	// preserved separately via recordNodeResult/nodeResults for downstream nodes.
	// Without this a single __NODE__ line can be tens of MB, exceeding the
	// runner's stdout scanner buffer; the runner then stops draining the pipe
	// and the executor blocks forever on the write, hanging the execution.
	if len(inputs) > 0 && inputs[0] != nil {
		evt["inputs"] = truncateEventValues(inputs[0])
	}
	if len(inputs) > 1 && inputs[1] != nil {
		evt["outputs"] = truncateEventValues(inputs[1])
	}
	b, _ := json.Marshal(evt)
	fmt.Fprintf(os.Stdout, "__NODE__:%s\n", b)
}

// maxEventStringBytes caps the length of any single string value emitted in a
// streamed __NODE__ event. 4KB is comfortably below the runner's stdout
// scanner buffer while still carrying enough of a value to be useful in the
// live editor view.
const maxEventStringBytes = 4096

// truncateEventValues returns a shallow copy of m in which any string value
// longer than maxEventStringBytes is truncated (on a valid UTF-8 boundary)
// with a marker noting the original size. The input map is never mutated, so
// callers' downstream copies of the data are unaffected. Non-string values are
// passed through unchanged.
func truncateEventValues(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok && len(s) > maxEventStringBytes {
			cut := strings.ToValidUTF8(s[:maxEventStringBytes], "")
			out[k] = fmt.Sprintf("%s… [truncated, %d bytes total]", cut, len(s))
			continue
		}
		out[k] = v
	}
	return out
}

// buildObfuscatedInputMap creates an input map with secret values masked.
func (f *Flow) buildObfuscatedInputMap(node *Node, resolvedInputs []*Connection) map[string]interface{} {
	inputMap := make(map[string]interface{})
	for _, c := range resolvedInputs {
		val := c.Value
		if node.Data != nil {
			for _, orig := range node.Data.Config.Inputs {
				if orig.Name == c.Name {
					origStr := fmt.Sprintf("%v", orig.Value)
					if secretPattern.MatchString(origStr) {
						val = "********"
					}
					break
				}
			}
		}
		inputMap[c.Name] = val
	}
	return inputMap
}

// recordNodeResult stores an ExecutionNodeResult for the given node.
func (f *Flow) recordNodeResult(node *Node, status string, resolvedInputs []*Connection, outputs map[string]interface{}, durationMs int64, errMsg string) {
	if f.nodeExecutionResults == nil {
		f.nodeExecutionResults = make(map[string]*ExecutionNodeResult)
	}

	inputMap := f.buildObfuscatedInputMap(node, resolvedInputs)

	nr := &ExecutionNodeResult{
		ID:       node.ID,
		Action:   node.Type,
		Label:    node.Data.Label,
		Status:   status,
		Inputs:   inputMap,
		Outputs:  outputs,
		Error:    errMsg,
		Duration: durationMs,
	}

	f.nodeExecutionResults[node.ID] = nr
}
