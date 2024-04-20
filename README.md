# Order-Client

Order-Client is a bot designed to continuously scan for eIBC demand orders and fulfill them. It operates by periodically searching through the Cosmos client and subscribing to eIBC events.

## Features

- **Order Fulfillment**: The bot is constantly on the lookout for eIBC demand orders. Once an order is detected, the bot fulfills it.

- **Balance Checks**: The bot periodically checks its balances for the denominations it needs to fulfill orders. If it lacks funds for a particular denomination, it will alert the development team via Slack and skip fulfilling orders for that denomination until its account is topped up.

- **Gas Payment**: The bot uses DYM coins to pay for gas. If it runs out of DYM coins, it will alert the development team via Slack and pause all order fulfillments until it is topped up.

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
