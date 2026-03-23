package s3

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func Test_Execute_ReturnsInputsAsOutputs(t *testing.T) {
	t.Parallel()
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "node-1", Type: "trigger/s3"}

	inputs := []*core.Connection{
		{Name: "bucket", Type: core.ConnectionTypeString, Value: "my-bucket"},
		{Name: "key", Type: core.ConnectionTypeString, Value: "path/to/object.txt"},
		{Name: "size", Type: core.ConnectionTypeInteger, Value: 1234},
		{Name: "last_modified", Type: core.ConnectionTypeString, Value: "2026-03-23T10:00:00Z"},
		{Name: "etag", Type: core.ConnectionTypeString, Value: "\"abc123\""},
		{Name: "event_type", Type: core.ConnectionTypeString, Value: "put"},
	}

	result, err := Execute(flow, node, inputs)

	Expect(err).To(BeNil())
	Expect(result).To(HaveKeyWithValue("bucket", "my-bucket"))
	Expect(result).To(HaveKeyWithValue("key", "path/to/object.txt"))
	Expect(result).To(HaveKeyWithValue("size", 1234))
	Expect(result).To(HaveKeyWithValue("etag", "\"abc123\""))
	Expect(result).To(HaveKeyWithValue("event_type", "put"))
}

func Test_Execute_SkipsNilValues(t *testing.T) {
	t.Parallel()
	RegisterTestingT(t)

	flow := &core.Flow{}
	node := &core.Node{ID: "node-1", Type: "trigger/s3"}

	inputs := []*core.Connection{
		{Name: "bucket", Type: core.ConnectionTypeString, Value: "my-bucket"},
		{Name: "key", Type: core.ConnectionTypeString, Value: nil},
	}

	result, err := Execute(flow, node, inputs)

	Expect(err).To(BeNil())
	Expect(result).To(HaveKeyWithValue("bucket", "my-bucket"))
	Expect(result).NotTo(HaveKey("key"))
}

func Test_Metadata(t *testing.T) {
	t.Parallel()
	RegisterTestingT(t)

	Expect(Type).To(Equal(core.ActionTypeTrigger))
	Expect(Name).To(Equal("S3 Trigger"))
	Expect(len(Inputs)).To(Equal(7))
	Expect(len(Outputs)).To(Equal(6))
	Expect(Inputs[0].Required).To(BeTrue())
	Expect(Inputs[2].Required).To(BeTrue())
	Expect(Inputs[3].Required).To(BeTrue())
	Expect(Inputs[4].Required).To(BeTrue())
}
