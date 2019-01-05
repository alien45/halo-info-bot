package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

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
	cmdName := strings.ToLower(strings.TrimPrefix(cmdArgs[0], commandPrefix))
	cmdArgs = cmdArgs[1:]
	numArgs := len(cmdArgs)
	cmcTicker, err := cmc.FindTicker(cmdName)
	cmds := commands
	if gCMDs, ok := guildCommands[message.GuildID]; ok {
		cmds = gCMDs
	}
	command, found := cmds[cmdName]
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
		cmdName = "cmc"
		cmdArgs = []string{cmcTicker.Symbol}
		numArgs = 1
	}

	if numArgs == 0 {
		cmdArgs = []string{}
	}
	if _, found := privateCmds[cmdName]; found && !isPrivateMsg {
		// Private command requested from a channel/server
		_, err := discordSend(discord, channelID, "Private commands are not allowed in public channels.", true)
		logErrorTS(debugTag, err)
		return
	}

	debugTag = "cmd] [" + cmdName
	logTS(debugTag, fmt.Sprintf("Author: %s, GuildID: %s, ChannelID: %s, Message: %s", message.Author, message.GuildID, message.ChannelID, message.Content))
	if command.Type == "text" {
		textCmdHandler(discord, message.GuildID, channelID, debugTag, command, cmdArgs, numArgs)
		return
	}
	switch cmdName {
	case "address":
		cmdAddress(discord, channelID, fmt.Sprint(message.Author), debugTag, cmdArgs, numArgs)
		break
	case "alert":
		cmdAlert(discord, message.GuildID, channelID, message.Author.ID, username, debugTag, cmdArgs, numArgs)
		break
	case "balance":
		cmdBalance(discord, channelID, debugTag, cmdArgs, userAddresses, numArgs, numAddresses)
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
	case "dexbalance": // Private Command
		cmdDexBalance(discord, channelID, debugTag, cmdArgs, userAddresses, numArgs, numAddresses)
		break
	case "guildcmd":
		guildCMDHandler(discord, message)
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
	case "help":
		helpHanlder(discord, channelID, message.GuildID, debugTag, isPrivateMsg, cmdArgs, numArgs)
		break
	case "mn":
		cmdMN(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "nodes": // Private Command
		cmdNodes(discord, channelID, debugTag, cmdArgs, userAddresses, numArgs, numAddresses)
		break
	case "orders":
		fallthrough
	case "orderbook":
		fallthrough
	case "trades":
		cmdDexTrades(discord, channelID, debugTag, cmdArgs, userAddresses, numArgs, numAddresses, cmdName)
		break
	case "ticker":
		cmdDexTicker(discord, channelID, debugTag, cmdArgs, numArgs)
		break
	case "tokens":
		cmdDexTokens(discord, channelID, debugTag, cmdArgs, numArgs)
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

// userHasRole checks if a user has a specific role on a server/guild
func userHasRole(discord *discordgo.Session, guildID, userID, roleName string) bool {
	debugTag := "userHasRole"
	roles, err := discord.GuildRoles(guildID)
	member, err2 := discord.GuildMember(guildID, userID)
	if logErrorTS(debugTag, err) || logErrorTS(debugTag, err2) {
		return false
	}
	roleID := ""
	for _, role := range roles {
		if strings.ToLower(role.Name) == roleName {
			roleID = role.ID
		}
	}
	for _, mRoleID := range member.Roles {
		if mRoleID == roleID {
			return true
		}
	}
	return false
}
