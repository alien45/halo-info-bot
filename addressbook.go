package main

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func cmdAddress(discord *discordgo.Session, channelID, user, debugTag string, cmdArgs []string, numArgs int) {
	addresses := data.AddressBook[user]
	tempAddresses := []string{}
	action := ""
	txt := ""
	saveFile := false
	var err error
	if numArgs == 0 {
		if len(addresses) == 0 {
			txt = "No addresses available!"
			goto SendMessage
		}
		// Reply all addresses by user
		txt = strings.Join(addresses, "\n")
		goto SendMessage
	}
	// Add/Remove addresses
	action = strings.ToLower(cmdArgs[0])
	switch action {
	case "add":
		if numArgs == 1 {
			txt = "No address provided!"
			goto SendMessage
		}
		addresses = append(addresses, cmdArgs[1:]...)
		saveFile = true
		break
	case "remove":
		fallthrough
	case "delete":
		for i := 0; i < len(addresses); i++ {
			remove := false
			for a := 1; a < len(cmdArgs); a++ {
				if addresses[i] == cmdArgs[a] {
					remove = true
				}
			}
			if !remove {
				tempAddresses = append(tempAddresses, addresses[i])
				continue
			}
			saveFile = true
		}
		if !saveFile {
			txt = "No changes made."
			goto SendMessage
		}
		addresses = tempAddresses
		break
	default:
		txt = "Invalid action. Supported actions: add, remove"
		break
	}
	if !saveFile {
		goto SendMessage
	}

	data.AddressBook[user] = []string{}
	for i := 0; i < len(addresses); i++ {
		if strings.TrimSpace(addresses[i]) == "" {
			continue
		}
		data.AddressBook[user] = append(data.AddressBook[user], addresses[i])
	}
	err = saveDiscordFile()
	if commandErrorIf(err, discord, channelID, "Failed to save changes!", debugTag) {
		return
	}
	// Action success. Reply with modified list
	if len(data.AddressBook[user]) == 0 {
		txt = "Changes saved"
		goto SendMessage
	}
	txt = "Saved addresses:\n" + strings.Join(data.AddressBook[user], "\n")
SendMessage:
	discordSend(discord, channelID, "\n"+txt, true)
}
