package nosql_redis

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

	Expect(Name).To(Equal("Redis Command"))
	Expect(Type).To(Equal(core.ActionTypeAction))
	Expect(Author).To(Equal("Andy Esser"))
	Expect(Icon).To(Equal("bolt"))
}

func TestInputsConfiguration(t *testing.T) {
	RegisterTestingT(t)

	Expect(len(Inputs)).To(Equal(8))

	hostInput := Inputs[0]
	Expect(hostInput.Name).To(Equal("host"))
	Expect(hostInput.Required).To(BeTrue())

	portInput := Inputs[1]
	Expect(portInput.Name).To(Equal("port"))
	Expect(portInput.Required).To(BeTrue())

	cmdInput := Inputs[4]
	Expect(cmdInput.Name).To(Equal("command"))
	Expect(cmdInput.Required).To(BeTrue())
	Expect(len(cmdInput.Options)).To(Equal(13))

	keyInput := Inputs[5]
	Expect(keyInput.Name).To(Equal("key"))
	Expect(keyInput.Required).To(BeTrue())
}

func TestOutputsConfiguration(t *testing.T) {
	RegisterTestingT(t)

	Expect(len(Outputs)).To(Equal(3))
	Expect(Outputs[1].Name).To(Equal("result"))
	Expect(Outputs[1].Type).To(Equal(core.ConnectionTypeObject))
	Expect(Outputs[2].Name).To(Equal("success"))
	Expect(Outputs[2].Type).To(Equal(core.ConnectionTypeBoolean))
}

func TestUnsupportedCommand(t *testing.T) {
	RegisterTestingT(t)

	node := &core.Node{ID: "redis-1", Type: "nosql/redis", Data: &core.NodeData{ID: "redis-1"}}
	flow := &core.Flow{Nodes: []*core.Node{node}}

	inputs := makeInputs(map[string]interface{}{
		"host":    "localhost",
		"port":    int64(6379),
		"command": "INVALID",
		"key":     "test-key",
	})

	// Will fail on ping (no server) before reaching the unsupported command path,
	// but verifies no nil pointer panic
	_, err := Execute(flow, node, inputs)
	Expect(err).ToNot(BeNil())
}

func TestConnectionFailure(t *testing.T) {
	RegisterTestingT(t)

	node := &core.Node{ID: "redis-1", Type: "nosql/redis", Data: &core.NodeData{ID: "redis-1"}}
	flow := &core.Flow{Nodes: []*core.Node{node}}

	inputs := makeInputs(map[string]interface{}{
		"host":    "localhost",
		"port":    int64(59999),
		"command": "GET",
		"key":     "test-key",
	})

	_, err := Execute(flow, node, inputs)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("failed to connect to Redis"))
}

func TestPasswordAndDatabaseDefaults(t *testing.T) {
	RegisterTestingT(t)

	// Verify that nil password/database connections don't panic
	node := &core.Node{ID: "redis-1", Type: "nosql/redis", Data: &core.NodeData{ID: "redis-1"}}
	flow := &core.Flow{Nodes: []*core.Node{node}}

	// Only provide required fields — password and database should default gracefully
	inputs := []*core.Connection{
		{Name: "host", Type: core.ConnectionTypeString, Value: "localhost"},
		{Name: "port", Type: core.ConnectionTypeInteger, Value: int64(59999)},
		{Name: "command", Type: core.ConnectionTypeString, Value: "GET"},
		{Name: "key", Type: core.ConnectionTypeString, Value: "test-key"},
	}

	_, err := Execute(flow, node, inputs)
	// Should fail on connection, not on nil pointer
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("failed to connect to Redis"))
}
