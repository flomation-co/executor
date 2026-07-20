package azureservicebus_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	deadletter_peek "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/deadletter_peek"
	deadletter_receive "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/deadletter_receive"
	message_dead_letter "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/message_dead_letter"
	queue_peek "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_peek"
	queue_receive "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_receive"
	queue_receive_deferred "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_receive_deferred"
	session_receive "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/session_receive"
	subscription_peek "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_peek"
	subscription_receive "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_receive"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

func TestQueueReceiveCompletesByDefault(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{
		receivedMessage("m-1", `{"order":1}`),
		receivedMessage("m-2", "plain"),
	}}
	withReceiver(t, r)

	out, err := queue_receive.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		integer("max_messages", 5),
		boolean("parse_json", true),
	))
	items := wantResults(t, out, err, 2)

	if r.spec.Queue != "orders" || r.spec.Topic != "" {
		t.Errorf("receiver spec = %+v, want the queue", r.spec)
	}
	if r.spec.Mode != azservicebus.ReceiveModePeekLock {
		t.Error("an unset receive_mode must be peek-lock — receive-and-delete destroys the message before the flow sees it")
	}
	if r.maxAsked != 5 {
		t.Errorf("asked for %d messages, want 5", r.maxAsked)
	}
	if out["received"] != true {
		t.Errorf("received = %v, want true", out["received"])
	}

	// Settlement is in this action, on this connection, because a lock token
	// cannot cross nodes.
	if len(r.settled) != 2 {
		t.Fatalf("settled %d message(s), want 2 — an unsettled peek-lock message is released the moment this link closes", len(r.settled))
	}
	for _, s := range r.settled {
		if s.action != "complete" {
			t.Errorf("settled with %q, want complete", s.action)
		}
	}

	first, _ := items[0].(map[string]interface{})
	if first["body_json"] == nil {
		t.Error("parse_json was on but body_json is missing")
	}
	if !r.closed {
		t.Error("the receiver was not closed")
	}
}

func TestQueueReceiveHonoursEveryDisposition(t *testing.T) {
	cases := []struct {
		disposition string
		want        string
	}{
		{"complete", "complete"},
		{"abandon", "abandon"},
		{"defer", "defer"},
		{"dead_letter", "dead_letter"},
	}
	for _, tc := range cases {
		t.Run(tc.disposition, func(t *testing.T) {
			r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}}
			withReceiver(t, r)

			out, err := queue_receive.Execute(nil, nil, authInputs(
				str("queue", "orders"),
				str("disposition", tc.disposition),
				str("dead_letter_reason", "ValidationFailed"),
				str("dead_letter_error_description", "no customer reference"),
			))
			wantResults(t, out, err, 1)

			if len(r.settled) != 1 || r.settled[0].action != tc.want {
				t.Fatalf("settled = %+v, want %q", r.settled, tc.want)
			}
			if tc.want == "dead_letter" {
				if r.settled[0].reason != "ValidationFailed" || r.settled[0].descr != "no customer reference" {
					t.Errorf("the dead-letter diagnostics did not reach the broker: %+v", r.settled[0])
				}
			}
		})
	}
}

// ReceiveAndDelete messages are not settleable at all — the SDK says so
// explicitly — and the message is already gone, which is what was asked for.
func TestQueueReceiveDoesNotSettleInReceiveAndDeleteMode(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}}
	withReceiver(t, r)

	out, err := queue_receive.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		str("receive_mode", "receive_and_delete"),
	))
	wantResults(t, out, err, 1)

	if r.spec.Mode != azservicebus.ReceiveModeReceiveAndDelete {
		t.Error("receive_and_delete did not reach the receiver options")
	}
	if len(r.settled) != 0 {
		t.Errorf("settled a receive-and-delete message with %q — the SDK rejects that outright", r.settled[0].action)
	}
}

