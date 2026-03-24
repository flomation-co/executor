package core

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestNodeResultsPopulatedForEachNode(t *testing.T) {
	RegisterTestingT(t)

	echoAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for _, c := range inputs {
			if c.String() != nil {
				result[c.Name] = *c.String()
			}
		}
		return result, nil
	}

	actions := map[string]Action{
		"trigger/manual": echoAction,
		"action/echo":    echoAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/manual",
				Data: &NodeData{
					Label: "trigger/manual",
					Config: NodeConfig{
						ID:   "trigger-1",
						Type: ActionTypeTrigger,
					},
				},
			},
			{
				ID:   "action-1",
				Type: "action/echo",
				Data: &NodeData{
					Label: "action/echo",
					Config: NodeConfig{
						ID:   "action-1",
						Type: ActionTypeAction,
						Inputs: []*Connection{
							{Name: "message", Type: ConnectionTypeString, Value: "hello"},
						},
					},
				},
			},
			{
				ID:   "action-2",
				Type: "action/echo",
				Data: &NodeData{
					Label: "action/echo",
					Config: NodeConfig{
						ID:   "action-2",
						Type: ActionTypeAction,
						Inputs: []*Connection{
							{Name: "greeting", Type: ConnectionTypeString, Value: "world"},
						},
					},
				},
			},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "action-1"},
			{ID: "e2", Source: "action-1", Target: "action-2"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
	}

	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())

	results := f.GetNodeExecutionResults()
	Expect(results).To(Not(BeNil()))
	Expect(results).To(HaveLen(3))

	// Check trigger node result
	Expect(results["trigger-1"]).To(Not(BeNil()))
	Expect(results["trigger-1"].Status).To(Equal("success"))
	Expect(results["trigger-1"].Action).To(Equal("trigger/manual"))

	// Check action-1 result
	Expect(results["action-1"]).To(Not(BeNil()))
	Expect(results["action-1"].Status).To(Equal("success"))
	Expect(results["action-1"].Inputs["message"]).To(Equal("hello"))
	Expect(results["action-1"].Outputs["message"]).To(Equal("hello"))

	// Check action-2 result
	Expect(results["action-2"]).To(Not(BeNil()))
	Expect(results["action-2"].Status).To(Equal("success"))
	Expect(results["action-2"].Inputs["greeting"]).To(Equal("world"))
}

func TestNodeResultsFailedNodeHasErrorStatus(t *testing.T) {
	RegisterTestingT(t)

	echoAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}

	failAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return nil, fmt.Errorf("something went wrong")
	}

	actions := map[string]Action{
		"trigger/manual": echoAction,
		"action/fail":    failAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/manual",
				Data: &NodeData{
					Label: "trigger/manual",
					Config: NodeConfig{
						ID:   "trigger-1",
						Type: ActionTypeTrigger,
					},
				},
			},
			{
				ID:   "action-1",
				Type: "action/fail",
				Data: &NodeData{
					Label: "action/fail",
					Config: NodeConfig{
						ID:   "action-1",
						Type: ActionTypeAction,
					},
				},
			},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "action-1"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
	}

	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(Not(BeNil()))

	results := f.GetNodeExecutionResults()
	Expect(results["action-1"]).To(Not(BeNil()))
	Expect(results["action-1"].Status).To(Equal("failed"))
	Expect(results["action-1"].Error).To(Equal("something went wrong"))
}

func TestNodeResultsSecretInputsObfuscated(t *testing.T) {
	RegisterTestingT(t)

	echoAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		result := make(map[string]interface{})
		for _, c := range inputs {
			if c.String() != nil {
				result[c.Name] = *c.String()
			}
		}
		return result, nil
	}

	actions := map[string]Action{
		"trigger/manual": echoAction,
		"action/echo":    echoAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/manual",
				Data: &NodeData{
					Label: "trigger/manual",
					Config: NodeConfig{
						ID:   "trigger-1",
						Type: ActionTypeTrigger,
					},
				},
			},
			{
				ID:   "action-1",
				Type: "action/echo",
				Data: &NodeData{
					Label: "action/echo",
					Config: NodeConfig{
						ID:   "action-1",
						Type: ActionTypeAction,
						Inputs: []*Connection{
							{Name: "api_key", Type: ConnectionTypeString, Value: "${secrets.MY_API_KEY}"},
							{Name: "name", Type: ConnectionTypeString, Value: "visible-value"},
							{Name: "token", Type: ConnectionTypeString, Value: "${secret.TOKEN}"},
						},
					},
				},
			},
		},
		Edges: []*Edge{
			{ID: "e1", Source: "trigger-1", Target: "action-1"},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
	}

	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())

	results := f.GetNodeExecutionResults()
	nr := results["action-1"]
	Expect(nr).To(Not(BeNil()))

	// Secret inputs should be obfuscated
	Expect(nr.Inputs["api_key"]).To(Equal("********"))
	Expect(nr.Inputs["token"]).To(Equal("********"))

	// Non-secret inputs should be visible
	Expect(nr.Inputs["name"]).To(Equal("visible-value"))
}

func TestNodeResultsDurationTracked(t *testing.T) {
	RegisterTestingT(t)

	echoAction := func(flow *Flow, node *Node, inputs []*Connection) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}

	actions := map[string]Action{
		"trigger/manual": echoAction,
	}

	f := &Flow{
		Nodes: []*Node{
			{
				ID:   "trigger-1",
				Type: "trigger/manual",
				Data: &NodeData{
					Label: "trigger/manual",
					Config: NodeConfig{
						ID:   "trigger-1",
						Type: ActionTypeTrigger,
					},
				},
			},
		},
		nodeResults:          make(map[string]map[string]interface{}),
		nodeExecutionResults: make(map[string]*ExecutionNodeResult),
		outputs:              make(map[string]interface{}),
	}

	entry := "trigger-1"
	_, err := f.Execute(actions, &entry, nil)
	Expect(err).To(BeNil())

	results := f.GetNodeExecutionResults()
	Expect(results["trigger-1"]).To(Not(BeNil()))
	Expect(results["trigger-1"].Duration).To(BeNumerically(">=", 0))
}
