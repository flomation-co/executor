package dvsa

// BaseURL is the DVSA MOT History API root. Package variable so tests can point
// it at a mock server.
var BaseURL = "https://history.mot.api.gov.uk"

// LoginBaseURL is the Microsoft Entra ID (login.microsoftonline.com) host used
// for the OAuth2 token exchange. Package variable so tests can override it.
var LoginBaseURL = "https://login.microsoftonline.com"

// Scope is the exact OAuth2 scope required by the DVSA MOT History API.
const Scope = "https://tapi.dvsa.gov.uk/.default"
