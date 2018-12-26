package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CoinCap as as the CoinCap.io API Client
type CoinCap struct {
	BaseURL string `json:"url"`
}

// Init instantiates a new CoinCap instance
func (cc *CoinCap) Init(baseURL string) {
	cc.BaseURL = baseURL
	return
}

// GetIntervals returns intervals names and in minutes supported by CoinCap
func (cc CoinCap) GetIntervals() map[string]string {
	return map[string]string{
		// "1":     "m1",
		// "5":     "m5",
		// "15":    "m15",
		"30":    "m30",
		"60":    "h1",
		"120":   "h2",
		"240":   "h4",
		"480":   "h8",
		"720":   "h12",
		"1440":  "d1",
		"10080": "w1",
	}
}

// CCHistoryItem CoinCap price history item
type CCHistoryItem struct {
	PriceUSD          float64 `json:"priceUsd,string"`
	CirculatingSupply float64 `json:"circulatingSupply"`
	Time              int64   `json:"time,string"`
	Date              time.Time
}

// GetHistory fetches price history of a specific ticker by time range
func (cc CoinCap) GetHistory(baseID, interval string, timeFrom, timeTo int64) (history []CCHistoryItem, err error) {
	baseID = strings.ToLower(strings.Join(strings.Split(baseID, " "), "-"))
	url := fmt.Sprintf("%s/assets/%s/history?interval=%s&start=%d&end=%d",
		cc.BaseURL, baseID, interval, timeFrom, timeTo)
	response, err := http.Get(url)
	if err != nil {
		return
	}
	result := struct {
		Error string          `json:"error"`
		Data  []CCHistoryItem `json:"Data"`
	}{}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return
	}
	if result.Error != "" {
		err = errors.New(result.Error)
		return
	}
	history = result.Data
	return
}

// CCCandle CoinCap candle item
type CCCandle struct {
	UnixTime     int64   `json:"period"` // Unix Epoch time in milliseconds
	ClosingPrice float64 `json:"close,string"`
	OpeningPrice float64 `json:"open,string"`
	HighPrice    float64 `json:"high,string"`
	LowPrice     float64 `json:"low,string"`
	Volume       float64 `json:"volume,string"`
}

// GetCandles retrieves candles from CoinCap.io
func (cc CoinCap) GetCandles(baseID, quoteID, exchange, interval string, start, end int64) (candles []CCCandle, err error) {
	baseID = strings.ToLower(strings.Join(strings.Split(baseID, " "), "-"))
	quoteID = strings.ToLower(strings.Join(strings.Split(quoteID, " "), "-"))
	url := fmt.Sprintf("%s/candles?baseId=%s&quoteId=%s&exchange=%s&interval=%s&start=%d&end=%d",
		cc.BaseURL, baseID, quoteID, exchange, interval, start, end)
	fmt.Println(DashLine, url, DashLine)
	response, err := http.Get(url)
	if err != nil {
		return
	}
	result := struct {
		Error string     `json:"error"`
		Data  []CCCandle `json:"data"`
	}{}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return
	}
	if result.Error != "" {
		err = errors.New(result.Error)
		return
	}
	candles = result.Data
	return
}
