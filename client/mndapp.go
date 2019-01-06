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
	CheckPayout     bool   `json:"checkpayout"`
	IntervalSeconds int    `json:"intervalseconds"`
	AutomateAlert   bool   `json:"automatealert"`
	BaseURL         string `json:"url"`
	MainnetGQL      string `json:"urlgql"`
	// Number of coins minted on each minting cycle
	BlockReward float64 `json:"blockreward"`
	// Number of minutes of each minting cycle. NOT the actual block time of the chain.
	BlockTimeMins float64 `json:"blocktimemins"`
	// Collateral required for different tiers
	Collateral map[string]float64 `json:"collateral"`
	// Smart contract address to retrieve minting pool balance and service fees
	RewardPoolContract string `json:"rewardpoolcontract"`
	// Smart contract address to retrieve MN tier distribution
	TierDistContract string           `json:"tierdistcontract"`
	TierBlockRewards TierBlockRewards `json:"tierblockrewards"`
	HostingFeeUSD    float64          `json:"hostingfeeusd"`
	// Cached data
	RewardPool         Payout
	LastPayout         Payout
	LastAlert          time.Time
	tierDistCache      map[int]float64
	tierDistCachedTime time.Time
}

// TierBlockRewards block reward distribution per tier
type TierBlockRewards map[string]struct {
	// Number of minted coins for the tier from each block
	Minted float64 `json:"minted"`
	// Percent of fees for the tier. Ex: use 0.05 for 5%
	FeesPercent float64 `json:"feespercent"`
}

// Init instantiates MNDApp  struct
func (m *MNDApp) Init(baseURL, mainnetGQL string) {
	m.BaseURL = baseURL
	m.MainnetGQL = mainnetGQL

	m.BlockReward = 38
	m.BlockTimeMins = 4
	return
}

// PayoutTX Temp.
type PayoutTX struct {
	BlockHash        string    `json:"blockHash"`        //: "0xade90ac9ca5786bbd56139b6d10a6dc931dd14bfce55150b3ab46c75f409c29a",
	BlockNumber      int64     `json:"blockNumber"`      //: 33320561,
	From             string    `json:"from"`             //: "0xc31aC2C9a88F8427f1a5Ac3Ae92768De34cf2a65",
	Gas              int64     `json:"gas"`              //: 599000000,
	GasPrice         string    `json:"gasPrice"`         //: "0",
	Hash             string    `json:"hash"`             //: "0x0a7afe86712e79fc1f29e86d95a4327556c848abad1b38d776c7d7682408f21d",
	Input            string    `json:"input"`            //: "0xdf6c39fb000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000030cdb",
	Nonce            int64     `json:"nonce"`            //: 2209,
	To               string    `json:"to"`               //: "0xC660934eC084698E373AC844cE29cf27B104F696",
	TransactionIndex int64     `json:"transactionIndex"` //: 0,
	Value            string    `json:"value"`            //: "0",
	V                string    `json:"v"`                //: "0x1c",
	R                string    `json:"r"`                //: "0xf6c914d7a8cd0325701fd75d5b1a272ebb095e866a045c5ade133676ae9617a2",
	S                string    `json:"s"`                //: "0x6dc7ea2ed9d0ba5729d88c88bfa94ee17498b592be2c4a2860bf0522ab972a13"
	TS               time.Time `json:"ts"`
	Processed        bool      `json:"processed"`
}

// Payout stores data of a given payout
type Payout struct {
	Minted         float64            `json:"minted"`
	Fees           float64            `json:"fees"`
	Total          float64            `json:"total"`
	Duration       string             `json:"duration"` // duration string with "hh:mm" format
	Time           time.Time          `json:"time"`
	Tiers          map[string]float64 `json:"tiers,,omitempty"` // rewards/mn for each tier
	HostingFeeUSD  float64            `json:"hostingfeeusd"`
	HostingFeeHalo float64            `json:"hostingfeehalo"`
	Price          float64            `json:"price"` // Price US$/Halo
	BlockNumber    int64              `json:"blocknumber"`
	AlertData      AlertData          `json:"alertdata"`
}

