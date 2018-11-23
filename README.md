# Halo Info Bot
## Intro
Discord bot for the Halo Platform that incorporates internal and external APIs from HaloDEX, Halo Explorer, Halo Masternodes DApp, CoinMarketCap.com, Etherscan.io etc. 

Wanna try it out? If you are on Discord, why not check it out here: https://discord.gg/5Z7ZqeJ


## Supported Commands

### !balance \<address> [ticker]
  - Check your account balance. Supported addresses/chains: HALO & ETH. Address keywords: 'reward pool', 'charity', 'h-eth'. 
  - Example: !balance 0x1234567890abcdef

### !cmc \<ticker-symbol>
  - Fetch CoinMarketCap tickers 
  - Example: !cmc btc OR, !cmc bitcoin cash

### !dexbalance \<address> [{0} or [ticker ticker2 ticker3...]] 
  - Check your DEX balances. USE YOUR HALO CHAIN ADDRESS FOR ALL TOKEN BALANCES WITHIN DEX. 
  - Example: !dexbalance 0x123... 0 OR, !dexbalance 0x123... ETH
  - Private command. Only available by PMing the bot.

### !halo
  - Get a digest of information about Halo. 
  - Example: !halo, !vet

### !help 
  - Prints this message 

## !mn [{info}]
  - Masternode reward pool and nodes distribution information. Or get masternode collateral info. 
  - Example: !mn OR, !mn info

### !nodes \<address> [address2] [address3....]
  - Lists masternodes owned by a specific address 
  - Example: !nodes 0x1234567890abcdef
  - Private command. Only available by PMing the bot.

### !orders \<quote-ticker> \<base-ticker> \<limit> \<address>
  - Get HaloDEX orders by user address. 
  - Example: !orders halo eth 10 0x1234567890abcdef
  - Private command. Only available by PMing the bot.

### !ticker [quote-ticker] [base-ticker]
  - Get ticker information from HaloDEX. 
  - Example: !ticker OR !ticker vet OR, !ticker dbet eth

### !tokens [ticker]
  - Lists all tokens supported on HaloDEX 
  - Example: !tokens OR, !tokens halo

### !trades [quote-symbol] [base-symbol] [limit]
  - Recent trades from HaloDEX 
  - Example: !trades halo eth 10


\<argument> => required\
[argument] => optional\
{argument} => indicates exact value

Defaults where applicable:\
  Base ticker => ETH\
  Quote ticker => Halo
