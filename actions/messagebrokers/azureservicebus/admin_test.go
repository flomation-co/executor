package azureservicebus_test

import (
	"strings"
	"testing"
	"time"

	core "flomation.app/automate/executor"

	namespace_get "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/namespace_get"
	queue_create "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_create"
	queue_delete "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_delete"
	queue_get "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_get"
	queue_list "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_list"
	queue_runtime_properties "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_runtime_properties"
	queue_update "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/queue_update"
	rule_create "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/rule_create"
	rule_delete "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/rule_delete"
	rule_list "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/rule_list"
	subscription_create "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_create"
	subscription_delete "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_delete"
	subscription_list "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_list"
	subscription_runtime_properties "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/subscription_runtime_properties"
	topic_create "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/topic_create"
	topic_delete "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/topic_delete"
	topic_list "flomation.app/automate/executor/actions/messagebrokers/azureservicebus/topic_list"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

// The EntityPath gotcha, runtime half. A queue-scoped connection string cannot
// drive the management API at all, and the downstream failure is an
// unrelated-looking 401 — so every admin action must refuse it up front, with
// the entity named.
func TestManagementActionsRejectAnEntityScopedConnectionString(t *testing.T) {
	withAdmin(t, &stubAdmin{})

	cases := map[string]func() (map[string]interface{}, error){
		"queue_list": func() (map[string]interface{}, error) {
			return queue_list.Execute(nil, nil, entityScopedAuthInputs("orders"))
		},
		"queue_get": func() (map[string]interface{}, error) {
			return queue_get.Execute(nil, nil, entityScopedAuthInputs("orders", str("queue", "orders")))
		},
		"queue_create": func() (map[string]interface{}, error) {
			return queue_create.Execute(nil, nil, entityScopedAuthInputs("orders", str("queue", "orders")))
		},
		"queue_delete": func() (map[string]interface{}, error) {
			return queue_delete.Execute(nil, nil, entityScopedAuthInputs("orders", str("queue", "orders")))
		},
		"topic_list": func() (map[string]interface{}, error) {
			return topic_list.Execute(nil, nil, entityScopedAuthInputs("orders"))
		},
		"namespace_get": func() (map[string]interface{}, error) {
			return namespace_get.Execute(nil, nil, entityScopedAuthInputs("orders"))
		},
		"subscription_list": func() (map[string]interface{}, error) {
			return subscription_list.Execute(nil, nil, entityScopedAuthInputs("orders", str("topic", "order-events")))
		},
		"rule_list": func() (map[string]interface{}, error) {
			return rule_list.Execute(nil, nil, entityScopedAuthInputs("orders", str("topic", "t"), str("subscription", "s")))
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := run()
			wantSoftFailure(t, out, err, "scoped to the entity \"orders\"")
		})
	}
}

func TestQueueList(t *testing.T) {
	a := &stubAdmin{queues: []admin.QueueItem{
		{QueueName: "orders", QueueProperties: admin.QueueProperties{MaxDeliveryCount: ptr(int32(10))}},
		{QueueName: "invoices"},
	}}
	withAdmin(t, a)

	out, err := queue_list.Execute(nil, nil, authInputs(integer("limit", 5)))
	items := wantResults(t, out, err, 2)

	if a.listedLimit != 5 {
		t.Errorf("limit = %d, want 5", a.listedLimit)
	}
	first, _ := items[0].(map[string]interface{})
	if first["name"] != "orders" || first["max_delivery_count"] != 10 {
		t.Errorf("item = %v", first)
	}
}

