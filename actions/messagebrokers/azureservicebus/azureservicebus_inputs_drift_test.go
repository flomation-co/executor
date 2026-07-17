// Cross-action invariants for the Message Brokers ▸ Azure Service Bus node, on
// the precedent of actions/azure/storage/storage_inputs_drift_test.go.
//
// What this file is really for: all 31 actions re-declare the credential block
// INLINE, because the manifest generator AST-parses the Inputs literal and
// cannot see through a package-level variable. azureservicebus.AuthInputs is
// therefore documentation, not enforcement — 31 copies of six fields, free to
// drift one paste at a time. This is the enforcement. A copy that drifts fails
// CI with the action and the field named.
package azureservicebus_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	sb "flomation.app/automate/executor/actions/messagebrokers/azureservicebus"

	sb_deadletter_peek "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/deadletter_peek"
	sb_deadletter_receive "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/deadletter_receive"
	sb_message_dead_letter "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/message_dead_letter"
	sb_namespace_get "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/namespace_get"
	sb_queue_cancel_scheduled "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_cancel_scheduled"
	sb_queue_create "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_create"
	sb_queue_delete "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_delete"
	sb_queue_get "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_get"
	sb_queue_list "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_list"
	sb_queue_peek "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_peek"
	sb_queue_receive "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_receive"
	sb_queue_receive_deferred "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_receive_deferred"
	sb_queue_runtime_properties "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_runtime_properties"
	sb_queue_schedule "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_schedule"
	sb_queue_send "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_send"
	sb_queue_send_batch "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_send_batch"
	sb_queue_update "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_update"
	sb_rule_create "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/rule_create"
	sb_rule_delete "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/rule_delete"
	sb_rule_list "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/rule_list"
	sb_session_receive "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/session_receive"
	sb_subscription_create "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_create"
	sb_subscription_delete "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_delete"
	sb_subscription_list "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_list"
	sb_subscription_peek "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_peek"
	sb_subscription_receive "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_receive"
	sb_subscription_runtime_properties "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_runtime_properties"
	sb_topic_create "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/topic_create"
	sb_topic_delete "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/topic_delete"
	sb_topic_list "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/topic_list"
	sb_topic_send "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/topic_send"
)

// sbActionInputs is the table every assertion below ranges over. All 31 actions.
func sbActionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azureservicebus/deadletter_peek":                 sb_deadletter_peek.Inputs[:],
		"azureservicebus/deadletter_receive":              sb_deadletter_receive.Inputs[:],
		"azureservicebus/message_dead_letter":             sb_message_dead_letter.Inputs[:],
		"azureservicebus/namespace_get":                   sb_namespace_get.Inputs[:],
		"azureservicebus/queue_cancel_scheduled":          sb_queue_cancel_scheduled.Inputs[:],
		"azureservicebus/queue_create":                    sb_queue_create.Inputs[:],
		"azureservicebus/queue_delete":                    sb_queue_delete.Inputs[:],
		"azureservicebus/queue_get":                       sb_queue_get.Inputs[:],
		"azureservicebus/queue_list":                      sb_queue_list.Inputs[:],
		"azureservicebus/queue_peek":                      sb_queue_peek.Inputs[:],
		"azureservicebus/queue_receive":                   sb_queue_receive.Inputs[:],
		"azureservicebus/queue_receive_deferred":          sb_queue_receive_deferred.Inputs[:],
		"azureservicebus/queue_runtime_properties":        sb_queue_runtime_properties.Inputs[:],
		"azureservicebus/queue_schedule":                  sb_queue_schedule.Inputs[:],
		"azureservicebus/queue_send":                      sb_queue_send.Inputs[:],
		"azureservicebus/queue_send_batch":                sb_queue_send_batch.Inputs[:],
		"azureservicebus/queue_update":                    sb_queue_update.Inputs[:],
		"azureservicebus/rule_create":                     sb_rule_create.Inputs[:],
		"azureservicebus/rule_delete":                     sb_rule_delete.Inputs[:],
		"azureservicebus/rule_list":                       sb_rule_list.Inputs[:],
		"azureservicebus/session_receive":                 sb_session_receive.Inputs[:],
		"azureservicebus/subscription_create":             sb_subscription_create.Inputs[:],
		"azureservicebus/subscription_delete":             sb_subscription_delete.Inputs[:],
		"azureservicebus/subscription_list":               sb_subscription_list.Inputs[:],
		"azureservicebus/subscription_peek":               sb_subscription_peek.Inputs[:],
		"azureservicebus/subscription_receive":            sb_subscription_receive.Inputs[:],
		"azureservicebus/subscription_runtime_properties": sb_subscription_runtime_properties.Inputs[:],
		"azureservicebus/topic_create":                    sb_topic_create.Inputs[:],
		"azureservicebus/topic_delete":                    sb_topic_delete.Inputs[:],
		"azureservicebus/topic_list":                      sb_topic_list.Inputs[:],
		"azureservicebus/topic_send":                      sb_topic_send.Inputs[:],
	}
}

