package calendly_webhook

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestExecuteEchoesEventDataNotConfig(t *testing.T) {
	RegisterTestingT(t)
	res, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "access_token", Value: "tok_secret"},
		{Name: "scope", Value: "user"},
		{Name: "events", Value: "invitee.created,invitee.canceled"},
		{Name: "__node_id", Value: "n1"},
		{Name: "event", Value: "invitee.created"},
		{Name: "invitee_name", Value: "Jane Doe"},
		{Name: "invitee_email", Value: "jane@example.com"},
		{Name: "payload", Value: map[string]interface{}{"email": "jane@example.com"}},
	})
	Expect(err).To(BeNil())

	// Config fields are never echoed into outputs.
	Expect(res).NotTo(HaveKey("access_token"))
	Expect(res).NotTo(HaveKey("scope"))
	Expect(res).NotTo(HaveKey("events"))
	Expect(res).NotTo(HaveKey("__node_id"))

	// Event data passes through.
	Expect(res["event"]).To(Equal("invitee.created"))
	Expect(res["invitee_name"]).To(Equal("Jane Doe"))
	Expect(res["content"]).To(Equal("[Calendly] invitee.created — Jane Doe"))
}

func TestContentSummaryFallbacks(t *testing.T) {
	RegisterTestingT(t)
	res, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "event", Value: "invitee.canceled"},
		{Name: "invitee_email", Value: "jane@example.com"},
	})
	Expect(err).To(BeNil())
	Expect(res["content"]).To(Equal("[Calendly] invitee.canceled — jane@example.com"))

	res, err = Execute(&core.Flow{}, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(res["content"]).To(Equal("[Calendly] event"))
}