func TestQueueListClampsTheLimit(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	if _, err := queue_list.Execute(nil, nil, authInputs()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.listedLimit != 50 {
		t.Errorf("default limit = %d, want 50", a.listedLimit)
	}

	if _, err := queue_list.Execute(nil, nil, authInputs(integer("limit", 9999))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.listedLimit != 100 {
		t.Errorf("limit = %d, want the 100 cap — the management plane is rate limited hard", a.listedLimit)
	}
}

func TestQueueListReportsAFailureSoftly(t *testing.T) {
	withAdmin(t, &stubAdmin{err: serviceBusError(azservicebus.CodeUnauthorizedAccess)})
	out, err := queue_list.Execute(nil, nil, authInputs())
	wantSoftFailure(t, out, err, "Could not list queues")
}

func TestQueueGet(t *testing.T) {
	status := admin.EntityStatusActive
	a := &stubAdmin{queue: admin.QueueProperties{
		LockDuration:     ptr("PT1M"),
		MaxDeliveryCount: ptr(int32(10)),
		RequiresSession:  ptr(true),
		Status:           &status,
	}}
	withAdmin(t, a)

	out, err := queue_get.Execute(nil, nil, authInputs(str("queue", "orders")))
	result := wantSuccess(t, out, err)

	if result["lock_duration"] != "PT1M" || result["requires_session"] != true || result["status"] != "Active" {
		t.Errorf("result = %v", result)
	}
	if out["id"] != "orders" {
		t.Errorf("id = %v", out["id"])
	}
}

func TestQueueGetReportsFailuresSoftly(t *testing.T) {
	withAdmin(t, &stubAdmin{})
	out, err := queue_get.Execute(nil, nil, authInputs())
	wantSoftFailure(t, out, err, "queue is required")

	withAdmin(t, &stubAdmin{err: serviceBusError(azservicebus.CodeNotFound)})
	out, err = queue_get.Execute(nil, nil, authInputs(str("queue", "nope")))
	wantSoftFailure(t, out, err, "Could not get queue nope")
}

// The read a monitoring flow is built on: a rising dead-letter count is
// usually the first sign anything is wrong, so it gets a first-class output
// rather than living only inside the result object.
func TestQueueRuntimeProperties(t *testing.T) {
	withAdmin(t, &stubAdmin{queueRuntime: admin.QueueRuntimeProperties{
		ActiveMessageCount:     7,
		DeadLetterMessageCount: 3,
		ScheduledMessageCount:  1,
		SizeInBytes:            2048,
		CreatedAt:              time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}})

	out, err := queue_runtime_properties.Execute(nil, nil, authInputs(str("queue", "orders")))
	result := wantSuccess(t, out, err)

	if out["active_message_count"] != 7 || out["dead_letter_message_count"] != 3 {
		t.Errorf("the counts must be first-class outputs a flow can branch on: %v", out)
	}
	if result["size_in_bytes"] != int64(2048) || result["scheduled_message_count"] != 1 {
		t.Errorf("result = %v", result)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "3 dead-lettered") {
		t.Errorf("tool_result = %q, want the counts quoted", summary)
	}
}

func TestQueueCreate(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := queue_create.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		integer("lock_duration_seconds", 90),
		integer("max_delivery_count", 5),
		integer("default_message_time_to_live_seconds", 3600),
		boolean("requires_session", true),
		boolean("dead_lettering_on_message_expiration", true),
		str("user_metadata", "built by a flow"),
	))
	wantSuccess(t, out, err)

	props := a.createdQueue
	if props == nil {
		t.Fatal("no properties reached CreateQueue")
	}
	// The management plane takes ISO-8601 duration strings; the operator gives
	// us seconds.
	if props.LockDuration == nil || *props.LockDuration != "PT1M30S" {
		t.Errorf("LockDuration = %v, want PT1M30S", props.LockDuration)
	}
	if props.DefaultMessageTimeToLive == nil || *props.DefaultMessageTimeToLive != "PT1H" {
		t.Errorf("DefaultMessageTimeToLive = %v, want PT1H", props.DefaultMessageTimeToLive)
	}
	if props.MaxDeliveryCount == nil || *props.MaxDeliveryCount != 5 {
		t.Errorf("MaxDeliveryCount = %v", props.MaxDeliveryCount)
	}
	if props.RequiresSession == nil || !*props.RequiresSession {
		t.Errorf("RequiresSession = %v", props.RequiresSession)
	}
	if props.UserMetadata == nil || *props.UserMetadata != "built by a flow" {
		t.Errorf("UserMetadata = %v", props.UserMetadata)
	}
}

// An untouched checkbox must stay nil, not become false: nil means "use the
// service default", and false is a decision the operator did not make.
func TestQueueCreateLeavesUnsetPropertiesToTheService(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := queue_create.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantSuccess(t, out, err)

	props := a.createdQueue
	if props.LockDuration != nil || props.MaxDeliveryCount != nil || props.RequiresSession != nil ||
		props.RequiresDuplicateDetection != nil || props.UserMetadata != nil {
		t.Errorf("unset inputs became explicit values: %+v", props)
	}
}

func TestQueueCreateReportsFailuresSoftly(t *testing.T) {
	withAdmin(t, &stubAdmin{})
	out, err := queue_create.Execute(nil, nil, authInputs())
	wantSoftFailure(t, out, err, "queue is required")

	withAdmin(t, &stubAdmin{err: serviceBusError(azservicebus.CodeUnauthorizedAccess)})
	out, err = queue_create.Execute(nil, nil, authInputs(str("queue", "orders")))
	wantSoftFailure(t, out, err, "Could not create queue orders")
}

