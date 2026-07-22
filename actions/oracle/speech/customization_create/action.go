// Package oracle_speech_customization_create creates an OCI Speech customization: a domain-specific
// vocabulary trained from one or more files in an Object Storage bucket, used to boost transcription
// accuracy for your own terms, names and phrases. Asynchronous — the customization comes back in a
// CREATING state; poll Get Customization until it is ACTIVE before using it.
package oracle_speech_customization_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	sp "flomation.app/automate/executor/actions/oracle/speech"

	"github.com/oracle/oci-go-sdk/v65/aispeech"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Speech: Create Customization"
	Description  = "Create a Speech customization — a custom vocabulary trained from files in an Object Storage bucket that boosts transcription accuracy for your own terms. Returns the customization in a CREATING state; poll Get Customization until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microphone"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the customization (optional)"},
	{Name: "model_domain", Type: core.ConnectionTypeString, Label: "Model Domain", Placeholder: "ASR model domain (optional)", Options: []core.ConnectionOption{
		{Name: "Generic", Value: "GENERIC"},
		{Name: "Medical", Value: "MEDICAL"},
	}},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Language Code", Placeholder: "e.g. en-US, es-ES, en-GB, fr-FR (optional)"},
	{Name: "training_namespace", Type: core.ConnectionTypeString, Label: "Training Namespace", Placeholder: "Object Storage namespace holding the training file", Required: true},
	{Name: "training_bucket", Type: core.ConnectionTypeString, Label: "Training Bucket", Placeholder: "Bucket holding the training file", Required: true},
	{Name: "training_object", Type: core.ConnectionTypeString, Label: "Training Object", Placeholder: "Name of the training file, e.g. vocab/terms.json", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "customization", Type: core.ConnectionTypeObject, Label: "Customization"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Customization OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := sp.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}

	// ModelDetails is a mandatory field on the request, though its own subfields are optional.
	model := &aispeech.CustomizationModelDetails{}
	if lc := sp.OptionalString("language_code", inputs); lc != "" {
		model.LanguageCode = &lc
	}
	if d := sp.OptionalString("model_domain", inputs); d != "" {
		domain, ok := aispeech.GetMappingCustomizationModelDetailsDomainEnum(d)
		if !ok {
			return sp.ErrorResult(fmt.Sprintf("model domain %q is not supported — use GENERIC or MEDICAL", d)), nil
		}
		model.Domain = domain
	}

	// TrainingDataset is a mandatory polymorphic field; build the Object Storage concrete type
	// pointing at the training file(s) in a bucket (minimum one object).
	trainNamespace, err := sp.RequiredString("training_namespace", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	trainBucket, err := sp.RequiredString("training_bucket", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	trainObject, err := sp.RequiredString("training_object", inputs)
	if err != nil {
		return sp.ErrorResult(err.Error()), nil
	}
	dataset := aispeech.ObjectStorageDataset{
		LocationDetails: aispeech.ObjectListDataset{
			NamespaceName: &trainNamespace,
			BucketName:    &trainBucket,
			ObjectNames:   []string{trainObject},
		},
	}

	details := aispeech.CreateCustomizationDetails{
		CompartmentId:   &compartment,
		ModelDetails:    model,
		TrainingDataset: dataset,
	}
	if dn := sp.OptionalString("display_name", inputs); dn != "" {
		details.DisplayName = &dn
	}

	resp, err := client.CreateCustomization(sp.Context(), aispeech.CreateCustomizationRequest{CreateCustomizationDetails: details})
	if err != nil {
		return sp.ErrorResult(auth.OCIError(err)), nil
	}
	cust := sp.SummariseCustomization(&resp.Customization)
	return sp.Result(fmt.Sprintf("Created customization %v (%v)", cust["id"], cust["lifecycle_state"]), map[string]interface{}{
		"customization":   cust,
		"id":              cust["id"],
		"lifecycle_state": cust["lifecycle_state"],
	}), nil
}
