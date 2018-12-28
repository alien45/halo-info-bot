package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	client "github.com/alien45/halo-info-bot/client"
	"github.com/bwmarrin/discordgo"
)

const configFile = "./config.json"
const dataFile = "./discord.json"
const commandsFile = "./commands.json"
const debugFile = "./debug.log"
const guildAdminRole = "butleradmin" // case-insensitive allowed
const guildCMD = "guildcmd"

var (
	botID string
	conf  Config
	data  DiscordData
	// API clients
	cmc       client.CMC
	dex       client.DEX
	explorer  client.Explorer
	etherscan client.Etherscan
	mndapp    client.MNDApp
	//
	addressKeywords map[string]string
	// Default commands
	commands = Commands{}
	// guildCommands
	guildCommands = GuildCommands{}
	// Commands names that are not allowed in the public chats. Key: command, value: unused.
	privateCmds = map[string]string{}
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
	LastPayout      client.Payout     `json:"lastpayout"`
	AddressKeywords map[string]string `json:"addresskeywords"`
	Info            map[string]string `json:"info"`
	// Global Info commands
	GlobalInfoCommands Commands `json:"globalinfocmds"`
	// Guild specific commands
	// Use this to override or disable builtin and global info commands
	// "guildid" : {"commandname" : CommandStruct}
	GuildInfoCommands GuildCommands `json:"guildinfocmds"`
	Alerts            struct {
		Payout map[string]string `json:"payout"`
	} `json:"alerts"` // key: channel id, value: channel id/username
	PrivacyExceptions map[string]string `json:"privacyexceptions"` // key: channel id, value: name
	AddressBook       map[string][]string
}

func main() {
	setLogFile()
	logTS("start", "Application started")
	// Load configuration
	configStr, err := client.ReadFile(configFile)
	panicIf(err, "Failed to read config file")
	err = json.Unmarshal([]byte(configStr), &conf)
	panicIf(err, "Failed to load "+configFile+" file")

	// generate list of commands
	generateCommandLists()

	// address keywords
	addressKeywords = data.AddressKeywords

	// generate private commands' list
	for cmdStr, cm := range commands {
		if !cm.IsPublic {
			privateCmds[cmdStr] = ""
		}
	}

	cmc = conf.Client.CMC
	// Force cache CMC tickers
	cmc.GetTicker("eth")
	dex = conf.Client.DEX
	etherscan = conf.Client.Etherscan
	explorer = conf.Client.Explorer
	conf.Client.MNDApp.LastPayout = data.LastPayout
	mndapp = conf.Client.MNDApp

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
		numServers := len(discord.State.Guilds)
		err = discord.UpdateStatus(1, fmt.Sprintf("Halo Bulter on %d servers", numServers))
		if logErrorTS("Discord] [Error", err) {
			return
		}

		logTS("Discord] [Ready", fmt.Sprintf("Halo Info Bot has started on %d servers", numServers))

		go discordInterval(discord, 120, true, checkPayout)
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
	log.Printf("[%s] : %s\n", debugTag, strings.Replace(str, "\n", "\\n", 0))
}

func logErrorTS(debugTag string, err error) (hasError bool) {
	if err == nil {
		return
	}
	logTS(debugTag, "[Error] => "+err.Error())
	return true
}

func saveDataFile() (err error) {
	return client.SaveJSONFile(dataFile, data)
}

func generateCommandLists() {
	// Load defalt commands
	commandsStr, err := client.ReadFile(commandsFile)
	panicIf(err, "Failed to read config file")
	err = json.Unmarshal([]byte(commandsStr), &commands)
	panicIf(err, "Failed to load "+commandsFile+" file")

	// Load discord data
	discordStr, err := client.ReadFile(dataFile)
	panicIf(err, "Failed to read Discord data file")
	err = json.Unmarshal([]byte(discordStr), &data)
	panicIf(err, "Failed to load "+dataFile+" file")

	// Add global info commands. Will override default commands if command name (case-sensitive) matches.
	for cmdName, cmd := range data.GlobalInfoCommands {
		commands[cmdName] = cmd
	}
	// Construct a list of guild specific commands (if any specified in the data file)
	// along with default and global info commands (overrides default/global info commands if exact name specified)
	for gID, gCommands := range data.GuildInfoCommands {
		guildCommands[gID] = Commands{}
		for cmdName, cmd := range commands {
			guildCommands[gID][cmdName] = cmd
		}

		for cmdName, cmd := range gCommands {
			if cmdName != guildCMD {
				// avoid overriding the !cmd command
				guildCommands[gID][cmdName] = cmd
			}
		}
	}
}

// setLogFile sets log output file
func setLogFile() {
	if _, err := os.Stat(debugFile); os.IsNotExist(err) {
		_, err = os.Create(debugFile)
		panicIf(err, "Failed to create debug file "+debugFile)
	}
	file, err := os.OpenFile(debugFile, os.O_APPEND|os.O_WRONLY, 0666)
	panicIf(err, "Couldn't open debug file"+debugFile)
	//defer file.Close()
	multi := io.MultiWriter(file, os.Stdout)
	log.SetOutput(multi)
}
