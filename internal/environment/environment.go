package environment

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	DefaultIdentityService = "https://id.flomation.app"
)

const (
	CredentialTypeNone = iota
	CredentialTypeUsernamePassword
	CredentialTypeToken
	CredentialTypeCertificate
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
	token          *string
	credentialType int64
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
		token:          &token.Value,
		credentialType: CredentialTypeUsernamePassword,
	}
}

func Token(token string) *Credentials {
	return &Credentials{
		token:          &token,
		credentialType: CredentialTypeToken,
	}
}

func Key(executionID string, key string) (*Credentials, error) {
	var privateKey *rsa.PrivateKey
	b, err := os.ReadFile(key)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("invalid pem file")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		privateKey = key

	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("invalid pem format")
		}

		privateKey = rsaKey
	default:
		return nil, errors.New("invalid private key type")
	}

	hash := sha256.Sum256([]byte(executionID))

	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hash[:], nil)
	if err != nil {
		return nil, err
	}

	token := hex.EncodeToString(signature)
	creds := Credentials{
		token:          &token,
		credentialType: CredentialTypeCertificate,
	}

	return &creds, nil
}

type Environment struct {
	name        string
	identifier  string
	execution   string
	url         string
	credentials *Credentials

	properties map[string]CachedProperty
	secrets    map[string]CachedSecret
}

func NewEnvironment(name string, url *string, execution string, credentials *Credentials) (*Environment, error) {
	e := Environment{
		name:        name,
		url:         *url,
		execution:   execution,
		credentials: credentials,

		properties: make(map[string]CachedProperty),
		secrets:    make(map[string]CachedSecret),
	}

	var summary Summary
	b, err := e.fetch(fmt.Sprintf("%v/api/v1/execution/%v/environment/%v", e.url, execution, e.name))
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

// GetCredential fetches an OAuth credential's current access token by name.
// credentialResponse is the execution-time credential resolution returned by
// the API. Metadata carries the per-account identifier captured at OAuth time
// (QuickBooks realm_id / Xero tenant_id), absent for token-only providers.
type credentialResponse struct {
	Value    string                 `json:"value"`
	Metadata map[string]interface{} `json:"metadata"`
}

func (e *Environment) fetchCredential(name string) (*credentialResponse, error) {
	b, err := e.fetch(fmt.Sprintf("%v/api/v1/execution/%v/environment/%v/credential/%v", e.url, e.execution, e.identifier, url.PathEscape(name)))
	if err != nil {
		return nil, err
	}
	var result credentialResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (e *Environment) GetCredential(name string) (*string, error) {
	result, err := e.fetchCredential(name)
	if err != nil {
		return nil, err
	}
	if result.Value == "" {
		return nil, nil
	}
	return &result.Value, nil
}

// GetCredentialMeta returns one metadata value captured for a managed
// credential at OAuth time — e.g. ${credentials.MyQBO.realm_id} or
// ${credentials.MyXero.tenant_id}. Non-string values are stringified; a missing
// key or empty value returns (nil, nil).
func (e *Environment) GetCredentialMeta(name, key string) (*string, error) {
	result, err := e.fetchCredential(name)
	if err != nil {
		return nil, err
	}
	if result.Metadata == nil {
		return nil, nil
	}
	v, ok := result.Metadata[key]
	if !ok {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok {
		// JSON numbers decode to float64; format without scientific notation
		// so a numeric identifier (e.g. a QuickBooks realmId) stays intact.
		if f, isFloat := v.(float64); isFloat {
			s = strconv.FormatFloat(f, 'f', -1, 64)
		} else {
			s = fmt.Sprintf("%v", v)
		}
	}
	if s == "" {
		return nil, nil
	}
	return &s, nil
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
	b, err := e.fetch(fmt.Sprintf("%v/api/v1/execution/%v/environment/%v/property/%v", e.url, e.execution, e.identifier, url.PathEscape(name)))
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
	b, err := e.fetch(fmt.Sprintf("%v/api/v1/execution/%v/environment/%v/secret/%v", e.url, e.execution, e.identifier, url.PathEscape(name)))
	if err != nil {
		return nil, err
	}

	var result Secret
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (e *Environment) fetch(address string) ([]byte, error) {
	client := http.Client{
		Timeout: time.Second * 30,
	}

	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	q := u.Query()

	if e.credentials != nil && e.credentials.credentialType == CredentialTypeCertificate {
		q.Set("token", *e.credentials.token)
	}

	environmentUrl := fmt.Sprintf("%v?%v", u.String(), q.Encode())

	req, err := http.NewRequest(http.MethodGet, environmentUrl, nil)
	if err != nil {
		return nil, err
	}

	if e.credentials != nil {
		if e.credentials.credentialType == CredentialTypeUsernamePassword || e.credentials.credentialType == CredentialTypeToken {
			req.Header.Set("Authorization", "Bearer "+*e.credentials.token)
		}
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
