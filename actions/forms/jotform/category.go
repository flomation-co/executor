package jotform

const (
	CategoryName        = "JotForm"
	CategoryIcon        = "clipboard-list"
	CategoryDescription = "Create and manage JotForm forms, list submissions and register webhooks"
)

// BaseURL, when non-empty, overrides the region-derived API root. It is left
// empty in production so the region input selects the correct regional host
// (see resolveBaseURL in common.go); tests set it to point at an httptest
// server.
var BaseURL = ""
