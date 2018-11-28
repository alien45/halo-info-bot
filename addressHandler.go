package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func cmdAddress(discord *discordgo.Session, channelID, user, debugTag string, cmdArgs []string, numArgs int) {
	addresses := data.AddressBook[user]
	addrMap := map[string]bool{}
	action := ""
	txt := ""
	var err error
	if numArgs == 0 {
		if len(addresses) == 0 {
			txt = "No addresses available!"
			goto SendMessage
		}

		for i := 0; i < len(addresses); i++ {
			txt += fmt.Sprintf("%d. %s\n", i+1, addresses[i])
		}
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
		addresses = append(addresses, strings.Split(strings.Replace(strings.Join(cmdArgs[1:], " "), "\n", " ", -1), " ")...)
		for i := 0; i < len(addresses); i++ {
			addrMap[addresses[i]] = true
		}
		fmt.Println(addresses)
		break
	case "remove":
		fallthrough
	case "delete":
		for i := 0; i < len(addresses); i++ {
			for a := 1; a < len(cmdArgs); a++ {
				addrMap[addresses[i]] = addresses[i] != cmdArgs[a]
			}
		}
		break
	default:
		txt = "Invalid action. Supported actions: add, remove"
		goto SendMessage
	}

	data.AddressBook[user] = []string{}
	for i := 0; i < len(addresses); i++ {
		if !addrMap[addresses[i]] || strings.TrimSpace(addresses[i]) == "" || !strings.HasPrefix(addresses[i], "0x") {
			continue
		}
		data.AddressBook[user] = append(data.AddressBook[user], addresses[i])
		fmt.Println(addresses[i], addrMap[addresses[i]])
		delete(addrMap, addresses[i])
	}
	err = saveDiscordFile()
	if logErrorTS(debugTag, err) {
		txt = "Failed to save changes!"
		goto SendMessage
	}
	txt = "Changes saved"
SendMessage:
	discordSend(discord, channelID, "\n"+txt, true)
}
