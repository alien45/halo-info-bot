package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

func cmdBalance(discord *discordgo.Session, channelID, debugTag string, cmdArgs, addresses []string, numArgs, numAddresses int) {
	address := ""
	txt := ""
	i := 0
	balance := float64(0)
	var err error
	balfunc := explorer.GetHaloBalance
	ticker := "HALO"
	dp := 0

	// Check if balance enquiry is for Ethereum
	if numArgs > 0 && strings.ToLower(cmdArgs[numArgs-1]) == "eth" {
		balfunc = etherscan.GetEthBalance
		logTS(debugTag, "Ethereum address supplied")
		// remove token argument to keep only addresses/keywords
		cmdArgs = cmdArgs[:numArgs-1]
		numArgs = len(cmdArgs)
		ticker = "ETH"
		dp = 8
	}
	// Handle coin/token balance commands
	if numArgs == 0 || cmdArgs[0] == "" {
		// No address/address book index supplied
		if numAddresses == 0 {
			txt = "Address required."
			goto SendMessage
		}
		// Use first item from user's address book
		cmdArgs = append(cmdArgs, addresses[0])
		numArgs++
	}

	address = cmdArgs[0]
	if addr, found := addressKeywords[strings.ToLower(strings.Join(cmdArgs, "-"))]; found {
		// Valid keyword supplied
		address = addr
	}
	if !strings.HasPrefix(strings.ToLower(address), "0x") {
		// Invalid address supplied
		i, err = strconv.Atoi(address)
		if err != nil {
			// Use first address from user's addressbook, if available
			i = 1
		}
		if numAddresses == 0 || i < 1 || numAddresses < int(i) {
			// No/invalid address supplied and user has no address saved
			txt = "Valid address or address book item number required."
			goto SendMessage
		}
		i--
		address = addresses[i]
	}

	balance, err = balfunc(address)
	if commandErrorIf(err, discord, channelID, "Failed to retrieve balance for "+address, debugTag) {
		return
	}
	txt = fmt.Sprintf("Balance: %s %s", client.FormatNum(balance, dp), ticker)
SendMessage:
	_, err = discordSend(discord, channelID, "js\n"+txt, true)
	logErrorTS(debugTag, err)
}
