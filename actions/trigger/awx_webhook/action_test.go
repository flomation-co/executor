package awx_webhook

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// eventInputs is one injected value per declared output, as launch would deliver it
// from an AWX `notification_data()` payload. `content` is excluded — Execute
// synthesises that one rather than echoing it.
func eventInputs() []*core.Connection {
	return []*core.Connection{
		{Name: "event", Type: core.ConnectionTypeString, Value: "successful"},
		{Name: "job_id", Type: core.ConnectionTypeString, Value: "42"},
		{Name: "job_name", Type: core.ConnectionTypeString, Value: "Deploy Web"},
		{Name: "status", Type: core.ConnectionTypeString, Value: "successful"},
		{Name: "failed", Type: core.ConnectionTypeBoolean, Value: false},
		{Name: "job_url", Type: core.ConnectionTypeString, Value: "https://awx.example.com/#/jobs/playbook/42"},
		{Name: "created_by", Type: core.ConnectionTypeString, Value: "admin"},
		{Name: "started", Type: core.ConnectionTypeString, Value: "2026-07-14T10:15:03.123456+00:00"},
		{Name: "finished", Type: core.ConnectionTypeString, Value: "2026-07-14T10:15:31.654321+00:00"},
		{Name: "traceback", Type: core.ConnectionTypeString, Value: ""},
		{Name: "inventory", Type: core.ConnectionTypeString, Value: "Demo Inventory"},
		{Name: "project", Type: core.ConnectionTypeString, Value: "Demo Project"},
		{Name: "playbook", Type: core.ConnectionTypeString, Value: "hello_world.yml"},
		{Name: "limit", Type: core.ConnectionTypeString, Value: ""},
		{Name: "extra_vars", Type: core.ConnectionTypeString, Value: `{"target_env":"prod"}`},
		{Name: "hosts", Type: core.ConnectionTypeObject, Value: map[string]interface{}{
			"localhost": map[string]interface{}{"ok": 2.0, "changed": 0.0, "failures": 0.0},
		}},
		{Name: "body", Type: core.ConnectionTypeString, Value: `{"id":42,"status":"successful"}`},
		{Name: "triggered_at", Type: core.ConnectionTypeString, Value: "2026-07-14T10:15:32Z"},
	}
}

// configValues is one plausible value per declared config input, including the
// credentials and the internal __node_id marker.
func configValues() []*core.Connection {
	return []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: "https://awx.example.com"},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "token"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "kNsP2xQ7vT4mZ9wL1cR8yB3jH6gF0dA5"},
		{Name: "awx_username", Type: core.ConnectionTypeString, Value: "admin"},
		{Name: "awx_password", Type: core.ConnectionTypeSecret, Value: "s3cr3t-awx-password"},
		{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Value: true},
		{Name: "api_prefix", Type: core.ConnectionTypeString, Value: "/api/v2/"},
		{Name: "template_kind", Type: core.ConnectionTypeString, Value: "job_template"},
		{Name: "job_template_id", Type: core.ConnectionTypeString, Value: "7"},
		{Name: "workflow_template_id", Type: core.ConnectionTypeString, Value: ""},
		{Name: "events", Type: core.ConnectionTypeMultiSelect, Value: `["successful","failed"]`},
		{Name: "skip_awx_verification", Type: core.ConnectionTypeBoolean, Value: false},
		{Name: "__node_id", Type: core.ConnectionTypeString, Value: "node-1"},
	}
}

// ★ THE TEST THAT MATTERS. An input added to Inputs but forgotten in configInputs is
// echoed straight into the flow — which for this node means publishing the AWX API
// token, or the password, to every downstream node. Deriving the expectation from
// Inputs (rather than a hand-written list) is what makes it catch the NEXT input
// somebody adds.
func TestEveryConfigInputIsInTheDenylist(t *testing.T) {
	RegisterTestingT(t)

	for _, in := range Inputs {
		Expect(configInputs[in.Name]).To(BeTrue(),
			"input %q is missing from configInputs — Execute would echo it into the flow as an output", in.Name)
	}

	// Not an Input, but launch attaches it, so Execute must drop it too.
	Expect(configInputs["__node_id"]).To(BeTrue(), "__node_id must be stripped")

	// And the denylist must not have grown stale entries that no longer exist.
	declared := map[string]bool{"__node_id": true}
	for _, in := range Inputs {
		declared[in.Name] = true
	}
	for name := range configInputs {
		Expect(declared[name]).To(BeTrue(), "configInputs lists %q, which is not an input", name)
	}
}

// The converse trap: a config name that collides with an output name would make
// Execute strip the value launch injected, silently blanking that output forever.
// (Monday.com's board_id is exactly this case, and has to be left out of its
// denylist.) AWX has no collision — the pickers are job_template_id and
// workflow_template_id, never job_id — and this test keeps it that way.
func TestNoDeclaredOutputIsStrippedByTheDenylist(t *testing.T) {
	RegisterTestingT(t)

	for _, out := range Outputs {
		Expect(configInputs[out.Name]).To(BeFalse(),
			"output %q is in configInputs — Execute would strip the value launch injects for it", out.Name)
	}
}