// UpdateQueue replaces the entity's properties wholesale. Sending a
// partially-filled struct would silently reset every property left blank to
// its default — including MaxDeliveryCount, which would quietly change when
// messages get dead-lettered.
func TestQueueUpdateIsReadModifyWrite(t *testing.T) {
	a := &stubAdmin{queue: admin.QueueProperties{
		LockDuration:     ptr("PT1M"),
		MaxDeliveryCount: ptr(int32(10)),
		UserMetadata:     ptr("existing note"),
		RequiresSession:  ptr(true),
	}}
	withAdmin(t, a)

	out, err := queue_update.Execute(nil, nil, authInputs(
		str("queue", "orders"),
		integer("max_delivery_count", 3),
	))
	wantSuccess(t, out, err)

	props := a.updatedQueue
	if props == nil {
		t.Fatal("no properties reached UpdateQueue")
	}
	if props.MaxDeliveryCount == nil || *props.MaxDeliveryCount != 3 {
		t.Errorf("MaxDeliveryCount = %v, want the new value", props.MaxDeliveryCount)
	}
	if props.LockDuration == nil || *props.LockDuration != "PT1M" {
		t.Errorf("LockDuration = %v — an untouched property must survive the update", props.LockDuration)
	}
	if props.UserMetadata == nil || *props.UserMetadata != "existing note" {
		t.Errorf("UserMetadata = %v — an untouched property must survive the update", props.UserMetadata)
	}
	if props.RequiresSession == nil || !*props.RequiresSession {
		t.Errorf("RequiresSession = %v — the immutable flags must be carried through unchanged", props.RequiresSession)
	}
}

func TestQueueUpdateSetsStatus(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := queue_update.Execute(nil, nil, authInputs(str("queue", "orders"), str("status", "Disabled")))
	wantSuccess(t, out, err)

	if a.updatedQueue.Status == nil || *a.updatedQueue.Status != admin.EntityStatusDisabled {
		t.Errorf("Status = %v, want Disabled", a.updatedQueue.Status)
	}
}

func TestQueueUpdateFailsBeforeWritingIfTheReadFails(t *testing.T) {
	a := &stubAdmin{err: serviceBusError(azservicebus.CodeNotFound)}
	withAdmin(t, a)

	out, err := queue_update.Execute(nil, nil, authInputs(str("queue", "nope")))
	wantSoftFailure(t, out, err, "Could not read queue nope before updating it")

	if a.updatedQueue != nil {
		t.Error("the update went ahead on properties that were never read — it would reset the queue to defaults")
	}
}

func TestQueueDelete(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := queue_delete.Execute(nil, nil, authInputs(str("queue", "orders")))
	result := wantSuccess(t, out, err)

	if a.deletedQueue != "orders" {
		t.Errorf("deleted %q, want orders", a.deletedQueue)
	}
	if result["deleted"] != true {
		t.Errorf("result = %v", result)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "every message in it") {
		t.Errorf("tool_result = %q, want it to say plainly that the messages went too", summary)
	}
}

func TestQueueDeleteReportsFailuresSoftly(t *testing.T) {
	withAdmin(t, &stubAdmin{})
	out, err := queue_delete.Execute(nil, nil, authInputs())
	wantSoftFailure(t, out, err, "queue is required")

	withAdmin(t, &stubAdmin{err: serviceBusError(azservicebus.CodeNotFound)})
	out, err = queue_delete.Execute(nil, nil, authInputs(str("queue", "nope")))
	wantSoftFailure(t, out, err, "Could not delete queue nope")
}

func TestTopicList(t *testing.T) {
	withAdmin(t, &stubAdmin{topics: []admin.TopicItem{
		{TopicName: "order-events", TopicProperties: admin.TopicProperties{SupportOrdering: ptr(true)}},
	}})

	out, err := topic_list.Execute(nil, nil, authInputs())
	items := wantResults(t, out, err, 1)

	first, _ := items[0].(map[string]interface{})
	if first["name"] != "order-events" || first["support_ordering"] != true {
		t.Errorf("item = %v", first)
	}
}

