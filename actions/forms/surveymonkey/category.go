package surveymonkey

const (
	CategoryName        = "SurveyMonkey"
	CategoryIcon        = "clipboard-list"
	CategoryDescription = "Create SurveyMonkey surveys, list responses, manage collectors and register webhooks"
)

// BaseURL is the SurveyMonkey v3 API root. Package variable so tests can point
// it at a mock httptest server.
var BaseURL = "https://api.surveymonkey.com/v3"
