package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

func cmdNodes(discord *discordgo.Session, channelID, debugTag string, cmdArgs, userAddresses []string, numArgs, numAddresses int) {
	addresses := cmdArgs
	addrs := map[string]int{}
	nodes := []client.Masternode{}
	txt := ""
	summary := ""
	// action := ""
	// if numArgs >= 0 {
	// 	action := strings.ToLower(cmdArgs[0])
	// 	switch action {
	// 	case "summary", "full":
	// 		// Remove arg so only addresses remain
	// 		cmdArgs = cmdArgs[1:]
	// 		break
	// 	default:
	// 		action = "table"
	// 	}
	// }

	// TODO : separate summary with an argument
	// TODO : add argument to list full addresses and balance and display using cards layout

	if numArgs == 0 {
		// No address supplied
		if numAddresses == 0 {
			// User has no saved addresses
			txt = "Owner address required"
			goto SendMessage
		}
		addresses = userAddresses
	} else {

		for i, addr := range addresses {
			// Check if address book index supplied
			itemNum, err := strconv.Atoi(addr)
			if err != nil || itemNum <= 0 || itemNum > numAddresses {
				continue
			}
			// replace index with address
			addresses[i] = userAddresses[itemNum-1]
		}
	}
	// fmt.Println("Addresses: ", addresses)
	for i := 0; i < len(addresses); i++ {
		address := strings.ToUpper(addresses[i])
		if _, skip := addrs[address]; skip || !strings.HasPrefix(addresses[i], "0x") {
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
	m := mndapp
	var txt string
	var err error
	arg0 := strings.ToLower(strings.Join(cmdArgs, "-"))
	switch arg0 {
	case "collateral":
		txt = fmt.Sprintf(""+
			"Tier 1: %s\n"+
			"Tier 2: %s\n"+
			"Tier 3: %s\n"+
			"Tier 4: %s",
			client.FormatNumShort(m.Collateral["t1"], 0),
			client.FormatNumShort(m.Collateral["t2"], 0),
			client.FormatNumShort(m.Collateral["t3"], 0),
			client.FormatNumShort(m.Collateral["t4"], 0),
		)
		break
	case "nodes", "tier-distribution":
		t1, t2, t3, t4, err := m.GetAllTierDistribution()
		if logErrorTS(debugTag, err) {
			txt = fmt.Sprintf("Failed to retrieve tier distribution. Error: %v", err)
			break
		}
		txt = fmt.Sprintf(""+
			"Tier 1: %.0f\n"+
			"Tier 2: %.0f\n"+
			"Tier 3: %.0f\n"+
			"Tier 4: %.0f",
			t1, t2, t3, t4,
		)
		break
	case "payout", "last-payout":
		fallthrough
	default:
		txt = "________________/  Last Payout \\_______________\n"
		txt += m.LastPayout.Format()
		txt += "\n____________________/ ROI  \\____________________\n"
		txt += m.LastPayout.FormatROI(m.BlockReward, m.BlockTimeMins, m.Collateral)
		break
	case "pool", "reward-pool":
		txt, err = m.GetFormattedPoolData()
		break
	case "roi":
		txt = m.LastPayout.FormatROI(m.BlockReward, m.BlockTimeMins, m.Collateral)
		break
	}
	_, err = discordSend(discord, channelID, "js\n"+txt, true)
	logErrorTS(debugTag, err)
}
