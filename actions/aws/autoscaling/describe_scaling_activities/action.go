// Package aws_autoscaling_describe_scaling_activities lists scaling activities.
package aws_autoscaling_describe_scaling_activities

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Auto Scaling Describe Activities"
	Description  = "List recent scaling activities for an Auto Scaling group."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-up-down+list"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

const maxActivities = 50

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
		{Name: "Managed Role (Credential)", Value: "credential"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "AWS Role Credential", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"credential"}}},
	{Name: "auto_scaling_group_name", Type: core.ConnectionTypeString, Label: "Auto Scaling Group Name (optional)", Placeholder: "my-asg"},
	{Name: "activity_ids", Type: core.ConnectionTypeString, Label: "Activity IDs (comma-separated, optional)", Placeholder: "abc-123,def-456"},
	{Name: "max_records", Type: core.ConnectionTypeInteger, Label: "Max Records", Placeholder: "50"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "activities", Type: core.ConnectionTypeString, Label: "Activities (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.DescribeScalingActivitiesInput{}
	if g := awscommon.InputString("auto_scaling_group_name", inputs); g != "" {
		in.AutoScalingGroupName = aws.String(g)
	}
	if ids := awscommon.InputStrings("activity_ids", inputs); len(ids) > 0 {
		in.ActivityIds = ids
	}
	if n, ok := awscommon.InputInt("max_records", inputs); ok {
		in.MaxRecords = aws.Int32(int32(n))
	}

	var activities []map[string]interface{}
	paginator := autoscaling.NewDescribeScalingActivitiesPaginator(client, in)
	for paginator.HasMorePages() && len(activities) < maxActivities {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, a := range page.Activities {
			if len(activities) >= maxActivities {
				break
			}
			var startTime string
			if a.StartTime != nil {
				startTime = a.StartTime.Format("2006-01-02T15:04:05Z07:00")
			}
			var progress int32
			if a.Progress != nil {
				progress = *a.Progress
			}
			activities = append(activities, map[string]interface{}{
				"activity_id": aws.ToString(a.ActivityId),
				"description": aws.ToString(a.Description),
				"status_code": string(a.StatusCode),
				"progress":    progress,
				"start_time":  startTime,
			})
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d scaling activities", len(activities)),
		"activities":  activities,
		"count":       len(activities),
	}, nil
}
