package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

var (
	botID           string
	config          Config
	cmc             client.CMC
	dex             client.DEX
	explorer        client.Explorer
	etherscan       client.Etherscan
	mndapp          client.MNDApp
	addressKeywords = map[string]map[string]string{
		"halo": {
			// Halo Masternode reward pool
			"reward-pool": "0xd674dd3cdf07139ffda85b8589f0e2ca600f996e",
			// Charity address by the Halo Platform community
			"charity": "0xaefaffa2272098b4ab6f9a87a76f25944aee746d",
		},
		"eth": {"h-eth": "0x70a41917365E772E41D404B3F7870CA8919b4fBe"}, // Ethereum address for H-ETH token
	}
	// Commands names that are not allowed in the public chats. Key: command, value: unused.
	privateCmds     = map[string]string{}
	helpText        string
	helpTextPrivate string
)

// Config describes data that's required to run the application
type Config struct {
	CommandPrefix string

	// User who controls the bot globally.
	// Possible future use cases:
	//  - Change bot avatar and other profile settings etc.
	//	- Enable/disable command.
	//	- Update/change caching preferences.
	//	- Update API endpoint URLs without restarting the bot server application.
	//
	// Channel admin controls can be done programmatically in the future.
	BotRootUserID string

	// CoinMarketCap.com API base URL
	CMCBaseURL string

	// CoinMarketCap.com API Key
	CMCAPIKEY string

	// HaloDEX GraphQL API base URL
	DEXURLGQL string

	//HaloDEX REST API base URL
	DEXURLREST string

	// Halo explorer REST API base URL
	ExplorerREST string

	// Etherscan API base URL
	EtherscanREST string

	// Etherscan API Key
	EtherscanAPIKey string

	// Halo mainnet GraphQL API base URL
	MainnetGQL string

	// Halo masternodes DApp API base URL
	MNDAppREST string
}

func main() {
	token := os.Getenv("BotToken")
	config = Config{
		os.Getenv("CommandPrefix"),
		os.Getenv("BotRootUserID"),
		os.Getenv("URL_CMC_REST"),
		os.Getenv("API_KEY_CMC"),
		os.Getenv("URL_DEX_GQL"),
		os.Getenv("URL_DEX_REST"),
		os.Getenv("URL_Explorer_REST"),
		os.Getenv("URL_Etherscan_REST"),
		os.Getenv("API_Key_Etherscan"),
		os.Getenv("URL_Mainnet_GQL"),
		os.Getenv("URL_MNDApp_REST"),
	}
	if config.CommandPrefix == "" {
		//set default
		config.CommandPrefix = "!"
	}

	// create private commands' list
	for cmdStr, details := range supportedCommands {
		if !details.IsPublic {
			privateCmds[cmdStr] = ""
		}
	}
	// Instantiate API clients with appropriate settings
	cmc.Init(config.CMCBaseURL, config.CMCAPIKEY)
	dex.Init(config.DEXURLGQL, config.DEXURLREST)
	etherscan.Init(config.EtherscanREST, config.EtherscanAPIKey)
	explorer.Init(config.ExplorerREST, config.MainnetGQL)
	mndapp.Init(config.MNDAppREST)

	helpText = generateHelpText(supportedCommands, true)
	helpTextPrivate = generateHelpText(supportedCommands, false)

	// Connect to discord as a bot
	discord, err := discordgo.New("Bot " + token)
	panicIf(err, "error creating discord session")
	bot, err := discord.User("@me")
	panicIf(err, "error retrieving account")

	botID = bot.ID
	discord.AddHandler(commandHandler)
	discord.AddHandler(func(discord *discordgo.Session, ready *discordgo.Ready) {
		err = discord.UpdateStatus(1, "Halo Bulter")
		if err != nil {
			fmt.Println("Error attempting to set bot status")
		}
		fmt.Printf("Halo Info Bot has started on %d servers\n", len(discord.State.Guilds))
	})

	err = discord.Open()
	panicIf(err, "Error opening connection to Discord")
	defer discord.Close()

	// Keep application open
	<-make(chan struct{})
}

func panicIf(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %+v\n", msg, err)
		panic(err)
	}
}

