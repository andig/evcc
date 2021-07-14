package tariff

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/tariff/tibber"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
)

type Tibber struct {
	mux    sync.Mutex
	log    *util.Logger
	Token  string
	HomeID string
	Cheap  float64
	client *graphql.Client
	data   []tibber.PriceInfo
}

var _ api.Tariff = (*Tibber)(nil)

func NewTibber(other map[string]interface{}) (*Tibber, error) {
	t := Tibber{}

	if err := util.DecodeOther(other, &t); err != nil {
		return nil, err
	}

	t.log = util.NewLogger("tibber")

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: t.Token})
	tc := oauth2.NewClient(context.Background(), ts)
	tc.Transport = request.NewTripper(t.log, tc.Transport)
	t.client = graphql.NewClient(tibber.URI, tc)

	if t.HomeID == "" {
		var res struct {
			Viewer struct {
				Homes []tibber.Home
			}
		}

		if err := t.client.Query(context.Background(), &res, nil); err != nil {
			return nil, err
		}

		if len(res.Viewer.Homes) != 1 {
			return nil, fmt.Errorf("could not determine home id: %v", res.Viewer.Homes)
		}

		t.HomeID = res.Viewer.Homes[0].ID
	}

	go t.Run()

	return &t, nil
}

func (t *Tibber) Run() {
	for ; true; <-time.NewTicker(time.Hour).C {
		var res struct {
			Viewer struct {
				Home struct {
					ID                  string
					TimeZone            string
					CurrentSubscription tibber.Subscription
				} //`graphql:"home(id: $id)"`
			}
		}

		v := map[string]interface{}{
			"id": graphql.ID(t.HomeID),
		}

		if err := t.client.Query(context.Background(), &res, v); err != nil {
			t.log.ERROR.Println(err)
			continue
		}

		t.mux.Lock()
		t.data = res.Viewer.Home.CurrentSubscription.PriceInfo.Today
		t.mux.Unlock()
	}
}

func (t *Tibber) IsCheap() bool {
	t.mux.Lock()
	defer t.mux.Unlock()

	for i := len(t.data) - 1; i >= 0; i-- {
		pi := t.data[i]

		if pi.StartsAt.Before(time.Now()) {
			return pi.Total <= t.Cheap
		}
	}

	return false
}
