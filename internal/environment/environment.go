package environment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	DefaultIdentityService = "https://id.flomation.app"
)

type Summary struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	OwnerID        string  `json:"owner_id"`
	OrganisationID *string `json:"organisation_id"`
}

type Property struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

type Secret struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Value    *string `json:"value"`
	Provider string  `json:"provider"`
}

type CachedProperty struct {
	Name    string
	Expires *time.Time
	Value   Property
}

type CachedSecret struct {
	Name    string
	Expires *time.Time
	Value   Secret
}

type Credentials struct {
	token *string
}

type LoginRequest struct {
	Username string `json:"username"`
	Hash     string `json:"hash"`
}

type LoginResponse struct {
	Value string `json:"token"`
}

func Authenticate(username string, password string, identity *string) *Credentials {
	lr := LoginRequest{
		Username: username,
		Hash:     password,
	}

	b, err := json.Marshal(lr)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to marshal request")
		return nil
	}

	client := http.Client{
		Timeout: time.Second * 10,
	}

	identityServiceURL := DefaultIdentityService
	if identity != nil {
		identityServiceURL = *identity
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%v/api/token", identityServiceURL), bytes.NewReader(b))
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error creating http request")
		return nil
	}

	res, err := client.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to perform request")
		return nil
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		log.WithFields(log.Fields{
			"status code": res.StatusCode,
		}).Error("invalid status code")
		return nil
	}

	if res.Body == nil {
		return nil
	}

	defer func() {
		_ = res.Body.Close()
	}()

	b, err = io.ReadAll(res.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to read request body")
		return nil
	}

	var token LoginResponse
	if err = json.Unmarshal(b, &token); err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("unable to unmarshal response body")
		return nil
	}

	return &Credentials{
		token: &token.Value,
	}
}

func Token(token string) *Credentials {
	return &Credentials{
		token: &token,
	}
}

type Environment struct {
	name        string
	identifier  string
	url         string
	credentials *Credentials

	properties map[string]CachedProperty
	secrets    map[string]CachedSecret
}

func NewEnvironment(name string, url *string, credentials *Credentials) (*Environment, error) {
	e := Environment{
		name:        name,
		url:         *url,
		credentials: credentials,

		properties: make(map[string]CachedProperty),
		secrets:    make(map[string]CachedSecret),
	}

	var summary Summary
	b, err := e.fetch(fmt.Sprintf("%v/api/v1/environment/%v", e.url, e.name))
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(b, &summary); err != nil {
		return nil, err
	}

	e.identifier = summary.ID

	return &e, nil
}

func (e *Environment) GetProperty(name string) (*Property, error) {
	if v, ok := e.properties[name]; ok {
		if v.Expires == nil || time.Now().Before(*v.Expires) {
			return &v.Value, nil
		}
	}

	prop, err := e.fetchProperty(name)
	if err != nil {
		return nil, err
	}

	if prop != nil {
		expiry := time.Now().Add(time.Second * 30)
		e.properties[name] = CachedProperty{
			Name:    name,
			Value:   *prop,
			Expires: &expiry,
		}
	}

	return prop, nil
}

func (e *Environment) GetSecret(name string) (*Secret, error) {
	if v, ok := e.secrets[name]; ok {
		if v.Expires == nil || time.Now().Before(*v.Expires) {
			return &v.Value, nil
		}
	}

	prop, err := e.fetchSecret(name)
	if err != nil {
		return nil, err
	}

	if prop != nil {
		expiry := time.Now().Add(time.Second * 30)
		e.secrets[name] = CachedSecret{
			Name:    name,
			Value:   *prop,
			Expires: &expiry,
		}
	}

	return prop, nil
}

func (e *Environment) fetchProperty(name string) (*Property, error) {
	b, err := e.fetch(fmt.Sprintf("%v/api/v1/environment/%v/property/%v", e.url, e.identifier, url.PathEscape(name)))
	if err != nil {
		return nil, err
	}

	var result Property
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (e *Environment) fetchSecret(name string) (*Secret, error) {
	b, err := e.fetch(fmt.Sprintf("%v/api/v1/environment/%v/secret/%v?decrypt=true", e.url, e.identifier, url.PathEscape(name)))
	if err != nil {
		return nil, err
	}

	var result Secret
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (e *Environment) fetch(url string) ([]byte, error) {
	client := http.Client{
		Timeout: time.Second * 30,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if e.credentials != nil {
		req.Header.Set("Authorization", "Bearer "+*e.credentials.token)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.New("invalid status code: " + res.Status)
	}

	if res.Body == nil {
		return nil, errors.New("invalid response body")
	}

	defer func() {
		_ = res.Body.Close()
	}()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}
