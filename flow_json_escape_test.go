package core

import (
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// A "rows" input is a hand-authored JSON 2D array like
// [["${name}"]] that downstream actions (google/sheets/append,
// microsoft/excel/append_rows) json.Unmarshal. If a substituted variable
// value contains a double quote, backslash or newline — routine in
// free-text form answers, or when a user pastes a JSON blob — the naive
// string replacement used to break the surrounding JSON and the action
// failed to parse its input. Rows inputs now JSON-escape substituted
// values; other input types keep verbatim substitution.

// special contains the three characters that break a JSON string literal.
const special = `He said "hi"` + "\n" + `path\to\file`

// runConsumerFlow executes a manual-trigger → consumer flow where the
// consumer's single "data" input carries the given type and template. The
// firing trigger outputs {"answer": special}, so a ${answer} reference in
// the template resolves to a value containing quotes/newlines/backslashes.
// Returns the raw "data" string the consumer received.
func runConsumerFlow(t *testing.T, inputType, template string) string {
	t.Helper()
	var captured string

	triggerAction := func(*Flow, *Node, []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{"answer": special}, nil
	}
	consumerAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		for _, c := range inputs {
			if c.Name == "data" {
				if s := c.String(); s != nil {
					captured = *s
				}
			}
		}
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
						{Name: "data", Type: inputType, Value: template},
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

	_, err := f.Execute(map[string]Action{
		"trigger/manual": triggerAction,
		"test/consumer":  consumerAction,
	}, nil, nil)
	Expect(err).To(BeNil())
	return captured
}

func TestRowsInput_JSONEscapesSubstitutedValues(t *testing.T) {
	RegisterTestingT(t)

	captured := runConsumerFlow(t, ConnectionTypeRows, `[["${answer}"]]`)

	// The substituted rows string must still be valid JSON…
	var rows [][]string
	Expect(json.Unmarshal([]byte(captured), &rows)).To(Succeed(),
		"rows input must remain valid JSON after substituting a value with quotes/newlines/backslashes")
	// …and round-trip back to the original value.
	Expect(rows).To(HaveLen(1))
	Expect(rows[0]).To(HaveLen(1))
	Expect(rows[0][0]).To(Equal(special))
}

func TestStringInput_DoesNotEscape(t *testing.T) {
	RegisterTestingT(t)

	// A plain string input keeps verbatim substitution — no JSON escaping —
	// so the raw quote survives unescaped (correct: this isn't a JSON
	// container).
	captured := runConsumerFlow(t, ConnectionTypeString, `answer: ${answer}`)

	Expect(captured).To(ContainSubstring(`He said "hi"`))
	Expect(captured).To(ContainSubstring("\n"))
	Expect(strings.Contains(captured, `\"`)).To(BeFalse())
}
