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
	summary := ""

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
	txt, summary = mndapp.FormatNodes(nodes)
SendMessage:
	_, err := discordSend(discord, channelID, "diff\n"+txt, true)
	logErrorTS(debugTag, err)
	if summary != "" {
		_, err = discordSend(discord, channelID, "js\n"+summary, true)
		logErrorTS(debugTag, err)
	}
}

func cmdMN(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int) {
	txt, err := mndapp.GetFormattedMNInfo()
	if commandErrorIf(err, discord, channelID, "Failed to retrieve data", debugTag) {
		return
	}
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
	mintedTime := time.Now().UTC()
	minted, err := mndapp.GetMintedBalance()
	if err != nil {
		logTS(debugTag+"] [GetMintedBalance", fmt.Sprint("Minted: ", minted, " [Error]: ", err))
		return
	}

	fees, err := mndapp.GetServiceFeesBalance()
	logErrorTS(debugTag+"] [GetServiceFeesBalance", err)
	tag := "] [NotPayout"
	prevRP := mndapp.RewardPool
	// Payout minimum 8 hours in minutes
	minDuration := 8 * 60 / mndapp.BlockTimeMins
	minPayout := mndapp.BlockReward * minDuration
	// Duration in minutes by calculating minted balance
	mintedDur := (minted / mndapp.BlockReward) * mndapp.BlockTimeMins
	// Duration in minutes since last payout
	lastDur := mintedTime.Sub(mndapp.LastPayout.Time).Minutes()
	// If Duration since last payout matches calculated duration from minted balance with +-block time
	validDiff := (lastDur-mndapp.BlockTimeMins) <= mintedDur || (lastDur+mndapp.BlockTimeMins) >= mintedDur
	if validDiff {
		// Prevent storing if API returned incorrect data from testnet
		mndapp.RewardPool.Minted = minted
		mndapp.RewardPool.Fees = fees
		mndapp.RewardPool.Time = mintedTime
	} else {
		tag += "] [FalsePositive"
	}
	isPayout := (minted <= mndapp.BlockReward || minted < prevRP.Minted) && prevRP.Minted != 0 && prevRP.Minted > minPayout
	if !isPayout || !validDiff {
		logTS(debugTag+tag, fmt.Sprintf(
			"Total: %f | Minted: %f | Fees: %f | Time: %s",
			minted+fees, minted, fees, client.FormatTS(mintedTime)))
		return
	}
	// Previously retrieved balance is higher than current
	// => means pool has been reset and payout occured
	tag = "] [Payout"
	p := client.Payout{}
	p.Total = prevRP.Minted + prevRP.Fees
	p.Minted = prevRP.Minted
	p.Fees = prevRP.Fees
	p.Time = prevRP.Time

	t1, t2, t3, t4, err := mndapp.GetAllTierDistribution()
	if logErrorTS(debugTag+tag, err) {
		return
	}
	if t1 < 1 || t2 < 1 || t3 < 1 || t4 < 1 {
		// Possible uncaught error occured during retrieving tier distribution. Retry.
		t1, t2, t3, t4, err = mndapp.GetAllTierDistribution()
		logErrorTS(debugTag+tag, err)
	}
	// Rewards received per MN on each tier and duration of reward cycle
	p.Tiers = map[string]float64{}
	p.Tiers["t1"], p.Tiers["t2"], p.Tiers["t3"], p.Tiers["t4"],
		p.Duration = mndapp.CalcReward(p.Minted, p.Fees, t1, t2, t3, t4)
	// Log
	logTS(debugTag+tag, fmt.Sprintf("Total: %f | Minted: %f | Fees: %f | Time: %s | "+
		"Distribution=> T1: %.0f, T2: %.0f, T3: %.0f, T4: %.0f",
		p.Total, p.Minted, p.Fees, client.FormatTS(p.Time), t1, t2, t3, t4))
	// update last payout details to config file
	mndapp.LastPayout = p
	data.LastPayout = p
	err = saveDiscordFile()
	if err != nil {
		logTS(debugTag+"] [File", fmt.Sprintf("Failed to save Payout Data to %s: %+v", discordFile, p, " | [Error]: ", err))
		// retry
		err = saveDiscordFile()
		if err != nil {
			logTS(debugTag+"] [File", fmt.Sprintf("Attempt 2: failed to save Payout Data to %s: %+v", discordFile, p, " | [Error]: ", err))
		}
	}

	alerts := data.Alerts.Payout
	// If last alert was sent within 8 hours, avoid duplicate/false alerts to masses in case of false positives
	if time.Now().Sub(mndapp.LastAlert).Minutes() <= minDuration {
		logTS(debugTag+tag, "Avoided sending false positive alert. Sent to test channel instead")
		alerts = map[string]string{
			"452277479160414223": "test channel",
		}
	}
	go sendPayoutAlerts(discord, p, alerts)
	mndapp.LastAlert = time.Now().UTC()
}

// sendPayoutAlerts sends out Discord payout alert to subscribed channels and users
func sendPayoutAlerts(discord *discordgo.Session, p client.Payout, channels map[string]string) (total, success, fail int) {
	total = len(channels)
	for channelID, name := range channels {
		txt := "Delicious payout is served!```js\n" + p.Format() + "``````fix\nDisclaimer: Actual amount received may vary from " +
			"the amounts displayed due to the tier distribution returned by API including ineligible node statuses.```"
		_, err := discordSend(discord, channelID, txt, false)
		if err != nil {
			logTS("PayoutAlert", fmt.Sprintf("Payout Alert Failed! Channel ID: %s, Name: %s", channelID, name))
			continue
		}
		success++
	}
	fail = total - success
	logTS("PayoutAlertSummary", fmt.Sprintf("Total channels: %d | Success: %d | Failure: %d", total, success, fail))
	return
}
