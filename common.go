package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

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

func logTS(debugTag, str string) {
	fmt.Printf("%s [%s] : %s\n", client.NowTS(), debugTag, str)
}

func logErrorTS(debugTag string, err error) (hasError bool) {
	if err == nil {
		return
	}
	logTS(debugTag, "[Error] => "+err.Error())
	return true
}

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
