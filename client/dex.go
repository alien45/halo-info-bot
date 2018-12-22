package client

import (
	"bytes"
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
	PublicURL string `json:"url"`

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
func (dex *DEX) Init(gqlURL string, publicURL string) {
	dex.GQLURL = gqlURL
	dex.PublicURL = publicURL
	dex.CachedTickerExpireMins = 3
	// Update list of tokens twice everyday
	dex.CachedTokenExpireMins = 720
	return
}

// Trade describes HaloDEX trade item
type Trade struct {
	ID            string  `json:"id"`
	TokenGet      string  `json:"tokenGet"`
	TokenGive     string  `json:"tokenGive"`
	AmountGet     float64 `json:"amountGet,string,omitempty"`
	AmountGive    float64 `json:"amountGive,string"`
	TimeEpochNano int64   `json:"blockTimestamp,string"`
	Time          time.Time
	IsBuy         bool
	Price         float64
	Amount        float64
}

// FormatTrades transforms Trade attributes into formatted signle line string
func (*DEX) FormatTrades(trades []Trade) (s string) {
	if len(trades) == 0 {
		return "No data available"
	}
	pricedp, amountdp := 8, 8
	sign := ""
	s = "  Price        | Amount       | hh:mm:ss DD-MMM\n" + DashLine
	for _, trade := range trades {
		sign = "- "
		if trade.IsBuy {
			sign = "+ "
		}
		pricedp = 8
		if trade.Price > 100 {
			pricedp = 0
		}
		amountdp = 8
		if trade.Amount > 100 {
			amountdp = 0
		}
		s += sign + FillOrLimit(ReadableNum(trade.Price, pricedp), " ", 12) + " | "
		s += FillOrLimit(ReadableNum(trade.Amount, amountdp), " ", 12) + " | "
		s += FormatTimeReverse(trade.Time.UTC()) + "\n" + DashLine
	}
	return
}

// GetTradesWithGQLStr retrieves trades using pre-constructed GraphQL query string
func (dex *DEX) GetTradesWithGQLStr(gqlQueryStr, baseAddr string) (trades []Trade, err error) {
	request, err := http.NewRequest("POST", dex.GQLURL, bytes.NewBuffer([]byte(gqlQueryStr)))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return
	}
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	return dex.GetTradesFromResult(responseBytes, baseAddr)
}

// GetTradesFromResult extracts list of trades from the byte array of the API response
func (dex *DEX) GetTradesFromResult(jsonResultBytes []byte, baseAddr string) (trades []Trade, err error) {
	tradesResult := struct {
		Data struct {
			Trades []Trade `json:"trades"`
		} `json:"data"`
	}{}
	err = json.Unmarshal(jsonResultBytes, &tradesResult)
	if err != nil {
		return
	}

	trades = tradesResult.Data.Trades
	// Process received data to extract IsBuy, Price and Amount
	for i := 0; i < len(trades); i++ {
		trades[i].IsBuy = strings.ToUpper(trades[i].TokenGet) == strings.ToUpper(baseAddr)
		trades[i].Time = time.Unix(0, trades[i].TimeEpochNano).UTC()
		if trades[i].IsBuy {
			// buy
			trades[i].Amount = trades[i].AmountGive / 1e18
			trades[i].Price = trades[i].AmountGet / trades[i].AmountGive
			continue
		}

		// sell
		trades[i].Amount = trades[i].AmountGet / 1e18
		trades[i].Price = trades[i].AmountGive / trades[i].AmountGet
	}
	log.Println("[DEX] [GetTrades] received trades: ", len(trades))
	return
}

// GetTrades function retrieves recent trades from HaloDEX
func (dex *DEX) GetTrades(quoteAddr, baseAddr, limit string) (trades []Trade, err error) {
	//quick and dirty GQL query
	gqlQueryStr := `{
		"operationName": "trades",
		"query": "query trades($baseAddress: String!, $quoteAddress: String!) { ` +
		`trades(where: {OR: [{tokenGet: $baseAddress, tokenGive: $quoteAddress}, {tokenGet: $quoteAddress, tokenGive: $baseAddress}]}, orderBy: blockTimestamp_DESC, first: ` +
		limit + `) {id tokenGet tokenGive amountGet amountGive blockTimestamp __typename}}",
		"variables": {
			"baseAddress" : "` + baseAddr + `",
			"quoteAddress" : "` + quoteAddr + `"
		}
	}`
	return dex.GetTradesWithGQLStr(gqlQueryStr, baseAddr)
}

