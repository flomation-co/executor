package azureservicebus_test

import (
	"errors"
	"strings"
	"testing"

	core "flomation.app/automate/executor"

	queue_cancel_scheduled "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_cancel_scheduled"
	queue_schedule "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_schedule"
	queue_send "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_send"
	queue_send_batch "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_send_batch"
	topic_send "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/topic_send"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

func TestQueueSend(t *testing.T) {
	sender := &stubSender{}
	withSender(t, sender)

	out, err := queue_send.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		text("body", `{"order":1}`),
		str("message_id", "m-1"),
		str("subject", "order.created"),
	))
	result := wantSuccess(t, out, err)

	if sender.entity != "orders" {
		t.Errorf("sender opened on %q, want orders", sender.entity)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sender.sent))
	}
	if string(sender.sent[0].Body) != `{"order":1}` {
		t.Errorf("body = %q", sender.sent[0].Body)
	}
	if out["id"] != "m-1" {
		t.Errorf("id = %v, want the message ID", out["id"])
	}
	if result["entity"] != "orders" {
		t.Errorf("result echoes %v, want orders — a send returns nothing, so the output must echo what went out", result["entity"])
	}
	if !sender.closed {
		t.Error("the sender was not closed — the AMQP connection would leak for the life of the process")
	}
}

func TestQueueSendRequiresAQueueAndABody(t *testing.T) {
	withSender(t, &stubSender{})

	out, err := queue_send.Execute(nil, nil, authInputs(text("body", "hi")))
	wantSoftFailure(t, out, err, "queue is required")

	out, err = queue_send.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantSoftFailure(t, out, err, "body is required")
}

func TestQueueSendReportsABrokerFailureSoftly(t *testing.T) {
	withSender(t, &stubSender{sendErr: serviceBusError(azservicebus.CodeUnauthorizedAccess)})

	out, err := queue_send.Execute(nil, nil, authInputs(str("queue", "orders"), text("body", "hi")))
	wantSoftFailure(t, out, err, "Could not send to queue orders")
}

func TestQueueSendReportsAConnectFailureSoftly(t *testing.T) {
	withSenderError(t, errors.New("dial tcp: connection refused"))

	out, err := queue_send.Execute(nil, nil, authInputs(str("queue", "orders"), text("body", "hi")))
	wantSoftFailure(t, out, err, "Could not open a sender")
}

// A queue-scoped connection string can only reach its own entity, and the
// runtime failure is an unauthorized-access error that reads like a bad key.
func TestQueueSendRejectsAMismatchedEntityScope(t *testing.T) {
	withSender(t, &stubSender{})

	out, err := queue_send.Execute(nil, nil, entityScopedAuthInputs("orders",
		str("queue", "invoices"), text("body", "hi")))
	wantSoftFailure(t, out, err, "cannot reach")

	out, err = queue_send.Execute(nil, nil, entityScopedAuthInputs("orders",
		str("queue", "orders"), text("body", "hi")))
	wantSuccess(t, out, err)
}

// Nothing in a credential may reach the flow's error output: the connection
// string carries the key inline.
func TestQueueSendRedactsTheCredentialFromErrors(t *testing.T) {
	withSenderError(t, errors.New("could not parse "+testConnString))

	out, err := queue_send.Execute(nil, nil, authInputs(str("queue", "orders"), text("body", "hi")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg, _ := out["error"].(string)
	if strings.Contains(msg, testKey) || strings.Contains(msg, testConnString) {
		t.Fatalf("the credential leaked into the error output: %q", msg)
	}
}

func TestTopicSend(t *testing.T) {
	sender := &stubSender{}
	withSender(t, sender)

	out, err := topic_send.Execute(nil, nil, authInputs(
		str("topic", "order-events"),
		text("body", "hi"),
	))
	wantSuccess(t, out, err)

	if sender.entity != "order-events" {
		t.Errorf("sender opened on %q, want order-events", sender.entity)
	}
}

func TestTopicSendRequiresATopic(t *testing.T) {
	withSender(t, &stubSender{})
	out, err := topic_send.Execute(nil, nil, authInputs(text("body", "hi")))
	wantSoftFailure(t, out, err, "topic is required")
}

func TestQueueSendBatch(t *testing.T) {
	sender := &stubSender{}
	withSender(t, sender)

	out, err := queue_send_batch.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		obj("messages", `[{"body":"first","message_id":"m-1"},"second"]`),
	))
	items := wantResults(t, out, err, 2)

	if len(sender.sent) != 2 {
		t.Fatalf("sent %d messages, want 2", len(sender.sent))
	}
	if string(sender.sent[0].Body) != "first" || string(sender.sent[1].Body) != "second" {
		t.Errorf("bodies = %q, %q", sender.sent[0].Body, sender.sent[1].Body)
	}
	first, _ := items[0].(map[string]interface{})
	if first["message_id"] != "m-1" {
		t.Errorf("echo = %v", first)
	}
}

// AddMessage measuring the encoded message against the remaining envelope is
// the only place we can say WHICH message broke the cap. Losing that would
// leave the operator bisecting the array by hand.
func TestQueueSendBatchNamesTheMessageThatDidNotFit(t *testing.T) {
	sender := &stubSender{batch: &stubBatch{maxMessages: 2}}
	withSender(t, sender)

	out, err := queue_send_batch.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		obj("messages", `["one","two","three"]`),
	))
	wantSoftFailure(t, out, err, "messages[2]")

	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "256KB") {
		t.Errorf("error %q does not explain the cap", msg)
	}
	if !strings.Contains(msg, "2 message(s) had already been added") {
		t.Errorf("error %q does not say how much of the batch fit", msg)
	}
	if len(sender.sent) != 0 {
		t.Error("an over-large batch was sent anyway")
	}
}