// sbActionOutputs backs the standard-outputs assertion.
func sbActionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azureservicebus/deadletter_peek":                 sb_deadletter_peek.Outputs[:],
		"azureservicebus/deadletter_receive":              sb_deadletter_receive.Outputs[:],
		"azureservicebus/message_dead_letter":             sb_message_dead_letter.Outputs[:],
		"azureservicebus/namespace_get":                   sb_namespace_get.Outputs[:],
		"azureservicebus/queue_cancel_scheduled":          sb_queue_cancel_scheduled.Outputs[:],
		"azureservicebus/queue_create":                    sb_queue_create.Outputs[:],
		"azureservicebus/queue_delete":                    sb_queue_delete.Outputs[:],
		"azureservicebus/queue_get":                       sb_queue_get.Outputs[:],
		"azureservicebus/queue_list":                      sb_queue_list.Outputs[:],
		"azureservicebus/queue_peek":                      sb_queue_peek.Outputs[:],
		"azureservicebus/queue_receive":                   sb_queue_receive.Outputs[:],
		"azureservicebus/queue_receive_deferred":          sb_queue_receive_deferred.Outputs[:],
		"azureservicebus/queue_runtime_properties":        sb_queue_runtime_properties.Outputs[:],
		"azureservicebus/queue_schedule":                  sb_queue_schedule.Outputs[:],
		"azureservicebus/queue_send":                      sb_queue_send.Outputs[:],
		"azureservicebus/queue_send_batch":                sb_queue_send_batch.Outputs[:],
		"azureservicebus/queue_update":                    sb_queue_update.Outputs[:],
		"azureservicebus/rule_create":                     sb_rule_create.Outputs[:],
		"azureservicebus/rule_delete":                     sb_rule_delete.Outputs[:],
		"azureservicebus/rule_list":                       sb_rule_list.Outputs[:],
		"azureservicebus/session_receive":                 sb_session_receive.Outputs[:],
		"azureservicebus/subscription_create":             sb_subscription_create.Outputs[:],
		"azureservicebus/subscription_delete":             sb_subscription_delete.Outputs[:],
		"azureservicebus/subscription_list":               sb_subscription_list.Outputs[:],
		"azureservicebus/subscription_peek":               sb_subscription_peek.Outputs[:],
		"azureservicebus/subscription_receive":            sb_subscription_receive.Outputs[:],
		"azureservicebus/subscription_runtime_properties": sb_subscription_runtime_properties.Outputs[:],
		"azureservicebus/topic_create":                    sb_topic_create.Outputs[:],
		"azureservicebus/topic_delete":                    sb_topic_delete.Outputs[:],
		"azureservicebus/topic_list":                      sb_topic_list.Outputs[:],
		"azureservicebus/topic_send":                      sb_topic_send.Outputs[:],
	}
}

