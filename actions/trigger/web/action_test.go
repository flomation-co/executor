package web

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestExecute(t *testing.T) {
	RegisterTestingT(t)

	// The API populates the trigger node's inputs from the HTTP request; the
	// trigger echoes them as outputs so they are referenceable bare.
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "method", Type: "string", Value: "POST"},
		{Name: "id", Type: "string", Value: "42"},
		{Name: "message", Type: "string", Value: "hello"},
	})
	Expect(err).To(BeNil())
	Expect(result).To(Not(BeNil()))
	Expect(result["method"]).To(Equal("POST"))
	Expect(result["id"]).To(Equal("42"))
	Expect(result["message"]).To(Equal("hello"))
}

func TestExecuteSkipsNilValues(t *testing.T) {
	RegisterTestingT(t)

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "method", Type: "string", Value: "GET"},
		{Name: "absent", Type: "string", Value: nil},
	})
	Expect(err).To(BeNil())
	Expect(result["method"]).To(Equal("GET"))
	_, exists := result["absent"]
	Expect(exists).To(BeFalse())
}