func TestQueueSendBatchRejectsABadArray(t *testing.T) {
	withSender(t, &stubSender{})

	cases := []struct {
		name     string
		messages *core.Connection
		contains string
	}{
		{"not an array", obj("messages", `{"body":"one"}`), "non-empty JSON array"},
		{"empty", obj("messages", `[]`), "non-empty JSON array"},
		{"missing", nil, "non-empty JSON array"},
		{"bad json", obj("messages", `[{`), "valid JSON"},
		{"element with no body", obj("messages", `[{"subject":"x"}]`), "messages[0] has no body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := authInputs(str("queue", "orders"))
			if tc.messages != nil {
				inputs = append(inputs, tc.messages)
			}
			out, err := queue_send_batch.Execute(nil, nil, inputs)
			wantSoftFailure(t, out, err, tc.contains)
		})
	}
}

func TestQueueSchedule(t *testing.T) {
	sender := &stubSender{scheduleSeq: []int64{101, 102}}
	withSender(t, sender)

	out, err := queue_schedule.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		str("scheduled_enqueue_time", "2026-07-17T09:30:00Z"),
		text("body", "later"),
	))
	result := wantSuccess(t, out, err)

	if sender.scheduleAt.UTC().Format("2006-01-02T15:04:05Z") != "2026-07-17T09:30:00Z" {
		t.Errorf("scheduled at %v", sender.scheduleAt)
	}
	// Without the sequence numbers the message can never be cancelled, so they
	// are the point of the action, not a detail of it.
	if out["id"] != "101" {
		t.Errorf("id = %v, want the first sequence number", out["id"])
	}
	nums, ok := out["sequence_numbers"].([]interface{})
	if !ok || len(nums) != 2 || nums[0] != int64(101) {
		t.Fatalf("sequence_numbers = %v, want both numbers on a dedicated output", out["sequence_numbers"])
	}
	if _, present := result["sequence_numbers"]; !present {
		t.Error("the result object should carry the sequence numbers too")
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "101") {
		t.Errorf("tool_result = %q, want it to quote the sequence number", summary)
	}
}

func TestQueueScheduleRequiresAValidTime(t *testing.T) {
	withSender(t, &stubSender{scheduleSeq: []int64{1}})

	out, err := queue_schedule.Execute(nil, nil, authInputs(str("queue", "orders"), text("body", "x")))
	wantSoftFailure(t, out, err, "scheduled_enqueue_time is required")

	out, err = queue_schedule.Execute(nil, nil, authInputs(
		str("queue", "orders"), text("body", "x"), str("scheduled_enqueue_time", "tomorrow")))
	wantSoftFailure(t, out, err, "RFC3339")
}

// A schedule with no sequence number back is an uncancellable message. Saying
// so is better than reporting a success the operator cannot act on.
func TestQueueScheduleReportsAMissingSequenceNumber(t *testing.T) {
	withSender(t, &stubSender{scheduleSeq: nil})

	out, err := queue_schedule.Execute(nil, nil, authInputs(
		str("queue", "orders"), text("body", "x"), str("scheduled_enqueue_time", "2026-07-17T09:30:00Z")))
	wantSoftFailure(t, out, err, "cannot be cancelled later")
}

func TestQueueScheduleReportsABrokerFailure(t *testing.T) {
	withSender(t, &stubSender{scheduleErr: serviceBusError(azservicebus.CodeNotFound)})

	out, err := queue_schedule.Execute(nil, nil, authInputs(
		str("queue", "orders"), text("body", "x"), str("scheduled_enqueue_time", "2026-07-17T09:30:00Z")))
	wantSoftFailure(t, out, err, "Could not schedule a message on queue orders")
}

func TestQueueCancelScheduled(t *testing.T) {
	sender := &stubSender{}
	withSender(t, sender)

	out, err := queue_cancel_scheduled.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		str("sequence_numbers", "[101,102]"),
	))
	wantSuccess(t, out, err)

	if len(sender.cancelled) != 2 || sender.cancelled[0] != 101 {
		t.Errorf("cancelled = %v, want [101 102]", sender.cancelled)
	}
}

func TestQueueCancelScheduledRejectsBadSequenceNumbers(t *testing.T) {
	withSender(t, &stubSender{})

	out, err := queue_cancel_scheduled.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantSoftFailure(t, out, err, "sequence_numbers is required")

	out, err = queue_cancel_scheduled.Execute(nil, nil, authInputs(
		str("queue", "orders"), str("sequence_numbers", "the first one")))
	wantSoftFailure(t, out, err, "not a sequence number")
}

func TestQueueCancelScheduledReportsABrokerFailure(t *testing.T) {
	withSender(t, &stubSender{cancelErr: serviceBusError(azservicebus.CodeNotFound)})

	out, err := queue_cancel_scheduled.Execute(nil, nil, authInputs(
		str("queue", "orders"), str("sequence_numbers", "101")))
	wantSoftFailure(t, out, err, "Could not cancel scheduled messages on queue orders")
}
