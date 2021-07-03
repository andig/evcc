package vw

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/oauth"
	"github.com/andig/evcc/util/request"
	"github.com/andig/evcc/vehicle/id"
	"github.com/andig/evcc/vehicle/skoda"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/oauth2"
)

const (
	// IdentityURI is the VW OIDC identity provider uri
	IdentityURI = "https://identity.vwgroup.io"

	// OauthTokenURI is used for refreshing tokens
	OauthTokenURI = "https://mbboauth-1d.prd.ece.vwg-connect.com/mbbcoauth/mobile/oauth2/v1/token"

	// OauthRevokeURI is used for revoking tokens
	OauthRevokeURI = "https://mbboauth-1d.prd.ece.vwg-connect.com/mbbcoauth/mobile/oauth2/v1/revoke"

	// AppsURI is the login uri for ID vehicles
	AppsURI = "https://login.apps.emea.vwapps.io"

	// TokenServiceURI is the token service uri (used for Skoda Enyaq vehicles)
	TokenServiceURI = "https://tokenrefreshservice.apps.emea.vwapps.io"
)

// Identity provides the identity.vwgroup.io login token source
type Identity struct {
	log *util.Logger
	*request.Helper
	oauth2.TokenSource
}

// NewIdentity creates VW identity
func NewIdentity(log *util.Logger) *Identity {
	v := &Identity{
		log:    log,
		Helper: request.NewHelper(log, request.WithMetricsPush),
	}

	return v
}

// Login performs the identity.vwgroup.io login
func (v *Identity) login(uri, user, password string) (url.Values, error) {
	// track cookies and don't follow redirects
	jar, _ := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})

	v.Client.Jar = jar
	defer func() { v.Client.Jar = nil }()

	v.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return http.ErrUseLastResponse
		}
		return nil
	}

	var vars FormVars

	// add nonce and state
	query := url.Values{
		"nonce": []string{RandomString(43)},
		"state": []string{RandomString(43)},
	}
	uri += "&" + query.Encode()

	// GET identity.vwgroup.io/signin-service/v1/signin/b7a5bb47-f875-47cf-ab83-2ba3bf6bb738@apps_vw-dilab_com?relayState=15404cb51c8b4cc5efeee1d2c2a73e5b41562faa
	resp, err := v.Get(uri)
	if err == nil {
		vars, err = FormValues(resp.Body, "form#emailPasswordForm")
		resp.Body.Close()
	}

	// POST identity.vwgroup.io/signin-service/v1/b7a5bb47-f875-47cf-ab83-2ba3bf6bb738@apps_vw-dilab_com/login/identifier
	if err == nil {
		data := url.Values(map[string][]string{
			"_csrf":      {vars.Inputs["_csrf"]},
			"relayState": {vars.Inputs["relayState"]},
			"hmac":       {vars.Inputs["hmac"]},
			"email":      {user},
		})

		uri = IdentityURI + vars.Action
		if resp, err = v.PostForm(uri, data); err == nil {
			vars, err = FormValues(resp.Body, "form#credentialsForm")
			resp.Body.Close()
		}
	}

	// POST identity.vwgroup.io/signin-service/v1/b7a5bb47-f875-47cf-ab83-2ba3bf6bb738@apps_vw-dilab_com/login/authenticate
	if err == nil {
		data := url.Values(map[string][]string{
			"_csrf":      {vars.Inputs["_csrf"]},
			"relayState": {vars.Inputs["relayState"]},
			"hmac":       {vars.Inputs["hmac"]},
			"email":      {user},
			"password":   {password},
		})

		uri = IdentityURI + vars.Action
		if resp, err = v.PostForm(uri, data); err == nil {
			resp.Body.Close()

			if e := resp.Request.URL.Query().Get("error"); e != "" {
				err = fmt.Errorf(e)
			}

			if u := resp.Request.URL.Query().Get("updated"); err == nil && u != "" {
				v.log.WARN.Println("accepting updated", u)
				if resp, err = v.postTos(resp.Request.URL.String()); err == nil {
					resp.Body.Close()
				}
			}
		}
	}

	// GET identity.vwgroup.io/oidc/v1/oauth/sso?clientId=b7a5bb47-f875-47cf-ab83-2ba3bf6bb738@apps_vw-dilab_com&relayState=15404cb51c8b4cc5efeee1d2c2a73e5b41562faa&userId=bca09cc0-8eba-4110-af71-7242868e1bf1&HMAC=2b01ce6a351fad4dd97dc8110d0967b46c95889ab5010c660a616462e66a83ca
	// GET identity.vwgroup.io/signin-service/v1/consent/users/bca09cc0-8eba-4110-af71-7242868e1bf1/b7a5bb47-f875-47cf-ab83-2ba3bf6bb738@apps_vw-dilab_com?scopes=openid%20profile%20birthdate%20nickname%20address%20phone%20cars%20mbb&relayState=15404cb51c8b4cc5efeee1d2c2a73e5b41562faa&callback=https://identity.vwgroup.io/oidc/v1/oauth/client/callback&hmac=a590931ca3cd9dc3a27f1d1c0c162bf1e5c5c32c9f5b40fcb36d4c6edc631e03
	// GET identity.vwgroup.io/oidc/v1/oauth/client/callback/success?user_id=bca09cc0-8eba-4110-af71-7242868e1bf1&client_id=b7a5bb47-f875-47cf-ab83-2ba3bf6bb738@apps_vw-dilab_com&scopes=openid%20profile%20birthdate%20nickname%20address%20phone%20cars%20mbb&consentedScopes=openid%20profile%20birthdate%20nickname%20address%20phone%20cars%20mbb&relayState=f89a0b750c93e278a7ace170ce374e9cb9eb0a74&hmac=2b728f463c3cfe80f3271fbb35680e5e5218ca70025a46e7fadf7c7982decc2b

	var location *url.URL
	if err == nil {
		loc := strings.ReplaceAll(resp.Header.Get("Location"), "#", "?") //  convert to parseable url
		if location, err = url.Parse(loc); err == nil {
			return location.Query(), nil
		}
	}

	return nil, err
}