// A quiet queue is the ordinary state of a queue. If this reported an error,
// every polling flow would look broken.
func TestQueueReceiveReportsAnIdleQueueAsData(t *testing.T) {
	r := &stubReceiver{receiveErr: context.DeadlineExceeded}
	withReceiver(t, r)

	out, err := queue_receive.Execute(nil, nil, authInputs(str("queue", "orders"), integer("max_wait_seconds", 1)))
	wantResults(t, out, err, 0)

	if out["received"] != false {
		t.Errorf("received = %v, want false", out["received"])
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "No messages") {
		t.Errorf("tool_result = %q, want it to say plainly that nothing arrived", summary)
	}
}

func TestQueueReceiveReportsFailuresSoftly(t *testing.T) {
	t.Run("receive", func(t *testing.T) {
		withReceiver(t, &stubReceiver{receiveErr: serviceBusError(azservicebus.CodeUnauthorizedAccess)})
		out, err := queue_receive.Execute(nil, nil, authInputs(str("queue", "orders")))
		wantSoftFailure(t, out, err, "Could not receive from queue orders")
	})

	t.Run("settle", func(t *testing.T) {
		withReceiver(t, &stubReceiver{
			messages:  []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")},
			settleErr: serviceBusError(azservicebus.CodeLockLost),
		})
		out, err := queue_receive.Execute(nil, nil, authInputs(str("queue", "orders")))
		wantSoftFailure(t, out, err, "could not settle")
	})

	t.Run("bad inputs", func(t *testing.T) {
		withReceiver(t, &stubReceiver{})
		out, err := queue_receive.Execute(nil, nil, authInputs())
		wantSoftFailure(t, out, err, "queue is required")

		out, err = queue_receive.Execute(nil, nil, authInputs(str("queue", "orders"), str("receive_mode", "nonsense")))
		wantSoftFailure(t, out, err, "receive_mode must be")

		out, err = queue_receive.Execute(nil, nil, authInputs(str("queue", "orders"), str("disposition", "reject")))
		wantSoftFailure(t, out, err, "disposition must be")
	})
}

func TestSubscriptionReceive(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}}
	withReceiver(t, r)

	out, err := subscription_receive.Execute(nil, nil, authInputs(
		str("topic", "order-events"),
		str("subscription", "billing"),
	))
	wantResults(t, out, err, 1)

	// Messages live on the subscription, never on the topic — a different
	// constructor, hence a different spec.
	if r.spec.Topic != "order-events" || r.spec.Subscription != "billing" || r.spec.Queue != "" {
		t.Errorf("receiver spec = %+v, want the topic subscription", r.spec)
	}
}

func TestSubscriptionReceiveRequiresBothNames(t *testing.T) {
	withReceiver(t, &stubReceiver{})

	out, err := subscription_receive.Execute(nil, nil, authInputs(str("subscription", "billing")))
	wantSoftFailure(t, out, err, "topic is required")

	out, err = subscription_receive.Execute(nil, nil, authInputs(str("topic", "order-events")))
	wantSoftFailure(t, out, err, "subscription is required")
}

func TestQueuePeekDoesNotSettle(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}}
	withReceiver(t, r)

	out, err := queue_peek.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		integer("max_messages", 3),
	))
	wantResults(t, out, err, 1)

	if len(r.settled) != 0 {
		t.Errorf("a peek settled a message with %q — peek is the safe question, it must consume nothing", r.settled[0].action)
	}
	if r.maxAsked != 3 {
		t.Errorf("asked to peek %d, want 3", r.maxAsked)
	}
	if r.peekOpts == nil || r.peekOpts.FromSequenceNumber != nil {
		t.Error("an unset from_sequence_number must leave the SDK to continue from the last peek")
	}
}

func TestQueuePeekFromASequenceNumber(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}}
	withReceiver(t, r)

	out, err := queue_peek.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		str("from_sequence_number", "77"),
	))
	wantResults(t, out, err, 1)

	if r.peekOpts.FromSequenceNumber == nil || *r.peekOpts.FromSequenceNumber != 77 {
		t.Errorf("FromSequenceNumber = %v, want 77", r.peekOpts.FromSequenceNumber)
	}
}

