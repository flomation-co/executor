package scheduling_calcom_slot_reserve

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Reserve Slot"
	Description  = "Temporarily hold a Cal.com time slot so it can't be double-booked while an attendee completes booking."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+lock"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "event_type_id", Type: core.ConnectionTypeInteger, Label: "Event Type ID", Required: true},
	{Name: "slot_start", Type: core.ConnectionTypeString, Label: "Slot Start", Placeholder: "2026-07-10T09:00:00Z (ISO 8601, UTC)", Required: true},
	{Name: "slot_duration", Type: core.ConnectionTypeInteger, Label: "Slot Duration (minutes)", Placeholder: "For variable-length events (optional)"},
	{Name: "reservation_duration", Type: core.ConnectionTypeInteger, Label: "Hold For (minutes)", Placeholder: "How long to hold the slot (default 5)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Reservation UID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Reservation"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, ok := calcom.OptionalInt("event_type_id", inputs)
	if !ok {
		return calcom.ErrorResult("event_type_id is required"), nil
	}
	slotStart, err := calcom.RequiredString("slot_start", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"eventTypeId": id,
		"slotStart":   slotStart,
	}
	calcom.SetIfInt(body, inputs, "slotDuration", "slot_duration")
	calcom.SetIfInt(body, inputs, "reservationDuration", "reservation_duration")

	resp, err := calcom.PostResource(token, "/slots/reservations", calcom.VersionSlots, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	// Reservations identify themselves via reservationUid, which the generic
	// id resolver doesn't know about — surface it explicitly.
	resUID, _ := resp["reservationUid"].(string)
	if resUID == "" {
		if uid, ok := resp["uid"].(string); ok {
			resUID = uid
		}
	}
	return map[string]interface{}{
		"id":          resUID,
		"result":      resp,
		"tool_result": fmt.Sprintf("Reserved slot %s for event type %d", slotStart, id),
		"success":     true,
		"error":       "",
	}, nil
}
