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
	BaseURL    string `json:"url"`
	MainnetGQL string `json:"urlgql"`
	// Number of coins minted on each minting cycle
	BlockReward float64 `json:"blockreward"`
	// Number of minutes of each minting cycle. NOT the actual block time of the chain.
	BlockTimeMins float64 `json:"blocktimemins"`
	// Collateral required for different tiers
	Collateral map[string]float64 `json:"collateral"`
	// Smart contract address to retrieve MN tier distribution
	TierDistContract string `json:"tierdistcontract"`
	// Smart contract address to retrieve minting pool balance and service fees
	RewardPoolContract string `json:"rewardpoolcontract"`
	RewardPool         Payout
	LastPayout         Payout
	LastAlert          time.Time
}

// Init instantiates MNDApp  struct
func (m *MNDApp) Init(baseURL, mainnetGQL string) {
	m.BaseURL = baseURL
	m.MainnetGQL = mainnetGQL

	m.BlockReward = 38
	m.BlockTimeMins = 4
	return
}

// Payout stores data of a given payout
type Payout struct {
	Minted   float64            `json:"minted"`
	Fees     float64            `json:"fees"`
	Total    float64            `json:"total"`
	Duration string             `json:"duration"` // duration string with "hh:mm" format
	Time     time.Time          `json:"time"`
	Tiers    map[string]float64 `json:"tiers,,omitempty"` // rewards/mn for each tier
}

// Format returns payout data as strings
func (p *Payout) Format() (s string) {
	s = fmt.Sprintf("\n-----------------Last Payout-------------------\n"+
		"Minted : %s    | Fees     : %s\n"+DashLine+
		"Total  : %s    | Duration : %s\n"+DashLine+
		"Time   : %s UTC (approx.)\n",
		FillOrLimit(p.Minted, " ", 10), FillOrLimit(p.Fees, " ", 10),
		FillOrLimit(p.Total, " ", 10), p.Duration,
		FillOrLimit(p.Time.UTC().String(), " ", 16),
	)
	s += fmt.Sprintf(DashLine+
		"Tier 1     | Tier 2    | Tier 3    | Tier 4\n"+DashLine+
		"%s     | %s    | %s    | %s\n"+DashLine,
		FillOrLimit(p.Tiers["t1"], " ", 6),
		FillOrLimit(p.Tiers["t2"], " ", 6),
		FillOrLimit(p.Tiers["t3"], " ", 6),
		FillOrLimit(p.Tiers["t4"], " ", 6),
	)
	return
}

// CalcReward calculates reward per masternode given minted coins, service fees and tier distribution
func (m MNDApp) CalcReward(minted, fees, t1, t2, t3, t4 float64) (t1r, t2r, t3r, t4r float64, duration string) {
	if t1 > 0 {
		t1r = (minted * 5 / m.BlockReward / t1) + (fees * 0.05 / t1)
	}
	if t2 > 0 {
		t2r = (minted * 8 / m.BlockReward / t2) + (fees * 0.10 / t2)
	}
	if t3 > 0 {
		t3r = (minted * 9 / m.BlockReward / t3) + (fees * 0.15 / t3)
	}
	if t4 > 0 {
		t4r = (minted * 15 / m.BlockReward / t4) + (fees * 0.275 / t4)
	}
	totalMins := (int(minted / m.BlockReward * m.BlockTimeMins))
	duration = fmt.Sprintf("%02d:%02d", int(totalMins/60), totalMins%60)
	return
}

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
		status = "Initializing"
		break
	case 2:
		status = "Deposited"
		break
	case 3:
		status = "Active"
		colorSign = "+"
		break
	}
	mlen := len(m.Address)
	return fmt.Sprintf(
		"%s | %s |  %d | %s | %s\n",
		colorSign,
		m.Address[:6]+"..."+m.Address[mlen-4:],
		m.Tier,
		FillOrLimit(fmt.Sprintf("%.0f", m.Shares), " ", 7),
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
	for i := 0; i < len(nodes); i++ {
		nodes[i].Shares /= 1e18
	}
	return
}

// FormatNodes formats a list of nodes in to table-like string
func (MNDApp) FormatNodes(nodes []Masternode) (list, summary string) {
	num := len(nodes)
	if num == 0 {
		list = "No masternodes available"
		return
	}

	tierShares := map[int64]float64{}
	totalInvested := float64(0)
	inactive := float64(0)

	list = "    Address       |Tier|  Shares | Status\n" + DashLine
	for i := 0; i < num; i++ {
		n := nodes[i]
		list += n.Format() + DashLine
		tierShares[n.Tier] += n.Shares
		totalInvested += n.Shares
		if n.State != 3 {
			inactive += n.Shares
		}
	}

	summary = "================== Summary ===================\n" +
		"Invested  | Active    | Inactive  | Nodes\n" + DashLine +
		fmt.Sprintf("%s| %s| %s| %d\n",
			FillOrLimit(totalInvested, " ", 10),
			FillOrLimit(totalInvested-inactive, " ", 10),
			FillOrLimit(inactive, " ", 10),
			num)
	summary += "\nTier 1    | Tier 2    | Tier 3    | Tier 4\n" + DashLine +
		fmt.Sprintf("%s| %s| %s| %s",
			FillOrLimit(tierShares[1], " ", 10),
			FillOrLimit(tierShares[2], " ", 10),
			FillOrLimit(tierShares[3], " ", 10),
			FillOrLimit(tierShares[4], " ", 10),
		)
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
	return m.GetETHCallWeiToBalance(m.RewardPoolContract, "0xbc3cde60")
}