// A topic with no subscriptions silently discards every message sent to it and
// reports success, so the create says so.
func TestTopicCreateWarnsAboutTheBlackHole(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := topic_create.Execute(nil, nil, authInputs(
		str("topic", "order-events"),
		integer("default_message_time_to_live_seconds", 60),
		boolean("support_ordering", true),
	))
	wantSuccess(t, out, err)

	if a.createdTopic.DefaultMessageTimeToLive == nil || *a.createdTopic.DefaultMessageTimeToLive != "PT1M" {
		t.Errorf("DefaultMessageTimeToLive = %v", a.createdTopic.DefaultMessageTimeToLive)
	}
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, "add a subscription") {
		t.Errorf("tool_result = %q, want it to warn that a topic with no subscriptions discards everything", summary)
	}
}

func TestTopicCreateReportsFailuresSoftly(t *testing.T) {
	withAdmin(t, &stubAdmin{})
	out, err := topic_create.Execute(nil, nil, authInputs())
	wantSoftFailure(t, out, err, "topic is required")

	withAdmin(t, &stubAdmin{err: serviceBusError(azservicebus.CodeUnauthorizedAccess)})
	out, err = topic_create.Execute(nil, nil, authInputs(str("topic", "t")))
	wantSoftFailure(t, out, err, "Could not create topic t")
}

func TestTopicDelete(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := topic_delete.Execute(nil, nil, authInputs(str("topic", "order-events")))
	wantSuccess(t, out, err)

	if a.deletedTopic != "order-events" {
		t.Errorf("deleted %q", a.deletedTopic)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "subscriptions") {
		t.Errorf("tool_result = %q, want it to say the subscriptions went too", summary)
	}
}

func TestSubscriptionList(t *testing.T) {
	a := &stubAdmin{subscriptions: []admin.SubscriptionPropertiesItem{
		{SubscriptionName: "billing", SubscriptionProperties: admin.SubscriptionProperties{MaxDeliveryCount: ptr(int32(10))}},
	}}
	withAdmin(t, a)

	out, err := subscription_list.Execute(nil, nil, authInputs(str("topic", "order-events")))
	items := wantResults(t, out, err, 1)

	if a.listedSubsTopic != "order-events" {
		t.Errorf("listed subscriptions of %q", a.listedSubsTopic)
	}
	first, _ := items[0].(map[string]interface{})
	if first["name"] != "billing" || first["topic"] != "order-events" {
		t.Errorf("item = %v", first)
	}
}

func TestSubscriptionListRequiresATopic(t *testing.T) {
	withAdmin(t, &stubAdmin{})
	out, err := subscription_list.Execute(nil, nil, authInputs())
	wantSoftFailure(t, out, err, "topic is required")
}

func TestSubscriptionCreate(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := subscription_create.Execute(nil, nil, authInputs(
		str("topic", "order-events"),
		str("subscription", "billing"),
		integer("lock_duration_seconds", 30),
		integer("max_delivery_count", 5),
		boolean("dead_lettering_on_filter_evaluation_exceptions", true),
		str("forward_to", "audit"),
	))
	wantSuccess(t, out, err)

	props := a.createdSub
	if props.LockDuration == nil || *props.LockDuration != "PT30S" {
		t.Errorf("LockDuration = %v", props.LockDuration)
	}
	if props.EnableDeadLetteringOnFilterEvaluationExceptions == nil || !*props.EnableDeadLetteringOnFilterEvaluationExceptions {
		t.Errorf("EnableDeadLetteringOnFilterEvaluationExceptions = %v", props.EnableDeadLetteringOnFilterEvaluationExceptions)
	}
	if props.ForwardTo == nil || *props.ForwardTo != "audit" {
		t.Errorf("ForwardTo = %v", props.ForwardTo)
	}

	// Operators repeatedly add a filter and find it narrows nothing, because
	// $Default is still there matching everything.
	summary, _ := out["tool_result"].(string)
	if !strings.Contains(summary, "$Default") {
		t.Errorf("tool_result = %q, want it to mention the $Default rule", summary)
	}
}

func TestSubscriptionCreateRequiresBothNames(t *testing.T) {
	withAdmin(t, &stubAdmin{})

	out, err := subscription_create.Execute(nil, nil, authInputs(str("subscription", "billing")))
	wantSoftFailure(t, out, err, "topic is required")

	out, err = subscription_create.Execute(nil, nil, authInputs(str("topic", "order-events")))
	wantSoftFailure(t, out, err, "subscription is required")
}

func TestSubscriptionDelete(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := subscription_delete.Execute(nil, nil, authInputs(
		str("topic", "order-events"), str("subscription", "billing")))
	wantSuccess(t, out, err)

	if a.deletedSub != "billing" {
		t.Errorf("deleted %q", a.deletedSub)
	}
}

