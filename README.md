# eibc-client

eibc-client is a bot designed to continuously scan for eIBC demand orders and fulfill them.

## Features

- **Order Scanning**: The bot scans the hub for demand orders that are unfulfilled and have not expired.
    - Gets unfulfilled demand orders from the indexer by polling periodically
    - Gets orders from events emitted by the Hub for new demand orders.
- **Order Fulfillment Criteria**:
    - An order needs to provide the minimum amount of fee earnings to the operator and the LP
    - The Rollapp where the order comes needs to be supported by at least one LP
    - The Denom of the order needs to be supported by at least one LP
    - The order needs to be within the max price of the LP
    - The order needs to be within the spend limit of the LP
- **LP Grants Refresh**: The bot periodically refreshes its list of LP grants to ensure it has the most up-to-date LP information.
- **Order Fulfillment**: The bot fulfills unfulfilled pending demand orders it finds on the hub, by sourcing the funds from LPs.
- **Fee/Gas Payment**: The operator grants all the running bot accounts the permission to pay for fees on their behalf.
- **Concurrency**: Run multiple bots concurrently to fulfill orders faster. Also, fulfill multiple orders per transaction.

## Setup

To set up the bot, you need to have Go installed on your machine. Once Go is installed, you can clone this repository and run the bot.

- **Operator Setup**: The eibc client will expect certain environment to be set up before it can run.
1. The operator account needs to be set up with some adym coins to pay for gas/fees.
   dymd keys add operator
2. A group needs to be created where the operator account is the admin:

        dymd tx group create-group operator --keyring-backend test "==A" members.json --fees 1dym -y

members.json
   ```
   {
      "members": [
           {
                "address": "<operator_address>",
                "weight": "1", "metadata": "president"
           }
        ]
   }
   ```
3. The group needs a policy that can be granted by LPs in order to source funds from them and sign order fulfillment messages on their behalf:

        dymd tx group create-group-policy operator 1 "==A" policy.json --fees 1dym -y

policy.json
```
{
    "@type": "/cosmos.group.v1.PercentageDecisionPolicy",
    "percentage": "0.0001",
    "windows": {
        "voting_period": "1s",
        "min_execution_period": "0s"
    }
}
```

4. At least one LP needs to grant authorization to the group to fulfill orders:

            dymd tx eibc grant <POLICY_ADDRESS> \
                      --from lp_key --keyring-backend test \
                      --spend-limit 10000adym \
                      --rollapp rollapp1 \ 
                      --denoms "adym,uatom" \
                      --min-lp-fee-percentage "0.1" \
                      --max-price 10000adym \
                      --operator-fee-share 0.1 \
                      --settlement-validated --fees 1dym -y

## Usage

To run the bot, use the following command:

### Local

```bash
make install

eibc-client start --config <path-to-config-file>
```
