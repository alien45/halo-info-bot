package main

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func cmdAlert(discord *discordgo.Session, channelID, username, debugTag string, cmdArgs []string, numArgs int) {
	// Enable/disable alerts. For personal chat. Possibly for channels as well but should only be setup by admins
	// TODO: dex status notification // using realtime API
	// TODO: feather update notification
	txt := ""
	action := ""
	exists := false
	if numArgs == 0 {
		txt = "Alert type required. Suppreted types: payout"
		goto AlertMessage
	}
	if numArgs >= 2 {
		action = strings.ToLower(cmdArgs[1])
	}
	_, exists = data.Alerts.Payout[channelID]

	switch strings.ToLower(cmdArgs[0]) {
	case "payout":
		txt = "Payout alert "
		if action == "on" {
			data.Alerts.Payout[channelID] = username
			txt += "turned on"
		} else if action == "off" {
			delete(data.Alerts.Payout, channelID)
			txt += "turned off"
		} else if exists {
			txt += "is on"
		} else {
			txt += "is off"
		}
		err := saveDiscordFile()
		if commandErrorIf(err, discord, channelID, "Failed to save preferences", debugTag) {
			return
		}
		break
	default:
		txt = "Not implemented or unavailable"
		break
	}
AlertMessage:
	discordSend(discord, channelID, txt, true)
}
