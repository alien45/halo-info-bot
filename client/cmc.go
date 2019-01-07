package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// CMC acts as the CoinMarketCap.com Professional API client.
type CMC struct {
	BaseURL          string `json:"url"`
	APIKEY           string `json:"apikey"`
	CacheLastUpdated time.Time
	CacheExpireMins  float64 `json:"cacheexpiremins"`
	DailyCreditLimit int64   `json:"dailycreditlimit"`
	DailyCreditUsed  int64
	CachedTickers    map[string]CMCTicker // cached CMC Ticker container
	CacheOnStart     bool                 `json:"cacheonstart"`
}

// Init instantiates a new CMC instance
func (cmc *CMC) Init(baseURL, apiKey string) {
	cmc.BaseURL = baseURL
	cmc.APIKEY = apiKey
	cmc.CacheExpireMins = 10
	cmc.DailyCreditLimit = 333
	return
}

// GetTicker retrieves tickers from CoinMarketCap.com. Caching enabled.
func (cmc *CMC) GetTicker(nameOrSymbol string) (ticker CMCTicker, err error) {
	tickerURL := cmc.BaseURL + "/cryptocurrency/listings/latest?start=1&limit=5000&convert=USD"

	// Use cache if available and not expired
	if cmc.DailyCreditUsed == cmc.DailyCreditLimit || (len(cmc.CachedTickers) > 0 &&
		time.Now().Sub(cmc.CacheLastUpdated).Minutes() < cmc.CacheExpireMins) {
		log.Println("[CMC] [GetTicker] Using cached tickers")
		return cmc.FindTicker(nameOrSymbol)
	}
	// Cache expired. Update cache
	if cmc.APIKEY == "" {
		err = errors.New("Missing CMC API Key")
		return
	}

	log.Println("[CMC] [GetTicker] Updating CMC tickers cache")
	client := &http.Client{}
	request, err := http.NewRequest("GET", tickerURL, nil)
	if err != nil {
		return
	}

	request.Header.Set("X-CMC_PRO_API_KEY", cmc.APIKEY)
	response, err := client.Do(request)
	if err != nil {
		return
	}
	if response.StatusCode != 200 {
		err = errors.New("Failed to retrieve CMC ticker")
		return
	}

	tickersResult := struct {
		Status CMCResultStatus `json:"status,string,omitempty"`
		Data   []CMCTicker     `json:"data"`
	}{}
	err = json.NewDecoder(response.Body).Decode(&tickersResult)
	if err != nil {
		return
	}

	cmc.DailyCreditUsed = tickersResult.Status.CreditCount
	if tickersResult.Status.ErrorCode != 0 || tickersResult.Status.ErrorMessage != "" {
		err = errors.New(tickersResult.Status.ErrorMessage)
		return
	}

	// Cache result
	cmc.CachedTickers = map[string]CMCTicker{}
	for _, t := range tickersResult.Data {
		cmc.CachedTickers[t.Symbol] = t
	}
	cmc.CacheLastUpdated = time.Now()
	return cmc.FindTicker(nameOrSymbol)
}

// FindTicker searches for a single ticker by name or symbol within the cached tickers
func (cmc *CMC) FindTicker(nameOrSymbol string) (ticker CMCTicker, err error) {
	if nameOrSymbol == "" {
		// ticker name or symbol not provided
		err = errors.New("CMC ticker name or symbol required")
		return
	}
	// nameOrSymbol supplied
	if t, ok := cmc.CachedTickers[strings.ToUpper(nameOrSymbol)]; ok {
		// symbol supplied
		ticker = t
		return
	}
	// check if name provided
	for _, t := range cmc.CachedTickers {
		if strings.ToLower(t.Name) == strings.ToLower(nameOrSymbol) {
			ticker = t
			return
		}
	}
	err = errors.New("CMC ticker not found")
	return
}

// CMCTickersResult describes CMC API result for cryptocurrencies
type CMCTickersResult struct {
	Status CMCResultStatus `json:"status,string,omitempty"`
	Data   []CMCTicker     `json:"data"`
}

// CMCResultStatus describes CMC API response attribute "status"
type CMCResultStatus struct {
	ErrorCode    int64  `json:"error_code"`
	ErrorMessage string `json:"error_message,string,omitempty"`
	CreditCount  int64  `json:"credit_count"`
}

// CMCTicker descrives CoinMarketCap Tickers
type CMCTicker struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Symbol            string    `json:"symbol"`
	Slug              string    `json:"slug"`
	CirculatingSupply float64   `json:"circulating_supply"`
	TotalSupply       float64   `json:"total_supply"`
	MaxSupply         float64   `json:"max_supply"`
	DateAdded         time.Time `json:"date_added,string,omitempty"`
	NumMarketPairs    int64     `json:"num_market_pairs"`
	Rank              int64     `json:"cmc_rank"`
	LastUpdated       time.Time `json:"last_updated,string,omitempty"`
	Quote             map[string]CMCQuote
}

// CMCQuote describes ticker quotes returned by CMC API along with Tickers
type CMCQuote struct {
	Price            float64 `json:"price"`
	Volume24H        float64 `json:"volume_24h"`
	PercentChange1H  float64 `json:"percent_change_1h"`
	PercentChange24H float64 `json:"percent_change_24h"`
	PercentChange7D  float64 `json:"percent_change_7d"`
	MarketCap        float64 `json:"market_cap"`
}

// Format formats important ticker values into a string
func (ticker *CMCTicker) Format() string {
	return fmt.Sprintf("Ticker           : %s (%s)\n"+DashLine+
		"Price USD        : $%.8f\n"+DashLine+
		"24H Price Change : %.4f%%\n"+DashLine+
		"24H Volume USD   : $%s\n"+DashLine+
		"Market Cap USD   : $%s\n"+DashLine+
		"Last Updated     : %s UTC\n"+DashLine,
		ticker.Name, ticker.Symbol,
		ticker.Quote["USD"].Price,
		ticker.Quote["USD"].PercentChange24H,
		FormatNumShort(ticker.Quote["USD"].Volume24H, 4),
		FormatNumShort(ticker.Quote["USD"].MarketCap, 4),
		FormatTimeReverse(ticker.LastUpdated.UTC()),
	)
}
