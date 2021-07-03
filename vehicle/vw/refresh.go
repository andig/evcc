package vw

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/oauth"
	"github.com/andig/evcc/util/request"
	"golang.org/x/oauth2"
)

type tokenRefresher struct {
	*request.Helper
	login    func() (oauth.Token, error)
	clientID string
}

func Refresher(log *util.Logger, login func() (oauth.Token, error), clientID string) oauth.TokenRefresher {
	return &tokenRefresher{
		Helper:   request.NewHelper(log, request.WithMetricsPush),
		login:    login,
		clientID: clientID,
	}
}

// RefreshToken implements oauth.TokenRefresher
func (tr *tokenRefresher) RefreshToken(token *oauth2.Token) (*oauth2.Token, error) {
	data := url.Values(map[string][]string{
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.RefreshToken},
		"scope":         {"sc2:fal"},
	})

	req, err := request.New(http.MethodPost, OauthTokenURI, strings.NewReader(data.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
		"X-Client-Id":  tr.clientID,
	})

	var res oauth.Token
	if err == nil {
		err = tr.DoJSON(req, &res)
	}

	if se, ok := err.(request.StatusError); ok && se.HasStatus(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden) {
		res, err = tr.login()
	}

	return (*oauth2.Token)(&res), err
}
