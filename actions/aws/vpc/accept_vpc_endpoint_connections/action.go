// Package aws_vpc_accept_vpc_endpoint_connections accepts one or more consumer
// connection requests to a VPC endpoint service you own.
package aws_vpc_accept_vpc_endpoint_connections

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
	Name         = "AWS VPC Accept Endpoint Connections"
	Description  = "Accept consumer connection requests to a VPC endpoint service you own."
	Website      = "https://www.flomation.co"
	Icon         = "link+arrow-right-arrow-left"
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
	{Name: "service_id", Type: core.ConnectionTypeString, Label: "Service ID", Placeholder: "vpce-svc-0abc", Required: true},
	{Name: "vpc_endpoint_id", Type: core.ConnectionTypeString, Label: "VPC Endpoint IDs", Placeholder: "Comma-separated, e.g. vpce-0abc,vpce-0def", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "unsuccessful", Type: core.ConnectionTypeObject, Label: "Unsuccessful"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	serviceID := strings.TrimSpace(awscommon.InputString("service_id", inputs))
	if serviceID == "" {
		return nil, fmt.Errorf("service_id is required")
	}
	endpointIDs := awscommon.InputStrings("vpc_endpoint_id", inputs)
	if len(endpointIDs) == 0 {
		return nil, fmt.Errorf("vpc_endpoint_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.AcceptVpcEndpointConnections(ctx, &ec2.AcceptVpcEndpointConnectionsInput{
		ServiceId:      aws.String(serviceID),
		VpcEndpointIds: endpointIDs,
	})
	if err != nil {
		return nil, err
	}

	unsuccessful := flattenUnsuccessful(out.Unsuccessful)
	ok := len(unsuccessful) == 0
	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Accepted %d connection(s) to %s, %d unsuccessful", len(endpointIDs)-len(unsuccessful), serviceID, len(unsuccessful)),
		"unsuccessful": unsuccessful,
		"success":      ok,
	}, nil
}

func flattenUnsuccessful(items []ec2types.UnsuccessfulItem) []map[string]interface{} {
	var out []map[string]interface{}
	for i := range items {
		it := &items[i]
		entry := map[string]interface{}{
			"resource_id": aws.ToString(it.ResourceId),
		}
		if it.Error != nil {
			entry["code"] = aws.ToString(it.Error.Code)
			entry["message"] = aws.ToString(it.Error.Message)
		}
		out = append(out, entry)
	}
	return out
}