// sbActionIcons backs the icon-resolution assertion.
func sbActionIcons() map[string]string {
	return map[string]string{
		"azureservicebus/deadletter_peek":                 sb_deadletter_peek.Icon,
		"azureservicebus/deadletter_receive":              sb_deadletter_receive.Icon,
		"azureservicebus/message_dead_letter":             sb_message_dead_letter.Icon,
		"azureservicebus/namespace_get":                   sb_namespace_get.Icon,
		"azureservicebus/queue_cancel_scheduled":          sb_queue_cancel_scheduled.Icon,
		"azureservicebus/queue_create":                    sb_queue_create.Icon,
		"azureservicebus/queue_delete":                    sb_queue_delete.Icon,
		"azureservicebus/queue_get":                       sb_queue_get.Icon,
		"azureservicebus/queue_list":                      sb_queue_list.Icon,
		"azureservicebus/queue_peek":                      sb_queue_peek.Icon,
		"azureservicebus/queue_receive":                   sb_queue_receive.Icon,
		"azureservicebus/queue_receive_deferred":          sb_queue_receive_deferred.Icon,
		"azureservicebus/queue_runtime_properties":        sb_queue_runtime_properties.Icon,
		"azureservicebus/queue_schedule":                  sb_queue_schedule.Icon,
		"azureservicebus/queue_send":                      sb_queue_send.Icon,
		"azureservicebus/queue_send_batch":                sb_queue_send_batch.Icon,
		"azureservicebus/queue_update":                    sb_queue_update.Icon,
		"azureservicebus/rule_create":                     sb_rule_create.Icon,
		"azureservicebus/rule_delete":                     sb_rule_delete.Icon,
		"azureservicebus/rule_list":                       sb_rule_list.Icon,
		"azureservicebus/session_receive":                 sb_session_receive.Icon,
		"azureservicebus/subscription_create":             sb_subscription_create.Icon,
		"azureservicebus/subscription_delete":             sb_subscription_delete.Icon,
		"azureservicebus/subscription_list":               sb_subscription_list.Icon,
		"azureservicebus/subscription_peek":               sb_subscription_peek.Icon,
		"azureservicebus/subscription_receive":            sb_subscription_receive.Icon,
		"azureservicebus/subscription_runtime_properties": sb_subscription_runtime_properties.Icon,
		"azureservicebus/topic_create":                    sb_topic_create.Icon,
		"azureservicebus/topic_delete":                    sb_topic_delete.Icon,
		"azureservicebus/topic_list":                      sb_topic_list.Icon,
		"azureservicebus/topic_send":                      sb_topic_send.Icon,
	}
}

// sbListActions are the actions that return a collection, and so swap
// id/result for results/count in the standard output contract. Every receive
// and peek is one: Service Bus hands back "up to N" messages, never one.
func sbListActions() map[string]bool {
	return map[string]bool{
		"azureservicebus/deadletter_peek":        true,
		"azureservicebus/deadletter_receive":     true,
		"azureservicebus/message_dead_letter":    true,
		"azureservicebus/queue_list":             true,
		"azureservicebus/queue_peek":             true,
		"azureservicebus/queue_receive":          true,
		"azureservicebus/queue_receive_deferred": true,
		"azureservicebus/queue_send_batch":       true,
		"azureservicebus/rule_list":              true,
		"azureservicebus/session_receive":        true,
		"azureservicebus/subscription_list":      true,
		"azureservicebus/subscription_peek":      true,
		"azureservicebus/subscription_receive":   true,
		"azureservicebus/topic_list":             true,
	}
}

// sbBadges is the badge glyphs these icons compose onto the azure base. Every
// name here was checked against editor/app/components/icons/paths.ts — a badge
// missing there renders as a silent "?" in the palette, which no compiler and
// no other test would catch.
var sbBadges = map[string]bool{
	"ban":                  true,
	"bullhorn":             true,
	"circle-exclamation":   true,
	"circle-nodes":         true,
	"circle-xmark":         true,
	"clock":                true,
	"envelope-open-text":   true,
	"envelopes-bulk":       true,
	"eye":                  true,
	"filter":               true,
	"gauge":                true,
	"hourglass-start":      true,
	"layer-group":          true,
	"list":                 true,
	"magnifying-glass":     true,
	"paper-plane":          true,
	"pen":                  true,
	"plus":                 true,
	"trash":                true,
	"triangle-exclamation": true,
}

