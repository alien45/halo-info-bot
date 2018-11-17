package client

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// MNDApp struct handles API calls to Halo Masternodes DApp
type MNDApp struct {
	BaseURL string
}

// Init instantiates MNDApp  struct
func (m *MNDApp) Init(baseURL string) {
	m.BaseURL = baseURL
	return
}

var _ = `
"error": null,
"result": [
	{
		"ADDRESS": "0x3f2317d534262fcfEC4c076C7140d4D402913E6B",
		"TIMESTAMP": "1542490202232",
		"STATE": 3,
		"OWNER": "0x9516cfAD6C45c2F2b93b7c0F7B05297E24215A2E",
		"TIER": 4,
		"SHARES": 1e+21
	}
`

// Masternode describes Halo Platform masternode details
type Masternode struct {
	Address string  `json:"ADDRESS"`
	Owner   string  `json:"OWNER"`
	Tier    int64   `json:"TIER"`
	Shares  float64 `json:"SHARES"`
	State   int64   `json:"STATE"`
	EpochTS uint64  `json:"TIMESTAMP,string"`
}

// Format formats Masternode into string
func (m Masternode) Format() string {
	status := "Unknown"
	switch m.State {
	case 1:
		status = "Created"
		break
	case 2:
		status = "Deposited"
		break
	case 3:
		status = "Active"
		break
	}
	return fmt.Sprintf(
		"%s... |   %d  |  %s  | %s\n",
		FillOrLimit(m.Address, " ", 15),
		m.Tier,
		FillOrLimit(fmt.Sprintf("%.0f", m.Shares/1e18), " ", 7),
		status,
	)
}

// GetMasternodes retrieves list of masternodes by owner address
func (m *MNDApp) GetMasternodes(ownerAddress string) (nodes []Masternode, err error) {
	response, err := http.Get(m.BaseURL + "/owned/" + ownerAddress)
	if err != nil {
		return
	}

	result := struct {
		Error  string       `json:"error,omitempty"`
		Result []Masternode `json:"result,omitempty"`
	}{}

	err = json.NewDecoder(response.Body).Decode(&result)
	if err == nil {
		nodes = result.Result
	}
	return
}

// GetMasternodesFormatted returns a formatted string with masternodes owned by address
func (m *MNDApp) GetMasternodesFormatted(ownerAddress string) (s string, err error) {
	nodes, err := m.GetMasternodes(ownerAddress)
	if err != nil {
		s = "No masternodes available"
		return
	}
	s = "  Address          | Tier |  Shares   | Status\n" + dashLine
	for i := 0; i < len(nodes); i++ {
		s += nodes[i].Format() + dashLine
	}
	return
}
