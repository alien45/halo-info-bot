# Halo Info Bot
## Intro
Discord bot for the Halo Platform that incorporates internal and external APIs from HaloDEX, Halo Explorer, Halo Masternodes DApp, CoinMarketCap.com, Etherscan.io etc. 

Wanna try it out? If you are on Discord, why not check it out here: https://discord.gg/5Z7ZqeJ

## ***Halo Info Bot has been added Halo Platform discord server*** 
Check it out here: https://discord.gg/zCXW3uj


## Supported Commands

### !address [action] [address1] [address2...]: 
  - Add, remove and get list of saved addresses. 
  - Example:
    <ul>
      <li>!addresses</li>
      <li>!addresses add 0x1234</li>
      <li>!addresses remove 0x1234</li>
    </ul>
  - Private command. Only available by PMing the bot.

### !alert \<type> [action]:
  - Enable/disable automatic alerts. Alert types: payout. Actions:on, off, status, send. Only root user can use 'send' to trigger payout alert manually. 
  - Example:
    <ul>
      <li>!alert payout on</li>
      <li>!alert payout status</li>
    </ul>

### !balance \<address> [ticker]: 
  - Check your account balance. Supported addresses/chains: HALO & ETH. Address keywords: 'reward-pool', 'charity', 'h-eth', 'dex-halo'. If no address supplied, the first item of user's address book will be used. To get balance of a specific item from address book just type the index number of the address. 
  - Example:
    <ul>
      <li>!balance 0x1234567890abcdef</li>
      <li>!balance dex-halo</li>
      <li>!balance</li>
      <li>!balance 2 (for 2nd item in the address book)</li>
    </ul>

### !cmc \<symbol>: 
  - Fetch CoinMarketCap ticker information. Alternatively, use the ticker itself as command. 
  - Example:
    <ul>
      <li>!cmc powr, </li>
      <li>!cmc power ledger, </li>
      <li>!powr (shorthand for '!cmc powr')</li>
    </ul>

### !dexbalance \<address> [ticker1] [ticker2...]: 
  - Shows user's HaloDEX balances. USE YOUR HALO CHAIN ADDRESS FOR ALL TOKEN BALANCES WITHIN DEX. 
  - Example:
    <ul>
      <li>!dexbalance 0x123... </li>
      <li>!dexbalance 0x123... ETH</li>
    </ul>
  - Private command. Only available by PMing the bot.

### !halo : 
  - Get a digest of information about Halo including ticker info from DEX, reward pool and recent trades.

### !help [command-name]: 
  - Prints list of commands and supported arguments. If argument 'command' is provided will display detailed information about the command along with examples. 
  - Example:
    <ul>
      <li>!help </li>
      <li>!help balance</li>
    </ul>

### !mn : 
  - Shows masternode collateral, reward pool balances, nodes distribution, last payout and ROI based on last payout. 

### !nodes \<address> [address2] [address3....]: 
  - Lists masternodes owned by a specific address. If no address supplied, will use user's first address book item when available. 
  - Example: 
    <ul>
      <li>!nodes 0x1234</li>
      <li>!nodes</li>
      <li>!nodes 0x123 0x324 0x234</li>
    </ul>
  - Private command. Only available by PMing the bot.

### !orders [quote-ticker] [base-ticker] [limit] [address]: 
  - Get HaloDEX orders by user address. If no address supplied, will use user's first address book item when available. 
  - Example: 
    <ul>
      <li>!orders halo eth 10 0x1234567890abcdef </li>
      <li>!orders</li>
      <li>!orders vet eth</li>
    </ul>
  - Private command. Only available by PMing the bot.

### !ticker [quote-ticker] [base-ticker]: 
  - Get ticker information from HaloDEX. 
  - Example:
    <ul> 
      <li>!ticker</li>
      <li>!ticker vet</li>
      <li>!ticker dbet eth</li>
    </ul>

### !tokens [ticker]: 
  - Lists all tokens supported on HaloDEX 
  - Example: 
    <ul>
      <li>!tokens </li>
      <li>!tokens halo</li>
    </ul>

### !trades [quote-symbol] [base-symbol] [limit]: 
  - Recent trades from HaloDEX 
  - Example:
    <ul>
      <li>!trades halo eth 10 </li>
      <li>!trades eth halo</li>
      <li>!trades</li>
    </ul>


\<argument> => required\
[argument] => optional\
{argument} => indicates exact value

Defaults where applicable:\
--Base ticker => ETH\
--Quote ticker => Halo\
--Address(es) => first/all item(s) saved on address book, if avaiable