// AlertData payout alert data
type AlertData struct {
	AlertSent    bool      `json:"alertsent"`
	Total        int       `json:"total"`
	SuccessCount int       `json:"successcount"`
	FailCount    int       `json:"failcount"`
	Messages     []Message `json:"messages"` // channel id : message ID
}

// Message alert message information
type Message struct {
	ID        string `json:"id"`
	ChannelID string `json:"channelid"`
	Sent      bool   `json:"sent"`
	Error     string `json:"error"`
}

// Format returns payout data as strings
func (p *Payout) Format() (s string) {
	s = fmt.Sprintf("\n------------------- Payout -------------------\n"+
		"Time   : %s UTC (approx.)\n"+DashLine+
		"Minted : %s    | Fees     : %s\n"+DashLine+
		"Total  : %s    | Duration : %s\n"+DashLine+
		"Hosting Fee/MN: $%s (%sH) @ $%s/H\n",
		FillOrLimit(p.Time.UTC().String(), " ", 16),
		FillOrLimit(FormatNum(p.Minted, 0), " ", 10),
		FillOrLimit(FormatNum(p.Fees, 0), " ", 10),
		FillOrLimit(FormatNum(p.Total, 0), " ", 10),
		p.Duration,
		FormatNum(p.HostingFeeUSD, 4),
		FillOrLimit(FormatNum(p.HostingFeeHalo, 0), " ", 4),
		FillOrLimit(p.Price, " ", 10),
	)
	s += fmt.Sprintf(DashLine+
		"Tier 1     | Tier 2    | Tier 3    | Tier 4\n"+DashLine+
		"%s     | %s    | %s    | %s\n"+DashLine,
		FillOrLimit(FormatNum(p.Tiers["t1"], 0), " ", 6),
		FillOrLimit(FormatNum(p.Tiers["t2"], 0), " ", 6),
		FillOrLimit(FormatNum(p.Tiers["t3"], 0), " ", 6),
		FillOrLimit(FormatNum(p.Tiers["t4"], 0), " ", 6),
	)
	return
}

