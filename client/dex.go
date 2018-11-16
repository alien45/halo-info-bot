package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const dashLine = "--------------------------------------------------\n"

// DEX struct handles all API requests relating to HaloDEX
type DEX struct {
	GQLURL    string
	PublicURL string

	// Smart Contract addresses of the tokens available on the HaloDEX
	TokenAddresses map[string]Token

	// Container for Ticker caching
	// key: pair (eg: halo/eth), value: Ticker
	CachedTickers map[string]Ticker

	// Cache duration in minutes
	CacheExpireMins  float64
	CacheLastUpdated time.Time
}

// Init instantiates HaloDEXHelper struct
//
// Params:
// gqlURL string : GraphQL based API URL
// publicURL string : Public REST API URL
func (dex *DEX) Init(gqlURL string, publicURL string) {
	dex.GQLURL = gqlURL
	dex.PublicURL = publicURL
	dex.CacheExpireMins = 3
	// TODO: retrieve & cache token address from HaloDEX API
	dex.TokenAddresses = map[string]Token{
		"eth":  Token{HaloChainAddress: "0xd314d564c36c1b9fbbf6b440122f84da9a551029"},
		"halo": Token{HaloChainAddress: "0x0000000000000000000000000000000000000000"},
		"vet":  Token{HaloChainAddress: "0x280750ccb7554faec2079e8d8719515d6decdc84"},
	}
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

	client := &http.Client{}
	request, err := http.NewRequest("POST", dex.GQLURL, bytes.NewBuffer([]byte(gqlQueryStr)))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return
	}
	tradesResult := map[string]map[string][]Trade{}

	err = json.NewDecoder(response.Body).Decode(&tradesResult)
	if err != nil {
		return
	}

	trades = tradesResult["data"]["trades"]
	// Process received data to extract IsBuy, Price and Amount
	for i := 0; i < len(trades); i++ {
		trades[i].IsBuy = strings.ToLower(trades[i].TokenGet) == strings.ToLower(baseAddr)
		trades[i].Time = time.Unix(0, trades[i].TimeEpochNano)
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
		return "No trades available"
	}
	s = "diff\n    Price        | Amount      | Time     DD-MM\n" + dashLine
	for _, trade := range trades {
		t := trade.Time
		if trade.IsBuy {
			s += "+ | "
		} else {
			s += "- | "
		}
		s += FillOrLimit(fmt.Sprintf("%.8f", trade.Price), " ", 12) + " | "
		s += FillOrLimit(fmt.Sprintf("%.2f", trade.Amount), " ", 7) + "     | "
		s += fmt.Sprintf("%02d:%02d:%02d %02d-%02d\n", t.Hour(), t.Minute(), t.Second(), t.Day(), t.Month()) + dashLine
	}
	return
}

// Ticker describes a HaloDEX ticker response
type Ticker struct {
	Pair               string  `json:"pair"`                      //": "HALO/ETH",
	BaseTicker         string  `json:"baseTicker"`                //": "ETH",
	QuoteTicker        string  `json:"quoteTicker"`               //": "HALO",
	Base               string  `json:"base"`                      //": "0xd314d564c36c1b9fbbf6b440122f84da9a551029",
	Quote              string  `json:"quote"`                     //": "0x0000000000000000000000000000000000000000",
	QuoteVolume        string  `json:"quoteVolume"`               //": "0.00000000",
	BaseVolume         string  `json:"baseVolume"`                //": "0.00000000",
	Bid                string  `json:"bid"`                       //": "0.00000000",
	Ask                string  `json:"ask"`                       //": "0.00000000",
	Avg                string  `json:"avg"`                       //": "0.00000000",
	TwoFourQuoteVolume float64 `json:"twoFourQuoteVolume,string"` //": "1963.26898478",
	TwoFourBaseVolume  float64 `json:"twoFourBaseVolume,string"`  //": "31.19938919",
	TwoFourBid         float64 `json:"twoFourBid,string"`         //": "0.01612903",
	TwoFourAsk         float64 `json:"twoFourAsk,string"`         //": "0.01500000",
	TwoFourAvg         float64 `json:"twoFourAvg,string"`         //": "0.01558046",
	Last               float64 `json:"last,string"`               //": "0.01550000",
	Timestamp          int64   `json:"timestamp,string"`          //": "1541851612755"
	// External/calculated attributes
	LastPriceUSD        float64
	TwoFourBidUSD       float64
	TwoFourAskUSD       float64
	TwoFourVolumeUSD    float64
	QuoteTokenSupply    float64
	QuoteTokenMarketCap float64
	LastUpdated         time.Time
}

// GetTicker function retrieves available tickers from HaloDEX. Caching enabled.
func (dex *DEX) GetTicker(symbolQuote, symbolBase string, baseTokenPriceUSD, quoteTokenSupply float64) (ticker Ticker, err error) {
	// Use cache if available and not expired
	cachedTicker, available := dex.CachedTickers[symbolQuote+"/"+symbolBase]
	if available && time.Now().Sub(dex.CacheLastUpdated).Minutes() < dex.CacheExpireMins {
		fmt.Println("Using cached tickers")
		return cachedTicker, nil
	}

	tickerURL := fmt.Sprintf("%s/single/%s/%s", dex.PublicURL, symbolQuote, symbolBase)
	fmt.Println(time.Now().UTC(), "[GetTicker] ticker URL: ", tickerURL)
	response := &(http.Response{})
	response, err = http.Get(tickerURL)
	if err != nil {
		return
	}
	body, err := ioutil.ReadAll(response.Body)
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
	dex.CacheLastUpdated = ticker.LastUpdated

	return
}

