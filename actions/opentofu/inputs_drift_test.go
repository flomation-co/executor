package opentofu_test

import (
	"reflect"
	"testing"

	core "flomation.app/automate/executor"
	applyaction "flomation.app/automate/executor/actions/opentofu/apply"
	destroyaction "flomation.app/automate/executor/actions/opentofu/destroy"
	planaction "flomation.app/automate/executor/actions/opentofu/plan"
)

// sharedInputNames are the backend-auth / binary-selection / state inputs that
// MUST be identical across plan, apply, and destroy. They're declared as inline
// composite literals in each action because the manifest generator only extracts
// `var Inputs` when it is an inline literal (it cannot resolve a shared package
// var). This test is the guard against the copies drifting: edit one action's
// shared input and forget the others, and this fails.
var sharedInputNames = []string{
	"working_directory",
	"variables", "backend_config", "credentials",
	"backend_auth",
	"aws_access_key_id", "aws_secret_access_key", "aws_session_token", "aws_region",
	"arm_client_id", "arm_client_secret", "arm_tenant_id", "arm_subscription_id", "arm_access_key",
	"gcp_credentials_json",
	"gitlab_username", "gitlab_token", "gitlab_address",
	"tofu_version", "binary_path", "allow_local_state",
}

func findInput(name string, conns []core.Connection) (core.Connection, bool) {
	for _, c := range conns {
		if c.Name == name {
			return c, true
		}
	}
	return core.Connection{}, false
}

func TestSharedInputsDoNotDrift(t *testing.T) {
	plan := planaction.Inputs[:]
	apply := applyaction.Inputs[:]
	destroy := destroyaction.Inputs[:]

	for _, name := range sharedInputNames {
		p, ok := findInput(name, plan)
		if !ok {
			t.Errorf("plan is missing shared input %q", name)
			continue
		}
		a, ok := findInput(name, apply)
		if !ok {
			t.Errorf("apply is missing shared input %q", name)
			continue
		}
		d, ok := findInput(name, destroy)
		if !ok {
			t.Errorf("destroy is missing shared input %q", name)
			continue
		}
		if !reflect.DeepEqual(p, a) {
			t.Errorf("input %q differs between plan and apply:\n plan=%+v\napply=%+v", name, p, a)
		}
		if !reflect.DeepEqual(p, d) {
			t.Errorf("input %q differs between plan and destroy:\n   plan=%+v\ndestroy=%+v", name, p, d)
		}
	}
}
