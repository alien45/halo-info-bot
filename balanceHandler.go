package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alien45/gobcy"
	"github.com/alien45/halo-info-bot/client"
	_ "github.com/blockcypher/gobcy"
	"github.com/bwmarrin/discordgo"
)

func cmdBalance(discord *discordgo.Session, channelID, debugTag string, cmdArgs, addresses []string, numArgs, numAddresses int) {
	address := ""
	txt := ""
	i := 0
	var balance float64
	var err error
	balfunc := explorer.GetHaloBalance
	ticker := "HALO"
	dp := 0

	// Check if balance enquiry is for Ethereum
	if numArgs > 0 {
		ticker = strings.ToUpper(cmdArgs[numArgs-1])
		switch ticker {
		case "BTC":
			balfunc = getBTCBalance
			break
		case "DASH":
			balfunc = getDashBalance
			break
		case "ETH":
			balfunc = etherscan.GetEthBalance
			break
		case "LTC":
			balfunc = getLTCBalance
			break
		}
		// remove token argument to keep only addresses/keywords
		cmdArgs = cmdArgs[:numArgs-1]
		numArgs = len(cmdArgs)
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
	if address == "" || len(address) <= 3 {
		// Invalid address supplied
		i, err = strconv.Atoi(address)
		if err != nil {
			// Use first address from user's addressbook, if available
			i = 1
		}
		if numAddresses == 0 || i < 1 || numAddresses < i {
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

// Retrieve balance using BlockCypher API
func getBalanceBC(ticker, address string) (balance float64, err error) {
	// Use BlockCypher for Bitcoin, Litecon, Doge etc.
	bc := gobcy.API{
		Token: conf.Client.BlockCypher.Token,
		Coin:  strings.ToLower(ticker),
		Chain: "main",
	}
	addr, err := bc.GetAddrBal(address, map[string]string{})
	if err == nil {
		balance = float64(addr.FinalBalance) / math.Pow10(8)
	}
	return
}

func getBTCBalance(address string) (balance float64, err error) {
	return getBalanceBC("BTC", address)
}
func getLTCBalance(address string) (balance float64, err error) {
	return getBalanceBC("LTC", address)
}
func getDashBalance(address string) (balance float64, err error) {
	return getBalanceBC("DASH", address)
}