// TestAuthBlockDoesNotDrift is the whole reason this file exists.
//
// Every action's first six inputs must reproduce sb.AuthInputs exactly — name,
// type, label, placeholder, options and visibility, in order. The names are
// the sharp edge: they are AST-parsed by the manifest generator and mirrored
// by the api's dynamic-options params lists, so one drifted copy breaks the
// live dropdowns for that action only — the kind of defect nobody notices
// until a customer does.
func TestAuthBlockDoesNotDrift(t *testing.T) {
	want := sb.AuthInputs

	for id, inputs := range sbActionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from sb.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestNoResourceInputShadowsACredential guards the collision that
// core.FindConnection makes possible: it returns the FIRST input whose name
// matches, and the credential block is declared first — so a resource field
// that reuses a credential's name silently reads the CREDENTIAL instead.
//
// "namespace" is the live trap here: it is a credential field, and it is also
// the word an operator would naturally use for a queue's container.
func TestNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range sb.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range sbActionInputs() {
		if len(inputs) < len(sb.AuthInputs) {
			continue // TestAuthBlockDoesNotDrift already reports this
		}
		for _, c := range inputs[len(sb.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — "+
					"core.FindConnection would return the credential; rename it", id, c.Name)
			}
		}
	}
}

// TestIconsResolve keeps every icon inside the glyph set the editor actually
// ships. An unknown base or badge compiles cleanly, passes every other test,
// and renders as a "?" in the node palette.
func TestIconsResolve(t *testing.T) {
	for id, icon := range sbActionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "azure" {
			t.Errorf("%s: icon base is %q, not \"azure\" — every action in this node wears the Azure mark", id, base)
		}
		if !sbBadges[badge] {
			t.Errorf("%s: icon badge %q is not in the verified badge set (checked against editor paths.ts) — it would render as \"?\" in the palette", id, badge)
		}
	}
}

// TestStandardOutputsPresent pins the outputs the platform depends on: success
// drives the soft-failure path, error carries the message, tool_result is what
// the AI tool loop shows the model — plus the id/result vs results/count split
// between single-object and list actions.
func TestStandardOutputsPresent(t *testing.T) {
	listActions := sbListActions()

	for id, outputs := range sbActionOutputs() {
		have := map[string]bool{}
		for _, o := range outputs {
			have[o.Name] = true
		}
		for _, required := range []string{"success", "error", "tool_result"} {
			if !have[required] {
				t.Errorf("%s: missing the %q output", id, required)
			}
		}
		if listActions[id] {
			for _, required := range []string{"results", "count"} {
				if !have[required] {
					t.Errorf("%s: is a list action but missing the %q output", id, required)
				}
			}
		} else {
			for _, required := range []string{"id", "result"} {
				if !have[required] {
					t.Errorf("%s: missing the %q output", id, required)
				}
			}
		}
	}
}

// TestReceiveActionsReportEmptyAsData pins the "nothing arrived is not a
// failure" contract. A quiet queue is the ordinary state of a queue, so every
// action that receives or peeks must carry the received flag rather than push
// an empty result onto the error port.
func TestReceiveActionsReportEmptyAsData(t *testing.T) {
	receiving := map[string]bool{
		"azureservicebus/queue_receive":          true,
		"azureservicebus/subscription_receive":   true,
		"azureservicebus/queue_peek":             true,
		"azureservicebus/subscription_peek":      true,
		"azureservicebus/deadletter_receive":     true,
		"azureservicebus/deadletter_peek":        true,
		"azureservicebus/message_dead_letter":    true,
		"azureservicebus/queue_receive_deferred": true,
		"azureservicebus/session_receive":        true,
	}
	for id, outputs := range sbActionOutputs() {
		if !receiving[id] {
			continue
		}
		found := false
		for _, o := range outputs {
			if o.Name == "received" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: receives messages but has no %q output — an empty receive would have to be reported as an error", id, "received")
		}
	}
}

// TestSettlementIsNotAStandaloneAction pins the design decision the SDK forces
// on us, because it is the one a future contributor is most likely to "fix".
//
// A lock token belongs to the AMQP connection that took it, and a receiver's
// closing link releases its unsettled messages immediately — so a Complete or
// Abandon node downstream of a receive could never work, no matter how the
// lock token were passed to it. Settlement is a parameter of the receive.
func TestSettlementIsNotAStandaloneAction(t *testing.T) {
	forbidden := []string{
		"azureservicebus/message_complete",
		"azureservicebus/message_abandon",
		"azureservicebus/message_defer",
		"azureservicebus/message_renew_lock",
	}
	actions := sbActionInputs()
	for _, id := range forbidden {
		if _, exists := actions[id]; exists {
			t.Errorf("%s exists, but settlement cannot be a standalone action: a lock token is scoped to the AMQP connection "+
				"that took it, and the broker releases unsettled messages the moment that connection's link closes. "+
				"Settle inside the receive with the disposition parameter, or hand off with defer (sequence numbers are durable).", id)
		}
	}
}

