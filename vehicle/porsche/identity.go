package porsche

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	cv "github.com/nirasan/go-oauth-pkce-code-verifier"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/oauth2"
)

const (
	ClientID          = "4mPO3OE5Srjb1iaUGWsbqKBvvesya8oA"
	EmobilityClientID = "gZLSI7ThXFB4d2ld9t8Cx2DBRvGr1zN2"
)

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type AccessTokens struct {
	Token, EmobilityToken oauth2.Token
}

// Identity is the Porsche Identity client
type Identity struct {
	log *util.Logger
	*request.Helper
	user, password string
}

// NewIdentity creates a new Identity client
func NewIdentity(log *util.Logger, user, password string) *Identity {
	v := &Identity{
		log:      log,
		Helper:   request.NewHelper(log, request.WithMetricsPush),
		user:     user,
		password: password,
	}

	jar, _ := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})

	// track cookies and follow all (>10) redirects
	v.Client.Jar = jar
	v.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return nil
	}

	return v
}

func (v *Identity) Login() (AccessTokens, error) {
	var accessTokens AccessTokens

	// get the login page to get the cookies for the subsequent requests
	resp, err := v.Client.Get("https://login.porsche.com/auth/de/de_DE")
	if err != nil {
		return accessTokens, err
	}
	resp.Body.Close()

	query, err := url.ParseQuery(resp.Request.URL.RawQuery)
	if err != nil {
		return accessTokens, err
	}

	sec := query.Get("sec")
	resume := query.Get("resume")
	state := query.Get("state")
	thirdPartyID := query.Get("thirdPartyId")

	dataLoginAuth := url.Values{
		"sec":          []string{sec},
		"resume":       []string{resume},
		"thirdPartyId": []string{thirdPartyID},
		"state":        []string{state},
		"username":     []string{v.user},
		"password":     []string{v.password},
		"keeploggedin": []string{"false"},
	}

	req, err := request.New(http.MethodPost, "https://login.porsche.com/auth/api/v1/de/de_DE/public/login", strings.NewReader(dataLoginAuth.Encode()), request.URLEncoding)
	if err != nil {
		return accessTokens, err
	}

	// process the auth so the session is authenticated
	if resp, err = v.Client.Do(req); err != nil {
		return accessTokens, err
	}
	resp.Body.Close()

	// get the token for the generic API
	var pr tokenResponse
	if pr, err = v.fetchToken(false); err != nil {
		return accessTokens, err
	}

	accessTokens.Token.AccessToken = pr.AccessToken
	accessTokens.Token.Expiry = time.Now().Add(time.Duration(pr.ExpiresIn) * time.Second)

	if pr, err = v.fetchToken(true); err != nil {
		// we don't need to return this error, because we simply won't use the emobility API in this case
		return accessTokens, nil
	}

	accessTokens.EmobilityToken.AccessToken = pr.AccessToken
	accessTokens.EmobilityToken.Expiry = time.Now().Add(time.Duration(pr.ExpiresIn) * time.Second)

	return accessTokens, nil
}

func (v *Identity) fetchToken(emobility bool) (tokenResponse, error) {
	var pr tokenResponse

	actualClientID := ClientID
	redirectURI := "https://my.porsche.com/core/de/de_DE/"

	if emobility {
		actualClientID = EmobilityClientID
		redirectURI = "https://connect-portal.porsche.com/myservices/auth/auth.html"
	}

	var CodeVerifier, _ = cv.CreateCodeVerifier()
	codeChallenge := CodeVerifier.CodeChallengeS256()

	dataTokenAuth := url.Values{
		"redirect_uri":          []string{redirectURI},
		"client_id":             []string{actualClientID},
		"response_type":         []string{"code"},
		"state":                 []string{"uvobn7XJs1"},
		"scope":                 []string{"openid"},
		"access_type":           []string{"offline"},
		"country":               []string{"de"},
		"locale":                []string{"de_DE"},
		"code_challenge":        []string{codeChallenge},
		"code_challenge_method": []string{"S256"},
	}

	req, err := http.NewRequest(http.MethodGet, "https://login.porsche.com/as/authorization.oauth2", nil)
	if err != nil {
		return pr, err
	}

	req.URL.RawQuery = dataTokenAuth.Encode()

	resp, err := v.Client.Do(req)
	if err != nil {
		return pr, err
	}
	resp.Body.Close()

	query, err := url.ParseQuery(resp.Request.URL.RawQuery)
	if err != nil {
		return pr, err
	}

	authCode := query.Get("code")

	codeVerifier := CodeVerifier.CodeChallengePlain()

	dataAPIToken := url.Values{
		"grant_type":    []string{"authorization_code"},
		"client_id":     []string{actualClientID},
		"redirect_uri":  []string{redirectURI},
		"code":          []string{authCode},
		"code_verifier": []string{codeVerifier},
	}

	req, err = request.New(http.MethodPost, "https://login.porsche.com/as/token.oauth2", strings.NewReader(dataAPIToken.Encode()), request.URLEncoding)
	if err == nil {
		err = v.DoJSON(req, &pr)
	}

	if pr.AccessToken == "" || pr.ExpiresIn == 0 {
		return pr, errors.New("could not obtain token")
	}

	return pr, err
}

type Vehicle struct {
	VIN              string
	EmobilityVehicle bool
}

type VehicleResponse struct {
	VIN              string
	ModelDescription string
}

func (v *Identity) FindVehicle(accessTokens AccessTokens, vin string) (Vehicle, error) {
	uri := "https://connect-portal.porsche.com/core/api/v3/de/de_DE/vehicles"
	req, err := request.New(http.MethodGet, uri, nil, map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", accessTokens.Token.AccessToken),
	})

	var vehicles []VehicleResponse
	if err == nil {
		err = v.DoJSON(req, &vehicles)
	}

	var foundVehicle VehicleResponse
	var foundEmobilityVehicle bool

	if err == nil {
		if vin == "" && len(vehicles) == 1 {
			foundVehicle = vehicles[0]
		} else {
			for _, vehicleItem := range vehicles {
				if vehicleItem.VIN == strings.ToUpper(vin) {
					foundVehicle = vehicleItem
				}
			}
		}

		if foundVehicle.VIN != "" {
			v.log.DEBUG.Printf("found vehicle: %v", foundVehicle.VIN)

			// check if the found vehicle is a Taycan, because that one supports the emobility API
			if accessTokens.EmobilityToken.AccessToken != "" {
				if strings.Contains(foundVehicle.ModelDescription, "Taycan") {
					foundEmobilityVehicle = true
				}
			}
		} else {
			err = errors.New("vin not found")
		}
	}

	vehicle := Vehicle{
		VIN:              foundVehicle.VIN,
		EmobilityVehicle: foundEmobilityVehicle,
	}

	return vehicle, err
}
