package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Explorer struct handles all API requests relating to Halo Explorer
type Explorer struct {
	BaseURL           string `json:"url"`
	MainnetGQL        string `json:"urlgql"`
	Homepage          string `json:"homepage"`
	CachedTotalSupply float64
	CacheLastUpdated  time.Time
	CacheExpireMins   float64 `json:"cacheexpiremins"`
}

// Init instantiates HaloExplorer with API URL
func (explorer *Explorer) Init(baseURL, mainnetGQL string) {
	explorer.BaseURL = baseURL
	explorer.MainnetGQL = mainnetGQL
	explorer.CacheExpireMins = 3
	return
}

// GetHaloSupply retrieves current total supply of Halo
func (explorer Explorer) GetHaloSupply() (supply float64, err error) {
	err = errors.New("Deprecated")
	return
	// // Use cache if available and not expired
	// if explorer.CachedTotalSupply != 0 && time.Now().Sub(explorer.CacheLastUpdated).Minutes() < explorer.CacheExpireMins {
	// 	log.Println("[Explorer] [GetHaloSupply] Using cached Halo supply")
	// 	supply = explorer.CachedTotalSupply
	// 	return
	// }

	// // Update cache
	// log.Println("[Explorer] [GetHaloSupply] Retrieving total Halo supply ")
	// response, err := http.Get(explorer.BaseURL + "/coin/total")
	// if err != nil {
	// 	return
	// }
	// result := map[string]float64{}
	// err = json.NewDecoder(response.Body).Decode(&result)
	// if err != nil {
	// 	if response.StatusCode != http.StatusOK {
	// 		err = fmt.Errorf("API request failed! Status: %s", response.Status)
	// 	}
	// }
	// supply, _ = result["total"]

	// // TODO: TEMP FIX until Halo team fixes explorer API
	// if supply < 40*1e7 {
	// 	supply *= 800
	// }
	return
}

// GetHaloBalance retrieves Halo address balance
func (explorer Explorer) GetHaloBalance(address string) (balance float64, err error) {
	log.Println("[Explorer] [GetHaloBalance] Retrieving Halo balance.")
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
		if response.StatusCode != http.StatusOK {
			err = fmt.Errorf("API request failed! Status: %s", response.Status)
		}
		return
	}
	return WeiHexStrToFloat64(result.Balance)
}
