// Package aws_iam_get_service_last_accessed_details retrieves a
// service-last-accessed report by job id.
package aws_iam_get_service_last_accessed_details

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Get Service Last Accessed Details"
	Description  = "Retrieve a service-last-accessed report by its job ID (IAM is global; region is ignored)."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+magnifying-glass"
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
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "job_status", Type: core.ConnectionTypeString, Label: "Job Status"},
	{Name: "services_last_accessed", Type: core.ConnectionTypeString, Label: "Services Last Accessed (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type serviceLastAccessed struct {
	ServiceName                string `json:"service_name"`
	LastAuthenticated          string `json:"last_authenticated"`
	TotalAuthenticatedEntities int32  `json:"total_authenticated_entities"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	jobID := strings.TrimSpace(awscommon.InputString("job_id", inputs))
	if jobID == "" {
		return nil, fmt.Errorf("job ID is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	out, err := client.GetServiceLastAccessedDetails(ctx, &iam.GetServiceLastAccessedDetailsInput{
		JobId: aws.String(jobID),
	})
	if err != nil {
		return nil, err
	}

	services := make([]serviceLastAccessed, 0, len(out.ServicesLastAccessed))
	for _, s := range out.ServicesLastAccessed {
		entry := serviceLastAccessed{ServiceName: aws.ToString(s.ServiceName)}
		if s.LastAuthenticated != nil {
			entry.LastAuthenticated = s.LastAuthenticated.Format(time.RFC3339)
		}
		if s.TotalAuthenticatedEntities != nil {
			entry.TotalAuthenticatedEntities = *s.TotalAuthenticatedEntities
		}
		services = append(services, entry)
	}

	servicesJSON, err := json.Marshal(services)
	if err != nil {
		return nil, err
	}

	status := string(out.JobStatus)
	return map[string]interface{}{
		"tool_result":            fmt.Sprintf("Service-last-accessed job %s is %s with %d service(s)", jobID, status, len(services)),
		"job_status":             status,
		"services_last_accessed": string(servicesJSON),
		"count":                  len(services),
	}, nil
}
