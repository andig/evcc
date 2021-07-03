package id

import (
	"net/http"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/oauth"
	"github.com/andig/evcc/util/request"
	"golang.org/x/oauth2"
)

type tokenRefresher struct {
	*request.Helper
	login func() (Token, error)
}

func Refresher(log *util.Logger, login func() (Token, error)) oauth.TokenRefresher {
	return &tokenRefresher{
		Helper: request.NewHelper(log, request.WithMetricsPush),
		login:  login,
	}
}

// RefreshToken implements oauth.TokenRefresher
func (tr *tokenRefresher) RefreshToken(token *oauth2.Token) (*oauth2.Token, error) {
	uri := "https://login.apps.emea.vwapps.io/refresh/v1"

	req, err := request.New(http.MethodGet, uri, nil, map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer " + token.RefreshToken,
	})

	var res Token
	if err == nil {
		err = tr.DoJSON(req, &res)
	}

	if se, ok := err.(request.StatusError); ok && se.HasStatus(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden) {
		res, err = tr.login()
	}

	return (*oauth2.Token)(&res), err
}
