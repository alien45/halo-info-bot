package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

var payoutTXReceived bool
var payoutTX client.PayoutTX

func cmdAlert(discord *discordgo.Session, guildID, channelID, userID, username, debugTag string, cmdArgs []string, numArgs int) {
	// Enable/disable alerts. For personal chat. Possibly for channels as well but should only be setup by admins
	// TODO: dex status notification // using realtime API
	// TODO: feather update notification
	isRoot := username == conf.Client.DiscordBot.RootUser
	isAdmin := userHasRole(discord, guildID, userID, guildAdminRole)
	allowed := guildID == "" || isAdmin || isRoot
	txt := ""
	action := ""
	alertType := ""
	exists := false
	saveData := false
	var err error
	if numArgs == 0 {
		txt = "Alert type required. Supported types: payout"
		goto AlertMessage
	}
	alertType = strings.ToLower(cmdArgs[0])
	if numArgs < 2 {
		txt = "Action required"
		goto AlertMessage
	}
	action = strings.ToLower(cmdArgs[1])
	_, exists = data.Alerts.Payout[channelID]

	switch alertType + " " + action {
	case "payout send":
		if !isRoot {
			return
		}
		// Manually trigger payout alert. Only allowed by the root user
		if numArgs >= 4 {
			// Minted and fees supplied
			minted, err := strconv.ParseFloat(cmdArgs[2], 64)
			if commandErrorIf(err, discord, channelID, "Invalid minted total supplied", debugTag) {
				return
			}
			fees, err := strconv.ParseFloat(cmdArgs[3], 64)
			if commandErrorIf(err, discord, channelID, "Invalid service fees supplied", debugTag) {
				return
			}
			triggerPayoutsAlert(discord, channelID, minted, fees)
			return
		}
		discordSend(discord, channelID, "Payout alert triggered.", false)
		total, success, fail := sendPayoutAlerts(discord, mndapp.LastPayout, data.Alerts.Payout)
		txt = fmt.Sprintf("Payout alert sent. \nTotal channels: %d\nSuccess: %d\nFailed: %d", total, success, fail)
		break
	case "payout update":
		if !isRoot {
			return
		}
		if numArgs < 4 {
			txt = "Minted total and service fees required."
			break
		}

		minted, err := strconv.ParseFloat(cmdArgs[2], 64)
		if commandErrorIf(err, discord, channelID, "Invalid minted total supplied", debugTag) {
			return
		}
		fees, err := strconv.ParseFloat(cmdArgs[3], 64)
		if commandErrorIf(err, discord, channelID, "Invalid service fees supplied", debugTag) {
			return
		}
		data.LastPayout.Minted = minted
		data.LastPayout.Fees = fees
		data.LastPayout.Total = minted + fees
		t1, t2, t3, t4, err := mndapp.GetAllTierDistribution()
		if commandErrorIf(err, discord, channelID, "Failed to retrieve tier distribution", debugTag) {
			return
		}
		t1r, t2r, t3r, t4r, duration := mndapp.CalcReward(minted, fees, t1, t2, t3, t4)
		data.LastPayout.Duration = duration
		data.LastPayout.Tiers["t1"] = t1r
		data.LastPayout.Tiers["t2"] = t2r
		data.LastPayout.Tiers["t3"] = t3r
		data.LastPayout.Tiers["t4"] = t4r

		data.LastPayout.HostingFeeHalo,
			data.LastPayout.HostingFeeUSD,
			data.LastPayout.Price, _ = getHostingFee(duration)

		discordSend(discord, channelID, "Payout update triggered.", false)
		chMsgIDs := map[string]string{}
		for _, msg := range data.LastPayout.AlertData.Messages {
			chMsgIDs[msg.ChannelID] = msg.ID
		}
		total, success, fail := updatePayoutAlerts(discord, data.LastPayout, chMsgIDs)
		txt = fmt.Sprintf("Payout alert updated. \nTotal channels: %d\nSuccess: %d\nFailed: %d", total, success, fail)
		mndapp.LastPayout = data.LastPayout
		break
	case "payout on":
		if !allowed {
			txt = "You do not have permission to enable alerts on this channel."
			goto AlertMessage
		}
		data.Alerts.Payout[channelID] = fmt.Sprintf("%s#%s@%s|%s", guildID, channelID, username, userID)
		txt = "Payout alert is turned on"
		saveData = true
		break
	case "payout off":
		delete(data.Alerts.Payout, channelID)
		txt = "Payout alert is turned off"
		saveData = true
		break
	case "payout status":
		txt = "Payout alert is turned off"
		if exists {
			txt = "Payout alert is turned on"
		}
		break
	default:
		txt = "Not implemented or unavailable"
		break
	}
	if saveData {
		err = saveDataFile()
		if commandErrorIf(err, discord, channelID, "Failed to save preferences", debugTag) {
			return
		}
	}
AlertMessage:
	discordSend(discord, channelID, txt, true)
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

// Check if payout occured and send alert messages
func checkPayout(discord *discordgo.Session) {
	if !payoutTXReceived {
		checkPayoutEvent(discord)
	}
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
	// minDuration := 8 * 60 / mndapp.BlockTimeMins
	// minPayout := mndapp.BlockReward * minDuration
	// Duration in minutes by calculating minted balance
	mintedDur := (minted / mndapp.BlockReward) * mndapp.BlockTimeMins
	// Duration in minutes since last payout
	lastDur := mintedTime.Sub(data.LastPayout.Time).Minutes()
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
	// isPayout := (minted <= mndapp.BlockReward && minted < prevRP.Minted) && prevRP.Minted != 0 && prevRP.Minted > minPayout
	logTS(debugTag, fmt.Sprintf(
		"Total: %.0f | Minted: %.0f | Fees: %.0f | Time: %s",
		minted+fees, minted, fees, client.FormatTS(mintedTime)))
	if !payoutTXReceived || prevRP.Minted == 0 {
		return
	}
	// Payout TX received by Pusher client.
	debugTag += "] [Payout"
	p := client.Payout{}
	p.Total = prevRP.Minted + prevRP.Fees
	p.Minted = prevRP.Minted
	p.Fees = prevRP.Fees
	p.Time = prevRP.Time
	t1, t2, t3, t4, err := mndapp.GetAllTierDistribution()
	logErrorTS(debugTag, err)
	if t1 < 1 || t2 < 1 || t3 < 1 || t4 < 1 {
		// Possible uncaught error occured on external API during retrieving tier distribution. Retry.
		t1, t2, t3, t4, err = mndapp.GetAllTierDistribution()
		logErrorTS(debugTag, err)
	}
	// Rewards received per MN on each tier and duration of reward cycle
	p.Tiers = map[string]float64{}
	p.Tiers["t1"], p.Tiers["t2"], p.Tiers["t3"], p.Tiers["t4"],
		p.Duration = mndapp.CalcReward(p.Minted, p.Fees, t1, t2, t3, t4)
	p.HostingFeeHalo, p.HostingFeeUSD, p.Price, _ = getHostingFee(p.Duration)
	// Log
	logTS(debugTag, fmt.Sprintf("Total: %.0f | Minted: %.0f | Fees: %.0f | Time: %s | "+
		"HostingFee: %.0f Halo ($%.0f) |"+
		"Distribution=> T1: %.0f, T2: %.0f, T3: %.0f, T4: %.0f",
		p.Total, p.Minted, p.Fees, client.FormatTS(p.Time),
		p.HostingFeeHalo, p.HostingFeeUSD,
		t1, t2, t3, t4))
	if !payoutTXReceived {
		// Updated minted and fees balance for use when payout event is triggered
		return
	}
	p.Time = payoutTX.TS
	p.BlockNumber = payoutTX.BlockNumber
	sendPayoutAlerts(discord, p, data.Alerts.Payout)
	mndapp.LastAlert = p.Time
	mndapp.LastPayout = data.LastPayout
	payoutTXReceived = false
	logErrorTS("setPTXProcessed", setPTXProcessed())
}
func triggerPayoutsAlert(discord *discordgo.Session, userChannelID string, minted, fees float64) {
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
	p.Tiers["t1"], p.Tiers["t2"], p.Tiers["t3"], p.Tiers["t4"], p.Duration = mndapp.CalcReward(minted, fees, t1, t2, t3, t4)
	p.HostingFeeHalo, p.HostingFeeUSD, p.Price, _ = getHostingFee(p.Duration)
	if payoutTXReceived {
		p.BlockNumber = payoutTX.BlockNumber
		p.Time = payoutTX.TS
	}
	mndapp.LastPayout = p
	mndapp.LastAlert = p.Time
	data.LastPayout = p
	total, success, fail := sendPayoutAlerts(discord, p, data.Alerts.Payout)
	txt := fmt.Sprintf("Payout alert sent. \nTotal channels: %d\nSuccess: %d\nFailed: %d", total, success, fail)
	_, err = discordSend(discord, userChannelID, txt, true)
	logErrorTS(debugTag, err)
	payoutTXReceived = false
	logErrorTS("setPTXProcessed", setPTXProcessed())
}

// sendPayoutAlerts sends out Discord payout alert to subscribed channels and users
func sendPayoutAlerts(discord *discordgo.Session, p client.Payout, channels map[string]string) (total, success, fail int) {
	total = len(channels)
	msgs := []client.Message{}
	txt := fmt.Sprintf("Delicious payout is served!```js\n%s"+client.DashLine+
		"Tier 1     | Tier 2    | Tier 3    | Tier 4\n"+client.DashLine+
		"%s | %s | %s | %s```",
		p.Format(),
		client.FillOrLimit(client.FormatNum(p.Tiers["t1"], 0), " ", 10),
		client.FillOrLimit(client.FormatNum(p.Tiers["t2"], 0), " ", 10),
		client.FillOrLimit(client.FormatNum(p.Tiers["t3"], 0), " ", 10),
		client.FillOrLimit(client.FormatNum(p.Tiers["t4"], 0), " ", 10),
	)
	if p.BlockNumber > 0 {
		txt += fmt.Sprintf("%s/block/%d\n", explorer.Homepage, p.BlockNumber)
	}
	txt += "```fix\nDisclaimer: Actual amount received may vary from " +
		"the amounts displayed due to the tier distribution returned by " +
		"API includes ineligible node statuses.```"
	for channelID, name := range channels {
		msg := client.Message{ChannelID: channelID}
		dmsg, err := discordSend(discord, channelID, txt, false)
		if err != nil {
			logTS("PayoutAlert", fmt.Sprintf("Payout Alert Failed! Channel ID: %s, Name: %s", channelID, name))
			msg.Error = err.Error()
		} else {
			success++
			msg.Sent = true
			msg.ID = dmsg.ID
		}
		msgs = append(msgs, msg)
	}
	fail = total - success
	logTS("PayoutAlertSummary", fmt.Sprintf("Total channels: %d | Success: %d | Failure: %d", total, success, fail))

	// update last payout details to json file
	p.AlertData.Messages = msgs
	p.AlertData.Total = total
	p.AlertData.SuccessCount = success
	p.AlertData.FailCount = fail
	data.LastPayout = p
	mndapp.LastPayout = p
	err := saveDataFile()
	if err != nil {
		logTS("FileSaveError", fmt.Sprintf("Failed to save Payout Data to %s: %+v | [Error]: %v", dataFile, p, err))
	}

	// save to payouts log
	addPayoutLog(p)
	return
}

// updatePayoutAlerts sends out Discord payout alert to subscribed channels and users
func updatePayoutAlerts(discord *discordgo.Session, p client.Payout, channelMsgIDs map[string]string) (total, success, fail int) {
	total = len(channelMsgIDs)
	msgs := []client.Message{}
	txt := "Delicious payout is served!```js\n" + p.Format() + "```"
	if p.BlockNumber > 0 {
		txt += fmt.Sprintf("%s/block/%d\n", explorer.Homepage, p.BlockNumber)
	}
	txt += "```fix\nDisclaimer: Actual amount received may vary from " +
		"the amounts displayed due to the tier distribution returned by " +
		"API includes ineligible node statuses.```"
	for channelID, msgID := range channelMsgIDs {
		msg := client.Message{ChannelID: channelID}
		nmsg, err := discord.ChannelMessageEdit(channelID, msgID, txt)
		if err != nil {
			logTS("PayoutAlert", fmt.Sprintf("Payout Alert Failed! Channel ID: %s", channelID))
			msg.Error = err.Error()
		} else {
			success++
			msg.Sent = true
			msg.ID = nmsg.ID
		}
		msgs = append(msgs, msg)
	}
	fail = total - success
	logTS("PayoutAlertSummary", fmt.Sprintf("Total channels: %d | Success: %d | Failure: %d", total, success, fail))

	// update last payout details to json file
	p.AlertData.Messages = msgs
	p.AlertData.Total = total
	p.AlertData.SuccessCount = success
	p.AlertData.FailCount = fail
	data.LastPayout = p
	err := saveDataFile()
	if err != nil {
		logTS("FileSaveError", fmt.Sprintf("Failed to save Payout Data to %s: %+v | [Error]: %v", dataFile, p, err))
	}

	// save to payouts log
	addPayoutLog(p)
	return
}

// GetHostingFee estimates the Halo Platform hosting fee for each node using current price from HaloDEX
func getHostingFee(durationStr string) (feeHalo, feeUSD, haloUSD float64, err error) {
	hours := durationToNum(durationStr)
	eth, err := cmc.GetTicker("ETH")
	if err != nil {
		return
	}
	ticker, err := dex.GetTicker("HALO", "ETH", eth.Quote["USD"].Price, 1)
	if err != nil {
		return
	}
	feesPerHour := mndapp.HostingFeeUSD / 30 / 24
	feeUSD = math.Ceil(hours) * feesPerHour
	haloUSD = ticker.LastPriceUSD
	feeHalo = feeUSD / haloUSD
	return
}

// durationToNum converts HH:MM duration string to parseable duration: 12h:34m
func durationToNum(durationStr string) float64 {
	p := strings.Split(durationStr, ":")
	if len(p) < 2 {
		p = []string{"00", "00"}
	}
	duration, _ := time.ParseDuration(fmt.Sprintf("%sh%sm", p[0], p[1]))
	return duration.Minutes() / 60
}

func checkPayoutEvent(discord *discordgo.Session) bool {
	debugTag := "checkPayoutEvent"
	payoutsTX, err := getPayoutTXs()
	if logErrorTS(debugTag+"] [FileReadError", err) {
		return false
	}
	l := len(payoutsTX)
	if l == 0 || payoutsTX[l-1].Processed {
		// Skip if no data or processing/already processed the last payout alert
		return false
	}
	ptx := payoutsTX[l-1]
	logTS(debugTag+"] [PayoutReceived", fmt.Sprintf("Block: %d, Time: %v", ptx.BlockNumber, ptx.TS))
	payoutTXReceived = true
	payoutTX = ptx

	if conf.DebugChannelID != "" {
		discordSend(discord, conf.DebugChannelID, fmt.Sprintf("Payout TX: %+v", ptx), true)
	}
	return true
}

func getPayoutTXs() (payoutsTX []client.PayoutTX, err error) {
	str, err := client.ReadFile(payoutsTXFile)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(str), &payoutsTX)
	return
}

func setPTXProcessed() (err error) {
	payoutsTX, err := getPayoutTXs()
	if err != nil {
		return
	}
	payoutsTX[len(payoutsTX)-1].Processed = true
	client.SaveJSONFileLarge(payoutsTXFile, payoutsTX)
	return
}

func addPayoutLog(p client.Payout) (err error) {
	str, err := client.ReadFile(payoutLogFile)
	if err != nil {
		return
	}
	if str == "" {
		str = "[]"
	}
	payouts := []client.Payout{}
	err = json.Unmarshal([]byte(str), &payouts)
	if err != nil {
		return
	}
	payouts = append(payouts, p)
	return client.SaveJSONFileLarge(payoutLogFile, payouts)
}
