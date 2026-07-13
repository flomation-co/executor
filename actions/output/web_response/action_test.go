package web_response

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestExecute(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	result, err := Execute(flow, nil, []*core.Connection{
		{Name: "body", Type: "text", Value: `{"ok":true}`},
		{Name: "status_code", Type: "integer", Value: 201},
		{Name: "content_type", Type: "string", Value: "application/json"},
		{Name: "headers", Type: "text", Value: `{"Location":"/x"}`},
	})
	Expect(err).To(BeNil())
	Expect(result["set"]).To(BeTrue())

	// The response is captured under the reserved key for the API to read.
	captured := flow.GetOutput(WebResponseKey)
	Expect(captured).To(Not(BeNil()))
	resp, ok := captured.(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(resp["body"]).To(Equal(`{"ok":true}`))
	Expect(resp["status_code"]).To(Equal(201))
	Expect(resp["content_type"]).To(Equal("application/json"))
	Expect(resp["headers"]).To(Equal(`{"Location":"/x"}`))
}

func TestExecuteEmpty(t *testing.T) {
	RegisterTestingT(t)

	flow := &core.Flow{}
	result, err := Execute(flow, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(result["set"]).To(BeTrue())

	// Still captures an (empty) response object — the API falls back to a default.
	captured := flow.GetOutput(WebResponseKey)
	Expect(captured).To(Not(BeNil()))
	resp, ok := captured.(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(resp).To(BeEmpty())
}
