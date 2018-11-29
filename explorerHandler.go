package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func cmdBalance(discord *discordgo.Session, channelID, debugTag string, cmdArgs, addresses []string, numArgs, numAddresses int) {
	address := ""
	txt := ""
	i := 0
	balance := float64(0)
	var err error
	balfunc := explorer.GetHaloBalance

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

	// Check if balance enquiry is for Ethereum
	if cmdArgs[numArgs-1] == "eth" {
		balfunc = etherscan.GetEthBalance
		logTS(debugTag, "Ethereum address supplied")
		// remove token argument to keep only addresses/keywords
		cmdArgs = cmdArgs[:numArgs-1]
	}

	address = cmdArgs[0]
	if addr, found := addressKeywords[strings.ToLower(strings.Join(cmdArgs, "-"))]; found {
		// Valid keyword supplied
		address = addr
	}
	if !strings.HasPrefix(strings.ToLower(address), "0x") {
		// Not a valid address
		if numAddresses == 0 {
			txt = "Valid address or address book item number required."
			goto SendMessage
		}
		i, err = strconv.Atoi(address)
		if err == nil && i > 0 && int(i) <= numAddresses {
			i--
		}
		address = addresses[i]
	}

	balance, err = balfunc(address)
	if commandErrorIf(err, discord, channelID, "Failed to retrieve balance for "+address, debugTag) {
		return
	}
	txt = fmt.Sprintf("Balance: %.8f", balance)
SendMessage:
	_, err = discordSend(discord, channelID, txt, false)
	logErrorTS(debugTag, err)
}
