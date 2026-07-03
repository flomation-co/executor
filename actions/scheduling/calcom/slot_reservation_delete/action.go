package scheduling_calcom_slot_reservation_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Release Reserved Slot"
	Description  = "Release a previously reserved Cal.com slot so it becomes bookable again."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+trash"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "reservation_uid", Type: core.ConnectionTypeString, Label: "Reservation UID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	uid, err := calcom.RequiredString("reservation_uid", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	if err := calcom.DeleteResource(token, "/slots/reservations/"+url.PathEscape(uid), calcom.VersionSlots); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Released slot reservation %s", uid),
		"success":     true,
		"error":       "",
	}, nil
}
