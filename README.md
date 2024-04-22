# Order-Client

Order-Client is a bot designed to continuously scan for eIBC demand orders and fulfill them.

## Features

- **Account Setup**: The bot is configured with a Dymension account that can be found in the local keyring. The account is used to fulfill demand orders.

- **Order Refresh**: The bot periodically refreshes its demand order list to ensure it has the most up-to-date information. 
Apart from refreshing the order list, the bot also checks for new orders by subscribing to eIBC events.

- **Order Fulfillment**: The bot fulfills unfulfilled pending demand orders it finds on the hub.

- **Balance Checks**: Every time the bot attempts to fulfill orders it will first check its balances for the denominations it needs to fulfill them.
If it lacks funds for a particular denomination as specified in the order price, it will send an alert by posting a message on Slack,
and will skip trying to fulfill that order until its account is topped up.

- **Gas Payment**: The bot uses DYM coins to pay for gas. If it runs out of DYM coins, it will send an alert by posting a message on Slack,
and will pause all order fulfillments until it is topped up.

- **Order Cleanup**: The bot periodically checks and flushes fulfilled demand orders.

## Setup

To set up the bot, you need to have Go installed on your machine. Once Go is installed, you can clone this repository and run the bot.

## Usage

To run the bot, use the following command:

### Local

```bash
make install

order-client --config <path-to-config-file>
```

### Docker

```bash
make docker-build

docker run -v <path-to-config-file>:/.order-client.yaml order-client --config /.order-client.yaml
```