func TestQueuePeekReportsFailuresSoftly(t *testing.T) {
	withReceiver(t, &stubReceiver{peekErr: serviceBusError(azservicebus.CodeNotFound)})
	out, err := queue_peek.Execute(nil, nil, authInputs(str("queue", "nope")))
	wantSoftFailure(t, out, err, "Could not peek queue nope")

	withReceiver(t, &stubReceiver{})
	out, err = queue_peek.Execute(nil, nil, authInputs(str("queue", "orders"), str("from_sequence_number", "abc")))
	wantSoftFailure(t, out, err, "not a sequence number")
}

func TestSubscriptionPeek(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}}
	withReceiver(t, r)

	out, err := subscription_peek.Execute(nil, nil, authInputs(
		str("topic", "order-events"),
		str("subscription", "billing"),
	))
	wantResults(t, out, err, 1)

	if r.spec.Topic != "order-events" || r.spec.Subscription != "billing" {
		t.Errorf("receiver spec = %+v", r.spec)
	}
	if len(r.settled) != 0 {
		t.Error("a peek settled a message")
	}
}

// The dead-letter queue is a SUB-QUEUE, not a name: "<queue>/$deadletterqueue"
// is accepted as an entity name and then reported as not found, which is
// exactly how this mistake survives review.
func TestDeadLetterReceiveAddressesTheSubQueue(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}}
	withReceiver(t, r)

	out, err := deadletter_receive.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantResults(t, out, err, 1)

	if r.spec.SubQueue != azservicebus.SubQueueDeadLetter {
		t.Errorf("SubQueue = %v, want SubQueueDeadLetter", r.spec.SubQueue)
	}
	if r.spec.Queue != "orders" {
		t.Errorf("Queue = %q, want the plain queue name — the sub-queue is selected by option, not by name", r.spec.Queue)
	}
}

// The transfer DLQ is a distinct, separate place messages hide: the ones that
// failed auto-forwarding.
func TestDeadLetterReceiveCanTargetTheTransferQueue(t *testing.T) {
	r := &stubReceiver{}
	withReceiver(t, r)

	out, err := deadletter_receive.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		str("sub_queue", "transfer"),
	))
	wantResults(t, out, err, 0)

	if r.spec.SubQueue != azservicebus.SubQueueTransfer {
		t.Errorf("SubQueue = %v, want SubQueueTransfer", r.spec.SubQueue)
	}
}

func TestDeadLetterReceiveOnASubscription(t *testing.T) {
	r := &stubReceiver{}
	withReceiver(t, r)

	out, err := deadletter_receive.Execute(nil, nil, authInputs(
		str("entity_type", "subscription"),
		str("topic", "order-events"),
		str("subscription", "billing"),
	))
	wantResults(t, out, err, 0)

	if r.spec.Topic != "order-events" || r.spec.Subscription != "billing" || r.spec.SubQueue != azservicebus.SubQueueDeadLetter {
		t.Errorf("receiver spec = %+v", r.spec)
	}
}

func TestDeadLetterReceiveValidatesTheEntitySwitch(t *testing.T) {
	withReceiver(t, &stubReceiver{})

	out, err := deadletter_receive.Execute(nil, nil, authInputs())
	wantSoftFailure(t, out, err, "queue is required")

	out, err = deadletter_receive.Execute(nil, nil, authInputs(str("entity_type", "subscription")))
	wantSoftFailure(t, out, err, "topic is required")

	out, err = deadletter_receive.Execute(nil, nil, authInputs(str("entity_type", "elsewhere")))
	wantSoftFailure(t, out, err, "entity_type must be")
}

