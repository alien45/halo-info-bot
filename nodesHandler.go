package main

import (
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
