package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	client "github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

const configFile = "./config.json"
const discordFile = "./discord.json"

var (
	botID           string
	conf            Config
	data            DiscordData
	cmc             client.CMC
	dex             client.DEX
	explorer        client.Explorer
	etherscan       client.Etherscan
	mndapp          client.MNDApp
	addressKeywords = map[string]string{
		// Halo Masternode reward pool
		"reward-pool": "0xd674dd3cdf07139ffda85b8589f0e2ca600f996e",
		// Charity address by the Halo Platform community
		"charity": "0xaefaffa2272098b4ab6f9a87a76f25944aee746d",
		// Ethereum address for H-ETH token
		"h-eth": "0x70a41917365E772E41D404B3F7870CA8919b4fBe",
	}
	// Commands names that are not allowed in the public chats. Key: command, value: unused.
	privateCmds     = map[string]string{}
	helpText        string
	helpTextPrivate string
)

// DiscordBot contains Discord bot authentication and other details
type DiscordBot struct {
	Token    string `json:"token"`
	Prefix   string `json:"prefix"`
	RootUser string `json:"rootuser"`
}

// Config configurations including API clients
type Config struct {
	Client struct {
		DiscordBot DiscordBot       `json:"discordbot"`
		CMC        client.CMC       `json:"cmc"`
		Etherscan  client.Etherscan `json:"etherscan"`
		DEX        client.DEX       `json:"halodex"`
		Explorer   client.Explorer  `json:"explorer"`
		MNDApp     client.MNDApp    `json:"mndapp"`
	} `json:"apiclients"`
}

// DiscordData stores Discord user preferences and other data
type DiscordData struct {
	LastPayout client.Payout `json:"lastpayout"`
	Alerts     struct {
		Payout map[string]string `json:"payout"`
	} `json:"alerts"` // key: channel id, value: channel id/username
	PrivacyExceptions map[string]string `json:"privacyexceptions"` // key: channel id, value: name
	AddressBook       map[string][]string
}

func main() {
	// Load configuration file
	configStr, err := client.ReadFile(configFile)
	panicIf(err, "Failed to read config file")
	err = json.Unmarshal([]byte(configStr), &conf)
	panicIf(err, "Failed to load "+configFile+" file")

	// Load discord data file
	discordStr, err := client.ReadFile(discordFile)
	panicIf(err, "Failed to read Discord data file")
	err = json.Unmarshal([]byte(discordStr), &data)
	panicIf(err, "Failed to load "+discordFile+" file")

	conf.Client.MNDApp.LastPayout = data.LastPayout

	// generate private commands' list
	for cmdStr, details := range supportedCommands {
		if !details.IsPublic {
			privateCmds[cmdStr] = ""
		}
	}

	cmc = conf.Client.CMC
	// Pre-cache CMC tickers
	cmc.GetTicker("eth")
	dex = conf.Client.DEX
	etherscan = conf.Client.Etherscan
	explorer = conf.Client.Explorer
	mndapp = conf.Client.MNDApp

	helpText = generateHelpText(true)
	helpTextPrivate = generateHelpText(false)

	// Connect to discord as a bot
	discord, err := discordgo.New("Bot " + conf.Client.DiscordBot.Token)
	panicIf(err, "error creating discord session")
	bot, err := discord.User("@me")
	panicIf(err, "error retrieving account")

	botID = bot.ID
	discord.AddHandler(func(discord *discordgo.Session, message *discordgo.MessageCreate) {
		go commandHandler(discord, message, conf.Client.DiscordBot.Prefix)
	})
	discord.AddHandler(func(discord *discordgo.Session, ready *discordgo.Ready) {
		err = discord.UpdateStatus(1, "Halo Bulter")
		if logErrorTS("Discord] [Error", err) {
			return
		}

		logTS("Discord] [Ready", fmt.Sprintf("Halo Info Bot has started on %d servers\n", len(discord.State.Guilds)))

		//go discordInterval(discord, 120, true, checkPayout)
	})

	err = discord.Open()
	panicIf(err, "Error opening connection to Discord")
	defer discord.Close()

	// Keep application open infinitely
	<-make(chan struct{})
}

func panicIf(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %+v\n", msg, err)
		panic(err)
	}
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

func saveDiscordFile() (err error) {
	dataBytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return
	}
	return ioutil.WriteFile(discordFile, dataBytes, 644)
}