// TestReceiveActionsOfferEveryDisposition pins the settlement surface. If
// settlement cannot be a downstream node, the receive must offer all four
// outcomes or the operator has no way to express abandon/dead-letter/defer at
// all.
func TestReceiveActionsOfferEveryDisposition(t *testing.T) {
	settling := []string{
		"azureservicebus/queue_receive",
		"azureservicebus/subscription_receive",
		"azureservicebus/deadletter_receive",
		"azureservicebus/queue_receive_deferred",
		"azureservicebus/session_receive",
	}
	want := map[string]bool{"complete": true, "abandon": true, "dead_letter": true, "defer": true}

	actions := sbActionInputs()
	for _, id := range settling {
		inputs, ok := actions[id]
		if !ok {
			t.Errorf("%s is missing from the table", id)
			continue
		}
		var disposition *core.Connection
		for i := range inputs {
			if inputs[i].Name == "disposition" {
				disposition = &inputs[i]
			}
		}
		if disposition == nil {
			t.Errorf("%s: receives with peek-lock but has no disposition input — the messages could never be settled", id)
			continue
		}
		got := map[string]bool{}
		for _, o := range disposition.Options {
			got[o.Value] = true
		}
		for value := range want {
			if !got[value] {
				t.Errorf("%s: disposition has no %q option", id, value)
			}
		}
	}
}

// TestVisibleWhenValuesAreStrings pins the platform contract that visible_when
// compares strings — a boolean-typed field is matched as "true"/"false".
func TestVisibleWhenValuesAreStrings(t *testing.T) {
	for id, inputs := range sbActionInputs() {
		byName := map[string]core.Connection{}
		for _, in := range inputs {
			byName[in.Name] = in
		}
		for _, in := range inputs {
			if in.Visible == nil {
				continue
			}
			target, ok := byName[in.Visible.Field]
			if !ok {
				t.Errorf("%s: input %q is visible_when %q, which is not an input of this action", id, in.Name, in.Visible.Field)
				continue
			}
			if target.Type != core.ConnectionTypeBoolean {
				continue
			}
			for _, v := range in.Visible.Values {
				if v != "true" && v != "false" {
					t.Errorf("%s: input %q compares the boolean %q against %q — visible_when values are strings, so only \"true\"/\"false\" can ever match",
						id, in.Name, in.Visible.Field, v)
				}
			}
		}
	}
}

// TestTableCoversEveryActionOnDisk pins the designed count. If action 32 lands
// and nobody adds it to the tables in this file, this is what says so.
func TestTableCoversEveryActionOnDisk(t *testing.T) {
	const designed = 31
	if got := len(sbActionInputs()); got != designed {
		t.Errorf("sbActionInputs() covers %d actions, expected %d — a new Service Bus action must be added to the tables in this file", got, designed)
	}
	if got := len(sbActionOutputs()); got != designed {
		t.Errorf("sbActionOutputs() covers %d actions, expected %d", got, designed)
	}
	if got := len(sbActionIcons()); got != designed {
		t.Errorf("sbActionIcons() covers %d actions, expected %d", got, designed)
	}
}

// TestListControlsUseConsistentLabels pins the paging-control convention. A
// non-technical operator learns Limit and Max Messages once and must recognise
// them on every action that has them.
func TestListControlsUseConsistentLabels(t *testing.T) {
	canonical := map[string]string{
		"limit":            "Limit",
		"max_messages":     "Max Messages",
		"max_wait_seconds": "Max Wait (seconds)",
	}
	for id, inputs := range sbActionInputs() {
		for _, in := range inputs {
			if want, ok := canonical[in.Name]; ok && in.Label != want {
				t.Errorf("%s: %q Label = %q, want %q — the paging controls must read the same across every action",
					id, in.Name, in.Label, want)
			}
		}
	}
}
