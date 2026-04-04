package switchaction

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestSwitch_ExactMatch_FirstCase(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "switch-1"}

	inputs := []*core.Connection{
		{Name: "value", Type: core.ConnectionTypeString, Value: "error"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "equals"},
		{Name: "cases", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{
			map[string]interface{}{"key": "Success", "value": "success"},
			map[string]interface{}{"key": "Error", "value": "error"},
			map[string]interface{}{"key": "Warning", "value": "warning"},
		}},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["matched_case"]).To(Equal("case_1"))
	Expect(result["matched"]).To(Equal(true))
	Expect(result["value"]).To(Equal("error"))
}

func TestSwitch_NoMatch_ReturnsDefault(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "switch-1"}

	inputs := []*core.Connection{
		{Name: "value", Type: core.ConnectionTypeString, Value: "unknown"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "equals"},
		{Name: "cases", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{
			map[string]interface{}{"key": "Success", "value": "success"},
			map[string]interface{}{"key": "Error", "value": "error"},
		}},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["matched_case"]).To(Equal("default"))
	Expect(result["matched"]).To(Equal(false))
}

func TestSwitch_ContainsOperator(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "switch-1"}

	inputs := []*core.Connection{
		{Name: "value", Type: core.ConnectionTypeString, Value: "Hello World"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "contains"},
		{Name: "cases", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{
			map[string]interface{}{"key": "Greeting", "value": "hello"},
			map[string]interface{}{"key": "Farewell", "value": "goodbye"},
		}},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["matched_case"]).To(Equal("case_0"))
	Expect(result["matched"]).To(Equal(true))
}

func TestSwitch_RegexOperator(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "switch-1"}

	inputs := []*core.Connection{
		{Name: "value", Type: core.ConnectionTypeString, Value: "order-12345"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "regex"},
		{Name: "cases", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{
			map[string]interface{}{"key": "Invoice", "value": "^invoice-"},
			map[string]interface{}{"key": "Order", "value": "^order-\\d+$"},
		}},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["matched_case"]).To(Equal("case_1"))
}

func TestSwitch_StartsWithOperator(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "switch-1"}

	inputs := []*core.Connection{
		{Name: "value", Type: core.ConnectionTypeString, Value: "/api/v1/users"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "starts_with"},
		{Name: "cases", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{
			map[string]interface{}{"key": "API", "value": "/api"},
			map[string]interface{}{"key": "Web", "value": "/web"},
		}},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["matched_case"]).To(Equal("case_0"))
}

func TestSwitch_CaseInsensitiveEquals(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "switch-1"}

	inputs := []*core.Connection{
		{Name: "value", Type: core.ConnectionTypeString, Value: "ERROR"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "equals"},
		{Name: "cases", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{
			map[string]interface{}{"key": "Error", "value": "error"},
		}},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["matched_case"]).To(Equal("case_0"))
	Expect(result["matched"]).To(Equal(true))
}

func TestSwitch_EmptyCases_ReturnsDefault(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "switch-1"}

	inputs := []*core.Connection{
		{Name: "value", Type: core.ConnectionTypeString, Value: "test"},
		{Name: "cases", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{}},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["matched_case"]).To(Equal("default"))
	Expect(result["matched"]).To(Equal(false))
}

func TestSwitch_FirstMatchWins(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "switch-1"}

	inputs := []*core.Connection{
		{Name: "value", Type: core.ConnectionTypeString, Value: "hello world"},
		{Name: "operator", Type: core.ConnectionTypeString, Value: "contains"},
		{Name: "cases", Type: core.ConnectionTypeKeyValueArray, Value: []interface{}{
			map[string]interface{}{"key": "First", "value": "hello"},
			map[string]interface{}{"key": "Second", "value": "world"},
			map[string]interface{}{"key": "Both", "value": "hello world"},
		}},
	}

	result, err := Execute(flow, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["matched_case"]).To(Equal("case_0")) // First match wins
}