// GetMintedBalance retrieves the total minted pool balance during the on-going payout cycle
func (m MNDApp) GetMintedBalance() (balance float64, err error) {
	return m.GetETHCallWeiToBalance(m.RewardPoolContract, "0x405187f4")
}

// GetFormattedPoolData returns reward pool data including minting and service pool balances as formatted strings
func (m MNDApp) GetFormattedPoolData() (s string, err error) {
	minted, err := m.GetMintedBalance()
	if err != nil {
		return
	}
	fees, err := m.GetServiceFeesBalance()
	if err != nil {
		return
	}
	totalMins := (int(minted / m.BlockReward * m.BlockTimeMins))
	duration := fmt.Sprintf("%02d:%02d", int(totalMins/60), totalMins%60)
	s = fmt.Sprintf("-----------------Minting Pool------------------\n"+
		"Minted : %s    | Fees     : %s\n"+DashLine+
		"Total  : %s    | Duration : %s\n"+DashLine,
		FillOrLimit(minted, " ", 10), FillOrLimit(fees, " ", 10),
		FillOrLimit(minted+fees, " ", 10), duration,
	)
	return
}

// GetTierDistribution retrieves the total number of active MNs for a specific tier
func (m MNDApp) GetTierDistribution(tierNo int) (filled float64, err error) {
	if tierNo < 1 || tierNo > 4 {
		err = errors.New("Invalid tier")
		return
	}
	filled, err = m.GetETHCallWeiToBalance(
		m.TierDistContract,
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

// GetFormattedMNInfo returns formatted string with masternode tiers and their collateral requirement
func (m MNDApp) GetFormattedMNInfo() (s string, err error) {
	s, err = m.GetFormattedPoolData()
	if err != nil {
		return
	}

	l := m.LastPayout
	s += l.Format()
	s += fmt.Sprintf("                   _______\n"+
		"__________________/Per 500\\____________________\n"+

		"%s     | %s    | %s     | %s\n",
		FillOrLimit(l.Tiers["t1"], " ", 6),
		FillOrLimit(l.Tiers["t2"]/2, " ", 6),
		FillOrLimit(l.Tiers["t3"]/5, " ", 6),
		FillOrLimit(l.Tiers["t4"]/15, " ", 6),
	)
	lastRMins := l.Minted / m.BlockReward * m.BlockTimeMins
	t1DailyROI := (l.Tiers["t1"] / lastRMins * 1440) / m.Collateral["t1"] * 100
	t2DailyROI := (l.Tiers["t2"] / lastRMins * 1440) / m.Collateral["t2"] * 100
	t3DailyROI := (l.Tiers["t3"] / lastRMins * 1440) / m.Collateral["t3"] * 100
	t4DailyROI := (l.Tiers["t4"] / lastRMins * 1440) / m.Collateral["t4"] * 100
	s += fmt.Sprintf("                   _______\n"+
		"__________________/ROI/Day\\____________________\n"+
		"%s%%    | %s%%   | %s%%    | %s%%\n",
		FillOrLimit(t1DailyROI, " ", 6),
		FillOrLimit(t2DailyROI, " ", 6),
		FillOrLimit(t3DailyROI, " ", 6),
		FillOrLimit(t4DailyROI, " ", 6),
	)
	s += fmt.Sprintf("                   ________\n"+
		"__________________/ROI/Year\\___________________\n"+
		"%s%%    | %s%%   | %s%%    | %s%%\n",
		FillOrLimit(t1DailyROI*365, " ", 6),
		FillOrLimit(t2DailyROI*365, " ", 6),
		FillOrLimit(t3DailyROI*365, " ", 6),
		FillOrLimit(t4DailyROI*365, " ", 6),
	)

	t1, t2, t3, t4, err := m.GetAllTierDistribution()
	if err != nil {
		return
	}
	s += fmt.Sprintf("                 ____________\n"+
		"________________/Filled Nodes\\_________________\n"+
		"%s    | %s   | %s    | %s\n",
		FillOrLimit(fmt.Sprintf("%.0f", t1), " ", 7),
		FillOrLimit(fmt.Sprintf("%.0f", t2), " ", 7),
		FillOrLimit(fmt.Sprintf("%.0f", t3), " ", 7),
		FillOrLimit(fmt.Sprintf("%.0f", t4), " ", 7),
	)
	s += fmt.Sprintf("                  __________\n"+
		"_________________/Collateral\\__________________\n"+
		"%s  | %s | %s  | %s\n",
		FillOrLimit(ConvertNumber(m.Collateral["t1"], 0), " ", 9),
		FillOrLimit(ConvertNumber(m.Collateral["t2"], 0), " ", 9),
		FillOrLimit(ConvertNumber(m.Collateral["t3"], 0), " ", 9),
		FillOrLimit(ConvertNumber(m.Collateral["t4"], 0), " ", 9),
	)

	return
}