func (v *Identity) postTos(uri string) (*http.Response, error) {
	var vars FormVars
	resp, err := v.Get(uri)
	if err == nil {
		vars, err = FormValues(resp.Body, "form#emailPasswordForm")
	}

	if err == nil {
		data := make(url.Values)
		for k, v := range vars.Inputs {
			data.Set(k, v)
		}

		uri := IdentityURI + vars.Action
		resp, err = v.PostForm(uri, data)
	}

	return resp, err
}

// LoginVAG performs VAG login and finally exchanges id token for access and refresh tokens
func (v *Identity) LoginVAG(clientID string, query url.Values, user, password string) error {
	if user == "" || password == "" {
		return errors.New("missing credentials")
	}

	login := func() (oauth.Token, error) {
		var token oauth.Token
		uri := fmt.Sprintf("%s/oidc/v1/authorize?%s", IdentityURI, query.Encode())

		q, err := v.login(uri, user, password)
		if err == nil {
			data := url.Values(map[string][]string{
				"grant_type": {"id_token"},
				"scope":      {"sc2:fal"},
				"token":      {q.Get("id_token")},
			})

			var req *http.Request
			req, err = request.New(http.MethodPost, OauthTokenURI, strings.NewReader(data.Encode()), map[string]string{
				"Content-Type": "application/x-www-form-urlencoded",
				"X-Client-Id":  clientID,
			})

			if err == nil {
				err = v.DoJSON(req, &token)
			}
		}

		return token, err
	}

	token, err := login()
	if err == nil {
		v.TokenSource = oauth.RefreshTokenSource((*oauth2.Token)(&token), Refresher(v.log, login, clientID))
	}

	return err
}

// LoginSkoda performs Skoda login and finally exchanges code and id token for access and refresh tokens
func (v *Identity) LoginSkoda(query url.Values, user, password string) error {
	if user == "" || password == "" {
		return errors.New("missing credentials")
	}

	login := func() (oauth.Token, error) {
		var token oauth.Token
		uri := fmt.Sprintf("%s/oidc/v1/authorize?%s", IdentityURI, query.Encode())

		q, err := v.login(uri, user, password)
		if err == nil {
			data := url.Values(map[string][]string{
				"auth_code": {q.Get("code")},
				"id_token":  {q.Get("id_token")},
				"brand":     {"skoda"},
			})

			var req *http.Request
			uri = fmt.Sprintf("%s/exchangeAuthCode", TokenServiceURI)
			req, err = request.New(http.MethodPost, uri, strings.NewReader(data.Encode()), request.URLEncoding)

			if err == nil {
				err = v.DoJSON(req, &token)
			}
		}

		return token, err
	}

	token, err := login()
	if err == nil {
		v.TokenSource = oauth.RefreshTokenSource((*oauth2.Token)(&token), skoda.Refresher(v.log, login))
	}

	return err
}

// LoginID performs ID login and finally exchanges state and id token for access and refresh tokens
func (v *Identity) LoginID(query url.Values, user, password string) error {
	if user == "" || password == "" {
		return errors.New("missing credentials")
	}

	login := func() (id.Token, error) {
		var token id.Token
		uri := fmt.Sprintf("%s/authorize?%s", AppsURI, query.Encode())

		q, err := v.login(uri, user, password)
		if err == nil {
			data := map[string]string{
				"state":             q.Get("state"),
				"id_token":          q.Get("id_token"),
				"redirect_uri":      "weconnect://authenticated",
				"region":            "emea",
				"access_token":      q.Get("access_token"),
				"authorizationCode": q.Get("code"),
			}

			var req *http.Request
			uri = fmt.Sprintf("%s/login/v1", AppsURI)
			req, err = request.New(http.MethodPost, uri, request.MarshalJSON(data), request.JSONEncoding)

			if err == nil {
				err = v.DoJSON(req, &token)
			}
		}

		return token, err
	}

	token, err := login()
	if err == nil {
		v.TokenSource = oauth.RefreshTokenSource((*oauth2.Token)(&token), id.Refresher(v.log, login))
	}

	return err
}
