package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// DEX struct handles all API requests relating to HaloDEX
type DEX struct {
	// HaloDEX GraphQL API URL
	GQLURL string `json:"urlgql"`
	// HaloDEX public REST API URL
	BaseURL string `json:"baseurl"`

	// Smart Contract addresses of the tokens available on the HaloDEX
	CachedTokens map[string]Token
	// Cache expiration time in minutes.
	CachedTokenExpireMins float64 `json:"tokenexpiremins"`
	// Cached Token last updated timestamp
	CachedTokenLastUpdated time.Time

	// Container for Ticker caching
	// key: pair (eg: halo/eth), value: Ticker
	CachedTickers map[string]Ticker
	// Cache expiration time in minutes.
	CachedTickerExpireMins float64 `json:"tickerexpiremins"`
	// Cached Ticker last updated timestamp
	CachedTickerLastUpdated time.Time
}

// Init instantiates DEX struct with required values
//
// Params:
// gqlURL string : GraphQL based API URL
// publicURL string : Public REST API URL
func (dex *DEX) Init(gqlURL string, baseURL string) {
	dex.GQLURL = gqlURL
	dex.BaseURL = baseURL
	dex.CachedTickerExpireMins = 3
	// Update list of tokens twice everyday
	dex.CachedTokenExpireMins = 720
	return
}

// Trade describes HaloDEX trade item
type Trade struct {
	ID              int64     `json:"id"`
	Address         string    `json:"address"`
	TokenReceivedID int       `json:"tokenReceviedId"`
	AmountReceived  float64   `json:"amountReceived,string"`
	TokenSentID     int       `json:"tokenSentId"`
	AmountSent      float64   `json:"amountSent,string"`
	TxHash          string    `json:"txHash"`
	Block           int64     `json:"block"`
	Time            time.Time `json:"blockTimestamp"`
	Price           float64   `json:"price,string"`
	Side            string    `json:"side"` // buy/sell
	TickerSent      string    `json:"tickerSent"`
	TickerReceived  string    `json:"tickerReceived"`

	IsBuy        bool // Side == "buy"
	BasePriceUSD float64
	PriceUSD     float64
	Amount       float64
}

// FormatTrades transforms Trade attributes into formatted signle line string
func (dex *DEX) FormatTrades(trades []Trade) (s string) {
	if len(trades) == 0 {
		return "No data available"
	}
	pricedp, amountdp := 8, 8
	sign := ""
	s = "  Price      | Amount      | hh:mm:ss DD-MMM\n"
	for _, trade := range trades {
		sign = "- "
		if trade.IsBuy {
			sign = "+ "
		}
		if trade.PriceUSD <= 0.01 {
			amountdp = 0
		}
		if trade.BasePriceUSD <= 0.01 {
			pricedp = 0
		}
		s += DashLine
		s += sign + FillOrLimit(FormatNum(trade.Price, pricedp), " ", 10) + " | "
		s += FillOrLimit(FormatNum(trade.Amount, amountdp), " ", 11) + " | "
		s += FormatTimeReverse(trade.Time.UTC()) + "\n"
	}
	return
}

// GetTradesWithGQLStr retrieves trades using pre-constructed GraphQL query string
// TODO: deprecated
func (dex *DEX) GetTradesWithGQLStr(gqlQueryStr, baseAddr string, dp int64) (trades []Trade, err error) {
	err = errors.New("Deprecated")
	return
}

// GetTradesByTime retrieves trades since given blockstime
// TODO: deprecated
func (dex *DEX) GetTradesByTime(quoteAddr, baseAddr string, blockTime time.Time, dp int64) (trades []Trade, err error) {
	err = errors.New("Deprecated")
	return
}

// GetTrades function retrieves recent trades from HaloDEX
func (dex *DEX) GetTrades(quoteTicker, baseTicker string, limit, pageNo int64, basePriceUSD float64) (trades []Trade, err error) {
	if limit <= 0 {
		limit = 10
	}
	if pageNo <= 0 {
		pageNo = 1
	}
	qURL := fmt.Sprintf(
		"%s/dex/market/trades/%s/%s?order=DESC&limit=%d&page=%d",
		dex.BaseURL,
		quoteTicker,
		baseTicker,
		limit,
		pageNo,
	)
	request, err := http.NewRequest("GET", qURL, nil)
	if err != nil {
		log.Println("[DEX] [GetTrades] request error", err)
		return
	}
	response, err := (&http.Client{Timeout: time.Second * 30}).Do(request)
	if err != nil {
		return
	}

	result := struct {
		Total  int64   `json:"total,string"`
		Trades []Trade `json:"trades"`
	}{}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		if response.StatusCode != http.StatusOK {
			err = fmt.Errorf("API request failed! Status: %s", response.Status)
		}
		return
	}
	for i, trade := range result.Trades {
		result.Trades[i].IsBuy = strings.ToLower(trade.Side) == "sell"
		if basePriceUSD > 0 {
			result.Trades[i].BasePriceUSD = basePriceUSD
			result.Trades[i].PriceUSD = trade.Price * trade.BasePriceUSD
		}
		if result.Trades[i].IsBuy {
			result.Trades[i].Amount = trade.AmountReceived
		} else {
			result.Trades[i].Amount = trade.AmountSent
		}
	}
	trades = result.Trades
	return
}