// GetTradesByTime retrieves trades since given blockstime
func (dex *DEX) GetTradesByTime(quoteAddr, baseAddr string, blockTime time.Time) (trades []Trade, err error) {
	gqlQueryStr := `{
		"operationName": "trades",
		"query": "query trades($baseAddress: String!, $quoteAddress: String!, $timeFrom: String!) ` +
		`{ trades(where: {OR: [{tokenGive: $baseAddress, tokenGet: $quoteAddress, blockTimestamp_gte: $timeFrom}, ` +
		`{tokenGive: $quoteAddress, tokenGet: $baseAddress, blockTimestamp_gte: $timeFrom}]}, ` +
		`orderBy: blockTimestamp_DESC) { id tokenGet amountGet amountGive blockTimestamp timestamp __typename } }",
		"variables": {
			"baseAddress" : "` + baseAddr + `",
			"quoteAddress" : "` + quoteAddr + `",
			"timeFrom": "000000000000000000000` + fmt.Sprint(blockTime.UTC().UnixNano()) + `"
		}
	}`
	return dex.GetTradesWithGQLStr(gqlQueryStr, baseAddr)
}

// Ticker describes a HaloDEX ticker response
type Ticker struct {
	Pair               string  `json:"pair"`
	BaseTicker         string  `json:"baseTicker"`
	QuoteTicker        string  `json:"quoteTicker"`
	Base               string  `json:"base"`
	Quote              string  `json:"quote"`
	QuoteVolume        string  `json:"quoteVolume"`
	BaseVolume         string  `json:"baseVolume"`
	Bid                string  `json:"bid"`
	Ask                string  `json:"ask"`
	Avg                string  `json:"avg"`
	TwoFourQuoteVolume float64 `json:"twoFourQuoteVolume,string"`
	TwoFourBaseVolume  float64 `json:"twoFourBaseVolume,string"`
	TwoFourBid         float64 `json:"twoFourBid,string"`
	TwoFourAsk         float64 `json:"twoFourAsk,string"`
	TwoFourAvg         float64 `json:"twoFourAvg,string"`
	Last               float64 `json:"last,string"`
	Timestamp          int64   `json:"timestamp,string"`
	// External/calculated attributes
	LastPriceUSD        float64
	TwoFourBidUSD       float64
	TwoFourAskUSD       float64
	TwoFourVolumeUSD    float64
	QuoteTokenSupply    float64
	QuoteTokenMarketCap float64
	LastUpdated         time.Time
}

// Format formats important ticker values into a string
func (ticker *Ticker) Format() string {
	base := FillOrLimit(ticker.BaseTicker, " ", 6)
	return fmt.Sprintf(""+
		"Pair          : %s\n"+DashLine+
		"Last Price    : %.8f %s | $%.8f\n"+DashLine+
		"24H High      : %.8f %s | $%.8f\n"+DashLine+
		"24H Low       : %.8f %s | $%.8f\n"+DashLine+
		"Total Supply  : %s\n"+DashLine+
		"Market Cap USD: $%s\n"+DashLine+
		"24H Volume    :\n"+DashLine+
		"      -%s : %s\n"+DashLine+ // Base Bolume
		"      -%s : %s\n"+DashLine+ // Quote Volume
		"      -%s : $%s\n"+DashLine+ // USD Volume
		"Last Updated  : %v UTC\n"+DashLine,
		ticker.Pair,
		ticker.Last, base, ticker.LastPriceUSD,
		ticker.TwoFourAsk, base, ticker.TwoFourAskUSD,
		ticker.TwoFourBid, base, ticker.TwoFourBidUSD,
		ConvertNumber(ticker.QuoteTokenSupply, 4),
		ConvertNumber(ticker.QuoteTokenMarketCap, 4),
		base,
		ConvertNumber(ticker.TwoFourBaseVolume, 4),
		FillOrLimit(ticker.QuoteTicker, " ", 6),
		ConvertNumber(ticker.TwoFourQuoteVolume, 4),
		FillOrLimit("USD", " ", 6),
		ConvertNumber(ticker.TwoFourVolumeUSD, 4),
		FormatTimeReverse(ticker.LastUpdated.UTC()),
	)
}

