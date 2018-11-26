package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

func cmdNodes(discord *discordgo.Session, channelID, debugTag string, cmdArgs, userAddresses []string, numArgs, numAddresses int) {
	addresses := cmdArgs
	addrs := map[string]int{}
	nodes := []client.Masternode{}
	txt := ""
	list := ""

	if numArgs == 0 {
		// No address supplied
		if numAddresses == 0 {
			// User has no saved addresses
			txt = "Owner address required"
			goto SendMessage
		}
		addresses = userAddresses
	}

	for i := 0; i < len(addresses); i++ {
		address := strings.ToUpper(addresses[i])
		if _, skip := addrs[address]; skip {
			// Dupplicate address
			continue
		}
		addrs[address] = 0
		iNodes, err := mndapp.GetMasternodes(addresses[i])
		if commandErrorIf(err, discord, channelID, "Failed to retrieve masternodes for "+addresses[i], debugTag) {
			return
		}
		nodes = append(nodes, iNodes...)
	}
	list, txt = mndapp.FormatNodes(nodes)
	// separate summary and list into two code blocks
	txt = "diff\n" + list + "``````js\n" + txt
SendMessage:
	_, err := discordSend(discord, channelID, txt, true)
	logErrorTS(debugTag, err)
}

func cmdMN(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int) {
	txt, err := mndapp.GetFormattedMNInfo()
	_, err = discordSend(discord, channelID, "js\n"+txt, true)
	logErrorTS(debugTag, err)
}

// discordInterval invoke a function periodically and only supplies Discord session as parameter
func discordInterval(discord *discordgo.Session, seconds int, executeOnInit bool, f func(discord *discordgo.Session)) {
	if executeOnInit {
		f(discord)
	}
	// Execute on interval
	for range time.Tick(time.Second * time.Duration(seconds)) {
		f(discord)
	}
}

func checkPayout(discord *discordgo.Session) {
	debugTag := "CheckPayout"
	minted, err := mndapp.GetMintedBalance()
	if err != nil {
		logTS(debugTag+"] [GetMintedBalance ", fmt.Sprint(err))
		return
	}
	fees, err := mndapp.GetServiceFeesBalance()
	logErrorTS(debugTag+"] [GetServiceFeesBalance", err)
	tag := "] [NotPayout"
	rp := mndapp.RewardPool
	if rp.Minted > minted || minted == 0 {
		// Previously retrieved balance is higher than current
		// => means pool has been reset and payout occured
		tag = "] [Payout"
		p := client.Payout{}
		p.Total = rp.Minted + rp.Fees
		p.Minted = rp.Minted
		p.Fees = rp.Fees
		p.Time = rp.Time

		t1, t2, t3, t4, err := mndapp.GetAllTierDistribution()
		logErrorTS("CheckPayout", err)
		// Prevent "assignment to nil map error"
		if p.Tiers == nil {
			p.Tiers = map[string]float64{}
		}
		p.Tiers["t1"],
			p.Tiers["t2"],
			p.Tiers["t3"],
			p.Tiers["t4"],
			p.Duration = mndapp.CalcReward(p.Minted, p.Fees, t1, t2, t3, t4)

		// update last payout details to config file
		data.LastPayout = mndapp.LastPayout
		err = saveDiscordFile()
		if err != nil {
			logTS(debugTag+"] [File", fmt.Sprintf("Failed to save Payout Data to %s: %+v", discordFile, p, " | [Error]: ", err))
		}

		go sendPayoutAlerts(discord, p)
	}
	logTS(debugTag+tag, fmt.Sprintf(
		" Total: %f | Minted: %f | Fees: %f | ApproxTime: %s",
		minted+fees, minted, fees, client.FormatTS(time.Now())))

	mndapp.RewardPool.Minted = minted
	mndapp.RewardPool.Fees = fees
	mndapp.RewardPool.Time = time.Now()
}

// sendPayoutAlerts
func sendPayoutAlerts(discord *discordgo.Session, p client.Payout) {
	// TODO: send alert to subscribed (add opt-in/out command) channels and users
	success := 1
	total := len(data.Alerts.Payout)
	for channelID, name := range data.Alerts.Payout {
		_, err := discordSend(
			discord,
			channelID,
			"Delicious payout is served @here!```js\n"+
				p.Format()+"``````fix\nPS: Actual amount received may be slightly higher "+
				"due to the tier distribution returned by API includes deposited nodes!```",
			false)
		if err != nil {
			logTS("PayoutAlert", fmt.Sprintf("Payout Alert Failed! Channel ID: %s, Name: %s", channelID, name))
			continue
		}
		// success
		success++
	}
	logTS("PayoutAlertSummary", fmt.Sprintf("Total alerts: %d | Success: %d | Failure: %d", total, success, total-success))
}
