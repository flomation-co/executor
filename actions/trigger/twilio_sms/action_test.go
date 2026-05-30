package twilio_sms

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestExecute_PassesThroughInputs(t *testing.T) {
	RegisterTestingT(t)

	f := &core.Flow{}
	node := &core.Node{ID: "n1"}

	inputs := []*core.Connection{
		{Name: "from", Type: core.ConnectionTypeString, Value: "+441234567890"},
		{Name: "to", Type: core.ConnectionTypeString, Value: "+449876543210"},
		{Name: "content", Type: core.ConnectionTypeString, Value: "Hello there"},
		{Name: "message_sid", Type: core.ConnectionTypeString, Value: "SM123"},
		{Name: "channel_type", Type: core.ConnectionTypeString, Value: "twilio_sms"},
	}

	result, err := Execute(f, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["from"]).To(Equal("+441234567890"))
	Expect(result["to"]).To(Equal("+449876543210"))
	Expect(result["content"]).To(Equal("Hello there"))
	Expect(result["message_text"]).To(Equal("Hello there"))
	Expect(result["message_sid"]).To(Equal("SM123"))
}

func TestExecute_MapsMessageTextToContent(t *testing.T) {
	RegisterTestingT(t)

	f := &core.Flow{}
	node := &core.Node{ID: "n1"}

	inputs := []*core.Connection{
		{Name: "message_text", Type: core.ConnectionTypeString, Value: "Hi from SMS"},
	}

	result, err := Execute(f, node, inputs)
	Expect(err).To(BeNil())
	Expect(result["content"]).To(Equal("Hi from SMS"))
	Expect(result["message_text"]).To(Equal("Hi from SMS"))
}

func TestExecute_EmptyInputs(t *testing.T) {
	RegisterTestingT(t)

	f := &core.Flow{}
	node := &core.Node{ID: "n1"}

	result, err := Execute(f, node, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(result).ToNot(BeNil())
}