// GetTicker function retrieves available tickers from HaloDEX. Caching enabled.
func (dex *DEX) GetTicker(symbolQuote, symbolBase string, baseTokenPriceUSD, quoteTokenSupply float64) (ticker Ticker, err error) {
	// Use cache if available and not expired
	cachedTicker, available := dex.CachedTickers[symbolQuote+"/"+symbolBase]
	if available && time.Now().Sub(dex.CachedTickerLastUpdated).Minutes() < dex.CachedTickerExpireMins {
		log.Println("[DEX] [GetTicker] Using cached tickers")
		return cachedTicker, nil
	}

	tickerURL := fmt.Sprintf("%s/single/%s/%s", dex.PublicURL, symbolQuote, symbolBase)
	log.Println("[DEX] [GetTicker] ticker URL: ", tickerURL)

	response := &(http.Response{})
	response, err = http.Get(tickerURL)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	// API returns empty Array if not found!!
	if response.StatusCode != 200 || string(body) == "[]" {
		err = fmt.Errorf("Pair %s/%s not available on HaloDEX", symbolQuote, symbolBase)
		return
	}

	err = json.Unmarshal(body, &ticker)
	if err != nil {
		return
	}

	// TEMP FIX: switch 24H low and high price appropriately
	if ticker.TwoFourAsk < ticker.TwoFourBid {
		high := ticker.TwoFourBid
		low := ticker.TwoFourAsk
		ticker.TwoFourAsk = high
		ticker.TwoFourBid = low
	}
	ticker.LastPriceUSD = ticker.Last * baseTokenPriceUSD
	ticker.TwoFourAskUSD = ticker.TwoFourAsk * baseTokenPriceUSD
	ticker.TwoFourBidUSD = ticker.TwoFourBid * baseTokenPriceUSD
	ticker.QuoteTokenSupply = quoteTokenSupply
	ticker.TwoFourBaseVolume = ticker.TwoFourBaseVolume / 1e18
	ticker.TwoFourQuoteVolume = ticker.TwoFourQuoteVolume / 1e18
	ticker.TwoFourVolumeUSD = ticker.TwoFourQuoteVolume * ticker.LastPriceUSD
	ticker.QuoteTokenMarketCap = ticker.QuoteTokenSupply * ticker.LastPriceUSD
	ticker.LastUpdated = time.Now()
	dex.CachedTickerLastUpdated = ticker.LastUpdated

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
	if len(orders) == 0 {
		return "No orders available"
	}
	pricedp, amountdp := 8, 8
	sign := ""
	s = "diff\n  Price      | Amount    |Done| hh:mm:ss DD-MMM\n" + DashLine
	for _, order := range orders {
		sign = "- "
		if order.IsBuy {
			sign = "+ "
		}
		pricedp = 8
		if order.Price > 100 {
			pricedp = 0
		}
		amountdp = 8
		if order.Amount > 100 {
			amountdp = 0
		}
		percentDP := "0"
		if order.FilledPercent < 10 {
			percentDP = "1"
		}
		s += sign + FillOrLimit(ReadableNum(order.Price, pricedp), " ", 10) + " | "
		s += FillOrLimit(ReadableNum(order.Amount, amountdp), " ", 9) + " |"
		s += FillOrLimit(fmt.Sprintf("%."+percentDP+"f%%", order.FilledPercent), " ", 4) + "| "
		s += FormatTimeReverse(order.Time.UTC()) + "\n" + DashLine
	}
	return
}

