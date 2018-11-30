# Halo Info Bot
## Intro
Discord bot for the Halo Platform that incorporates internal and external APIs from HaloDEX, Halo Explorer, Halo Masternodes DApp, CoinMarketCap.com, Etherscan.io etc. 

Wanna try it out? If you are on Discord, why not check it out here: https://discord.gg/5Z7ZqeJ

## ***Halo Info Bot has been added Halo Platform discord server*** 
Check it out here: https://discord.gg/zCXW3uj


## Supported Commands

### !address [action \<address1> \<address2>...]: 
  - Add, remove and get list of saved addresses. 
  - Example: !addresses OR !addresses add 0x1234 OR !addresses remove 0x1234
  - Private command. Only available by PMing the bot.

### !alert \<type> [action]: 
  - Enable/disable automatic alerts. Alert types: payout. Actions:on/off 
  - Example:\
           !alert payout on
  - Private command. Only available by PMing the bot.

### !balance [address] [ticker]: 
  - Check your account balance. Supported addresses/chains: HALO & ETH. Address keywords: 'reward pool', 'charity', 'h-eth'. If not address supplied, the first item of user's address book will be used. To get balance of a specific item from address book just type the index number of the address. 
  - Example: !balance 0x1234567890abcdef OR !balance OR, !balance 2 (for 2nd item in the address book)

### !cmc \<symbol>: 
  - Fetch CoinMarketCap ticker information. Alternatively, use the ticker itself as command. 
  - Example: !cmc powr, OR, !cmc power ledger, OR !powr (shorthand for '!cmc powr')

### !dexbalance [address] [{0} or [ticker ticker2 ticker3...]]: 
  - Shows user's HaloDEX balances. USE YOUR HALO CHAIN ADDRESS FOR ALL TOKEN BALANCES WITHIN DEX. 
  - Example: !dexbalance 0x123... 0 OR, !dexbalance 0x123... ETH
  - Private command. Only available by PMing the bot.

### !halo : 
  - Get a digest of information about Halo. 
  - Example: !halo

### !help [command]: 
  - Prints list of commands and supported arguments. If argument 'command' is provided will display detailed information about the command along with examples.
  - Example: !help OR !help balance

### !mn : 
  - Shows masternode reward pool, nodes distribution, last payout and ROI based on last payout. Or get masternode collateral info. 

### !nodes [address] [address2] [address3....]: 
  - Lists masternodes owned by specific address(es) 
  - Example: !nodes 0x1234567890abcdef 0x1234567890abcdee 
  - Private command. Only available by PMing the bot.

### !orders [quote-ticker] [base-ticker] [limit] [address]: 
  - Get HaloDEX orders by user address. 
  - Example: !orders halo eth 10 0x1234567890abcdef
  - Private command. Only available by PMing the bot.

### !ticker [quote-ticker] [base-ticker]: 
  - Get ticker information from HaloDEX. 
  - Example: !ticker OR !ticker vet OR, !ticker dbet eth

### !tokens [ticker]: 
  - Lists all tokens supported on HaloDEX 
  - Example: !tokens OR, !tokens halo

### !trades [quote-symbol] [base-symbol] [limit]: 
  - Recent trades from HaloDEX 
  - Example: !trades halo eth 10 OR, !trades


\<argument> => required\
[argument] => optional\
{argument} => indicates exact value

Defaults where applicable:\
--Base ticker => ETH\
--Quote ticker => Halo\
--Address(es) => first/all item(s) saved on address book, if avaiable
