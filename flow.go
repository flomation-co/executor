package core

import (
	gocontext "context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"flomation.app/automate/executor/internal/environment"
	log "github.com/sirupsen/logrus"
)

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
	inErrorChain         bool
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
}

// InjectTriggerData merges trigger invocation data into the first trigger
// node's inputs, making dynamic event data available to the flow.
func (f *Flow) InjectTriggerData(data map[string]interface{}) {
	// If channel_type is present, try to match the specific trigger node first
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

		// Match channel type to trigger label (e.g. "slack" matches "trigger/slack")
		if channelType != "" {
			label := n.Data.Label
			if label == "trigger/"+channelType {
				targetNode = n
				break
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
			"id": node.ID,
		}).Debug("Node already executed, returning cached result")
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

		results, err = f.ExecuteNode(actions, p, environment)
		if err != nil {
			return nil, err
		}

		for k, v := range results {
			parentResults[k] = v
		}
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
					!strings.HasPrefix(name, "secret.") {
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
						if contextVal != "" {
							*val = strings.ReplaceAll(*val, "${"+m+"}", contextVal)
						} else {
							log.WithFields(log.Fields{
								"name": name,
							}).Warn("unknown flow variable")
						}
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
	} else {
		children = f.FindTarget(node.ID)
	}

	// Filter out trigger nodes from children
	var validChildren []*Node
	for _, c := range children {
		if c.Type != "" && strings.HasPrefix(c.Type, "trigger/") {
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

// secretPattern matches ${secrets.X} and ${secret.X} variable references.
var secretPattern = regexp.MustCompile(`\$\{secrets?\.`)

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