func commandHandler(discord *discordgo.Session, message *discordgo.MessageCreate) {
	user := message.Author
	channelID := message.ChannelID
	debugTag := "commandHandler"
	isPrivateMsg := message.GuildID == ""
	if user.ID == botID || user.Bot || !strings.HasPrefix(message.Content, config.CommandPrefix) {
		// Ignore messages from any bot or messages that are not commands
		return
	}

	cmdArgs := strings.Split(message.Content, " ")
	command := strings.ToLower(strings.TrimPrefix(cmdArgs[0], config.CommandPrefix))
	if _, found := supportedCommands[command]; !found {
		return
	}
	if _, found := privateCmds[command]; found && !isPrivateMsg {
		// Private command requested from a channel/server
		_, err := discordSend(discord, channelID, "Private commands are not allowed in public channels.", true)
		logErrorTS(debugTag, err)
		return
	}
	cmdArgs = cmdArgs[1:]
	numArgs := len(cmdArgs)

	debugTag = "cmd] [" + command
	logTS(debugTag, fmt.Sprintf("Author: %s, Message: %s\n", message.Author, message.Content))
	switch command {
	case "help":
		text := "css\n" + helpText
		if isPrivateMsg {
			text = "css\n" + helpTextPrivate
		}
		discordSend(discord, channelID, text, true)
		break
	case "cmc":
		// Handle CoinMarketCap related commands
		nameOrSymbol := strings.ToUpper(strings.Join(cmdArgs, " "))
		ticker, err := cmc.GetTicker(nameOrSymbol)
		if commandErrorIf(err, discord, channelID, "Ticker not found or query failed.", debugTag) {
			return
		}

		_, err = discordSend(discord, channelID, ticker.Format(), true)
		commandErrorIf(err, discord, channelID, "Failed to retrieve CMC ticker for "+nameOrSymbol, debugTag)
		logErrorTS(debugTag, err)
		break
	case "balance":
		// Handle coin/token balance commands
		if numArgs == 0 || cmdArgs[0] == "" {
			discordSend(discord, channelID, "Address or one of the reserved keywords(reward-pool, charity, h-eth...) required.", false)
			return
		}

		address := cmdArgs[0]
		if addr, found := addressKeywords["halo"][strings.ToLower(strings.Join(cmdArgs, "-"))]; found {
			// Valid keyword supplied
			address = addr
		}

		balfunc := explorer.GetHaloBalance
		token := "halo"
		if cmdArgs[numArgs-1] == "eth" {
			balfunc = etherscan.GetEthBalance
			logTS(debugTag, "Ethereum address supplied")
			token = "eth"
		}

		cmdArgs = cmdArgs[0 : numArgs-1]
		if tokenAddrs, ok := addressKeywords[token]; ok {
			if addr, found := tokenAddrs[strings.ToLower(strings.Join(cmdArgs, "-"))]; found {
				// Valid keyword supplied
				address = addr
			}
		}

		balance, err := balfunc(address)
		if commandErrorIf(err, discord, channelID, "Failed to retrieve balance for "+address, debugTag) {
			return
		}
		_, err = discordSend(discord, channelID, fmt.Sprintf("Balance: %.8f", balance), false)
		logErrorTS(debugTag, err)
		break
	case "ticker":
		symbolQuote := "HALO"
		symbolBase := "ETH"
		if numArgs >= 2 {
			symbolQuote = strings.ToUpper(cmdArgs[0])
			symbolBase = strings.ToUpper(cmdArgs[1])
		}
		haloTotalSupply, err := explorer.GetHaloSupply()
		logErrorTS(debugTag, err)

		// Get base token price
		cmcTicker, err := cmc.GetTicker(symbolBase)
		if commandErrorIf(err, discord, channelID, "Failed to retrieve retrieve price of "+symbolBase, debugTag) {
			return
		}
		basePriceUSD := cmcTicker.Quote["USD"].Price
		ticker, err := dex.GetTicker(symbolQuote, symbolBase, basePriceUSD, haloTotalSupply)
		if commandErrorIf(err, discord, channelID, "Failed to retrieve ticker", debugTag) {
			return
		}
		logTS(debugTag, fmt.Sprintf("%s/%s ticker received: %s", symbolBase, symbolQuote, ticker.Pair))

		_, err = discordSend(discord, channelID, ticker.Format(), true)
		commandErrorIf(err, discord, channelID, "Something went wrong!", debugTag)
		break
	case "orders":
		fallthrough
	case "trades":
		//TODO: swap base-quote if wrong direction provided. Cache available pairs from DEX for this.
		//TODO: add argument for timezone or allow user to save timezone??
		tokenAddresses, err := dex.GetTokens()
		if commandErrorIf(err, discord, channelID, "Failed to retrieve tokens", debugTag) {
			return
		}
		quoteAddr := tokenAddresses["halo"].HaloChainAddress
		baseAddr := tokenAddresses["eth"].HaloChainAddress
		limit := "10"

		if numArgs >= 2 {
			// Token symbol supplied
			quoteTicker, quoteOk := tokenAddresses[strings.ToLower(cmdArgs[0])]
			baseTicker, baseOk := tokenAddresses[strings.ToLower(cmdArgs[1])]
			if !quoteOk || !baseOk {
				_, err := discordSend(discord, channelID, fmt.Sprint("Invalid pair supplied: ", strings.Join(cmdArgs, "/")), true)
				logErrorTS(debugTag, err)
				return
			}
			quoteAddr = quoteTicker.HaloChainAddress
			baseAddr = baseTicker.HaloChainAddress
			logTS(debugTag, fmt.Sprintf("Quote Ticker: %s (%s),\n Base Ticker: %s (%s)",
				cmdArgs[0], quoteAddr, cmdArgs[1], baseAddr))
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
		break
	case "dexbalance": // Private Command
		if numArgs == 0 || cmdArgs[0] == "" {
			_, err := discordSend(discord, channelID, "Halo address required.", true)
			logErrorTS(debugTag, err)
			return
		}
		address := cmdArgs[0]
		ticker := "halo"
		if numArgs >= 2 {
			ticker = strings.ToLower(cmdArgs[1])
		}
		balancesStr, err := dex.GetBalanceFormatted(address, ticker)
		if commandErrorIf(err, discord, channelID, "Failed to retrieve balance.", debugTag) {
			return
		}
		discordSend(discord, channelID, balancesStr, true)
		break
	case "tokens":
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
		break
	case "my-nodes": // Private Command
		if numArgs == 0 {
			_, err := discordSend(discord, channelID, "Owner address required", true)
			logErrorTS(debugTag, err)
			return
		}

		strNodes, err := mndapp.GetMasternodesFormatted(strings.ToLower(cmdArgs[0]))
		if commandErrorIf(err, discord, channelID, "Failed to retrieve masternodes", debugTag) {
			return
		}
		_, err = discordSend(discord, channelID, strNodes, true)
		logErrorTS(debugTag, err)
		break
	case "alert":
		// Enable/disable alerts. For personal chat. Possibly for channels as well but should only be setup by admins
		// TODO: MN reward notification // using temp-hack
		// TODO: dex status notification // using realtime API
		// TODO: feather update notification
		discordSend(discord, channelID, "Not implemented", true)
		break
	}
}

// TODO: Use json config file or leave as is??
var supportedCommands = map[string]Command{
	"help": Command{
		Description: "Prints this message",
		IsPublic:    true,
	},
	"trades": Command{
		Description: "Recent trades from HaloDEX",
		IsPublic:    true,
		Arguments:   "[quote-symbol] [base-symbol] [limit]",
		Example:     "!trades halo eth 10",
	},
	"orders": Command{
		Description: "Get HaloDEX orders by user address.",
		IsPublic:    false,
		Arguments:   "<quote-ticerk> <base-ticker> <limit> <address>",
		Example:     "!orders halo eth 10 0x1234567890abcdef",
	},
	"ticker": Command{
		Description: "Get ticker information from HaloDEX.",
		IsPublic:    true,
		Arguments:   "<quote-ticker> <base-ticker>",
		Example:     "!ticker halo eth",
	},
	"cmc": Command{
		Description: "Fetch CoinMarketCap tickers",
		IsPublic:    true,
		Arguments:   "<symbol>",
		Example:     "!cmc btc",
	},
	"balance": Command{
		Description: "Check your account balance. Supported addresses/chains: HALO & ETH",
		IsPublic:    true,
		Arguments:   "<address> [ticker]",
		Example:     "!balance 0x1234567890abcdef",
	},
	"dexbalance": Command{
		Description: "Check your DEX balances. USE YOUR HALO CHAIN ADDRESS FOR ALL TOKEN BALANCES WITHIN DEX.",
		IsPublic:    false,
		Arguments:   "<address> [ticker]",
		Example:     "!dexbalance 0x1234567890abcdef",
	},
	"tokens": Command{
		Description: "Lists all tokens supported on HaloDEX",
		IsPublic:    true,
		Arguments:   "[ticker]",
		Example:     "!tokens",
	},
	"my-nodes": Command{
		Description: "Lists masternodes owned by a specific address",
		IsPublic:    false,
		Arguments:   "<address>",
		Example:     "!mn 0x1234567890abcdef",
	},
}
