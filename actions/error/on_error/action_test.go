package error_on_error

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestOnErrorReturnsDefaults(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "onerr-1", Data: &core.NodeData{ID: "onerr-1"}}

	output, err := Execute(flow, node, nil)
	Expect(err).To(BeNil())
	Expect(output["error_message"]).To(Equal(""))
	Expect(output["error_node_id"]).To(Equal(""))
	Expect(output["error_node_label"]).To(Equal(""))
	Expect(output["error_node_type"]).To(Equal(""))
}