// GetOrdersFromResult extracts orders from API response
func (dex DEX) GetOrdersFromResult(gqlQueryStr, quoteAddr string) (orders []Order, err error) {
	request, err := http.NewRequest("POST", dex.GQLURL, bytes.NewBuffer([]byte(gqlQueryStr)))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return
	}
	ordersResult := map[string]map[string][]Order{}

	err = json.NewDecoder(response.Body).Decode(&ordersResult)
	if err != nil {
		return
	}

	orders = ordersResult["data"]["orders"]
	// Process received data to extract IsBuy, Price and Amount
	for i := 0; i < len(orders); i++ {
		orders[i].IsBuy = strings.ToUpper(orders[i].TokenGet) == strings.ToUpper(quoteAddr)
		orders[i].Time = time.Unix(0, orders[i].TimeEpochNano).UTC()
		orders[i].FilledPercent = orders[i].FilledAmount / orders[i].AmountGet * 100
		if !orders[i].IsBuy {
			// buy
			orders[i].Amount = orders[i].AmountGive / 1e18
			orders[i].Price = orders[i].AmountGet / orders[i].AmountGive
			continue
		}

		// sell
		orders[i].Amount = orders[i].AmountGet / 1e18
		orders[i].Price = orders[i].AmountGive / orders[i].AmountGet
	}
	return
}

// GetOrders retrieves HaloDEX orders by user address
func (dex DEX) GetOrders(quoteAddr, baseAddr, limit, address string) (orders []Order, err error) {
	gqlQueryStr := `{
		"operationName": "users",
		"query": "query users($userAddress: String!, $baseAddress: String!, $quoteAddress: String!) ` +
		`{orders(where: {user: $userAddress, deleted: false, OR: [{tokenGive: $baseAddress, tokenGet: $quoteAddress}, {tokenGive: $quoteAddress, tokenGet: $baseAddress}]}, ` +
		`orderBy: blockTimestamp_DESC, first: ` + limit + `) {id amountGet amountGive blockTimestamp contract ` +
		`expires nonce deleted filled timestamp lastUpdated transactionID tokenGet tokenGive orderHash user __typename}}",
		"variables": { 
			"userAddress" : "` + address + `",
			"baseAddress" : "` + baseAddr + `",
			"quoteAddress" : "` + quoteAddr + `"
		  }
		}`
	addr := quoteAddr
	return dex.GetOrdersFromResult(gqlQueryStr, addr)
}

// GetOrderbook retrieves HaloDEX orderbook buy+sell
func (dex DEX) GetOrderbook(quoteAddr, baseAddr, limit string, buy bool) (orders []Order, err error) {
	//OR: [{tokenGive: $baseAddress, tokenGet: $quoteAddress}, {tokenGive: $quoteAddress, tokenGet: $baseAddress}]
	orderBy := "amountGet_DESC"
	if !buy {
		quoteAddr, baseAddr = baseAddr, quoteAddr
		orderBy = "amountGive_ASC"
	}
	gqlQueryStr := `{
		"operationName": "users",
		"query": "query users($baseAddress: String!, $quoteAddress: String!) ` +
		`{orders(where: {deleted: false, available_gt: \"0000000000000000000000000000000000000000\", ` +
		`tokenGive: $baseAddress, tokenGet: $quoteAddress}, ` +
		`orderBy: ` + orderBy + `, first: ` + limit + `) {id amountGet amountGive blockTimestamp contract ` +
		`expires nonce deleted filled timestamp lastUpdated transactionID tokenGet tokenGive orderHash user __typename}}",
		"variables": { 
			"baseAddress" : "` + baseAddr + `",
			"quoteAddress" : "` + quoteAddr + `"
		  }
		}`
	return dex.GetOrdersFromResult(gqlQueryStr, quoteAddr)
}

// Token describes data of available tokens on the HaloDEX
type Token struct {
	Number           int64  `json:"number"`
	Type             string `json:"type"` //"BASE" & "QUOTE"?
	BaseChain        string `json:"baseChain"`
	BaseChainAddress string `json:"baseChainAddress"`
	HaloChainAddress string `json:"haloAddress"`
	Ticker           string `json:"ticker"`
	Name             string `json:"name"`
	Decimals         int64  `json:"decimals"` // number of decimals supported
}

