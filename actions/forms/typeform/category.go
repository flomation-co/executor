package typeform

const (
	CategoryName        = "Typeform"
	CategoryIcon        = "clipboard-list"
	CategoryDescription = "Create and manage Typeform forms, list responses and register webhooks"
)

// BaseURL is the Typeform Create API root. Package variable so tests can point
// it at a mock httptest server.
var BaseURL = "https://api.typeform.com"
