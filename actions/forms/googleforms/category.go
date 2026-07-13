package googleforms

const (
	CategoryName        = "Google Forms"
	CategoryIcon        = "clipboard-list"
	CategoryDescription = "Create Google Forms, add questions and retrieve responses using a connected Google account"
)

// BaseURL is the Google Forms API root. Package variable so tests can point it
// at a mock httptest server.
var BaseURL = "https://forms.googleapis.com/v1"