// Ticker describes a HaloDEX ticker response
type Ticker struct {
	Bid           string    `json:"bid"`
	Ask           string    `json:"ask"`
	QuoteVolume   float64   `json:"quoteVolume,string"`
	BaseVolume    float64   `json:"baseVolume,string"`
	PercentChange float64   `json:"percentChange,string"` // New
	Last          float64   `json:"last,string"`
	Timestamp     time.Time `json:"timestamp"`
	BaseTicker    string    `json:"baseTicker"`
	QuoteTicker   string    `json:"quoteTicker"`
	Pair          string    `json:"pair"`
	Base          string    `json:"base"`
	Quote         string    `json:"quote"`

	// Deprecated
	Avg                string  `json:"avg"`
	TwoFourQuoteVolume float64 `json:"twoFourQuoteVolume,string"`
	TwoFourBaseVolume  float64 `json:"twoFourBaseVolume,string"`
	TwoFourBid         float64 `json:"twoFourBid,string"`
	TwoFourAsk         float64 `json:"twoFourAsk,string"`
	TwoFourAvg         float64 `json:"twoFourAvg,string"`

	// External/calculated attributes
	LastPriceUSD float64
	// TwoFourBidUSD       float64
	// TwoFourAskUSD       float64
	TwoFourVolumeUSD    float64
	QuoteTokenSupply    float64
	QuoteTokenMarketCap float64
	LastUpdated         time.Time
}

// Format formats important ticker values into a string
func (ticker *Ticker) Format() string {
	base := ticker.BaseTicker
	return fmt.Sprintf(""+
		"Pair       : %s\n"+DashLine+
		"Last Price : $%.8f | %.8f %s\n"+DashLine+
		// "24H High   : $%.8f | %.8f %s\n"+DashLine+ // deprecated ???
		// "24H Low    : $%.8f | %.8f %s\n"+DashLine+ // deprecated ???
		"24 Price Changed : %.2f%%\n"+DashLine+
		"Supply: %s | Market Cap: $%s\n"+DashLine+
		"                  24hr Volume\n"+DashLine+
		"%s| %s| $%s",
		ticker.Pair,
		ticker.LastPriceUSD, ticker.Last, base,
		// ticker.TwoFourAskUSD, ticker.TwoFourAsk, base,
		// ticker.TwoFourBidUSD, ticker.TwoFourBid, base,
		ticker.PercentChange,
		FormatNumShort(ticker.QuoteTokenSupply, 4),
		FormatNumShort(ticker.QuoteTokenMarketCap, 4),
		FillOrLimit(base+" "+FormatNumShort(ticker.BaseVolume, 4), " ", 14),
		FillOrLimit(ticker.QuoteTicker+" "+FormatNumShort(ticker.QuoteVolume, 4), " ", 14),
		FormatNumShort(ticker.TwoFourVolumeUSD, 4),
	)
}

