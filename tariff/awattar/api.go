package awattar

import (
	"encoding/json"
	"time"
)

const RegionURI = "https://api.awattar.%s/v1/marketdata"

type Prices struct {
	Data []PriceInfo
}

type PriceInfo struct {
	StartTimestamp time.Time `json:"start_timestamp"`
	EndTimestamp   time.Time `json:"end_timestamp"`
	Marketprice    float64   `json:"marketprice"`
	Unit           string    `json:"unit"`
}

func (p *PriceInfo) UnmarshalJSON(data []byte) error {
	var s struct {
		StartTimestamp int64   `json:"start_timestamp"`
		EndTimestamp   int64   `json:"end_timestamp"`
		Marketprice    float64 `json:"marketprice"`
		Unit           string  `json:"unit"`
	}

	err := json.Unmarshal(data, &s)
	if err == nil {
		p.StartTimestamp = time.Unix(s.StartTimestamp/1e3, 0)
		p.EndTimestamp = time.Unix(s.EndTimestamp/1e3, 0)
		p.Marketprice = s.Marketprice
		p.Unit = s.Unit
	}

	return err
}
