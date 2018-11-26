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
	BaseURL       string             `json:"url"`
	MainnetGQL    string             `json:"urlgql"`
	BlockReward   float64            `json:"blockreward"`
	BlockTimeMins float64            `json:"blocktimemins"`
	Collateral    map[string]float64 `json:"collateral"`
	RewardPool    Payout
	LastPayout    Payout
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
	Duration string             `json:"duration"`
	Time     time.Time          `json:"time"`
	Tiers    map[string]float64 `json:"tiers,,omitempty"`
}

// Format returns payout data as strings
func (p *Payout) Format() (s string) {
	t1, t2, t3, t4 := p.Tiers["t1"], p.Tiers["t2"], p.Tiers["t3"], p.Tiers["t4"]
	return fmt.Sprintf(""+
		"Reward Pool (Halo)    | Tier | Per MN | Per 500\n"+DashLine+
		"Minted   : %s |   1  | %s | %.4f\n"+DashLine+
		"Fees     : %s |   2  | %s | %.4f\n"+DashLine+
		"Total    : %s |   3  | %s | %.4f\n"+DashLine+
		"Duration : %s      |   4  | %s | %.4f\n"+DashLine+
		"Time     : %s UTC (approx.)\n"+DashLine,
		FillOrLimit(fmt.Sprintf("%.0f", p.Minted), " ", 10), FillOrLimit(t1, " ", 6), t1,
		FillOrLimit(fmt.Sprintf("%.8f", p.Fees), " ", 10), FillOrLimit(t2, " ", 6), t2/2,
		FillOrLimit(fmt.Sprintf("%.8f", p.Total), " ", 10), FillOrLimit(t3, " ", 6), t3/5,
		p.Duration, FillOrLimit(t4, " ", 6), t4/15,
		p.Time.UTC().Format("2006-01-02 15:04"))
}

// CalcReward calculates reward per masternode given minted coins, service fees and tier distribution
func (m MNDApp) CalcReward(minted, fees, t1, t2, t3, t4 float64) (t1r, t2r, t3r, t4r float64, duration string) {
	t1r = (minted * 5 / m.BlockReward / t1) + (fees * 0.05 / t1)
	t2r = (minted * 8 / m.BlockReward / t2) + (fees * 0.10 / t2)
	t3r = (minted * 9 / m.BlockReward / t3) + (fees * 0.15 / t3)
	t4r = (minted * 15 / m.BlockReward / t4) + (fees * 0.275 / t4)
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
	mlen := len(m.Address)
	return fmt.Sprintf(
		"%s | %s |  %d | %s | %s\n",
		colorSign,
		m.Address[:6]+"..."+m.Address[mlen-6:],
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

	tierShareCount := map[int64]float64{}
	totalInvested := float64(0)
	inactive := float64(0)

	list = "    Address         |Tier|  Shares   | Status\n" + DashLine
	for i := 0; i < num; i++ {
		n := nodes[i]
		list += n.Format() + DashLine
		tierShareCount[n.Tier] += n.Shares
		totalInvested += n.Shares
		if n.State != 3 {
			inactive += n.Shares
		}
	}

	summary = fmt.Sprintf("============Summary============\n"+
		"Total Halo Invested: %.0f\n"+
		"Total Active: %.0f\n"+
		"Total Inactive: %.0f\n",
		totalInvested,
		totalInvested-inactive,
		inactive)
	for i := int64(1); i <= 4; i++ {
		count, _ := tierShareCount[i]
		summary += fmt.Sprintf("Tier %d: %.0f\n", i, count)
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

// FormatMNPoolRewardData formats Halo masternode reward pool and node distribution information into presentable text
func (m MNDApp) FormatMNPoolRewardData(minted, fees, t1, t2, t3, t4 float64) string {
	t1r, t2r, t3r, t4r, _ := m.CalcReward(m.BlockReward*1440/m.BlockTimeMins, fees, t1, t2, t3, t4)
	minutes := int(minted / m.BlockReward * m.BlockTimeMins)
	duration := fmt.Sprintf("%02d:%02d", int(minutes/60), minutes%60)
	return fmt.Sprintf("--------------------------24hr Reward Estimate--\n"+
		"Reward Pool      |Tier|Filld|  Per MN | Per 500\n"+DashLine+
		"Minted : %s|  1 | %.0f | %s  | %.4f\n"+DashLine+
		"Fees   : %s|  2 | %.0f | %s  | %.4f\n"+DashLine+
		"Total  : %s|  3 | %.0f | %s  | %.4f\n"+DashLine+
		"Elapsed: %s   |  4 | %.0f | %s  | %.4f\n",
		FillOrLimit(fmt.Sprintf("%.0f", minted), " ", 8), t1, FillOrLimit(t1r, " ", 6), t1r,
		FillOrLimit(fmt.Sprintf("%.4f", fees), " ", 8), t2, FillOrLimit(t2r/2, " ", 6), t2r/2,
		FillOrLimit(fmt.Sprintf("%.4f", minted+fees), " ", 8), t3, FillOrLimit(t3r/5, " ", 6), t3r/5,
		duration, t4, FillOrLimit(t4r/15, " ", 6), t4r/15)
}

// GetMNFormattedInfo returns formatted string with masternode tiers and their collateral requirement
func (m MNDApp) GetMNFormattedInfo(serviceFees float64) (s string, err error) {
	t1, t2, t3, t4, err := m.GetAllTierDistribution()
	if err != nil {
		return
	}
	dailyMinted := m.BlockReward * (1440 / m.BlockTimeMins)
	t1r, t2r, t3r, t4r, _ := m.CalcReward(dailyMinted, serviceFees, t1, t2, t3, t4)
	t1roi := t1r / m.Collateral["t1"] * 365 * 100
	t2roi := t2r / m.Collateral["t2"] * 365 * 100
	t3roi := t3r / m.Collateral["t3"] * 365 * 100
	t4roi := t4r / m.Collateral["t4"] * 365 * 100
	fmt.Println("ROI: ", t1roi, t2roi, t3roi, t4roi)

	t1str := fmt.Sprintf("  1  |  %.0f | 5000  | %s | %s | %s%%\n", m.Collateral["t1"],
		FillOrLimit(t1r, " ", 6), FillOrLimit(t1r, " ", 6), FillOrLimit(t1roi, " ", 6))

	t2str := fmt.Sprintf("  2  | %.0f | 4000  | %s | %s | %s%%\n", m.Collateral["t2"],
		FillOrLimit(t2r, " ", 6), FillOrLimit(t2r/2, " ", 6), FillOrLimit(t2roi, " ", 6))

	t3str := fmt.Sprintf("  3  | %.0f | 1000  | %s | %s | %s%%\n", m.Collateral["t3"],
		FillOrLimit(t3r, " ", 6), FillOrLimit(t3r/5, " ", 6), FillOrLimit(t3roi, " ", 6))

	t4str := fmt.Sprintf("  4  | %.0f |  500  | %s | %s | %s%%\n", m.Collateral["t4"],
		FillOrLimit(t4r, " ", 6), FillOrLimit(t4r/15, " ", 6), FillOrLimit(t4roi, " ", 6))

	s = "--- Collateral ------- ROI/Day in Halo ---------\n" +
		"Tier | Halo | Max MN | PerMN | Per500 | Per Year\n" + DashLine +
		t1str + DashLine +
		t2str + DashLine +
		t3str + DashLine +
		t4str + DashLine
	return
}
