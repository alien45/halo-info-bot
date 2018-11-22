package main

import (
	"fmt"
	"os"
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
	mndapp.Init(config.MNDAppREST, config.MainnetGQL)

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

	go checkPayoutInfinitely(discord, checkPayout)

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
	logTS(debugTag, fmt.Sprintf("Author: %s, ChannelID: %s, Message: %s", message.Author, message.ChannelID, message.Content))
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
		cmdDexTicker(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "orders":
		fallthrough
	case "trades":
		cmdDexTrades(discord, channelID, debugTag, cmdArgs, numArgs, command)
		break
	case "dexbalance": // Private Command
		cmdDexBalance(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "tokens":
		cmdDexTokens(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "nodes": // Private Command
		cmdNodes(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "mn":
		cmdMN(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "halo":
		cmdDexTicker(discord, channelID, debugTag, []string{}, 0)
		cmdMN(discord, channelID, debugTag, []string{}, 0)
		cmdDexTrades(discord, channelID, debugTag, []string{"halo", "eth", "5"}, 3, "trades")
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
	"ticker": Command{
		Description: "Get ticker information from HaloDEX.",
		IsPublic:    true,
		Arguments:   "[quote-ticker] [base-ticker]",
		Example:     "!ticker OR !ticker vet OR, !ticker dbet eth",
	},
	"cmc": Command{
		Description: "Fetch CoinMarketCap tickers",
		IsPublic:    true,
		Arguments:   "<symbol>",
		Example:     "!cmc btc OR, !cmc bitcoin cash",
	},
	"balance": Command{
		Description: "Check your account balance. Supported addresses/chains: HALO & ETH. Address keywords: 'reward pool', 'charity', 'h-eth'.",
		IsPublic:    true,
		Arguments:   "<address> [ticker]",
		Example:     "!balance 0x1234567890abcdef",
	},
	"tokens": Command{
		Description: "Lists all tokens supported on HaloDEX",
		IsPublic:    true,
		Arguments:   "[ticker]",
		Example:     "!tokens OR, !tokens halo",
	},
	"mn": Command{
		Description: "Masternode reward pool and nodes distribution information. Or get masternode collateral info.",
		IsPublic:    true,
		Arguments:   "[{info}]",
		Example:     "!mn OR, !mn info",
	},
	"halo": Command{
		Description: "Get a digest of information about Halo.",
		IsPublic:    true,
		Example:     "!halo, !vet",
	},

	// Private Commands
	"nodes": Command{
		Description: "Lists masternodes owned by a specific address",
		IsPublic:    false,
		Arguments:   "<address> [address2] [address3....]",
		Example:     "!nodes 0x1234567890abcdef",
	},
	"dexbalance": Command{
		Description: "Check your DEX balances. USE YOUR HALO CHAIN ADDRESS FOR ALL TOKEN BALANCES WITHIN DEX.",
		IsPublic:    false,
		Arguments:   "<address> [{0} or [ticker ticker2 ticker3...]]",
		Example:     "!dexbalance 0x123... 0 OR, !dexbalance 0x123... ETH",
	},
	"orders": Command{
		Description: "Get HaloDEX orders by user address.",
		IsPublic:    false,
		Arguments:   "<quote-ticker> <base-ticker> <limit> <address>",
		Example:     "!orders halo eth 10 0x1234567890abcdef",
	},
}