// The diagnostic payload is the whole reason the DLQ actions exist.
func TestDeadLetterPeekSurfacesTheReason(t *testing.T) {
	m := receivedMessage("m-1", "x")
	m.DeadLetterReason = ptr("MaxDeliveryCountExceeded")
	m.DeadLetterErrorDescription = ptr("gave up after 10 tries")
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{m}}
	withReceiver(t, r)

	out, err := deadletter_peek.Execute(nil, nil, authInputs(str("queue", "orders")))
	items := wantResults(t, out, err, 1)

	if r.spec.SubQueue != azservicebus.SubQueueDeadLetter {
		t.Errorf("SubQueue = %v, want SubQueueDeadLetter", r.spec.SubQueue)
	}
	if len(r.settled) != 0 {
		t.Error("a peek settled a dead-lettered message")
	}
	first, _ := items[0].(map[string]interface{})
	if first["dead_letter_reason"] != "MaxDeliveryCountExceeded" {
		t.Errorf("dead_letter_reason = %v — 'why did my messages vanish' is what this action is for", first["dead_letter_reason"])
	}
}

func TestMessageDeadLetterAlwaysDeadLetters(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{
		receivedMessage("m-1", "x"),
		receivedMessage("m-2", "y"),
	}}
	withReceiver(t, r)

	out, err := message_dead_letter.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		integer("max_messages", 2),
		str("dead_letter_reason", "ValidationFailed"),
		str("dead_letter_error_description", "no customer reference"),
	))
	wantResults(t, out, err, 2)

	// Peek-lock is not a choice here: a ReceiveAndDelete message has no lock
	// to dead-letter with.
	if r.spec.Mode != azservicebus.ReceiveModePeekLock {
		t.Error("dead-lettering must receive in peek-lock mode")
	}
	if r.spec.SubQueue != 0 {
		t.Error("dead-lettering must read the live entity, not its dead-letter queue")
	}
	if len(r.settled) != 2 {
		t.Fatalf("settled %d, want 2", len(r.settled))
	}
	for _, s := range r.settled {
		if s.action != "dead_letter" || s.reason != "ValidationFailed" {
			t.Errorf("settled = %+v, want a dead-letter with the reason", s)
		}
	}
}

func TestMessageDeadLetterReportsFailuresSoftly(t *testing.T) {
	withReceiver(t, &stubReceiver{
		messages:  []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")},
		settleErr: serviceBusError(azservicebus.CodeLockLost),
	})
	out, err := message_dead_letter.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantSoftFailure(t, out, err, "could not dead-letter them")
}

// Deferral is the one handoff that survives a process boundary: sequence
// numbers are durable where lock tokens are not.
func TestQueueReceiveDeferred(t *testing.T) {
	r := &stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}}
	withReceiver(t, r)

	out, err := queue_receive_deferred.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		str("sequence_numbers", "[42,43]"),
	))
	wantResults(t, out, err, 1)

	if len(r.deferredFor) != 2 || r.deferredFor[0] != 42 {
		t.Errorf("asked for %v, want [42 43]", r.deferredFor)
	}
	if len(r.settled) != 1 || r.settled[0].action != "complete" {
		t.Errorf("settled = %+v, want a complete", r.settled)
	}
}

func TestQueueReceiveDeferredReportsFailuresSoftly(t *testing.T) {
	withReceiver(t, &stubReceiver{})
	out, err := queue_receive_deferred.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantSoftFailure(t, out, err, "sequence_numbers is required")

	withReceiver(t, &stubReceiver{deferredErr: serviceBusError(azservicebus.CodeNotFound)})
	out, err = queue_receive_deferred.Execute(nil, nil, authInputs(
		str("queue", "orders"), str("sequence_numbers", "42")))
	wantSoftFailure(t, out, err, "Could not receive deferred messages")
}