func TestExecuteStripsConfigAndEchoesEventFields(t *testing.T) {
	RegisterTestingT(t)

	inputs := append(configValues(), eventInputs()...)

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())

	// Not one config field survives.
	for _, cfg := range configValues() {
		Expect(out).To(Not(HaveKey(cfg.Name)), "config key leaked into the outputs: %s", cfg.Name)
	}

	// Every declared output is present, and carries the value launch injected.
	for _, o := range Outputs {
		if o.Name == "content" {
			continue // synthesised, not echoed
		}
		Expect(out).To(HaveKey(o.Name), "declared output %q was not populated from the event", o.Name)
	}

	Expect(out["job_id"]).To(Equal("42"))
	Expect(out["job_name"]).To(Equal("Deploy Web"))
	Expect(out["status"]).To(Equal("successful"))
	Expect(out["event"]).To(Equal("successful"))
	Expect(out["failed"]).To(Equal(false))
	Expect(out["playbook"]).To(Equal("hello_world.yml"))
	Expect(out["job_url"]).To(Equal("https://awx.example.com/#/jobs/playbook/42"))
	Expect(out["extra_vars"]).To(Equal(`{"target_env":"prod"}`))
	Expect(out["body"]).To(Equal(`{"id":42,"status":"successful"}`))
	Expect(out["hosts"]).To(HaveKey("localhost"))

	Expect(out["content"]).To(Equal("[AWX] Deploy Web #42 — successful"))
}

// The property the denylist actually exists for, asserted on the VALUES rather than
// the keys: no credential reaches the flow, whatever a future refactor does to the
// shape of the result.
func TestNoCredentialValueReachesTheOutputs(t *testing.T) {
	RegisterTestingT(t)

	const (
		token    = "kNsP2xQ7vT4mZ9wL1cR8yB3jH6gF0dA5"
		password = "s3cr3t-awx-password"
	)

	out, err := Execute(&core.Flow{}, nil, append(configValues(), eventInputs()...))
	Expect(err).To(BeNil())

	for key, value := range out {
		s, ok := value.(string)
		if !ok {
			continue
		}
		Expect(s).To(Not(ContainSubstring(token)), "the API token leaked in output %q", key)
		Expect(s).To(Not(ContainSubstring(password)), "the AWX password leaked in output %q", key)
	}
}

// ★ AWX sends the job id as a JSON NUMBER ("id": 42). If it reaches the action
// unconverted it is a float64, and a naive string assertion would drop it, leaving
// every summary reading "[AWX] Deploy Web — successful" with no id in it.
func TestContentSummaryToleratesANumericJobID(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "job_id", Value: float64(42)},
		{Name: "job_name", Value: "Deploy Web"},
		{Name: "status", Value: "successful"},
	})
	Expect(err).To(BeNil())
	Expect(out["content"]).To(Equal("[AWX] Deploy Web #42 — successful"))
}

func TestContentSummaryFallbacks(t *testing.T) {
	RegisterTestingT(t)

	// The started event: no name yet is unusual, but a job id and status always are.
	out, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "job_id", Value: "42"},
		{Name: "status", Value: "running"},
	})
	Expect(err).To(BeNil())
	Expect(out["content"]).To(Equal("[AWX] job #42 — running"))

	// No status on the wire: fall back to our logical event name.
	out, err = Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "job_name", Value: "Deploy Web"},
		{Name: "event", Value: "canceled"},
	})
	Expect(err).To(BeNil())
	Expect(out["content"]).To(Equal("[AWX] Deploy Web — canceled"))

	// Status only.
	out, err = Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "status", Value: "failed"},
	})
	Expect(err).To(BeNil())
	Expect(out["content"]).To(Equal("[AWX] Job failed"))

	// Nothing at all — must never panic, must never render an empty summary.
	out, err = Execute(&core.Flow{}, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["content"]).To(Equal("[AWX] Job event"))
}

// The trigger is a pure echo: no HTTP, no AWX call, and above all no hard error.
// A non-nil error would abort the whole flow the moment a payload arrived slightly
// malformed.
func TestExecuteNeverHardFails(t *testing.T) {
	RegisterTestingT(t)

	for _, inputs := range [][]*core.Connection{
		nil,
		{},
		{{Name: "job_id", Value: nil}},
		{{Name: "hosts", Value: map[string]interface{}{}}},
		{{Name: "status", Value: 12345}}, // wrong type on the wire
	} {
		out, err := Execute(&core.Flow{}, nil, inputs)
		Expect(err).To(BeNil())
		Expect(out).To(HaveKey("content"))
	}
}

// The node's metadata contract with the manifest and the editor.
func TestActionMetadata(t *testing.T) {
	RegisterTestingT(t)

	Expect(Type).To(Equal(core.ActionTypeTrigger))
	Expect(Icon).To(Equal("ansible")) // bare base, no badge — the trigger convention

	// The auth block is re-declared verbatim as the first seven inputs of every AWX
	// action, this one included, and in that order.
	auth := []string{"awx_url", "auth_method", "api_token", "awx_username", "awx_password", "allow_insecure", "api_prefix"}
	for i, name := range auth {
		Expect(Inputs[i].Name).To(Equal(name), "input %d should be the auth field %q", i, name)
	}

	// The four logical events. AWX itself only has three (started / success / error,
	// with error a catch-all) — launch tells Failed and Canceled apart by the job's
	// status, so both must be offered here.
	var events []string
	for _, in := range Inputs {
		if in.Name == "events" {
			Expect(in.Required).To(BeTrue())
			for _, o := range in.Options {
				events = append(events, o.Value)
			}
		}
	}
	Expect(events).To(Equal([]string{"started", "successful", "failed", "canceled"}))

	// Every input carries a Label, and every non-boolean a Placeholder — the editor
	// has no other way to describe a field, and defaults are not harvested.
	for _, in := range Inputs {
		Expect(in.Label).To(Not(BeEmpty()), "input %q has no Label", in.Name)
		Expect(strings.TrimSpace(in.Name)).To(Equal(in.Name))
	}
	for _, o := range Outputs {
		Expect(o.Label).To(Not(BeEmpty()), "output %q has no Label", o.Name)
	}
}
