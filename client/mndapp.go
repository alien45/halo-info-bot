package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// MNDApp struct handles API calls to Halo Masternodes DApp
type MNDApp struct {
	BaseURL               string
	MainnetGQL            string
	MintingPoolBalance    float64
	ServiceFeePoolBalance float64
	LastUpdated           time.Time

	PayoutTime   time.Time
	PayoutTotal  float64
	PayoutMinted float64
	PayoutFees   float64
}

// Init instantiates MNDApp  struct
func (m *MNDApp) Init(baseURL, mainnetGQL string) {
	m.BaseURL = baseURL
	m.MainnetGQL = mainnetGQL
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
	colorSign := "-"
	switch m.State {
	case 1:
		status = "Created"
		break
	case 2:
		status = "Deposited"
		break
	case 3:
		status = "Active"
		colorSign = "+"
		break
	}
	return fmt.Sprintf(
		"%s | %s... |   %d  |  %s  | %s\n",
		colorSign,
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
	nodes = result.Result
	return
}

// FormatNodes formats a list of nodes in to table-like string
func (MNDApp) FormatNodes(nodes []Masternode) (s string) {
	if len(nodes) == 0 {
		s = "No masternodes available"
		return
	}

	s = "diff\n      Address          | Tier |  Shares   | Status\n" + DashLine
	for i := 0; i < len(nodes); i++ {
		s += nodes[i].Format() + DashLine
	}
	return
}

// GetETHCallWeiToBalance retrieves invokes eth_call to a smart contract and converts retunted wei to balance
func (m MNDApp) GetETHCallWeiToBalance(contractAddress, data string) (balance float64, err error) {
	gqlQueryStr := fmt.Sprintf(`{
		"id": %d,
		"method": "eth_call",
		"params": [
		  {
			"to": "%s", 
			"data": "%s"
		  },
		  "latest"
		  ]
		}`,
		time.Now().UTC().UnixNano(), // use current epoch timestamp nano as ID
		contractAddress,
		data,
	)
	request, err := http.NewRequest("POST", m.MainnetGQL, bytes.NewBuffer([]byte(gqlQueryStr)))
	if err != nil {
		return
	}
	request.Header.Set("content-type", "application/json")

	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)
	result := struct {
		Result string `json:"result"`
	}{}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return
	}
	return WeiHexStrToFloat64(result.Result)
}

// GetServiceFeesBalance retrieves all service fees collected by Halo Platform during the on-going payout cycle
func (m MNDApp) GetServiceFeesBalance() (fees float64, err error) {
	return m.GetETHCallWeiToBalance("0xd674dd3cdf07139ffda85b8589f0e2ca600f996e", "0xbc3cde60")
}

// GetMintedBalance retrieves the total minted pool balance during the on-going payout cycle
func (m MNDApp) GetMintedBalance() (balance float64, err error) {
	return m.GetETHCallWeiToBalance("0xd674dd3cdf07139ffda85b8589f0e2ca600f996e", "0x405187f4")
}

// GetTierDistribution retrieves the total number of active MNs for a specific tier
func (m *MNDApp) GetTierDistribution(tierNo int) (filled float64, err error) {
	if tierNo < 1 || tierNo > 4 {
		err = errors.New("Invalid tier")
		return
	}
	filled, err = m.GetETHCallWeiToBalance(
		"0xb4bdce55ce08ad23715f160e9fed5f99275a9045",
		"0x993ed2a5000000000000000000000000000000000000000000000000000000000000000"+fmt.Sprint(tierNo),
	)
	if err == nil {
		filled *= 1e18
	}
	return
}

// GetAllTierDistribution returns number of active masternodes in each of the 4 tiers
func (m *MNDApp) GetAllTierDistribution() (t1, t2, t3, t4 float64, err error) {
	t1, err = m.GetTierDistribution(1)
	if err != nil {
		return
	}
	t2, err = m.GetTierDistribution(2)
	if err != nil {
		return
	}
	t3, err = m.GetTierDistribution(3)
	if err != nil {
		return
	}
	t4, err = m.GetTierDistribution(4)
	return
}

// FormatMNRewardDist formats Halo masternode reward pool and node distribution information into presentable text
func (MNDApp) FormatMNRewardDist(minted, fees, t1, t2, t3, t4 float64) string {
	return fmt.Sprintf("====Halo Reward Pool====  | ==Tier Distribution==\n"+
		"Minted       : %s | Tier 1 : %.0f/5000\n"+
		"Service Fees : %s | Tier 2 : %.0f/4000\n"+
		"Total        : %s | Tier 3 : %.0f/1000\n"+
		"Time         :            | Tier 4 : %.0f/500\n",
		FillOrLimit(fmt.Sprintf("%.0f", minted), " ", 10), t1,
		FillOrLimit(fmt.Sprintf("%.8f", fees), " ", 10), t2,
		FillOrLimit(fmt.Sprintf("%.8f", minted+fees), " ", 10), t3,
		t4)
}

// FormatPayout formats payout data
func (m *MNDApp) FormatPayout(minted, fees, t1, t2, t3, t4 float64) (s string) {
	t1Reward := 5/38*minted/t1 + 0.05*fees/t1
	t2Reward := 8/38*minted/t2 + 0.10*fees/t2
	t3Reward := 9/38*minted/t3 + 0.15*fees/t3
	t4Reward := 15/38*minted/t4 + 0.275*fees/t4

	return fmt.Sprintf("***********Payout***********\n"+
		"====Halo Reward Pool====  | ==Estimated Rewaed/MN==\n"+
		"Minted       : %s | Tier 1 : %.0f/5000\n"+
		"Service Fees : %s | Tier 2 : %.0f/4000\n"+
		"Total        : %s | Tier 3 : %.0f/1000\n"+
		"Time         :            | Tier 4 : %.0f/500\n",
		FillOrLimit(fmt.Sprintf("%.0f", minted), " ", 10), t1Reward,
		FillOrLimit(fmt.Sprintf("%.8f", fees), " ", 10), t2Reward,
		FillOrLimit(fmt.Sprintf("%.8f", minted+fees), " ", 10), t3Reward,
		t4Reward)
}
