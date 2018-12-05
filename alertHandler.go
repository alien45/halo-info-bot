package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

func cmdAlert(discord *discordgo.Session, guildID, channelID, userID, username, debugTag string, cmdArgs []string, numArgs int) {
	// Enable/disable alerts. For personal chat. Possibly for channels as well but should only be setup by admins
	// TODO: dex status notification // using realtime API
	// TODO: feather update notification
	txt := ""
	action := ""
	exists := false
	if numArgs == 0 {
		txt = "Alert type required. Supported types: payout"
		goto AlertMessage
	}
	if numArgs >= 2 {
		action = strings.ToLower(cmdArgs[1])
	}
	_, exists = data.Alerts.Payout[channelID]

	switch strings.ToLower(cmdArgs[0]) {
	case "payout":
		txt = "Payout alert "
		if action == "send" && username == conf.Client.DiscordBot.RootUser {
			// Manually trigger payout alert. Only allowed by the root user
			if numArgs >= 4 {
				// Minted and fees supplied
				minted, _ := strconv.ParseFloat(cmdArgs[2], 64)
				fees, _ := strconv.ParseFloat(cmdArgs[3], 64)
				sendPayoutsManual(discord, channelID, minted, fees)
				return
			}
			discordSend(discord, channelID, "Payout alert triggered.", false)
			total, success, fail := sendPayoutAlerts(discord, mndapp.LastPayout, data.Alerts.Payout)
			txt = fmt.Sprintf("Payout alert sent. \nTotal channels: %d\nSuccess: %d\nFailed: %d", total, success, fail)
			goto AlertMessage
		} else if action == "on" {
			data.Alerts.Payout[channelID] = username
			txt += "turned on"
		} else if action == "off" {
			delete(data.Alerts.Payout, channelID)
			txt += "turned off"
		} else if exists {
			txt += "is on"
		} else {
			txt += "is off"
		}
		err := saveDiscordFile()
		if commandErrorIf(err, discord, channelID, "Failed to save preferences", debugTag) {
			return
		}
		break
	default:
		txt = "Not implemented or unavailable"
		break
	}
AlertMessage:
	discordSend(discord, channelID, txt, true)
}

// Check if payout occured and send alert messages
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
		debugTag += "] [FalsePositive"
	}
	isPayout := (minted <= mndapp.BlockReward || minted < prevRP.Minted) && prevRP.Minted != 0 && prevRP.Minted > minPayout
	logTS(debugTag, fmt.Sprintf(
		"Total: %f | Minted: %f | Fees: %f | Time: %s",
		minted+fees, minted, fees, client.FormatTS(mintedTime)))
	if !isPayout || !validDiff {
		return
	}
	// Previously retrieved balance is higher than current
	// => means pool has been reset and payout occured
	debugTag += "] [Payout"
	p := client.Payout{}
	p.Total = prevRP.Minted + prevRP.Fees
	p.Minted = prevRP.Minted
	p.Fees = prevRP.Fees
	p.Time = prevRP.Time

	t1, t2, t3, t4, err := mndapp.GetAllTierDistribution()
	if logErrorTS(debugTag, err) {
		return
	}
	if t1 < 1 || t2 < 1 || t3 < 1 || t4 < 1 {
		// Possible uncaught error occured on external API during retrieving tier distribution. Retry.
		t1, t2, t3, t4, err = mndapp.GetAllTierDistribution()
		logErrorTS(debugTag, err)
	}
	// Rewards received per MN on each tier and duration of reward cycle
	p.Tiers = map[string]float64{}
	p.Tiers["t1"], p.Tiers["t2"], p.Tiers["t3"], p.Tiers["t4"],
		p.Duration = mndapp.CalcReward(p.Minted, p.Fees, t1, t2, t3, t4)
	// Log
	logTS(debugTag, fmt.Sprintf("Total: %f | Minted: %f | Fees: %f | Time: %s | "+
		"Distribution=> T1: %.0f, T2: %.0f, T3: %.0f, T4: %.0f",
		p.Total, p.Minted, p.Fees, client.FormatTS(p.Time), t1, t2, t3, t4))
	// update last payout details to config file
	mndapp.LastPayout = p
	data.LastPayout = p
	err = saveDiscordFile()
	if err != nil {
		logTS(debugTag+"] [File", fmt.Sprintf("Failed to save Payout Data to %s: %+v | [Error]: %v", discordFile, p, err))
	}

	alerts := map[string]string{ //data.Alerts.Payout
		"452277479160414223": "test channel",
	}
	// If last alert was sent within 8 hours, avoid duplicate/false alerts to masses in case of false positives
	// if time.Now().Sub(mndapp.LastAlert).Minutes() <= minDuration {
	// 	logTS(debugTag+tag, "Avoided sending false positive alert. Sent to test channel instead")
	// alerts = map[string]string{
	// 	"452277479160414223": "test channel",
	// }
	// }
	go sendPayoutAlerts(discord, p, alerts)
	mndapp.LastAlert = time.Now().UTC()
}
func sendPayoutsManual(discord *discordgo.Session, userChannelID string, minted, fees float64) {
	debugTag := "sendPayoutsManual"
	minimumMinted := 8 * 60 * mndapp.BlockReward / mndapp.BlockTimeMins
	if minted == 0 || minted < minimumMinted {
		_, err := discordSend(discord, userChannelID, fmt.Sprintf("Minted total required and must greater than or equal to %.0f", minimumMinted), true)
		logErrorTS(debugTag, err)
		return
	}
	t1, t2, t3, t4, err := mndapp.GetAllTierDistribution()
	if commandErrorIf(err, discord, userChannelID, "Failed to retrieve tier distribution. Try again.", debugTag) {
		return
	}
	if t1 == 0 || t2 == 2 || t3 == 0 || t4 == 0 {
		_, err = discordSend(discord, userChannelID, fmt.Sprintf("Invalid tier distribution received.\n"+
			"Tier 1 :%.0f \nTier 2: %.0f \nTier 3: %.0f \nTier 4: %.0f", t1, t2, t3, t4), true)
		logErrorTS(debugTag, err)
		return
	}
	p := client.Payout{}
	p.Minted = minted
	p.Fees = fees
	p.Total = minted + fees
	p.Time = time.Now()
	p.Tiers = map[string]float64{}
	p.Tiers["t1"], p.Tiers["t1"], p.Tiers["t1"], p.Tiers["t1"], p.Duration = mndapp.CalcReward(minted, fees, t1, t2, t3, t4)
	mndapp.LastPayout = p
	mndapp.LastAlert = p.Time
	total, success, fail := sendPayoutAlerts(discord, p, data.Alerts.Payout)
	txt := fmt.Sprintf("Payout alert sent. \nTotal channels: %d\nSuccess: %d\nFailed: %d", total, success, fail)
	_, err = discordSend(discord, userChannelID, txt, true)
	logErrorTS(debugTag, err)
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