// CalcReward calculates reward per masternode given minted coins, service fees and tier distribution
//
// Params:
// minted float64 : number of minted coins for the payout cycle
// fees   float64 : total accumulated fees for the cycle
// t1s    float64 : number of active nodes in tier 1
// t2s    float64 : number of active nodes in tier 2
// t3s    float64 : number of active nodes in tier 3
// t4s    float64 : number of active nodes in tier 4
func (m MNDApp) CalcReward(minted, fees, t1s, t2s, t3s, t4s float64) (
	t1r, t2r, t3r, t4r float64, duration string) {
	if t1s > 0 {
		r, _ := m.TierBlockRewards["t1"]
		t1r = (minted * r.Minted / m.BlockReward / t1s) + (fees * r.FeesPercent / t1s)
	}
	if t2s > 0 {
		r, _ := m.TierBlockRewards["t2"]
		t2r = (minted * r.Minted / m.BlockReward / t2s) + (fees * r.FeesPercent / t2s)
	}
	if t3s > 0 {
		r, _ := m.TierBlockRewards["t3"]
		t3r = (minted * r.Minted / m.BlockReward / t3s) + (fees * r.FeesPercent / t3s)
	}
	if t4s > 0 {
		r, _ := m.TierBlockRewards["t4"]
		t4r = (minted * r.Minted / m.BlockReward / t4s) + (fees * r.FeesPercent / t4s)
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
	status := "Inactive"
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
	case 4:
		status = "Terminated"
		break
	}
	mlen := len(m.Address)
	return fmt.Sprintf(
		"%s | %s |  %d | %s | %s\n",
		colorSign,
		m.Address[:6]+"..."+m.Address[mlen-4:],
		m.Tier,
		FillOrLimit(FormatNum(m.Shares, 0), " ", 7),
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
	if err != nil {
		if response.StatusCode != http.StatusOK {
			err = fmt.Errorf("API request failed! Status: %s", response.Status)
		}
		return
	}
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
			FillOrLimit(FormatNum(totalInvested, 0), " ", 10),
			FillOrLimit(FormatNum(totalInvested-inactive, 0), " ", 10),
			FillOrLimit(FormatNum(inactive, 0), " ", 10),
			num)
	summary += "\nTier 1    | Tier 2    | Tier 3    | Tier 4\n" + DashLine +
		fmt.Sprintf("%s| %s| %s| %s",
			FillOrLimit(FormatNum(tierShares[1], 0), " ", 10),
			FillOrLimit(FormatNum(tierShares[2], 0), " ", 10),
			FillOrLimit(FormatNum(tierShares[3], 0), " ", 10),
			FillOrLimit(FormatNum(tierShares[4], 0), " ", 10),
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
		if response.StatusCode != http.StatusOK {
			err = fmt.Errorf("API request failed! Status: %s", response.Status)
		}
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
func (m *MNDApp) GetFormattedPoolData() (s string, err error) {
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
	s = fmt.Sprintf("------------------Reward Pool------------------\n"+
		"Minted : %s    | Fees     : %s\n"+DashLine+
		"Total  : %s    | Duration : %s\n"+DashLine,
		FillOrLimit(FormatNum(minted, 0), " ", 10),
		FillOrLimit(FormatNum(fees, 0), " ", 10),
		FillOrLimit(FormatNum(minted+fees, 0), " ", 10), duration,
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
		"0x993ed2a5000000000000000000000000000000000000000000000000000000000000000"+
			fmt.Sprint(tierNo),
	)
	if err != nil {
		return
	}
	filled *= 1e18
	return
}

// GetAllTierDistribution returns number of active masternodes in each of the 4 tiers
func (m *MNDApp) GetAllTierDistribution() (t1, t2, t3, t4 float64, err error) {
	if m.tierDistCache == nil {
		m.tierDistCache = map[int]float64{}
	}
	expired := time.Now().Sub(m.tierDistCachedTime).Minutes() > 15
	for i := 1; i < 5; i++ {
		if !expired && m.tierDistCache[i] > 0 {
			// Use cache
			continue
		}
		d, erri := m.GetTierDistribution(i)
		if erri == nil {
			m.tierDistCache[i] = d
			continue
		}
		err = erri
		// In case of http/RPC failure, use cache if exists
		if m.tierDistCache[i] > 0 {
			continue
		}
	}
	m.tierDistCachedTime = time.Now()
	t := m.tierDistCache
	return t[1], t[2], t[3], t[4], err
}

// GetFormattedMNInfo returns formatted string with masternode tiers and their collateral requirement
func (m *MNDApp) GetFormattedMNInfo() (s string, err error) {
	s, err = m.GetFormattedPoolData()
	if err != nil {
		return
	}

	l := m.LastPayout
	s += l.Format()
	s += fmt.Sprintf("                  _______\n"+
		"_________________/Per 400K\\____________________\n"+

		"%s     | %s    | %s     | %s\n",
		FillOrLimit(FormatNum(l.Tiers["t1"], 0), " ", 6),
		FillOrLimit(FormatNum(l.Tiers["t2"]/2, 0), " ", 6),
		FillOrLimit(FormatNum(l.Tiers["t3"]/5, 0), " ", 6),
		FillOrLimit(FormatNum(l.Tiers["t4"]/15, 0), " ", 6),
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
		FillOrLimit(FormatNum(t1, 0), " ", 7),
		FillOrLimit(FormatNum(t2, 0), " ", 7),
		FillOrLimit(FormatNum(t3, 0), " ", 7),
		FillOrLimit(FormatNum(t4, 0), " ", 7),
	)
	s += fmt.Sprintf("                  __________\n"+
		"_________________/Collateral\\__________________\n"+
		"%s  | %s | %s  | %s\n",
		FillOrLimit(FormatNumShort(m.Collateral["t1"], 0), " ", 9),
		FillOrLimit(FormatNumShort(m.Collateral["t2"], 0), " ", 9),
		FillOrLimit(FormatNumShort(m.Collateral["t3"], 0), " ", 9),
		FillOrLimit(FormatNumShort(m.Collateral["t4"], 0), " ", 9),
	)

	return
}
