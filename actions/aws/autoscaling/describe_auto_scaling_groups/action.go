// Package aws_autoscaling_describe_auto_scaling_groups lists EC2 Auto Scaling groups.
package aws_autoscaling_describe_auto_scaling_groups

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Describe Auto Scaling Groups"
	Description  = "List EC2 Auto Scaling groups with size and health details."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-up-down+magnifying-glass"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

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
	{Name: "auto_scaling_group_names", Type: core.ConnectionTypeString, Label: "Group Names (comma-separated, optional)", Placeholder: "my-asg,other-asg"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "groups", Type: core.ConnectionTypeString, Label: "Auto Scaling Groups (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.DescribeAutoScalingGroupsInput{}
	var names []string
	for _, p := range strings.Split(awscommon.InputString("auto_scaling_group_names", inputs), ",") {
		if t := strings.TrimSpace(p); t != "" {
			names = append(names, t)
		}
	}
	if len(names) > 0 {
		in.AutoScalingGroupNames = names
	}

	groups := make([]map[string]interface{}, 0)
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range page.AutoScalingGroups {
			groups = append(groups, map[string]interface{}{
				"name":              aws.ToString(g.AutoScalingGroupName),
				"min_size":          aws.ToInt32(g.MinSize),
				"max_size":          aws.ToInt32(g.MaxSize),
				"desired_capacity":  aws.ToInt32(g.DesiredCapacity),
				"instance_count":    len(g.Instances),
				"health_check_type": aws.ToString(g.HealthCheckType),
				"status":            aws.ToString(g.Status),
			})
		}
	}

	groupsJSON, _ := json.Marshal(groups)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d Auto Scaling group(s)", len(groups)),
		"groups":      string(groupsJSON),
		"count":       len(groups),
	}, nil
}
