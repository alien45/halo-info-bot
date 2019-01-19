package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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
	BlockHash        string    `json:"blockHash"`
	BlockNumber      int64     `json:"blockNumber"`
	From             string    `json:"from"`
	Gas              int64     `json:"gas"`
	GasPrice         string    `json:"gasPrice"`
	Hash             string    `json:"hash"`
	Input            string    `json:"input"`
	Nonce            int64     `json:"nonce"`
	To               string    `json:"to"`
	TransactionIndex int64     `json:"transactionIndex"`
	Value            string    `json:"value"`
	V                string    `json:"v"`
	R                string    `json:"r"`
	S                string    `json:"s"`
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
	TierNodes      map[string]float64 `json:"tiernodes"`
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
func (p Payout) Format() (s string) {
	s = fmt.Sprintf(""+
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
	return
}

// FormatAlert returns payout data as string for payout alert
func (p Payout) FormatAlert(blockURL string) (s string) {
	s = fmt.Sprintf("Delicious payout is served!```js\n%s"+DashLine+
		"         Tier 1  | Tier 2  | Tier 3  | Tier 4\n"+DashLine+
		"Rewards: %s | %s | %s | %s\n"+DashLine+
		"Nodes  : %s | %s | %s | %s\n```",
		p.Format(),
		FillOrLimit(FormatNum(p.Tiers["t1"]-p.HostingFeeHalo, 0), " ", 7),
		FillOrLimit(FormatNum(p.Tiers["t2"]-p.HostingFeeHalo, 0), " ", 7),
		FillOrLimit(FormatNum(p.Tiers["t3"]-p.HostingFeeHalo, 0), " ", 7),
		FillOrLimit(FormatNum(p.Tiers["t4"]-p.HostingFeeHalo, 0), " ", 7),
		FillOrLimit(FormatNum(p.TierNodes["t1"], 0), " ", 7),
		FillOrLimit(FormatNum(p.TierNodes["t2"], 0), " ", 7),
		FillOrLimit(FormatNum(p.TierNodes["t3"], 0), " ", 7),
		FillOrLimit(FormatNum(p.TierNodes["t4"], 0), " ", 7),
	)
	if p.BlockNumber > 0 {
		s += blockURL
	}
	s += "```fix\nDisclaimer: Actual amount received may vary from " +
		"the amounts displayed due to the tier distribution returned by " +
		"API includes ineligible node statuses.```"
	return
}

// FormatROI returns ROI after hosting fees deducted formatted as a string
//
// Params:
// @blockReward   float64             : number of coins minted per minting cycle
// @blockTimemins float64             : minting cycle duration in minutes
// @collateral    map[string] float64 : required collateral for each tier
func (p Payout) FormatROI(blockReward, blockTimeMins float64, collateral map[string]float64) (s string) {
	// deduct fees
	t1Reward := p.Tiers["t1"] - p.HostingFeeHalo
	t2Reward := p.Tiers["t2"] - p.HostingFeeHalo
	t3Reward := p.Tiers["t3"] - p.HostingFeeHalo
	t4Reward := p.Tiers["t4"] - p.HostingFeeHalo

	s += fmt.Sprintf(""+
		"            Tier 1 | Tier 2 | Tier 3 | Tier 4\n"+DashLine+
		"Halo/MN   : %s | %s | %s | %s\n",
		FillOrLimit(FormatNum(t1Reward, 0), " ", 6),
		FillOrLimit(FormatNum(t2Reward, 0), " ", 6),
		FillOrLimit(FormatNum(t3Reward, 0), " ", 6),
		FillOrLimit(FormatNum(t4Reward, 0), " ", 6),
	)
	// Reward per 400k Halo
	s += fmt.Sprintf(DashLine+
		"Halo/400k : %s | %s | %s | %s\n",
		FillOrLimit(FormatNum(t1Reward, 0), " ", 6),
		FillOrLimit(FormatNum(t2Reward/2, 0), " ", 6),
		FillOrLimit(FormatNum(t3Reward/5, 0), " ", 6),
		FillOrLimit(FormatNum(t4Reward/15, 0), " ", 6),
	)
	lastRMins := p.Minted / blockReward * blockTimeMins
	t1HourlyH := t1Reward / lastRMins * 60
	t2HourlyH := t2Reward / lastRMins * 60
	t3HourlyH := t3Reward / lastRMins * 60
	t4HourlyH := t4Reward / lastRMins * 60
	// ROI per hour
	s += fmt.Sprintf(DashLine+
		"Halo/hour : %s | %s | %s | %s\n",
		FillOrLimit(t1HourlyH, " ", 6),
		FillOrLimit(t2HourlyH, " ", 6),
		FillOrLimit(t3HourlyH, " ", 6),
		FillOrLimit(t4HourlyH, " ", 6),
	)
	t1DailyROI := (t1Reward / lastRMins * 1440) / collateral["t1"] * 100
	t2DailyROI := (t2Reward / lastRMins * 1440) / collateral["t2"] * 100
	t3DailyROI := (t3Reward / lastRMins * 1440) / collateral["t3"] * 100
	t4DailyROI := (t4Reward / lastRMins * 1440) / collateral["t4"] * 100
	// ROI days
	s += fmt.Sprintf(DashLine+
		"Days/100%% : %s | %s | %s | %s\n",
		FillOrLimit(FormatNum(100/t1DailyROI, 0), " ", 6),
		FillOrLimit(FormatNum(100/t2DailyROI, 0), " ", 6),
		FillOrLimit(FormatNum(100/t3DailyROI, 0), " ", 6),
		FillOrLimit(FormatNum(100/t4DailyROI, 0), " ", 6),
	)
	// ROI per day
	s += fmt.Sprintf(DashLine+
		"Daily     : %s%% | %s%% | %s%% | %s%%\n",
		FillOrLimit(t1DailyROI, " ", 5),
		FillOrLimit(t2DailyROI, " ", 5),
		FillOrLimit(t3DailyROI, " ", 5),
		FillOrLimit(t4DailyROI, " ", 5),
	)
	// ROI per week
	s += fmt.Sprintf(DashLine+
		"Weekly    : %s%% | %s%% | %s%% | %s%%\n",
		FillOrLimit(t1DailyROI*7, " ", 5),
		FillOrLimit(t2DailyROI*7, " ", 5),
		FillOrLimit(t3DailyROI*7, " ", 5),
		FillOrLimit(t4DailyROI*7, " ", 5),
	)
	// ROI per month
	s += fmt.Sprintf(DashLine+
		"Monthly   : %s%% | %s%% | %s%% | %s%%\n",
		FillOrLimit(t1DailyROI*30, " ", 5),
		FillOrLimit(t2DailyROI*30, " ", 5),
		FillOrLimit(t3DailyROI*30, " ", 5),
		FillOrLimit(t4DailyROI*30, " ", 5),
	)
	// ROI per year
	s += fmt.Sprintf(DashLine+
		"Yearly    : %s%% | %s%% | %s%% | %s%%\n",
		FillOrLimit(t1DailyROI*365, " ", 5),
		FillOrLimit(t2DailyROI*365, " ", 5),
		FillOrLimit(t3DailyROI*365, " ", 5),
		FillOrLimit(t4DailyROI*365, " ", 5),
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
	Address       string  `json:"ADDRESS"`
	Owner         string  `json:"OWNER"`
	Tier          int64   `json:"TIER"`
	Shares        float64 `json:"SHARES"`
	State         int64   `json:"STATE"`
	EpochTS       uint64  `json:"TIMESTAMP,string"`
	RewardBalance float64
}

// GetStatusName returns name of node status by state
func (m Masternode) GetStatusName() string {
	switch m.State {
	case 1:
		return "Initialize"
	case 2:
		return "Deposited"
	case 3:
		return "Active"
	case 4:
		return "Terminate"
	default:
		return "Unknown"
	}
}

// Format formats Masternode into string with cards layout
func (m Masternode) Format() string {
	return fmt.Sprintf(""+
		"Owner Address   :\n%s\n"+DashLine+
		"Contract Address:\n%s\n"+DashLine+
		"Tier   : %d          | Shares  : %s\n"+DashLine+
		"Status : %s  | Rewards : %s",
		m.Owner,
		m.Address,
		m.Tier,
		FormatNum(m.Shares, 0),
		FillOrLimit(m.GetStatusName(), " ", 9),
		FormatNum(m.RewardBalance, 0),
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
		nodes[i].RewardBalance, _ = m.GetMNRewardBalance(nodes[i].Address, nodes[i].Owner)
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
	var totalInvested float64
	var inactive float64
	var rewardBalance float64

	list = "    Address  |T|  Shares | Rewards | Status\n" + DashLine
	for i := 0; i < num; i++ {
		n := nodes[i]
		colorSign := "-"
		if n.State == 3 {
			// Active status
			colorSign = "+"
		}
		mlen := len(n.Address)
		nTxt := fmt.Sprintf(
			"%s|%s |%d| %s | %s| %s\n",
			colorSign,
			n.Address[:5]+".."+n.Address[mlen-3:],
			n.Tier,
			FillOrLimit(FormatNum(n.Shares, 0), " ", 7),
			FillOrLimit(FormatNum(n.RewardBalance, 0), " ", 8),
			n.GetStatusName(),
		)
		list += nTxt + DashLine
		tierShares[n.Tier] += n.Shares
		totalInvested += n.Shares
		rewardBalance += n.RewardBalance
		if n.State != 3 {
			inactive += n.Shares
		}
	}

	summary = "================== Summary ===================\n" +
		"Invested    | Active      | Inactive    | Nodes\n" + DashLine +
		fmt.Sprintf("%s| %s| %s| %d\n",
			FillOrLimit(FormatNumShort(totalInvested, 4), " ", 12),
			FillOrLimit(FormatNumShort(totalInvested-inactive, 4), " ", 12),
			FillOrLimit(FormatNumShort(inactive, 4), " ", 12),
			num)
	summary += "\nTier 1    | Tier 2    | Tier 3    | Tier 4\n" + DashLine +
		fmt.Sprintf("%s| %s| %s| %s\n",
			FillOrLimit(FormatNumShort(tierShares[1], 2), " ", 10),
			FillOrLimit(FormatNumShort(tierShares[2], 2), " ", 10),
			FillOrLimit(FormatNumShort(tierShares[3], 2), " ", 10),
			FillOrLimit(FormatNumShort(tierShares[4], 2), " ", 10),
		)
	if rewardBalance > 0 {
		summary += fmt.Sprintf("%sRewards Balance: %s", DashLine, FormatNum(rewardBalance, 0))
	}
	return
}

// GetMNRewardBalance retrieves masternode reward balance
func (m MNDApp) GetMNRewardBalance(contractAddr, ownerAddr string) (balance float64, err error) {
	return m.GetETHCallWeiToBalance(
		contractAddr,
		"0x13692c4d000000000000000000000000"+strings.TrimPrefix(ownerAddr, "0x"),
	)
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
	s = fmt.Sprintf(""+
		"Minted Coins : %s\n"+
		"Service Fees : %s\n"+
		"Total        : %s\n"+
		"Duration     : %s",
		FormatNum(minted, 0),
		FormatNum(fees, 0),
		FormatNum(minted+fees, 0),
		duration,
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
		fmt.Sprintf(
			"0x993ed2a5000000000000000000000000000000000000000000000000000000000000000%d",
			tierNo,
		),
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