func TestSubscriptionRuntimeProperties(t *testing.T) {
	withAdmin(t, &stubAdmin{subRuntime: admin.SubscriptionRuntimeProperties{
		ActiveMessageCount:     4,
		DeadLetterMessageCount: 2,
	}})

	out, err := subscription_runtime_properties.Execute(nil, nil, authInputs(
		str("topic", "order-events"), str("subscription", "billing")))
	result := wantSuccess(t, out, err)

	if out["active_message_count"] != 4 || out["dead_letter_message_count"] != 2 {
		t.Errorf("the counts must be first-class outputs: %v", out)
	}
	if result["topic"] != "order-events" {
		t.Errorf("result = %v", result)
	}
}

func TestRuleCreateSQLFilter(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := rule_create.Execute(nil, nil, authInputs(
		str("topic", "order-events"),
		str("subscription", "billing"),
		str("rule_name", "high-value"),
		str("sql_expression", "total > 100"),
		str("action_sql_expression", "SET priority = 'high'"),
	))
	result := wantSuccess(t, out, err)

	filter, ok := a.createdRule.Filter.(*admin.SQLFilter)
	if !ok {
		t.Fatalf("filter = %T, want *admin.SQLFilter", a.createdRule.Filter)
	}
	if filter.Expression != "total > 100" {
		t.Errorf("expression = %q", filter.Expression)
	}
	action, ok := a.createdRule.Action.(*admin.SQLAction)
	if !ok || action.Expression != "SET priority = 'high'" {
		t.Errorf("action = %v", a.createdRule.Action)
	}
	if result["filter_type"] != "sql" || result["filter_expression"] != "total > 100" {
		t.Errorf("result = %v", result)
	}
	if summary, _ := out["tool_result"].(string); !strings.Contains(summary, "$Default") {
		t.Errorf("tool_result = %q, want the $Default reminder — without it the new filter narrows nothing", summary)
	}
}

// A correlation filter is a set of equality tests the broker evaluates far
// more cheaply than SQL, which is why it is worth exposing separately.
func TestRuleCreateCorrelationFilter(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := rule_create.Execute(nil, nil, authInputs(
		str("topic", "order-events"),
		str("subscription", "billing"),
		str("rule_name", "uk-orders"),
		str("filter_type", "correlation"),
		obj("correlation_filter", `{"subject":"order.created","application_properties":{"region":"UK"}}`),
	))
	result := wantSuccess(t, out, err)

	filter, ok := a.createdRule.Filter.(*admin.CorrelationFilter)
	if !ok {
		t.Fatalf("filter = %T, want *admin.CorrelationFilter", a.createdRule.Filter)
	}
	if filter.Subject == nil || *filter.Subject != "order.created" {
		t.Errorf("Subject = %v", filter.Subject)
	}
	if filter.ApplicationProperties["region"] != "UK" {
		t.Errorf("ApplicationProperties = %v", filter.ApplicationProperties)
	}
	if result["filter_type"] != "correlation" {
		t.Errorf("result = %v", result)
	}
}

func TestRuleCreateTrueAndFalseFilters(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  interface{}
	}{
		{"true", &admin.TrueFilter{}},
		{"false", &admin.FalseFilter{}},
	} {
		t.Run(tc.value, func(t *testing.T) {
			a := &stubAdmin{}
			withAdmin(t, a)

			out, err := rule_create.Execute(nil, nil, authInputs(
				str("topic", "t"), str("subscription", "s"), str("rule_name", "r"),
				str("filter_type", tc.value),
			))
			result := wantSuccess(t, out, err)

			if result["filter_type"] != tc.value {
				t.Errorf("filter_type = %v, want %v", result["filter_type"], tc.value)
			}
		})
	}
}

func TestRuleCreateRejectsBadFilters(t *testing.T) {
	withAdmin(t, &stubAdmin{})

	cases := []struct {
		name     string
		inputs   []*core.Connection
		contains string
	}{
		{"sql with no expression", nil, "sql_expression is required"},
		{"correlation with no fields", []*core.Connection{str("filter_type", "correlation")}, "correlation_filter is required"},
		{"correlation with an unknown field", []*core.Connection{
			str("filter_type", "correlation"), obj("correlation_filter", `{"nonsense":"x"}`),
		}, "has no field"},
		{"correlation with a non-string field", []*core.Connection{
			str("filter_type", "correlation"), obj("correlation_filter", `{"subject":7}`),
		}, "must be a string"},
		{"unknown filter type", []*core.Connection{str("filter_type", "regex")}, "filter_type must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := authInputs(str("topic", "t"), str("subscription", "s"), str("rule_name", "r"))
			inputs = append(inputs, tc.inputs...)
			out, err := rule_create.Execute(nil, nil, inputs)
			wantSoftFailure(t, out, err, tc.contains)
		})
	}
}

