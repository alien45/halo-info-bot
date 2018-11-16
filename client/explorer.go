package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Explorer struct handles all API requests relating to Halo Explorer
type Explorer struct {
	BaseURL           string
	MainnetGQL        string
	CachedTotalSupply float64
	CacheLastUpdated  time.Time
	CacheExpireMins   float64
}

// Init instantiates HaloExplorer with API URL
func (explorer *Explorer) Init(baseURL, mainnetGQL string) {
	explorer.BaseURL = baseURL
	explorer.MainnetGQL = mainnetGQL
	explorer.CacheExpireMins = 3
	return
}

// GetHaloSupply retrieves current total supply of Halo
func (explorer Explorer) GetHaloSupply() (total float64, err error) {
	// Use cache if available and not expired
	if explorer.CachedTotalSupply != 0 && time.Now().Sub(explorer.CacheLastUpdated).Minutes() < explorer.CacheExpireMins {
		fmt.Println("Using cached Halo supply")
		total = explorer.CachedTotalSupply
		return
	}

	// Update cache
	tickerURL := explorer.BaseURL + "/coin/total"
	fmt.Println("Retrieving total Halo supply from: ", tickerURL)
	response, err := http.Get(tickerURL)
	if err != nil {
		return
	}
	result := map[string]float64{}
	err = json.NewDecoder(response.Body).Decode(&result)
	total, _ = result["total"]
	return
}

// GetHaloBalance retrieves Halo address balance
func (explorer Explorer) GetHaloBalance(address string) (balance float64, err error) {
	fmt.Println("Retrieving Halo balance.")
	//quick and dirty GQL query
	gqlQueryStr := fmt.Sprintf(
		`{
			"id": %d,
			"method" : "eth_getBalance",
			"params": [ "%s", "latest" ]
		}`,
		time.Now().UTC().UnixNano(), // use current epoch timestamp nano as id
		address,
	)

	client := &http.Client{}
	request, err := http.NewRequest("POST", explorer.MainnetGQL, bytes.NewBuffer([]byte(gqlQueryStr)))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return
	}
	result := struct {
		Balance string `json:"result"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return
	}
	return WeiHexStrToBalance(result.Balance)
}
