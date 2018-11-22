package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

func cmdNodes(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int) {
	if numArgs == 0 {
		_, err := discordSend(discord, channelID, "Owner address required", true)
		logErrorTS(debugTag, err)
		return
	}

	addresses := cmdArgs
	addrs := map[string]int{}
	nodes := []client.Masternode{}
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
	strNodes := mndapp.FormatNodes(nodes)
	_, err := discordSend(discord, channelID, strNodes, true)
	logErrorTS(debugTag, err)
}

func cmdMN(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int) {
	if numArgs > 0 && strings.ToLower(cmdArgs[0]) == "info" {
		_, err := discordSend(
			discord,
			channelID,
			fmt.Sprint("Tier     । Collateral    । Max Nodes\n", client.DashLine+
				"Tier 1   ।  500          । 5000\n"+client.DashLine+ //0.4 Million
				"Tier 2   ।  1000         । 1000\n"+client.DashLine+ //0.8 Million
				"Tier 3   ।  2500         । 1000\n"+client.DashLine+ //2.0 Million
				"Tier 1   ।  7500         । 500\n"+client.DashLine), //6.0 Million
			true,
		)
		logErrorTS(debugTag, err)
		return
	}

	minted, err := mndapp.GetMintedBalance()
	if commandErrorIf(err, discord, channelID, "Failed to retrieve minting pool balance", debugTag) {
		return
	}
	fees, err := mndapp.GetServiceFeesBalance()
	if commandErrorIf(err, discord, channelID, "Failed to retrieve service fees pool balance", debugTag) {
		return
	}
	t1, t2, t3, t4, err := mndapp.GetAllTierDistribution()
	if commandErrorIf(err, discord, channelID, "Failed to retrieve tier distribution", debugTag) {
		return
	}
	fmt.Println("T1,2,3,4: ", t1, t2, t3, t4)

	_, err = discordSend(discord, channelID, mndapp.FormatMNRewardDist(minted, fees, t1, t2, t3, t4), true)
	logErrorTS(debugTag, err)
}

func checkPayoutInfinitely(discord *discordgo.Session, f func(discord *discordgo.Session)) {
	f(discord) // execute immediately
	for range time.Tick(time.Minute * 2) {
		f(discord)
	}
}

func checkPayout(discord *discordgo.Session) {
	minted, err := mndapp.GetMintedBalance()
	if err != nil {
		logTS("checkPayout()] [GetMintedBalance ", fmt.Sprint(err))
		return
	}
	fees, err := mndapp.GetServiceFeesBalance()
	logErrorTS("checkPayout()] [GetServiceFeesBalance() ", err)
	if mndapp.MintingPoolBalance > minted {
		// Previously retrieved balance is higher than current => means pool has been reset and payout occured
		logTS("checkPayout()] [Payout", "")
		total := mndapp.MintingPoolBalance + mndapp.ServiceFeePoolBalance
		mndapp.PayoutTotal = total
		mndapp.PayoutMinted = mndapp.MintingPoolBalance
		mndapp.PayoutFees = mndapp.ServiceFeePoolBalance
		mndapp.PayoutTime = mndapp.LastUpdated
		t1, t2, t3, t4, _ := mndapp.GetAllTierDistribution()
		_, err = discordSend(discord, "509810813083582465", mndapp.FormatPayout(minted, fees, t1, t2, t3, t4), true)

	}
	logTS("checkPayout()] [NotPayout", fmt.Sprintf(
		" Total: %f | Minted: %f | Fees: %f | ApproxTime: %s",
		minted+fees, minted, fees, client.FormatTS(time.Now())))
	mndapp.MintingPoolBalance = minted
	mndapp.ServiceFeePoolBalance = fees
	mndapp.LastUpdated = time.Now()
}