// Format formats Token
func (t *Token) Format() string {
	return fmt.Sprintf(""+
		"Name         : %s\n"+DashLine+
		"Ticker       : %s\n"+DashLine+
		"Type         : %s\n"+DashLine+
		"Decimals     : %d\n"+DashLine+
		"Base Chain   : %s\n"+DashLine+
		"Base Address : \n     %s\n"+DashLine+
		"Halo Address : \n     %s\n"+DashLine,
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
	if len(dex.CachedTokens) > 0 &&
		time.Now().Sub(dex.CachedTokenLastUpdated).Minutes() < dex.CachedTokenExpireMins {
		tokens = dex.CachedTokens
		log.Println("[DEX] [GetTokens] using cached tokens.")
		return
	}
	log.Println("[DEX] [GetTokens] updating DEX token cache")
	gqlQueryStr := `{"query":"{ supportedTokens { number type baseChain baseChainAddress haloAddress ticker name decimals }}"}`
	request, err := http.NewRequest("POST", dex.GQLURL, bytes.NewBuffer([]byte(gqlQueryStr)))
	if err != nil {
		log.Println("[DEX] [GetTokens] request error", err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return
	}

	result := struct {
		Data struct {
			SupportedTokens []Token `json:"supportedTokens"`
		} `json:"data"`
	}{}
	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return
	}
	dex.CachedTokens = map[string]Token{}
	for i, token := range result.Data.SupportedTokens {
		dex.CachedTokens[token.Ticker] = result.Data.SupportedTokens[i]
	}
	tokens = dex.CachedTokens
	dex.CachedTokenLastUpdated = time.Now()
	return
}

// Balance ////
type Balance struct {
	Available float64 `json:"available,string"`
	Balance   float64 `json:"balance,string"`
}

// GetBalance returns single balance of the specified address
func (dex DEX) GetBalance(userAddress string, tickerStr string) (balance float64, err error) {
	balances, err := dex.GetBalances(userAddress, []string{tickerStr})
	if err != nil {
		return
	}
	if len(balances) > 0 && len(balances[tickerStr]) > 0 {
		balance = balances[tickerStr][0].Balance
	}
	return
}

// GetBalances retrieves DEX account balances for one or more tickers by user address
func (dex DEX) GetBalances(userAddress string, tickers []string) (balances map[string][]Balance, err error) {
	// update cache if necessary
	tokens, err := dex.GetTokens()
	if err != nil {
		return
	}
	found := false
	variables := `"userAddress": "` + userAddress + `"`
	variableDeclarations := "$userAddress: String!"
	aliases := ""
	for i := 0; i < len(tickers); i++ {
		ticker, available := tokens[strings.ToUpper(tickers[i])]
		if !available {
			continue
		}
		found = true
		variables += `,"` + ticker.Ticker + `Address": "` + ticker.HaloChainAddress + `"`
		variableDeclarations += "$" + ticker.Ticker + "Address: String!"
		aliases += ticker.Ticker + `: balances(where: {user: $userAddress, token: $` +
			ticker.Ticker + `Address}, orderBy: blockTimestamp_DESC, first:1) {balance available}`

	}
	if !found {
		err = errors.New("Invalid or unsupported token")
		return
	}

	// TODO: improve GQL query to retrieve multiple token balances with a single query.
	gqlQueryStr := fmt.Sprintf(`{
		"operationName": "balances",
		"query":"query balances(%s) { %s }",
		"variables": { %s }
	}`, variableDeclarations, aliases, variables)

	request, err := http.NewRequest("POST", dex.GQLURL, bytes.NewBuffer([]byte(gqlQueryStr)))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	//client := &http.Client{}
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return
	}

	// result := map[string]map[string]map[string]float64{}
	result := struct {
		Data map[string][]Balance `json:"data"`
	}{}
	err = json.NewDecoder(response.Body).Decode(&result)
	for t := range result.Data {
		for k := range result.Data[t] {
			result.Data[t][k].Available /= 1e18
			result.Data[t][k].Balance /= 1e18
		}
	}
	balances = result.Data
	return
}

// GetBalancesFormatted returns formatted balances by user address and ticker symbol
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
		if !showZeroBalance && tokenBalance[0].Balance < 1e-10 {
			// Balance is zero
			continue
		}
		s += fmt.Sprintf("  %s| %s | %s\n%s",
			FillOrLimit(tickers[i], " ", 8),
			FillOrLimit(ReadableNum(tokenBalance[0].Balance, 8), " ", 14),
			FillOrLimit(ReadableNum(tokenBalance[0].Available, 8), " ", 14),
			DashLine,
		)
	}
	return
}
