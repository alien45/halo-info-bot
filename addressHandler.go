package main

import (
	"fmt"
	"strings"

	client "github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

func cmdAddress(discord *discordgo.Session, channelID, user, debugTag string, cmdArgs []string, numArgs int) {
	addresses := data.AddressBook[user]
	numAddrs := len(addresses)
	addrMap := map[string]bool{}
	action := ""
	txt := ""
	var err error
	if numArgs == 0 {
		if numAddrs == 0 {
			txt = "No addresses available!"
			goto SendMessage
		}
		txt = client.DashLine
		for i := 0; i < numAddrs; i++ {
			txt += fmt.Sprintf("%d. %s\n%s", i+1, addresses[i], client.DashLine)
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
		} else if numArgs >= 100 {
			txt = "You have reached the maximum number (100) of items in you address book."
			goto SendMessage
		}
		addresses = append(addresses, strings.Split(strings.Replace(strings.Join(cmdArgs[1:], " "), "\n", " ", -1), " ")...)
		for i := 0; i < len(addresses); i++ {
			addrMap[addresses[i]] = true
		}
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
		if !addrMap[addresses[i]] || strings.TrimSpace(addresses[i]) == "" {
			continue
		}
		data.AddressBook[user] = append(data.AddressBook[user], addresses[i])
		delete(addrMap, addresses[i])
	}
	err = saveDataFile()
	if logErrorTS(debugTag, err) {
		txt = "Failed to save changes!"
		goto SendMessage
	}
	txt = "Changes saved"
SendMessage:
	discordSend(discord, channelID, "js\n"+txt, true)
}
