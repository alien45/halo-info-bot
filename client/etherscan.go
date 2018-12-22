package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Etherscan handles API requests to http://etherscan.io
type Etherscan struct {
	BaseURL string `json:"url"`
	APIKey  string `json:"apikey"`
}

// Init instantiates Etherscan with required data to access the API
func (etherscan *Etherscan) Init(baseURL, apiKey string) {
	etherscan.BaseURL = baseURL
	etherscan.APIKey = apiKey
}

// GetEthBalance retrieves Ethereum address balance
func (etherscan Etherscan) GetEthBalance(address string) (balance float64, err error) {
	url := fmt.Sprintf(
		"%s?module=account&action=balance&address=%s&tag=latest&apikey=%s",
		etherscan.BaseURL,
		address,
		etherscan.APIKey,
	)
	log.Println("[Etherscan] [GetEthBalance] Retrieving ethereum balance from Etherscan")
	response, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	result := struct {
		Balance string `json:"result"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&result)
	if err != nil {
		return
	}
	return WeiHexStrToFloat64(result.Balance)
}
