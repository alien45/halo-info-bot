package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CMC struct handles all API requests relating to CoinMarketCap.com
type CMC struct {
	BaseURL          string
	APIKEY           string
	CachedTickers    map[string]CMCTicker // cached CMC Ticker container
	CacheLastUpdated time.Time            // update every 10 mins
	CacheExpireMins  float64
	DailyCreditLimit int64 // daily credit soft limit.
	DailyCreditUsed  int64
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
		fmt.Println("Using cached tickers")
		return cmc.FindTicker(nameOrSymbol)
	}
	// Cache expired. Update cache
	fmt.Println("Updating CMC tickers cache from: ", tickerURL)
	if cmc.APIKEY == "" {
		err = errors.New("Missing CMC API Key")
		return
	}

	client := &http.Client{}
	request, errN := http.NewRequest("GET", tickerURL, nil)
	if err = errN; err != nil {
		return
	}

	request.Header.Set("X-CMC_PRO_API_KEY", cmc.APIKEY)
	response, errN := client.Do(request)
	if err = errN; err != nil {
		return
	}
	if response.StatusCode != 200 {
		err = errors.New("Failed to retrieve CMC ticker")
		return
	}

	tickersResult := CMCTickersResult{}
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
	nameOrSymbol = strings.ToLower(nameOrSymbol)
	for _, t := range cmc.CachedTickers {
		if strings.ToLower(t.Name) == nameOrSymbol {
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
	return fmt.Sprintf("Ticker           : %s (%s)\n"+
		"Price USD        : $%.2f\n"+
		"24H Price Change : %.2f%%\n"+
		"24H Volume USD   : $%s\n"+
		"Market Cap USD   : $%s\n"+
		"Last Updated     : %s\n",
		ticker.Name, ticker.Symbol,
		ticker.Quote["USD"].Price,
		ticker.Quote["USD"].PercentChange24H,
		ConvertNumber(ticker.Quote["USD"].Volume24H, 2),
		ConvertNumber(ticker.Quote["USD"].MarketCap, 2),
		ticker.LastUpdated.String(),
	)
}
