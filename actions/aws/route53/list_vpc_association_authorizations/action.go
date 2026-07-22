// Package aws_route53_list_vpc_association_authorizations lists VPCs authorised to associate with a private hosted zone.
package aws_route53_list_vpc_association_authorizations

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 List VPC Association Authorizations"
	Description  = "List VPCs authorised to associate with a private hosted zone."
	Website      = "https://www.flomation.co"
	Icon         = "globe+list"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vpcs", Type: core.ConnectionTypeString, Label: "VPCs (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	zoneID := awscommon.InputString("hosted_zone_id", inputs)
	if zoneID == "" {
		return nil, fmt.Errorf("hosted zone id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.ListVPCAssociationAuthorizations(ctx, &route53.ListVPCAssociationAuthorizationsInput{
		HostedZoneId: aws.String(zoneID),
	})
	if err != nil {
		return nil, err
	}

	type vpcEntry struct {
		VPCId     string `json:"vpc_id"`
		VPCRegion string `json:"vpc_region"`
	}
	vpcs := make([]vpcEntry, 0, len(out.VPCs))
	for _, v := range out.VPCs {
		vpcs = append(vpcs, vpcEntry{
			VPCId:     aws.ToString(v.VPCId),
			VPCRegion: string(v.VPCRegion),
		})
	}
	vpcsJSON, _ := json.Marshal(vpcs)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d authorised VPC(s) for hosted zone %s", len(vpcs), zoneID),
		"vpcs":        string(vpcsJSON),
		"count":       len(vpcs),
	}, nil
}
