package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func cmdDexTokens(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int) {
	logTS(debugTag, "Retrieving HaloDEX tokens.")
	if numArgs == 0 {
		strTokens, err := dex.GetTokenList()
		if commandErrorIf(err, discord, channelID, "Failed to retrieve tokens.", debugTag) {
			return
		}
		_, err = discordSend(discord, channelID, strTokens, true)
		logErrorTS(debugTag, err)
		return
	}
	tokens, err := dex.GetTokens()
	if commandErrorIf(err, discord, channelID, "Failed to retrieve tokens", debugTag) {
		return
	}

	// ticker supplied
	ticker := strings.ToLower(cmdArgs[0])
	token, found := tokens[ticker]
	if !found {
		discordSend(discord, channelID, "Invalid/unsupported token.", true)
		return
	}
	discordSend(discord, channelID, token.Format(), true)
}

func cmdDexBalance(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int) {
	if numArgs == 0 || cmdArgs[0] == "" {
		_, err := discordSend(discord, channelID, "Halo address required.", true)
		logErrorTS(debugTag, err)
		return
	}
	address := cmdArgs[0]
	tickerSupplied := numArgs >= 2 && cmdArgs[1] != "0"
	showZero := numArgs == 2 && cmdArgs[1] == "0" || tickerSupplied
	tickers := cmdArgs[1:]
	if showZero && numArgs == 2 {
		tickers = cmdArgs[2:]
	}

	if !tickerSupplied {
		// All tickers if unspecified
		tokens, err := dex.GetTokens()
		if commandErrorIf(err, discord, channelID, "Failed to retrieve tokens", debugTag) {
			return
		}
		for ticker := range tokens {
			tickers = append(tickers, ticker)
		}
	}
	balancesStr, err := dex.GetBalancesFormatted(address, tickers, showZero)
	if commandErrorIf(err, discord, channelID, "Failed to retrieve balance.", debugTag) {
		return
	}
	discordSend(discord, channelID, balancesStr, true)
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

	fmt.Println("swapped tokens ", symbolBase, symbolQuote)
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

func cmdDexTrades(discord *discordgo.Session, channelID, debugTag string, cmdArgs []string, numArgs int, command string) {
	//TODO: swap base-quote if wrong direction provided. Cache available pairs from DEX for this.
	//TODO: add argument for timezone or allow user to save timezone??
	tokenAddresses, err := dex.GetTokens()
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
		if numArgs < 4 {
			discordSend(discord, channelID, "Address required.", true)
			return
		}
		orders, errO := dex.GetOrders(quoteAddr, baseAddr, limit, cmdArgs[3])
		err = errO
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