// GetTicker function retrieves available tickers from HaloDEX. Caching enabled.
func (dex *DEX) GetTicker(symbolQuote, symbolBase string, baseTokenPriceUSD, quoteTokenSupply float64) (ticker Ticker, err error) {
	// Use cache if available and not expired
	pair := strings.ToUpper(fmt.Sprintf("%s/%s", symbolQuote, symbolBase))
	cachedTicker, available := dex.CachedTickers[pair]
	if available && time.Now().Sub(dex.CachedTickerLastUpdated).Minutes() < dex.CachedTickerExpireMins {
		log.Println("[DEX] [GetTicker] Using cached tickers")
		return cachedTicker, nil
	}

	response := &(http.Response{})
	response, err = http.Get(dex.BaseURL + "/dex/public/pricing/all")
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	tickersArr := []Ticker{}
	err = json.Unmarshal(body, &tickersArr)
	if err != nil {
		if response.StatusCode != http.StatusOK {
			err = fmt.Errorf("API request failed! Status: %s", response.Status)
		} else if string(body) == "[]" {
			// API returns empty Array if not none found!!
			err = fmt.Errorf("Zero tickers returned from API")
		}
		return
	}
	tickers := map[string]Ticker{}
	for _, t := range tickersArr {
		tickers[t.Pair] = t
	}
	dex.CachedTickers = tickers
	now := time.Now()
	for key, t := range dex.CachedTickers {
		// // TEMP FIX: switch 24H low and high price appropriately
		// if t.TwoFourAsk < t.TwoFourBid {
		// 	t.TwoFourAsk, t.TwoFourBid = t.TwoFourBid, t.TwoFourAsk
		// }
		t.LastPriceUSD = t.Last * baseTokenPriceUSD
		// t.TwoFourAskUSD = t.TwoFourAsk * baseTokenPriceUSD
		// t.TwoFourBidUSD = t.TwoFourBid * baseTokenPriceUSD
		t.QuoteTokenSupply = quoteTokenSupply
		t.TwoFourVolumeUSD = t.QuoteVolume * t.LastPriceUSD
		t.QuoteTokenMarketCap = t.QuoteTokenSupply * t.LastPriceUSD
		t.LastUpdated = now
		dex.CachedTickers[key] = t
		if key == pair {
			ticker = t
		}
	}
	dex.CachedTickerLastUpdated = now
	if ticker.Pair == "" {
		err = fmt.Errorf("Pair %s/%s not available", symbolQuote, symbolBase)
		return
	}
	return
}

// Order describes a HaloDEX order item
type Order struct {
	Trade
	Contract      string  `json:"contract"`
	Deleted       bool    `json:"deleted"`
	FilledAmount  float64 `json:"filled,string,omitempty"`
	TransactionID string  `json:"transactionID"`
	OrderHash     string  `json:"orderHash"`
	UserAddress   string  `json:"user"`
	FilledPercent float64
}

// FormatOrders returns orders as a string formatted like a table
func (dex *DEX) FormatOrders(orders []Order) (s string) {
	s = "deprecated"
	return
}

// GetOrders retrieves HaloDEX orders by user address
func (dex *DEX) GetOrders(quoteAddr, baseAddr, limit, address string) (orders []Order, err error) {
	err = errors.New("deprecated")
	return
}

// GetOrderbook retrieves HaloDEX orderbook buy+sell
func (dex *DEX) GetOrderbook(quoteAddr, baseAddr, limit string, buy bool) (orders []Order, err error) {
	err = errors.New("not implemented")
	return
}

// Token describes data of available tokens on the HaloDEX
type Token struct {
	Number           int64  `json:"number"`
	Type             string `json:"type"` //"BASE" & "QUOTE"
	BaseChain        string `json:"baseChain"`
	BaseChainAddress string `json:"baseChainAddress"` // removed from new API
	HaloChainAddress string `json:"haloAddress"`
	Ticker           string `json:"ticker"`
	Name             string `json:"name"`
	Decimals         int64  `json:"decimals"` // number of decimals supported
	Description      string `json:"description"`
}

// Format formats Token
func (t *Token) Format() string {
	return fmt.Sprintf(""+
		"Name         : %s\n"+DashLine+
		"Ticker       : %s\n"+DashLine+
		"Type         : %s\n"+DashLine+
		"Decimals     : %d\n"+DashLine+
		"Base Chain   : %s\n"+DashLine+
		"Base Address : \n  %s\n"+DashLine+
		"Halo Address : \n  %s\n"+DashLine,
		t.Name, t.Ticker, t.Type, t.Decimals, t.BaseChain, t.BaseChainAddress, t.HaloChainAddress)
}

// GetFormattedTokens returns a string with provided tokens or all supported tokens on HaloDEX line by line
func (dex *DEX) GetFormattedTokens(tokens map[string]Token) (s string, err error) {
	if len(tokens) == 0 {
		tokens, err = dex.GetTokens()
		if err != nil {
			return
		}
	}
	if len(tokens) == 0 {
		s = "No tokens found"
		return
	}
	// List token names (with ticker) only
	keys := []string{}
	for ticker := range tokens {
		keys = append(keys, ticker)
	}
	sort.Strings(keys)
	// TODO: include 24 USD price and volume
	//Name           | Ticker | Price | 24H Volume
	s += "Name           | Ticker\n" + DashLine
	for i := 0; i < len(keys); i++ {
		token := tokens[keys[i]]
		s += fmt.Sprintf("%s | %s\n", FillOrLimit(token.Name, " ", 14), FillOrLimit(token.Ticker, " ", 6)) + DashLine
	}
	return
}

