package asana_task_comment_remove

import (
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Remove Comment"
	Description  = "Delete a comment (story) from an Asana task."
	Website      = "https://www.flomation.co"
	Icon         = "asana+trash"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID", Placeholder: "The ID of the comment (story) to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Comment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := asana.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	commentID, err := asana.RequiredString("comment_id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	if err := asana.DeleteResource(auth, "/stories/"+url.PathEscape(commentID)); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.SuccessResult(commentID, nil, "Deleted comment "+commentID), nil
}
