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

	// MaxToolRoundsDefault is the safety cap on tool calling rounds.
	MaxToolRoundsDefault = 25

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
)

type Action func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error)

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
}

func (c *Connection) String() *string {
	if c == nil {
		return nil
	}

	if c.Type == ConnectionTypeString || c.Type == ConnectionTypeText {
		v, ok := c.Value.(string)
		if !ok {
			return nil
		}

		return &v
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

func (c *Connection) Number() *int64 {
	if c == nil {
		return nil
	}

	if c.Type != ConnectionTypeInteger {
		return nil
	}

	v, ok := c.Value.(int64)
	if !ok {
		v, ok := c.Value.(float64)
		if !ok {
			v, ok := c.Value.(int)
			if !ok {
				v, err := strconv.ParseInt(c.Value.(string), 10, 64)
				if err != nil {
					return nil
				}

				return &v
			}

			val := int64(v)
			return &val
		}

		val := int64(v)
		return &val
	}

	return &v
}

func (c *Connection) Boolean() *bool {
	if c == nil {
		return nil
	}

	if c.Type != ConnectionTypeBoolean {
		return nil
	}

	v, ok := c.Value.(bool)
	if !ok {
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
	context              *ExecutionContext
	ctx                  gocontext.Context
	cancel               gocontext.CancelFunc
}

// ErrCancelled is returned when a flow execution is cancelled.
var ErrCancelled = errors.New("execution cancelled")

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
	"system_prompt": true,
	"__node_id":     true,
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
	var start *Node

	if entry != nil {
		start = f.FindNode(*entry)
	} else {
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
	if v, exists := f.nodeResults[node.ID]; exists {
		return v, nil
	}

	// Phase 1: execute this node's own action (resolve parents, substitute vars, run action, cache)
	outputs, err := f.executeNodeActionOnly(actions, node, environment)
	if err != nil {
		return nil, err
	}

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

	// Check for cancellation before executing
	if f.ctx != nil {
		select {
		case <-f.ctx.Done():
			f.emitNodeEvent(node.ID, node.Type, node.Data.Label, "cancelled", 0, ErrCancelled.Error())
			f.recordNodeResult(node, "cancelled", nil, nil, 0, ErrCancelled.Error())
			return nil, ErrCancelled
		default:
		}
	}

	if v, exists := f.nodeResults[node.ID]; exists {
		log.WithFields(log.Fields{
			"id":    node.ID,
			"label": node.Data.Label,
		}).Info("Node already executed, returning cached result")
		return v, nil
	}

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
		if f.isOnUnmatchedBranch(p.ID) {
			log.WithFields(log.Fields{
				"node":   node.ID,
				"parent": p.ID,
				"action": p.Data.Label,
			}).Debug("skipping parent on unmatched conditional branch")
			continue
		}

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
					// direct parent — separated by Switch/For loop).
					if dotIdx := strings.IndexByte(name, '.'); dotIdx > 0 {
						scopeNodeID := name[:dotIdx]
						scopeKey := name[dotIdx+1:]
						if nr, ok := f.nodeResults[scopeNodeID]; ok {
							if res, ok := nr[scopeKey]; ok {
								if _, isStr := res.(string); !isStr {
									configuration = append(configuration, &Connection{
										Name:  v.Name,
										Type:  v.Type,
										Value: res,
									})
									continue
								}
							}
						}
					}
				}
			}
		}

		if val != nil {
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

					*val = strings.ReplaceAll(*val, "${"+m+"}", *p.Value)
				} else if strings.HasPrefix(m, "flow.") {
					name := strings.TrimPrefix(m, "flow.")
					if f.context != nil {
						contextVal := f.context.Get(name)
						// Always substitute — empty string is a valid
						// resolved value (e.g. thread_id when there's no
						// thread). Leaving the literal ${flow.xxx} in place
						// causes downstream actions to receive it as text.
						*val = strings.ReplaceAll(*val, "${"+m+"}", contextVal)
					} else {
						log.WithFields(log.Fields{
							"name": name,
						}).Warn("no execution context for flow variable substitution")
					}
				} else if strings.HasPrefix(m, "var.") {
					name := strings.TrimPrefix(m, "var.")
					if f.variables != nil {
						if varVal, ok := f.variables[name]; ok {
							*val = strings.ReplaceAll(*val, "${"+m+"}", fmt.Sprintf("%v", varVal))
						} else {
							log.WithFields(log.Fields{
								"name": name,
							}).Warn("unknown flow variable")
						}
					}
				} else if strings.HasPrefix(m, "credentials.") {
					if environment == nil {
						log.WithFields(log.Fields{
							"name": m,
						}).Warn("No environment configured for credential substitution")
						continue
					}
					name := strings.TrimPrefix(m, "credentials.")
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
					*val = strings.ReplaceAll(*val, "${"+m+"}", *token)
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

					*val = strings.ReplaceAll(*val, "${"+m+"}", *p.Value)
				} else {
					if res, exists := parentResults[m]; exists {
						*val = strings.ReplaceAll(*val, "${"+m+"}", fmt.Sprintf("%v", res))
					} else if dotIdx := strings.IndexByte(m, '.'); dotIdx > 0 {
						// Scoped node reference: ${nodeId.key} — look up in
						// the global node results cache. This handles cases
						// where the referenced node is an ancestor but not a
						// direct parent (e.g. separated by a Switch or For loop).
						scopeNodeID := m[:dotIdx]
						scopeKey := m[dotIdx+1:]

						// If the node hasn't been executed yet (e.g. sibling
						// in a loop body), execute it now to populate results.
						if _, ok := f.nodeResults[scopeNodeID]; !ok {
							if scopeNode := f.FindNode(scopeNodeID); scopeNode != nil {
								log.WithFields(log.Fields{
									"node_id": scopeNodeID,
									"key":     scopeKey,
								}).Info("executing scoped dependency node")
								if _, err := f.ExecuteNode(actions, scopeNode, environment); err != nil {
									log.WithFields(log.Fields{
										"node_id": scopeNodeID,
										"error":   err,
									}).Warn("failed to execute scoped dependency node")
								}
							}
						}

						if nr, ok := f.nodeResults[scopeNodeID]; ok {
							if res, ok := nr[scopeKey]; ok {
								*val = strings.ReplaceAll(*val, "${"+m+"}", fmt.Sprintf("%v", res))
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
					} else {
						log.WithFields(log.Fields{
							"output": m,
						}).Warn("substitution upstream output does not exist")
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
		node.Data.Config.Type == ActionTypeLoop) {
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
	} else if node.Data.Config.Type == ActionTypeLoop {
		// Loop execution: iterate body children, then execute output children
		loopChildren := f.FindTargetByHandle(node.ID, "loop")
		outputChildren := f.FindTargetByHandle(node.ID, "output")

		action, exists := actions[node.Type]
		if !exists {
			action, exists = actions[node.Data.Label]
		}

		var configuration []*Connection
		for _, v := range node.Data.Config.Inputs {
			configuration = append(configuration, &Connection{
				Name:  v.Name,
				Type:  v.Type,
				Value: v.Value,
			})
		}

		maxIter := int64(1000)
		if mi, ok := outputs["max_iterations"].(int64); ok && mi > 0 {
			maxIter = mi
		}

		var iteration int64
		for iteration = 0; iteration < maxIter; iteration++ {
			// Re-evaluate the loop condition by re-executing the loop node's action
			// On first iteration, use the already-computed outputs
			if iteration > 0 {
				// Clear the loop node's own cached result so it re-evaluates
				delete(f.nodeResults, node.ID)
				// Also clear parent results that might feed dynamic values
				f.clearSubgraphResults(node.ID, "loop")

				// Re-resolve inputs and re-execute the loop action
				reOutputs, err := action(f, node, configuration)
				if err != nil {
					return nil, err
				}
				outputs = reOutputs
				f.nodeResults[node.ID] = outputs
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
					// Store final iteration count before returning error
					outputs["iterations"] = iteration + 1
					f.nodeResults[node.ID] = outputs
					return nil, err
				}
			}

			// Clear loop body results for next iteration
			f.clearSubgraphResults(node.ID, "loop")
		}

		// Store final iteration count
		outputs["iterations"] = iteration
		f.nodeResults[node.ID] = outputs

		// Execute output-handle children (post-loop)
		children = outputChildren
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
					// The AI emitted text alongside tool calls. Fire the
					// Response handle subgraph so the user sees the message
					// while tools execute. Treated like a loop iteration:
					// clear all cached results in the subgraph, set the
					// response output, execute fully, then clear again so
					// the final response can re-execute the same subgraph.
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
						f.SetVariable("tool.input."+k, fmt.Sprintf("%v", v))
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

						_, err := f.ExecuteNode(actions, matchedTool, environment)
						if err != nil {
							toolOutput = fmt.Sprintf("Tool execution error: %v", err)
							toolErr = true
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
	} else if node.Data != nil && (node.Data.Label == "subflow/invoke" || node.Type == "subflow/invoke") {
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
	} else {
		children = f.FindTarget(node.ID)
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
		validChildren = append(validChildren, c)
	}

	// Breadth-first: execute all siblings' actions before traversing any subtrees.
	// Pass 1: run each sibling's action only (no child traversal)
	for _, c := range validChildren {
		outputs, err := f.executeNodeActionOnly(actions, c, environment)
		if err != nil {
			return nil, err
		}
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
	reachable := map[string]bool{startID: true}
	queue := []string{startID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, e := range f.Edges {
			if e == nil {
				continue
			}
			if e.Source == current && !reachable[e.Target] {
				reachable[e.Target] = true
				queue = append(queue, e.Target)
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
		ConnectionTypeString:  "string",
		ConnectionTypeText:    "string",
		ConnectionTypeInteger: "integer",
		ConnectionTypeBoolean: "boolean",
		ConnectionTypeObject:  "object",
	}

	var tools []map[string]interface{}
	seenToolNames := make(map[string]bool)

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
func (f *Flow) isOnUnmatchedBranch(nodeID string) bool {
	return f.checkUnmatchedBranch(nodeID, make(map[string]bool))
}

func (f *Flow) checkUnmatchedBranch(nodeID string, visited map[string]bool) bool {
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

		// Recurse up through all parent types — a node is on an unmatched
		// branch if its ancestor chain leads through an unmatched branch,
		// even through intermediate regular actions (e.g. STT between a
		// Switch and data_rename).
		if !thisSourceUnmatched && f.checkUnmatchedBranch(sourceNode.ID, visited) {
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
	if len(inputs) > 0 && inputs[0] != nil {
		evt["inputs"] = inputs[0]
	}
	if len(inputs) > 1 && inputs[1] != nil {
		evt["outputs"] = inputs[1]
	}
	b, _ := json.Marshal(evt)
	fmt.Fprintf(os.Stdout, "__NODE__:%s\n", b)
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
