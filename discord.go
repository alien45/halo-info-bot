package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Command describes bot supported command information
type Command struct {
	// Brief/one-liner description about the command
	Description string
	// Arguments supported by the command
	Arguments string
	// Whether the command can be used in public channels/groups.
	// If true, the command can only be invoked by texting the "bot user" privately
	IsPublic bool
	// An example of how to use the command
	Example string
}

func commandHandler(discord *discordgo.Session, message *discordgo.MessageCreate, commandPrefix string) {
	user := message.Author
	username := fmt.Sprint(message.Author)
	userAddresses := data.AddressBook[username]
	numAddresses := len(userAddresses)
	channelID := message.ChannelID
	debugTag := "commandHandler"
	isPrivateMsg := message.GuildID == ""
	hasPrefix := strings.HasPrefix(message.Content, commandPrefix)
	if user.ID == botID || user.Bot || (!isPrivateMsg && !hasPrefix) {
		// Ignore messages from any bot or messages that are not commands
		return
	}

	cmdArgs := strings.Split(message.Content, " ")
	command := strings.ToLower(strings.TrimPrefix(cmdArgs[0], commandPrefix))
	cmcTicker, err := cmc.FindTicker(command)
	_, found := supportedCommands[command]
	if !found {
		// Ignore invalid commands on public channels
		if isPrivateMsg && err != nil {
			_, err = discordSend(discord, channelID,
				"Invalid command! Need help? Use the following command:```!help```", false)
			logErrorTS(debugTag, err)
			return
		}
		// CMC ticker command invoked | !eth, !btc....
		command = "cmc"
		cmdArgs = append(cmdArgs, cmcTicker.Symbol)
	}

	cmdArgs = cmdArgs[1:]
	numArgs := len(cmdArgs)
	if numArgs == 0 {
		cmdArgs = []string{}
	}
	if _, found := privateCmds[command]; found && !isPrivateMsg {
		// Private command requested from a channel/server
		_, err := discordSend(discord, channelID, "Private commands are not allowed in public channels.", true)
		logErrorTS(debugTag, err)
		return
	}

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
	case "balance":
		address := ""
		txt := ""
		num := 0
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
			cmdArgs = append(cmdArgs, userAddresses[0])
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
			num, err = strconv.Atoi(address)
			if err != nil || numAddresses < num {
				txt = "Valid address or address book item number required."
				goto SendMessage
			}
			address = userAddresses[num-1]
		}

		balance, err = balfunc(address)
		if commandErrorIf(err, discord, channelID, "Failed to retrieve balance for "+address, debugTag) {
			return
		}
		txt = fmt.Sprintf("Balance: %.8f", balance)
	SendMessage:
		_, err = discordSend(discord, channelID, txt, false)
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
		cmdDexBalance(discord, channelID, debugTag, cmdArgs, userAddresses, numArgs, numAddresses)
		break
	case "tokens":
		cmdDexTokens(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "nodes": // Private Command
		cmdNodes(discord, channelID, debugTag, cmdArgs, userAddresses, numArgs, numAddresses)
		break
	case "mn":
		cmdMN(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "halo":
		cmdDexTicker(discord, channelID, debugTag, []string{}, 0)
		cmdMN(discord, channelID, debugTag, []string{}, 0)
		cmdDexTrades(discord, channelID, debugTag, []string{"halo", "eth", "5"}, 3, "trades")
		break
	case strings.ToLower(cmcTicker.Symbol):
		fallthrough
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
	case "alert":
		// Enable/disable alerts. For personal chat. Possibly for channels as well but should only be setup by admins
		// TODO: MN reward notification // using temp-hack
		// TODO: dex status notification // using realtime API
		// TODO: feather update notification
		discordSend(discord, channelID, "Not implemented", true)
		break
	case "address":
		cmdAddress(discord, channelID, fmt.Sprint(message.Author), debugTag, cmdArgs, numArgs)
		break
	}
}

// commandErrorIf prints and sends error as message, if not nil
func commandErrorIf(err error, discord *discordgo.Session, channelID, message, debugTag string) (hasError bool) {
	if !logErrorTS(debugTag, err) {
		return
	}
	discordSend(discord, channelID, fmt.Sprintf("%s```%s```", message, err), false)
	return true
}

// discordSend sends a message to the supplied Discord channel. If the message is larger than Discord limit (2000
// characters), it will split the text and send multiple messages by recursively spliting on the last line break
// within the first 2000 character range. If line break is not existant within the range will use 2000.
func discordSend(discord *discordgo.Session, channelID, message string, codeBlock bool) (newMessage *discordgo.Message, err error) {
	debugTag := "discordSend()"
	messageLimit := 2000
	if len(strings.TrimSpace(message)) == 0 {
		logTS(debugTag, "skipped sending empty message")
		return
	}
	if codeBlock {
		messageLimit = 1994
	}
	if len(message) <= messageLimit {
		if codeBlock {
			message = "```" + message + "```"
		}
		return discord.ChannelMessageSend(channelID, message)
	}

	logTS(debugTag, "Message length higher than 2000. Spliting message.")
	// Find the last index within the first 2000 characters where line has break occured.
	lineBreakIndex := strings.LastIndex(message[0:messageLimit], "\n")
	if lineBreakIndex == -1 {
		// No line break found within range. Use 2000.
		lineBreakIndex = messageLimit
	}
	// Send the first message
	newMessage, err = discordSend(discord, channelID, message[0:lineBreakIndex], codeBlock)
	if logErrorTS(debugTag, err) {
		return
	}

	nextMessage := message[lineBreakIndex:]
	if strings.HasPrefix(message, "diff\n") {
		// Fixes discord markdown symbol "diff" not being applied to subsequent messages
		nextMessage = "diff\n" + nextMessage
	}
	// Send the subsequent message(s)
	newMessage, err = discordSend(discord, channelID, nextMessage, codeBlock)
	logErrorTS(debugTag, err)
	return
}

// Generate text for the help command
func generateHelpText(supportedCommands map[string]Command, publicOnly bool) (s string) {
	commands := []string{}
	for command := range supportedCommands {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	for i := 0; i < len(commands); i++ {
		cmd := commands[i]
		details := supportedCommands[commands[i]]
		if publicOnly && !details.IsPublic {
			continue
		}
		s += fmt.Sprintf("!%s %s: \n  - %s \n", cmd, details.Arguments, details.Description)
		if details.Example != "" {
			s += fmt.Sprintf("  - Example: %s\n", details.Example)
		}
		if !details.IsPublic {
			s += "  - Private command. Only available by PMing the bot.\n"
		}
		s += "\n"
	}
	s += "\n<argument> => required"
	s += "\n[argument] => optional"
	s += "\n{argument} => indicates exact value"
	s += "\n\nDefaults where applicable:\n  base ticker => ETH,\n  quote ticker => Halo"
	return
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
	"address": Command{
		Description: "Add, remove and get list of saved addresses.",
		IsPublic:    false,
		Arguments:   "[action <address1> <address2>...]",
		Example:     "!addresses OR !addresses add 0x1234 OR !addresses remove 0x1234",
	},
}