// GetTokens caches and returns HaloDEX tokens.
func (dex *DEX) GetTokens() (tokens map[string]Token, err error) {
	cacheExpired := time.Now().Sub(dex.CachedTokenLastUpdated).Minutes() >= dex.CachedTokenExpireMins
	if len(dex.CachedTokens) > 0 && !cacheExpired {
		tokens = dex.CachedTokens
		return
	}
	log.Println("[DEX] [GetTokens] updating DEX token cache")
	request, err := http.NewRequest("GET", dex.BaseURL+"/dex/public/tokens", nil)
	if err != nil {
		log.Println("[DEX] [GetTokens] request error", err)
		return
	}
	response, err := (&http.Client{Timeout: time.Second * 30}).Do(request)
	if err != nil {
		return
	}

	result := []Token{}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		if response.StatusCode != http.StatusOK {
			err = fmt.Errorf("API request failed! Status: %s", response.Status)
		}
		return
	}
	dex.CachedTokens = map[string]Token{}
	for i, token := range result {
		dex.CachedTokens[token.Ticker] = result[i]
	}
	tokens = dex.CachedTokens
	dex.CachedTokenLastUpdated = time.Now()
	return
}

// TokenPair token pair details from HaloDEX
type TokenPair struct {
	Pair          string `json:"pair"`
	BaseTicker    string `json:"baseTicker"`
	BaseAddress   string `json:"baseAddress"`
	BaseDecimals  int    `json:"baseDecimals"`
	BaseName      string `json:"baseName"`
	QuoteTicker   string `json:"quoteTicker"`
	QuoteAddress  string `json:"quoteAddress"`
	QuoteDecimals int    `json:"quoteDecimals"`
	QuoteName     string `json:"quoteName"`
	//pairNumber
	//baseNumber
	//quoteNumber
}

// GetTokenPairs retrieves available token pairs from HaloDEX
func (dex *DEX) GetTokenPairs() (pairs []TokenPair, err error) {
	response, err := http.Get(dex.BaseURL + "/dex/public/available")
	if err != nil {
		return
	}
	err = json.NewDecoder(response.Body).Decode(&pairs)
	if err != nil && response.StatusCode != http.StatusOK {
		err = fmt.Errorf("API request failed! Status: %s", response.Status)
	}
	return
}

// Balance ////
//TODO: deprecated
type Balance struct {
	Available float64 `json:"available,string"`
	Balance   float64 `json:"balance,string"`
}

// GetBalance returns single balance of the specified address
//TODO: deprecated
func (dex *DEX) GetBalance(userAddress string, tickerStr string) (balance float64, err error) {
	err = errors.New("Deprecated")
	return
}

// GetBalances retrieves DEX account balances for one or more tickers by user address
//TODO: deprecated
func (dex *DEX) GetBalances(userAddress string, tickers []string) (balances map[string][]Balance, err error) {
	err = errors.New("Deprecated")
	return
}

// GetBalancesFormatted returns formatted balances by user address and ticker symbol
//TODO: deprecated
func (dex *DEX) GetBalancesFormatted(address string, tickers []string, showZeroBalance bool) (s string, err error) {
	if len(tickers) == 0 {
		err = errors.New("Ticker required")
		return
	}
	// Make sure ticker symbols are in upper case
	for i := 0; i < len(tickers); i++ {
		tickers[i] = strings.ToUpper(tickers[i])
	}

	sort.Strings(tickers)

	tokenBalances, err := dex.GetBalances(address, tickers)
	if err != nil {
		return
	}
	if len(tokenBalances) == 0 {
		err = errors.New("No balances found")
		return
	}

	s = "  Ticker  | Balance        | Available   \n" + DashLine
	for i := 0; i < len(tickers); i++ {
		tokenBalance := tokenBalances[tickers[i]]
		if len(tokenBalance) == 0 {
			tokenBalance = append(tokenBalance, Balance{Balance: 0, Available: 0})
		}
		if !showZeroBalance && tokenBalance[0].Balance < 1e-8 {
			// Balance is zero
			continue
		}
		s += fmt.Sprintf("  %s| %s | %s\n%s",
			FillOrLimit(tickers[i], " ", 8),
			FillOrLimit(FormatNum(tokenBalance[0].Balance, 8), " ", 14),
			FillOrLimit(FormatNum(tokenBalance[0].Available, 8), " ", 14),
			DashLine,
		)
	}
	return
}
