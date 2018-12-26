package main

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func textCmdHandler(discord *discordgo.Session, guildID, channelID, debugTag string, command Command, args []string, numArgs int) {
	text := command.Message
	arg := Argument{}
	ok := false
	if numArgs > 0 {
		if arg, ok = command.Arguments[strings.ToLower(args[0])]; ok {
			text = arg.Message
			// json.Unmarshal([]byte(arg.Message), &text)
		}
	}
	_, err := discordSend(discord, channelID, text, false)
	logErrorTS(debugTag, err)
}
