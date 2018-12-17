package main

import (
	"fmt"
	"sort"
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
	channelID := message.ChannelID
	isPrivateMsg := message.GuildID == "" || data.PrivacyExceptions[channelID] != ""
	hasPrefix := strings.HasPrefix(message.Content, commandPrefix)
	if user.ID == botID || user.Bot || (!isPrivateMsg && !hasPrefix) {
		// Ignore messages from any bot or messages that are not commands
		return
	}

	username := fmt.Sprint(message.Author)
	userAddresses := data.AddressBook[username]
	numAddresses := len(userAddresses)
	debugTag := "commandHandler"
	cmdArgs := strings.Split(message.Content, " ")
	command := strings.ToLower(strings.TrimPrefix(cmdArgs[0], commandPrefix))
	cmdArgs = cmdArgs[1:]
	numArgs := len(cmdArgs)
	cmcTicker, err := cmc.FindTicker(command)
	_, found := supportedCommands[command]
	if !found {
		// Ignore invalid commands on public channels
		if isPrivateMsg && err != nil && message.GuildID == "" {
			_, err = discordSend(discord, channelID,
				"Invalid command! Need help? Use the following command:```!help```", false)
			logErrorTS(debugTag, err)
			return
		} else if cmcTicker.Symbol == "" || numArgs > 0 {
			// ignore if message is a chatter
			return
		}
		// CMC ticker command invoked | !eth, !btc....
		command = "cmc"
		cmdArgs = []string{cmcTicker.Symbol}
		numArgs = 1
	}

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
		txt := ""
		if numArgs > 0 {
			txt = commandHelpText(cmdArgs[0])
		} else if isPrivateMsg {
			txt = helpTextPrivate
		} else {
			txt = helpText
		}
		_, err := discordSend(discord, channelID, "css\n"+txt, true)
		logErrorTS(debugTag, err)
		break
	case "balance":
		cmdBalance(discord, channelID, debugTag, cmdArgs, userAddresses, numArgs, numAddresses)
		break
	case "ticker":
		cmdDexTicker(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "orders":
		fallthrough
	case "trades":
		cmdDexTrades(discord, channelID, debugTag, cmdArgs, userAddresses, numArgs, numAddresses, command)
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
		txt, err := mndapp.GetFormattedPoolData()
		if err == nil {
			_, err = discordSend(discord, channelID, "js\n"+txt, true)
		}
		logErrorTS(debugTag, err)
		cmdDexTrades(discord, channelID, debugTag, []string{"halo", "eth", "5"}, userAddresses, 3, numAddresses, "trades")
		break
	case "cmc":
		// Handle CoinMarketCap related commands
		nameOrSymbol := strings.ToUpper(strings.Join(cmdArgs, " "))
		ticker, err := cmc.GetTicker(nameOrSymbol)
		if commandErrorIf(err, discord, channelID, "Ticker not found or query failed.", debugTag) {
			return
		}
		_, err = discordSend(discord, channelID, "js\n"+ticker.Format(), true)
		logErrorTS(debugTag, err)
		break
	case "alert":
		if message.GuildID != "" && username != conf.Client.DiscordBot.RootUser {
			// Public channel. Only root user is allowed to send payout alert or enable alerts on public channels

			// isAdmin, _ := MemberHasPermission(discord, message.GuildID, message.Author.ID, 8)
			// canManageChannels, _ := MemberHasPermission(discord, message.GuildID, message.Author.ID, 16)
			// canManageServer, _ := MemberHasPermission(discord, message.GuildID, message.Author.ID, 32)
			// fmt.Println(isAdmin, canManageChannels, canManageServer)
			// fmt.Println(discord.State.UserChannelPermissions(message.Author.ID, channelID))
			// if !isAdmin && !canManageChannels && !canManageServer {
			_, err = discordSend(discord, channelID, "Sorry, you are not allowed to manage alerts on this channel.", true)
			logErrorTS(debugTag, err)
			return
			// }
		}
		cmdAlert(discord, message.GuildID, channelID, message.Author.ID, username, debugTag, cmdArgs, numArgs)
		break
	case "address":
		cmdAddress(discord, channelID, fmt.Sprint(message.Author), debugTag, cmdArgs, numArgs)
		break
	case "chart":
		url := data.Info["charturl"]
		if numArgs > 0 && strings.ToLower(cmdArgs[0]) == "dark" {
			url = data.Info["charturldark"]
		}
		_, err = discordSend(discord, channelID, url, false)
		logErrorTS(debugTag, err)
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
	debugTag := "discordSend"
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
func generateHelpText(publicOnly bool) (s string) {
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
		s += fmt.Sprintf("!%s %s\n", cmd, details.Arguments)
	}
	s += "\n<argument> => required"
	s += "\n[argument] => optional"
	s += "\n{argument} => indicates exact value"
	s += "\n\nDefaults where applicable:\n - Base ticker => ETH,\n - Quote ticker => Halo\n" +
		" - Address(es) => first/all item(s) saved on address book, if avaiable"
	return
}

// commandHelpText returns help text for a specific command
func commandHelpText(commandName string) (s string) {
	for cmd, details := range supportedCommands {
		if cmd != strings.ToLower(commandName) {
			continue
		}
		s += fmt.Sprintf("!%s %s: \n  - %s \n", cmd, details.Arguments, details.Description)
		if details.Example != "" {
			seperator := "\n           "
			exampleF := seperator + strings.Join(strings.Split(details.Example, "OR, "), seperator)
			s += fmt.Sprintf("  - Example: %s\n", exampleF)
		}
		if !details.IsPublic {
			s += "  - Private command. Only available by PMing the bot.\n"
		}
		s += "\n"
	}
	if s == "" {
		s = commandName + " is not a valid command"
	}
	return
}

// TODO: Use json config file
// TODO: add custom "info-only" server-specific commands
var supportedCommands = map[string]Command{
	"help": Command{
		Description: "Prints list of commands and supported arguments. If argument 'command' is " +
			"provided will display detailed information about the command along with examples.",
		Arguments: "[command-name]",
		IsPublic:  true,
		Example:   "!help OR, !help balance",
	},
	"trades": Command{
		Description: "Recent trades from HaloDEX",
		IsPublic:    true,
		Arguments:   "[quote-symbol] [base-symbol] [limit]",
		Example:     "!trades halo eth 10 OR, !trades",
	},
	"ticker": Command{
		Description: "Get ticker information from HaloDEX.",
		IsPublic:    true,
		Arguments:   "[quote-ticker] [base-ticker]",
		Example:     "!ticker OR, !ticker vet OR, !ticker dbet eth",
	},
	"cmc": Command{
		Description: "Fetch CoinMarketCap ticker information. Alternatively, use the ticker itself as command.",
		IsPublic:    true,
		Arguments:   "<symbol>",
		Example:     "!cmc powr, OR, !cmc power ledger, OR, !powr (shorthand for '!cmc powr')",
	},
	"balance": Command{
		Description: "Check your account balance. Supported addresses/chains: HALO & ETH. " +
			"Address keywords: 'reward pool', 'charity', 'h-eth'." +
			" If not address supplied, the first item of user's address book will be used. " +
			"To get balance of a specific item from address book just type the index number of the address.",
		IsPublic:  true,
		Arguments: "[address] [ticker]",
		Example:   "!balance 0x1234567890abcdef OR, !balance OR, !balance 2 (for 2nd item in the address book)",
	},
	"tokens": Command{
		Description: "Lists all tokens supported on HaloDEX",
		IsPublic:    true,
		Arguments:   "[ticker]",
		Example:     "!tokens OR, !tokens halo",
	},
	"mn": Command{
		Description: "Shows masternode collateral, reward pool balances, nodes distribution, last payout and ROI based on last payout.",
		IsPublic:    true,
	},
	"halo": Command{
		Description: "Get a digest of information about Halo.",
		IsPublic:    true,
		Example:     "!halo",
	},
	"chart": Command{
		Description: "Get the URL of the HaloDEX third-party charts.",
		IsPublic:    true,
		Arguments:   "[{dark}]",
		Example:     "!chart OR !chart dark",
	},

	// Private Commands
	"nodes": Command{
		Description: "Lists masternodes owned by a specific address",
		IsPublic:    false,
		Arguments:   "[address] [address2] [address3....]",
		Example:     "!nodes 0x1234567890abcdef",
	},
	"dexbalance": Command{
		Description: "Shows user's HaloDEX balances. USE YOUR HALO CHAIN ADDRESS FOR ALL TOKEN BALANCES WITHIN DEX.",
		IsPublic:    false,
		Arguments:   "[address] [{0} or [ticker ticker2 ticker3...]]",
		Example:     "!dexbalance 0x123... 0 OR, !dexbalance 0x123... ETH",
	},
	"orders": Command{
		Description: "Get HaloDEX orders by user address.",
		IsPublic:    false,
		Arguments:   "[quote-ticker] [base-ticker] [limit] [address]",
		Example:     "!orders halo eth 10 0x1234567890abcdef",
	},
	"address": Command{
		Description: "Add, remove and get list of saved addresses.",
		IsPublic:    false,
		Arguments:   "[action <address1> <address2>...]",
		Example:     "!addresses OR, !addresses add 0x1234 OR, !addresses remove 0x1234",
	},
	"alert": Command{
		Description: "Enable/disable automatic alerts. Alert types: payout. Actions:on/off",
		IsPublic:    false,
		Arguments:   "<type> [action]",
		Example:     "!alert payout on",
	},
}
