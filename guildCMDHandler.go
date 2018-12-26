package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func guildCMDHandler(discord *discordgo.Session, message *discordgo.MessageCreate) {
	if message.GuildID == "" {
		// ignore if not from a guild
		return
	}
	text := ""
	args := strings.Split(message.Content, " ")[1:]
	numArgs := len(args)
	userID := message.Author.ID
	guildID := message.GuildID
	var err error
	if !userHasRole(discord, guildID, userID, guildAdminRole) &&
		fmt.Sprint(message.Author) != conf.Client.DiscordBot.RootUser {
		text = "You do not have permission to manage guild commands"
		goto SendMessage
	}
	if numArgs < 1 {
		text = "Action required"
		goto SendMessage
	}
	if numArgs < 2 {
		text = "Custom command name required"
		goto SendMessage
	}
	switch strings.ToLower(args[0]) {
	case "add":
		_, exists := commands[args[1]]
		if !exists && numArgs < 3 {
			text = "Message required"
			goto SendMessage
		}
		if strings.ToLower(args[1]) == guildCMD {
			text = "This command cannot be overridden."
			goto SendMessage
		}
		msg := strings.Join(args[2:], " ")
		if len(msg) > 500 {
			text = "Message cannot be more than 500 characters"
			goto SendMessage
		}
		err = addGuildCommand(guildID, strings.ToLower(args[1]), msg)
		break
	case "remove", "delete":
		err = removeGuildCommand(guildID, args[1])
		break
	default:
		text = "Invalid action"
		goto SendMessage
	}

	text = "Action failed"
	if !logErrorTS(guildCMD, err) {
		text = "Saved"
		// generate list of commands
		generateCommandLists()
	}
SendMessage:
	discordSend(discord, message.ChannelID, text, true)
}

func addGuildCommand(guildID, name, message string) (err error) {
	if data.GuildInfoCommands == nil {
		data.GuildInfoCommands = GuildCommands{}
	}
	if data.GuildInfoCommands[guildID] == nil {
		data.GuildInfoCommands[guildID] = Commands{}
	}
	data.GuildInfoCommands[guildID][name] = Command{
		Type:     "text",
		IsPublic: true,
		Message:  strings.Replace(message, `"`, "'", 0),
	}
	return saveDataFile()
}

func removeGuildCommand(guildID, name string) (err error) {
	if data.GuildInfoCommands == nil {
		data.GuildInfoCommands = GuildCommands{}
	}
	if data.GuildInfoCommands[guildID] == nil {
		data.GuildInfoCommands[guildID] = Commands{}
	}
	delete(data.GuildInfoCommands[guildID], name)
	return saveDataFile()
}
