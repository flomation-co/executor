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

// The Web Trigger exposes an auth_mode config with exactly the publishable/public
// options the API projection and Launch's gate rely on. Guards against the input
// being renamed or its option values drifting out of lock-step with the edge.
func TestAuthModeInputContract(t *testing.T) {
	RegisterTestingT(t)

	var authMode *core.Connection
	for i := range Inputs {
		if Inputs[i].Name == "auth_mode" {
			authMode = &Inputs[i]
			break
		}
	}
	Expect(authMode).To(Not(BeNil()), "auth_mode input must exist")
	Expect(authMode.Type).To(Equal(core.ConnectionTypeString))

	values := map[string]bool{}
	for _, o := range authMode.Options {
		values[o.Value] = true
	}
	Expect(values).To(HaveKey("publishable"))
	Expect(values).To(HaveKey("public"))
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
