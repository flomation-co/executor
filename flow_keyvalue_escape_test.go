package core

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
)

// A key_value_array input (Bash env vars, and the script nodes' named Inputs)
// stores each pair's value INSIDE a JSON string ("value":"${x}"). A substituted
// value must be JSON-escaped or the pairs JSON is corrupted and KeyValuePairs()
// silently drops every row — which emptied a Run-JavaScript node's `inputs`.
// These lock in the escaping (jsonCtx enabled for key_value_array).

func TestKeyValueArrayInput_JSONEscapesSubstitutedValues(t *testing.T) {
	RegisterTestingT(t)

	// `special` (from flow_json_escape_test.go) holds a quote, a newline and a
	// backslash — the characters that break a JSON string literal.
	captured := runConsumerFlow(t, ConnectionTypeKeyValueArray, `[{"key":"v","value":"${answer}"}]`)

	var pairs []KeyValuePair
	Expect(json.Unmarshal([]byte(captured), &pairs)).To(Succeed(),
		"key_value_array must remain valid JSON after substituting a value with quotes/newlines/backslashes")
	Expect(pairs).To(HaveLen(1))
	Expect(pairs[0].Key).To(Equal("v"))
	Expect(pairs[0].Value).To(Equal(special)) // round-trips to the original value
}

func TestKeyValueArrayInput_WholeArrayReferenceRoundTrips(t *testing.T) {
	RegisterTestingT(t)

	// The exact scenario that emptied the Query MRs flow: a whole ${array} wired
	// into a Run-JavaScript "Inputs" row. Data includes a quote to prove the
	// escaping is correct even for values that themselves contain quotes.
	arr := []interface{}{
		map[string]interface{}{"iid": float64(1), "title": `Draft "release"`},
	}
	var captured *Connection

	trigger := func(*Flow, *Node, []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"answer": arr}, nil
	}
	consumer := func(_ *Flow, _ *Node, inputs []*Connection) (map[string]interface{}, error) {
		captured = FindConnection("input_vars", inputs)
		return map[string]interface{}{}, nil
	}

	f := &Flow{
		Nodes: []*Node{
			{ID: "trigger-1", Type: "trigger/manual", Data: &NodeData{
				Label: "trigger/manual", Config: NodeConfig{Type: ActionTypeTrigger},
			}},
			{ID: "consumer", Type: "test/consumer", Data: &NodeData{
				Label: "test/consumer",
				Config: NodeConfig{
					Type: ActionTypeAction,
					Inputs: []*Connection{
						{Name: "input_vars", Type: ConnectionTypeKeyValueArray, Value: `[{"key":"merge_requests","value":"${answer}"}]`},
					},
				},
			}},
		},
		Edges:                []*Edge{{ID: "e1", Source: "trigger-1", Target: "consumer"}},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
		variables:            make(map[string]interface{}),
	}

	_, err := f.Execute(map[string]Action{"trigger/manual": trigger, "test/consumer": consumer}, nil, nil)
	Expect(err).To(BeNil())
	Expect(captured).NotTo(BeNil())

	// The resolved pairs parse, and the row value is the array's JSON — recoverable
	// as the typed array (what script_common.BuildScriptInputs coerces into inputs.X).
	pairs := captured.KeyValuePairs()
	Expect(pairs).To(HaveLen(1))
	Expect(pairs[0].Key).To(Equal("merge_requests"))

	var back []interface{}
	Expect(json.Unmarshal([]byte(pairs[0].Value), &back)).To(Succeed(),
		"the row value must be the array's JSON, not corrupted by unescaped substitution")
	Expect(back).To(Equal(arr))
}
