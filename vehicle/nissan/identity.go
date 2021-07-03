package nissan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/oauth"
	"github.com/andig/evcc/util/request"
	"golang.org/x/oauth2"
)

type Identity struct {
	*request.Helper
	oauth2.TokenSource
}

func NewIdentity(log *util.Logger) *Identity {
	return &Identity{
		Helper: request.NewHelper(log, request.WithMetricsPush),
	}
}

func (v *Identity) Login(user, password string) error {
	uri := fmt.Sprintf("%s/json/realms/root/realms/%s/authenticate", AuthBaseURL, Realm)
	req, err := request.New(http.MethodPost, uri, nil, map[string]string{
		"Accept-Api-Version": APIVersion,
		"X-Username":         "anonymous",
		"X-Password":         "anonymous",
		"Accept":             "application/json",
	})

	var nToken Token
	var realm string
	var resp *http.Response
	var code string

	if err == nil {
		var res Auth
		if err = v.DoJSON(req, &res); err != nil {
			return err
		}

		for id, cb := range res.Callbacks {
			switch cb.Type {
			case "NameCallback":
				res.Callbacks[id].Input[0].Value = user
			case "PasswordCallback":
				res.Callbacks[id].Input[0].Value = password
			}
		}

		var body []byte
		body, err = json.Marshal(res)

		if err == nil {
			req, err = request.New(http.MethodPost, uri, bytes.NewReader(body), map[string]string{
				"Content-type":       "application/json",
				"Accept":             "application/json",
				"Accept-Api-Version": APIVersion,
				"X-Username":         "anonymous",
				"X-Password":         "anonymous",
			})
		}

		if err == nil {
			err = v.DoJSON(req, &nToken)
			realm = strings.Trim(nToken.Realm, "/")
		}
	}

	if err == nil {
		data := url.Values{
			"client_id":     []string{ClientID},
			"redirect_uri":  []string{RedirectURI},
			"response_type": []string{"code"},
			"scope":         []string{Scope},
			"nonce":         []string{"sdfdsfez"},
		}

		uri := fmt.Sprintf("%s/oauth2/%s/authorize?%s", AuthBaseURL, realm, data.Encode())
		req, err = request.New(http.MethodGet, uri, nil, map[string]string{
			"Cookie": "i18next=en-UK; amlbcookie=05; kauthSession=" + nToken.TokenID,
		})

		if err == nil {
			v.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
			resp, err = v.Do(req)
			v.CheckRedirect = nil

			if err == nil {
				resp.Body.Close()

				var location *url.URL
				if location, err = url.Parse(resp.Header.Get("Location")); err == nil {
					if code = location.Query().Get("code"); code == "" {
						err = fmt.Errorf("missing auth code: %v", location)
					}
				}
			}
		}
	}

	var token oauth.Token
	if err == nil {
		data := url.Values{
			"code":          []string{code},
			"client_id":     []string{ClientID},
			"client_secret": []string{ClientSecret},
			"redirect_uri":  []string{RedirectURI},
			"grant_type":    []string{"authorization_code"},
		}

		uri = fmt.Sprintf("%s/oauth2/%s/access_token?%s", AuthBaseURL, realm, data.Encode())
		req, err = request.New(http.MethodPost, uri, nil, request.URLEncoding)
		if err == nil {
			err = v.DoJSON(req, &token)
		}
	}

	if err == nil {
		v.TokenSource = oauth.RefreshTokenSource((*oauth2.Token)(&token), v)
	}

	return err
}

func (v *Identity) RefreshToken(token *oauth2.Token) (*oauth2.Token, error) {
	data := url.Values{
		"client_id":     []string{ClientID},
		"client_secret": []string{ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.RefreshToken},
	}

	uri := fmt.Sprintf("%s/oauth2/%s/access_token?%s", AuthBaseURL, Realm, data.Encode())
	req, err := request.New(http.MethodPost, uri, nil, request.URLEncoding)

	var res oauth.Token
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return (*oauth2.Token)(&res), err
}
