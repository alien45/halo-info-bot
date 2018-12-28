package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Commands Command list with command name as key
type Commands map[string]Command

// GuildCommands Commands lists with Guild ID as key
type GuildCommands map[string]Commands

// Command describes bot supported command information
type Command struct {
	// Type of the command. Valid types: text, welcometext,
	Type string `json:"type"`
	// Brief/one-liner description about the command
	Description string `json:"description"`
	// List supported arguments and/or construct.
	// This should be the one-liner showing the user how to construct the command arguments if available.
	// Leave empty, if command has no arguments.
	// Must NOT include the command name and follow the following rules:
	// 1. <argument> => required
	// 2. [argument] => optional
	// 3. {argument} => indicates exact value
	// Example 1: <add> {email} [address]
	// The above example shows the construct of a command (say for an address book) where,
	// 1. first argument "name" is a single-word variable text and it is a required argument
	// 2. second argument "email" is a keyword argument where the value MUST be exactly as typed, without the curly braces.
	// 3. third argument "address" is optional. Being the last argument it's value is not restricted to a single word.
	ArgumentsText string `json:"argumentstext"`
	// Whether the command can be used in public channels/groups.
	// If true, the command can only be invoked by texting the "bot user" privately
	IsPublic bool `json:"ispublic"`
	// Whether this command can only be executed by an admin when in a public channel.
	// Irrelevant for private messages/channels.
	IsAdminOnly bool `json:"isadminonly"`
	// An example of how to use the command
	Example string `json:"example"`
	// For Info/Custom commands
	Arguments Arguments `json:"arguments"`
	Message   string    `json:"message"`
}

// Arguments ...
type Arguments map[string]Argument

// Argument defines how arguments for custom commands are constructed
type Argument struct {
	Message     string `json:"message"`
	Description string `json:"description"`
}

// Generate text for the help command
func generateHelpText(commands Commands, publicOnly bool) (s string) {
	cmdNames := []string{}
	for name := range commands {
		if commands[name].Type == "text" && commands[name].Message == "" {
			// Ignore empty-message commands
			continue
		}
		cmdNames = append(cmdNames, name)
	}
	sort.Strings(cmdNames)
	for i := 0; i < len(cmdNames); i++ {
		name := cmdNames[i]
		command := commands[cmdNames[i]]
		if (publicOnly && !command.IsPublic) || (!publicOnly && command.IsAdminOnly) {
			continue
		}
		s += fmt.Sprintf("!%s %s\n", name, command.ArgumentsText)
	}
	s += "\n<argument> => required"
	s += "\n[argument] => optional"
	s += "\n{argument} => indicates exact value"
	s += "\n\nDefaults where applicable:\n - Base ticker => ETH,\n - Quote ticker => Halo\n" +
		" - Address(es) => first/all item(s) saved on address book, if avaiable"
	return
}

// commandHelpText returns help text for a specific command
func commandHelpText(commands Commands, commandName string) (s string) {
	for cmdName, command := range commands {
		if cmdName != strings.ToLower(commandName) {
			continue
		}
		if command.Type == "text" && command.Message == "" {
			return
		}
		s += fmt.Sprintf("!%s %s: \n  - %s \n", cmdName, command.ArgumentsText, command.Description)
		if command.Example != "" {
			seperator := "\n           "
			exampleF := seperator + strings.Join(strings.Split(command.Example, "OR, "), seperator)
			if exampleF != "" {
				s += fmt.Sprintf("  - Example: %s\n", exampleF)
			}
		}
		if command.IsAdminOnly {
			s += "  - Guild admin command. Only available in guilds and with user role: ButlerAdmin\n"
		}
		if !command.IsPublic {
			s += "  - Private command. Only available by PMing the bot.\n"
		}
		s += "\n"
	}
	if s == "" {
		s = commandName + " is not a valid command"
	}
	return
}

func helpHanlder(discord *discordgo.Session, channelID, guildID, debugTag string, isPrivateMsg bool, cmdArgs []string, numArgs int) {
	txt := ""
	isGuild := guildID != ""
	fmt.Println("gID", guildID, txt)
	if numArgs > 0 && !isGuild {
		txt = commandHelpText(commands, cmdArgs[0])
	} else if numArgs > 0 && isGuild {
		txt = commandHelpText(guildCommands[guildID], cmdArgs[0])
	} else if isPrivateMsg && isGuild {
		txt = generateHelpText(guildCommands[guildID], false)
	} else if isPrivateMsg {
		txt = generateHelpText(commands, false)
	} else if isGuild && guildCommands[guildID] != nil {
		txt = generateHelpText(guildCommands[guildID], true)
	} else {
		txt = generateHelpText(commands, true)
	}
	_, err := discordSend(discord, channelID, "css\n"+txt, true)
	logErrorTS(debugTag, err)
}
