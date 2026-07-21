// Package aws_vpc_replace_route_table_association swaps the route table for a subnet association.
package aws_vpc_replace_route_table_association

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Replace Route Table Association"
	Description  = "Associate a different route table with an existing subnet association."
	Website      = "https://www.flomation.co"
	Icon         = "route+link"
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
	{Name: "association_id", Type: core.ConnectionTypeString, Label: "Association ID", Placeholder: "rtbassoc-0abc...", Required: true},
	{Name: "route_table_id", Type: core.ConnectionTypeString, Label: "New Route Table ID", Placeholder: "rtb-0abc...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "new_association_id", Type: core.ConnectionTypeString, Label: "New Association ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	assocID := strings.TrimSpace(awscommon.InputString("association_id", inputs))
	if assocID == "" {
		return nil, fmt.Errorf("association_id is required")
	}
	rtID := strings.TrimSpace(awscommon.InputString("route_table_id", inputs))
	if rtID == "" {
		return nil, fmt.Errorf("route_table_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.ReplaceRouteTableAssociation(ctx, &ec2.ReplaceRouteTableAssociationInput{
		AssociationId: aws.String(assocID),
		RouteTableId:  aws.String(rtID),
	})
	if err != nil {
		return nil, err
	}

	newID := aws.ToString(out.NewAssociationId)

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Replaced association %s → route table %s (new association %s)", assocID, rtID, newID),
		"new_association_id": newID,
	}, nil
}
