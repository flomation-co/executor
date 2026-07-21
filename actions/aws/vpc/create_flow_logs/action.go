// Package aws_vpc_create_flow_logs creates VPC flow logs for a VPC, subnet, or ENI.
package aws_vpc_create_flow_logs

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Create Flow Logs"
	Description  = "Create flow logs capturing traffic for a VPC, subnet, or network interface."
	Website      = "https://www.flomation.co"
	Icon         = "list+plus"
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
	{Name: "resource_id", Type: core.ConnectionTypeString, Label: "Resource IDs", Placeholder: "Comma-separated, e.g. vpc-0abc,subnet-0def", Required: true},
	{Name: "resource_type", Type: core.ConnectionTypeString, Label: "Resource Type", Required: true, Options: []core.ConnectionOption{
		{Name: "VPC", Value: "VPC"},
		{Name: "Subnet", Value: "Subnet"},
		{Name: "Network Interface", Value: "NetworkInterface"},
	}},
	{Name: "traffic_type", Type: core.ConnectionTypeString, Label: "Traffic Type", Required: true, Options: []core.ConnectionOption{
		{Name: "Accept", Value: "ACCEPT"},
		{Name: "Reject", Value: "REJECT"},
		{Name: "All", Value: "ALL"},
	}},
	{Name: "log_destination_type", Type: core.ConnectionTypeString, Label: "Log Destination Type", Options: []core.ConnectionOption{
		{Name: "CloudWatch Logs", Value: "cloud-watch-logs"},
		{Name: "S3", Value: "s3"},
	}},
	{Name: "log_group_name", Type: core.ConnectionTypeString, Label: "CloudWatch Log Group Name", Placeholder: "/vpc/flow-logs"},
	{Name: "log_destination", Type: core.ConnectionTypeString, Label: "S3 Destination ARN", Placeholder: "arn:aws:s3:::my-bucket/prefix/"},
	{Name: "deliver_logs_permission_arn", Type: core.ConnectionTypeString, Label: "IAM Role ARN (for CloudWatch Logs)", Placeholder: "arn:aws:iam::<account>:role/flow-logs"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "flow_logs", Type: core.ConnectionTypeObject, Label: "Flow Logs"},
	{Name: "flow_log_id", Type: core.ConnectionTypeString, Label: "First Flow Log ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	resourceIDs := awscommon.InputStrings("resource_id", inputs)
	if len(resourceIDs) == 0 {
		return nil, fmt.Errorf("at least one resource_id is required")
	}
	resourceType := strings.TrimSpace(awscommon.InputString("resource_type", inputs))
	if resourceType == "" {
		return nil, fmt.Errorf("resource_type is required")
	}
	trafficType := strings.TrimSpace(awscommon.InputString("traffic_type", inputs))
	if trafficType == "" {
		return nil, fmt.Errorf("traffic_type is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateFlowLogsInput{
		ResourceIds:  resourceIDs,
		ResourceType: ec2types.FlowLogsResourceType(resourceType),
		TrafficType:  ec2types.TrafficType(trafficType),
	}
	if v := strings.TrimSpace(awscommon.InputString("log_destination_type", inputs)); v != "" {
		in.LogDestinationType = ec2types.LogDestinationType(v)
	}
	if v := strings.TrimSpace(awscommon.InputString("log_group_name", inputs)); v != "" {
		in.LogGroupName = aws.String(v)
	}
	if v := strings.TrimSpace(awscommon.InputString("log_destination", inputs)); v != "" {
		in.LogDestination = aws.String(v)
	}
	if v := strings.TrimSpace(awscommon.InputString("deliver_logs_permission_arn", inputs)); v != "" {
		in.DeliverLogsPermissionArn = aws.String(v)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeVpcFlowLog,
			Tags:         tags,
		}}
	}

	out, err := client.CreateFlowLogs(ctx, in)
	if err != nil {
		return nil, err
	}

	firstID := ""
	if len(out.FlowLogIds) > 0 {
		firstID = out.FlowLogIds[0]
	}
	unsuccessful := make([]map[string]interface{}, 0, len(out.Unsuccessful))
	for i := range out.Unsuccessful {
		u := &out.Unsuccessful[i]
		item := map[string]interface{}{"resource_id": aws.ToString(u.ResourceId)}
		if u.Error != nil {
			item["error"] = aws.ToString(u.Error.Message)
		}
		unsuccessful = append(unsuccessful, item)
	}
	flowLogs := map[string]interface{}{
		"flow_log_ids":       out.FlowLogIds,
		"unsuccessful":       unsuccessful,
		"unsuccessful_count": len(out.Unsuccessful),
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created %d flow log(s), %d failed", len(out.FlowLogIds), len(out.Unsuccessful)),
		"flow_logs":   flowLogs,
		"flow_log_id": firstID,
	}, nil
}

func buildTags(inputs []*core.Connection) []ec2types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []ec2types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
