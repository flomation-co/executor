package nosql_mongodb

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func makeInputs(vals map[string]interface{}) []*core.Connection {
	var inputs []*core.Connection
	for _, inp := range Inputs {
		c := &core.Connection{
			Name: inp.Name,
			Type: inp.Type,
		}
		if v, ok := vals[inp.Name]; ok {
			c.Value = v
		}
		inputs = append(inputs, c)
	}
	return inputs
}

func TestMetadata(t *testing.T) {
	RegisterTestingT(t)

	Expect(Name).To(Equal("MongoDB Query"))
	Expect(Type).To(Equal(core.ActionTypeAction))
	Expect(Author).To(Equal("Andy Esser"))
	Expect(Icon).To(Equal("leaf"))
}

func TestInputsConfiguration(t *testing.T) {
	RegisterTestingT(t)

	Expect(len(Inputs)).To(Equal(6))

	uriInput := Inputs[0]
	Expect(uriInput.Name).To(Equal("connection_uri"))
	Expect(uriInput.Required).To(BeTrue())

	dbInput := Inputs[1]
	Expect(dbInput.Name).To(Equal("database"))
	Expect(dbInput.Required).To(BeTrue())

	collInput := Inputs[2]
	Expect(collInput.Name).To(Equal("collection"))
	Expect(collInput.Required).To(BeTrue())

	opInput := Inputs[3]
	Expect(opInput.Name).To(Equal("operation"))
	Expect(opInput.Required).To(BeTrue())
	Expect(len(opInput.Options)).To(Equal(6))
}

func TestOutputsConfiguration(t *testing.T) {
	RegisterTestingT(t)

	Expect(len(Outputs)).To(Equal(3))
	Expect(Outputs[1].Name).To(Equal("results"))
	Expect(Outputs[1].Type).To(Equal(core.ConnectionTypeObject))
	Expect(Outputs[2].Name).To(Equal("count"))
	Expect(Outputs[2].Type).To(Equal(core.ConnectionTypeInteger))
}

func TestConnectionFailure(t *testing.T) {
	RegisterTestingT(t)

	node := &core.Node{ID: "mongo-1", Type: "nosql/mongodb", Data: &core.NodeData{ID: "mongo-1"}}
	flow := &core.Flow{Nodes: []*core.Node{node}}

	// Use an invalid URI to ensure connection fails quickly
	inputs := makeInputs(map[string]interface{}{
		"connection_uri": "mongodb://localhost:99999/?connectTimeoutMS=500&serverSelectionTimeoutMS=500",
		"database":       "testdb",
		"collection":     "testcoll",
		"operation":      "find",
	})

	_, err := Execute(flow, node, inputs)
	Expect(err).ToNot(BeNil())
}

func TestParseJSON(t *testing.T) {
	RegisterTestingT(t)

	doc, err := parseJSON(`{"name": "test", "value": 42}`)
	Expect(err).To(BeNil())
	Expect(doc["name"]).To(Equal("test"))
	Expect(doc["value"]).To(BeNumerically("==", 42))
}

func TestParseJSONInvalid(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseJSON(`{invalid json}`)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("invalid JSON"))
}

func TestParseJSONArray(t *testing.T) {
	RegisterTestingT(t)

	docs, err := parseJSONArray(`[{"a": 1}, {"b": 2}]`)
	Expect(err).To(BeNil())
	Expect(len(docs)).To(Equal(2))
}

func TestParseJSONArrayInvalid(t *testing.T) {
	RegisterTestingT(t)

	_, err := parseJSONArray(`not an array`)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("invalid JSON array"))
}