func TestSessionReceive(t *testing.T) {
	r := &stubSessionReceiver{
		stubReceiver: stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}},
		sessionID:    "cust-7",
		lockedUntil:  time.Date(2026, 7, 17, 9, 5, 0, 0, time.UTC),
		state:        []byte("step-2"),
	}
	accepted := withSessionReceiver(t, r, nil)

	out, err := session_receive.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		str("session_id", "cust-7"),
	))
	wantResults(t, out, err, 1)

	if *accepted != "cust-7" {
		t.Errorf("accepted session %q, want cust-7", *accepted)
	}
	if out["session_id"] != "cust-7" {
		t.Errorf("session_id = %v", out["session_id"])
	}
	if out["session_state"] != "step-2" {
		t.Errorf("session_state = %v", out["session_state"])
	}
	if out["locked_until"] != "2026-07-17T09:05:00Z" {
		t.Errorf("locked_until = %v — the session lock expires like a message lock, so the flow needs it", out["locked_until"])
	}
	if len(r.settled) != 1 || r.settled[0].action != "complete" {
		t.Errorf("settled = %+v", r.settled)
	}
}

// A blank session ID means "the next available one", which is a different SDK
// call and the reason the action does not simply require the field.
func TestSessionReceiveAcceptsTheNextSession(t *testing.T) {
	r := &stubSessionReceiver{sessionID: "whichever"}
	accepted := withSessionReceiver(t, r, nil)

	out, err := session_receive.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantResults(t, out, err, 0)

	if *accepted != "" {
		t.Errorf("accepted %q, want the empty next-session request", *accepted)
	}
	if out["session_id"] != "whichever" {
		t.Errorf("session_id = %v, want the session the broker handed us", out["session_id"])
	}
}

func TestSessionReceiveSetsSessionState(t *testing.T) {
	r := &stubSessionReceiver{
		stubReceiver: stubReceiver{messages: []*azservicebus.ReceivedMessage{receivedMessage("m-1", "x")}},
		sessionID:    "cust-7",
	}
	withSessionReceiver(t, r, nil)

	out, err := session_receive.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		text("session_state", "step-3"),
	))
	wantResults(t, out, err, 1)

	if string(r.setState) != "step-3" {
		t.Errorf("setState = %q, want step-3", r.setState)
	}
	if out["session_state"] != "step-3" {
		t.Errorf("session_state = %v, want the state read back after the write", out["session_state"])
	}
}

// An ordinary receive against a session-enabled entity fails, and the flag is
// immutable — so the error has to say what to do rather than what happened.
func TestSessionReceiveExplainsAMissingSession(t *testing.T) {
	withSessionReceiver(t, &stubSessionReceiver{}, serviceBusError(azservicebus.CodeTimeout))

	out, err := session_receive.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantSoftFailure(t, out, err, "Requires Session")

	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "cannot be added later") {
		t.Errorf("error %q does not say the flag is immutable", msg)
	}
}

func TestSessionReceiveOnASubscription(t *testing.T) {
	r := &stubSessionReceiver{sessionID: "s"}
	withSessionReceiver(t, r, nil)

	out, err := session_receive.Execute(nil, nil, authInputs(
		str("entity_type", "subscription"),
		str("topic", "order-events"),
		str("subscription", "billing"),
	))
	wantResults(t, out, err, 0)

	if r.spec.Topic != "order-events" || r.spec.Subscription != "billing" {
		t.Errorf("receiver spec = %+v", r.spec)
	}
}

func TestSessionReceiveReportsStateFailuresSoftly(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		withSessionReceiver(t, &stubSessionReceiver{sessionID: "s", stateErr: errors.New("boom")}, nil)
		out, err := session_receive.Execute(nil, nil, authInputs(str("queue", "orders")))
		wantSoftFailure(t, out, err, "could not read the session state")
	})

	t.Run("set", func(t *testing.T) {
		withSessionReceiver(t, &stubSessionReceiver{sessionID: "s", setStateErr: errors.New("boom")}, nil)
		out, err := session_receive.Execute(nil, nil, authInputs(
			str("queue", "orders"), text("session_state", "x")))
		wantSoftFailure(t, out, err, "could not set the session state")
	})
}
