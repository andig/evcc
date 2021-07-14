package tibber

import (
	"time"
)

const URI = "https://api.tibber.com/v1-beta/gql"

type Home struct {
	ID                string
	TimeZone          string
	Address           Address
	MeteringPointData struct {
		GridCompany string
	}
}

type Address struct {
	Address1, PostalCode, City, Country string
}

type Subscription struct {
	ID        string
	Status    string
	PriceInfo struct {
		Current PriceInfo
		Today   []PriceInfo
		// Tomorrow []PriceInfo
	}
}

type PriceInfo struct {
	Level    string
	StartsAt time.Time
	Total    float64
	// Energy, Tax float64
}
