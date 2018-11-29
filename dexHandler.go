package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

func cmdDexTokens(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int) {
	txt := "Invalid/unsupported token."
	ticker := ""
	token := client.Token{}
	found := false
	tokens, err := dex.GetTokens()
	if numArgs == 0 {
		txt, err = dex.GetFormattedTokens(tokens)
		if logErrorTS(debugTag, err) {
			txt = "Failed to retrieve tokens"
		}
		goto SendMessage
	}
	if logErrorTS(debugTag, err) {
		txt = "Failed to retrieve tokens"
		goto SendMessage
	}

	// ticker supplied
	ticker = strings.ToUpper(cmdArgs[0])
	if token, found = tokens[ticker]; found {
		txt = token.Format()
	}
SendMessage:
	discordSend(discord, channelID, "js\n"+txt, true)
}

func cmdDexBalance(discord *discordgo.Session, channelID, debugTag string, cmdArgs, addresses []string, numArgs, numAddresses int) {
	txt := ""
	var err error
	address := ""
	if numArgs == 0 {
		cmdArgs = []string{""}
	}
	address = strings.ToLower(cmdArgs[0])
	tickerSupplied := numArgs >= 2 && cmdArgs[1] != "0"
	showZero := numArgs == 2 && cmdArgs[1] == "0" || tickerSupplied
	tickers := cmdArgs[1:]
	if showZero && numArgs == 2 {
		tickers = cmdArgs[2:]
	}
	if address == "" || !strings.HasPrefix(address, "0x") {
		// Invalid address supplied
		i, err := strconv.ParseInt(address, 10, 64)
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

	if !tickerSupplied {
		// No ticker supplied, show all tickers' balance
		tokens, err := dex.GetTokens()
		if err != nil {
			txt = "Failed to retrieve tokens"
			logErrorTS(debugTag, err)
			goto SendMessage
		}
		for ticker := range tokens {
			tickers = append(tickers, ticker)
		}
	}
	logTS(debugTag, "Address: "+address)
	txt, err = dex.GetBalancesFormatted(address, tickers, showZero)
	if err != nil {
		txt = "Failed to retrieve balance."
		logErrorTS(debugTag, err)
	}
SendMessage:
	discordSend(discord, channelID, txt, true)
}

func cmdDexTicker(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int) {
	symbolQuote := "HALO"
	symbolBase := "ETH"
	if numArgs > 0 {
		symbolQuote = strings.ToUpper(cmdArgs[0])
	}
	if numArgs > 1 {
		symbolBase = strings.ToUpper(cmdArgs[1])
	}

	tokens, err := dex.GetTokens()
	if commandErrorIf(err, discord, channelID, "Failed to retrieve tokens", debugTag) {
		return
	}

	baseToken, existsB := tokens[symbolBase]
	quoteToken, existsQ := tokens[symbolQuote]
	if !existsB || !existsQ {
		_, err = discordSend(discord, channelID, "Invalid/unsupported token", true)
		commandErrorIf(err, discord, channelID, "Something went wrong!", debugTag)
	}

	if strings.ToUpper(quoteToken.Type) == "BASE" && strings.ToUpper(baseToken.Type) == "TOKEN" {
		// Tokens are in wrong direction. Swap 'em
		fmt.Println("swap tokens ", symbolBase, symbolQuote)
		tempB := symbolBase
		symbolBase = symbolQuote
		symbolQuote = tempB
	}

	// Get base token price
	cmcTicker, err := cmc.GetTicker(symbolBase)
	if commandErrorIf(err, discord, channelID, "Failed to retrieve retrieve price of "+symbolBase, debugTag) {
		return
	}
	basePriceUSD := cmcTicker.Quote["USD"].Price
	quoteSupply := float64(0)

	if symbolQuote == "HALO" {
		// Because Halo isn't yet listed on CMC
		quoteSupply, err = explorer.GetHaloSupply()
		logErrorTS(debugTag, err)
	} else {
		quoteTicker, err := cmc.GetTicker(symbolQuote)
		logErrorTS(debugTag, err)
		quoteSupply = quoteTicker.TotalSupply
	}

	ticker, err := dex.GetTicker(symbolQuote, symbolBase, basePriceUSD, quoteSupply)
	if commandErrorIf(err, discord, channelID, "Failed to retrieve ticker", debugTag) {
		return
	}
	logTS(debugTag, fmt.Sprintf("%s/%s ticker received: %s", symbolBase, symbolQuote, ticker.Pair))

	_, err = discordSend(discord, channelID, ticker.Format(), true)
	commandErrorIf(err, discord, channelID, "Something went wrong!", debugTag)
}

func cmdDexTrades(discord *discordgo.Session, channelID, debugTag string, cmdArgs, userAddresses []string, numArgs, numAddresses int, command string) {
	//TODO: swap base-quote if wrong direction provided. Cache available pairs from DEX for this.
	//TODO: add argument for timezone or allow user to save timezone??
	tokenAddresses, err := dex.GetTokens()
	address := ""
	if commandErrorIf(err, discord, channelID, "Failed to retrieve tokens", debugTag) {
		return
	}
	quoteAddr := tokenAddresses["HALO"].HaloChainAddress
	baseAddr := tokenAddresses["ETH"].HaloChainAddress
	limit := "10"

	if numArgs >= 2 {
		// Token symbol supplied
		quoteTicker, quoteOk := tokenAddresses[strings.ToUpper(cmdArgs[0])]
		baseTicker, baseOk := tokenAddresses[strings.ToUpper(cmdArgs[1])]
		if !quoteOk || !baseOk {
			_, err := discordSend(discord, channelID, fmt.Sprint("Invalid pair supplied: ", strings.Join(cmdArgs, "/")), true)
			logErrorTS(debugTag, err)
			return
		}
		quoteAddr = quoteTicker.HaloChainAddress
		baseAddr = baseTicker.HaloChainAddress
	}
	if numArgs >= 3 {
		// if limit argument is set
		if l, err := strconv.ParseInt(cmdArgs[2], 10, 32); err == nil && l <= 50 {
			limit = fmt.Sprint(l)
		} else {
			_, err = discordSend(discord, channelID, "Limit must be a valid number and max 50.", true)
			logErrorTS(debugTag, err)
			return
		}
	}
	dataStr := ""
	if command == "orders" {
		if numArgs >= 4 {
			address = strings.ToLower(cmdArgs[3])
		}
		if !strings.HasPrefix(address, "0x") {
			// Invalid address supplied
			i, err := strconv.ParseInt(address, 10, 64)
			if err != nil {
				// Use first address from user's addressbook, if available
				i = 1
			}
			if numAddresses == 0 || i < 1 || numAddresses < int(i) {
				// No/invalid address supplied and user has no address saved
				discordSend(discord, channelID, "Valid address or address book item number required.", true)
				return
			}
			i--
			address = userAddresses[i]
		}

		orders, err := dex.GetOrders(quoteAddr, baseAddr, limit, address)
		logErrorTS(debugTag, err)
		dataStr = dex.FormatOrders(orders)
	} else {
		trades, errT := dex.GetTrades(quoteAddr, baseAddr, limit)
		err = errT
		logErrorTS(debugTag, err)
		dataStr = dex.FormatTrades(trades)
	}
	if commandErrorIf(err, discord, channelID, "Failed to retrieve "+command, debugTag) {
		return
	}
	_, err = discordSend(discord, channelID, dataStr, true)
	logErrorTS(debugTag, err)
}