// Format formats important ticker values into a string
func (ticker *Ticker) Format() string {
	tickerMaxLength := len(ticker.BaseTicker)
	if len(ticker.BaseTicker) < len(ticker.QuoteTicker) {
		tickerMaxLength = len(ticker.QuoteTicker)
	}
	return fmt.Sprintf("Pair          : %s\n"+
		"Last Price    : %.8f | $%.4f\n"+
		"24H High      : %.8f | $%.4f\n"+
		"24H Low       : %.8f | $%.4f\n"+
		"Total Supply  : %s\n"+
		"Market Cap USD: $%s\n"+
		"24H Volume    :\n"+
		"  -%s : %s\n"+ // Base Bolume
		"  -%s : %s\n"+ // Quote Volume
		"  -%s : $%s\n"+ // USD Volume
		"Last Updated  : %v\n",
		ticker.Pair,
		ticker.Last, ticker.LastPriceUSD,
		ticker.TwoFourAsk, ticker.TwoFourAskUSD,
		ticker.TwoFourBid, ticker.TwoFourBidUSD,
		ConvertNumber(ticker.QuoteTokenSupply, 2),
		ConvertNumber(ticker.QuoteTokenMarketCap, 2),
		FillOrLimit(ticker.BaseTicker, " ", tickerMaxLength),
		ConvertNumber(ticker.TwoFourBaseVolume, 2),
		FillOrLimit(ticker.QuoteTicker, " ", tickerMaxLength),
		ConvertNumber(ticker.TwoFourQuoteVolume, 2),
		FillOrLimit("USD", " ", tickerMaxLength),
		ConvertNumber(ticker.TwoFourVolumeUSD, 2),
		ticker.LastUpdated.String(),
	)
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
	s = "diff\n    Price      | Amount | Filled | Time     DD-MM\n" + dashLine
	for _, order := range orders {
		t := order.Time
		if order.IsBuy {
			s += "+ | "
		} else {
			s += "- | "
		}
		s += FillOrLimit(fmt.Sprintf("%.8f", order.Price), " ", 10) + " | "
		s += FillOrLimit(fmt.Sprintf("%.4f", order.Amount), " ", 6) + " | "
		s += FillOrLimit(fmt.Sprintf("%.2f%%", order.FilledPercent), " ", 6) + " | "
		s += fmt.Sprintf("%02d:%02d:%02d %02d-%02d\n", t.Hour(), t.Minute(), t.Second(), t.Day(), t.Month()) + dashLine
	}
	return
}

// GetOrders retrieves HaloDEX orders by user address
func (dex *DEX) GetOrders(quoteAddr, baseAddr, limit, address string) (orders []Order, err error) {
	fmt.Println("GetOrders(), quote: ", quoteAddr, " base: ", baseAddr, " address: ", address)
	//quick and dirty GQL query
	gqlQueryStr := `{
		"operationName": "users",
		"query": "query users($userAddress: String!, $baseAddress: String!, $quoteAddress: String!) ` +
		`{\norders(where: {user: $userAddress, deleted: false, OR: [{tokenGive: $baseAddress, tokenGet: $quoteAddress}, {tokenGive: $quoteAddress, tokenGet: $baseAddress}]}, ` +
		`orderBy: blockTimestamp_DESC, first: ` + limit + `) {id amountGet amountGive blockTimestamp contract ` +
		`expires nonce deleted filled timestamp lastUpdated transactionID tokenGet tokenGive orderHash user __typename}}",
		"variables": { 
			"userAddress" : "` + address + `",
			"baseAddress" : "` + baseAddr + `",
			"quoteAddress" : "` + quoteAddr + `"
		  }
		}`

	client := &http.Client{}
	request, err := http.NewRequest("POST", dex.GQLURL, bytes.NewBuffer([]byte(gqlQueryStr)))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
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
	fmt.Println("Orders received: ", len(orders))
	// Process received data to extract IsBuy, Price and Amount
	for i := 0; i < len(orders); i++ {
		if i == 0 {
			fmt.Printf("%+v\n", orders[i])
		}
		amtGet := orders[i].AmountGet
		amtGive := orders[i].AmountGive
		orders[i].IsBuy = strings.ToLower(orders[i].TokenGet) == strings.ToLower(baseAddr)
		orders[i].Time = time.Unix(0, orders[i].TimeEpochNano)
		if orders[i].IsBuy {
			// buy
			orders[i].Amount = amtGet / 1e18
			orders[i].Price = amtGet / amtGive
			continue
		}

		// sell
		orders[i].Amount = amtGet / 1e18
		orders[i].Price = amtGive / amtGet
		orders[i].FilledPercent = orders[i].FilledAmount / amtGet * 100
		if i == 0 {
			fmt.Printf("%+v\n amountGive:%f \namountGet:%f \n Price: %f\n", orders[i], amtGive, amtGet, amtGive/amtGet)
		}
	}
	return
}

//Token TODO:
type Token struct {
	//"number": 0,
	//"type": "BASE",
	//"baseChain": "Ethereum",
	//"baseChainAddress": "0x70a41917365E772E41D404B3F7870CA8919b4fBe",
	HaloChainAddress string //"haloAddress": "0xd314d564c36c1b9fbbf6b440122f84da9a551029",
	Ticker           string //"ticker": "ETH",
	// Name             string //"name": "Ethereum",
	// Decimals         int    //"decimals": 18,
	//"__typename": "SupportedToken"
}

// Format formats Token
func (t *Token) Format() string {
	return fmt.Sprintf("%s", "Not implemented")
}

// GetTokens ....
func (dex *DEX) GetTokens() {

}