func TestRuleCreateRequiresTheNames(t *testing.T) {
	withAdmin(t, &stubAdmin{})
	out, err := rule_create.Execute(nil, nil, authInputs(str("topic", "t"), str("subscription", "s")))
	wantSoftFailure(t, out, err, "rule_name is required")
}

// $Default is the reason rule_list exists at all: an operator whose filter
// "does nothing" needs to be able to see it.
func TestRuleListNamesTheFilterTypes(t *testing.T) {
	withAdmin(t, &stubAdmin{rules: []admin.RuleProperties{
		{Name: "$Default", Filter: &admin.TrueFilter{}},
		{Name: "high-value", Filter: &admin.SQLFilter{Expression: "total > 100"}, Action: &admin.SQLAction{Expression: "SET x = 1"}},
	}})

	out, err := rule_list.Execute(nil, nil, authInputs(str("topic", "t"), str("subscription", "s")))
	items := wantResults(t, out, err, 2)

	first, _ := items[0].(map[string]interface{})
	if first["name"] != "$Default" || first["filter_type"] != "true" {
		t.Errorf("the $Default TrueFilter must be visible for what it is: %v", first)
	}
	second, _ := items[1].(map[string]interface{})
	if second["filter_expression"] != "total > 100" || second["action_expression"] != "SET x = 1" {
		t.Errorf("item = %v", second)
	}
}

func TestRuleDelete(t *testing.T) {
	a := &stubAdmin{}
	withAdmin(t, a)

	out, err := rule_delete.Execute(nil, nil, authInputs(
		str("topic", "t"), str("subscription", "s"), str("rule_name", "$Default")))
	wantSuccess(t, out, err)

	if a.deletedRule != "$Default" {
		t.Errorf("deleted %q", a.deletedRule)
	}
}

func TestRuleDeleteReportsFailuresSoftly(t *testing.T) {
	withAdmin(t, &stubAdmin{})
	out, err := rule_delete.Execute(nil, nil, authInputs(str("topic", "t"), str("subscription", "s")))
	wantSoftFailure(t, out, err, "rule_name is required")

	withAdmin(t, &stubAdmin{err: serviceBusError(azservicebus.CodeNotFound)})
	out, err = rule_delete.Execute(nil, nil, authInputs(
		str("topic", "t"), str("subscription", "s"), str("rule_name", "nope")))
	wantSoftFailure(t, out, err, "Could not delete rule nope")
}

// The tier decides the message size cap, and an operator who has read the
// general Service Bus literature will assume 256KB. Saying which one applies
// is what makes this more than a ping.
func TestNamespaceGetReportsTheTierSizeCap(t *testing.T) {
	cases := []struct {
		sku      string
		wantSize int
	}{
		{"Standard", 256 * 1024},
		{"Premium", 100 * 1024 * 1024},
		{"premium", 100 * 1024 * 1024},
		{"Basic", 256 * 1024},
	}
	for _, tc := range cases {
		t.Run(tc.sku, func(t *testing.T) {
			withAdmin(t, &stubAdmin{namespace: admin.NamespaceProperties{Name: "myns", SKU: tc.sku}})

			out, err := namespace_get.Execute(nil, nil, authInputs())
			result := wantSuccess(t, out, err)

			if out["sku"] != tc.sku {
				t.Errorf("sku = %v", out["sku"])
			}
			if out["max_message_bytes"] != tc.wantSize {
				t.Errorf("max_message_bytes = %v, want %d", out["max_message_bytes"], tc.wantSize)
			}
			if result["name"] != "myns" || out["id"] != "myns" {
				t.Errorf("result = %v", result)
			}
		})
	}
}

func TestNamespaceGetReportsAFailureSoftly(t *testing.T) {
	withAdmin(t, &stubAdmin{err: serviceBusError(azservicebus.CodeUnauthorizedAccess)})
	out, err := namespace_get.Execute(nil, nil, authInputs())
	wantSoftFailure(t, out, err, "Could not get the namespace properties")
}
