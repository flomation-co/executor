// Package oracle_dataflow_application_create creates a Data Flow application: a reusable Spark
// job template that fixes the driver/executor shapes, executor count, Spark version, language and
// the object-storage URI of the Spark program, ready to be launched as runs.
package oracle_dataflow_application_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Create Application"
	Description  = "Create a Data Flow application — a reusable Apache Spark job template fixing the driver/executor shapes, executor count, Spark version, language and the object-storage URI of the Spark program."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the application", Required: true},
	{Name: "driver_shape", Type: core.ConnectionTypeString, Label: "Driver Shape", Placeholder: "e.g. VM.Standard.E4.Flex", Required: true},
	{Name: "executor_shape", Type: core.ConnectionTypeString, Label: "Executor Shape", Placeholder: "e.g. VM.Standard.E4.Flex", Required: true},
	{Name: "num_executors", Type: core.ConnectionTypeString, Label: "Number of Executors", Placeholder: "e.g. 1", Required: true},
	{Name: "spark_version", Type: core.ConnectionTypeString, Label: "Spark Version", Placeholder: "e.g. 3.5.0", Required: true},
	{Name: "language", Type: core.ConnectionTypeString, Label: "Language", Placeholder: "The Spark language", Required: true, Options: []core.ConnectionOption{
		{Name: "Scala", Value: "SCALA"},
		{Name: "Python", Value: "PYTHON"},
		{Name: "Java", Value: "JAVA"},
		{Name: "SQL", Value: "SQL"},
	}},
	{Name: "file_uri", Type: core.ConnectionTypeString, Label: "File URI", Placeholder: "oci://bucket@namespace/path/to/app.py — the Spark program object-storage URI", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "application", Type: core.ConnectionTypeObject, Label: "Application"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Application OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := df.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	name, err := df.RequiredString("display_name", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	driverShape, err := df.RequiredString("driver_shape", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	executorShape, err := df.RequiredString("executor_shape", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	numExecutors, err := df.RequiredInt("num_executors", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	sparkVersion, err := df.RequiredString("spark_version", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	languageRaw, err := df.RequiredString("language", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	language, ok := dataflow.GetMappingApplicationLanguageEnum(languageRaw)
	if !ok {
		return df.ErrorResult("language must be one of SCALA, PYTHON, JAVA or SQL"), nil
	}
	fileURI, err := df.RequiredString("file_uri", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}

	details := dataflow.CreateApplicationDetails{
		CompartmentId: &compartment,
		DisplayName:   &name,
		DriverShape:   &driverShape,
		ExecutorShape: &executorShape,
		NumExecutors:  &numExecutors,
		SparkVersion:  &sparkVersion,
		Language:      language,
		FileUri:       &fileURI,
	}

	resp, err := client.CreateApplication(df.Context(), dataflow.CreateApplicationRequest{CreateApplicationDetails: details})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}
	app := df.SummariseApplication(&resp.Application)
	return df.Result(fmt.Sprintf("Created application %q (%s)", app["display_name"], app["lifecycle_state"]), map[string]interface{}{
		"application": app, "id": app["id"], "lifecycle_state": app["lifecycle_state"],
	}), nil
}
