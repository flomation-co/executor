// Package aws_route53_associate_vpc_with_hosted_zone associates a VPC with a private hosted zone.
package aws_route53_associate_vpc_with_hosted_zone

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Associate VPC With Hosted Zone"
	Description  = "Associate an Amazon VPC with a private Route 53 hosted zone."
	Website      = "https://www.flomation.co"
	Icon         = "globe+link"
	Date         = "22/07/2026"
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
	{Name: "hosted_zone_id", Type: core.ConnectionTypeString, Label: "Hosted Zone ID", Placeholder: "Z0123456789ABCDEFGHIJ", Required: true},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID", Placeholder: "vpc-0abc123", Required: true},
	{Name: "vpc_region", Type: core.ConnectionTypeString, Label: "VPC Region", Placeholder: "eu-west-2", Required: true},
	{Name: "comment", Type: core.ConnectionTypeString, Label: "Comment", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "change_id", Type: core.ConnectionTypeString, Label: "Change ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	zoneID := awscommon.InputString("hosted_zone_id", inputs)
	if zoneID == "" {
		return nil, fmt.Errorf("hosted zone id is required")
	}
	vpcID := awscommon.InputString("vpc_id", inputs)
	if vpcID == "" {
		return nil, fmt.Errorf("vpc id is required")
	}
	vpcRegion := awscommon.InputString("vpc_region", inputs)
	if vpcRegion == "" {
		return nil, fmt.Errorf("vpc region is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	in := &route53.AssociateVPCWithHostedZoneInput{
		HostedZoneId: aws.String(zoneID),
		VPC: &r53types.VPC{
			VPCId:     aws.String(vpcID),
			VPCRegion: r53types.VPCRegion(vpcRegion),
		},
	}
	if comment := awscommon.InputString("comment", inputs); comment != "" {
		in.Comment = aws.String(comment)
	}

	out, err := client.AssociateVPCWithHostedZone(ctx, in)
	if err != nil {
		return nil, err
	}

	var changeID string
	if out.ChangeInfo != nil {
		changeID = aws.ToString(out.ChangeInfo.Id)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Associated VPC %s with hosted zone %s", vpcID, zoneID),
		"change_id":   changeID,
	}, nil
}
