#!/bin/bash

export bot_keyring_dir=$1
export default_bot_keyring_dir="${HOME}/.order-client"

if [[ -z "$1" ]]; then
    echo 'no custom key provided'
    echo "defaulting to  ${default_bot_keyring_dir}"
    echo 'to provide a custom bot keyring directory, use the following syntax:'
    echo './get-bot-balances.sh /path/to/your/bots'
    export bot_keyring_dir=$default_bot_keyring_dir
fi

# Define the command to fetch the list of objects
objects=$(dymd keys list --keyring-backend test --keyring-dir ${bot_keyring_dir} --output json)

# Parse the JSON output to get an array of addresses and names
addresses=$(echo "$objects" | jq -r '.[] | .address')
names=$(echo "$objects" | jq -r '.[] | .name')

# Read addresses and names into arrays
readarray -t addressArray <<<"$addresses"
readarray -t nameArray <<<"$names"

# Print the header of the table
echo "| Name | Address | Denom | Amount |"

# Loop through the addresses
for i in "${!addressArray[@]}"; do
    address="${addressArray[$i]}"
    name="${nameArray[$i]}"

    # Run the command to get balances for each address
    balance_output=$(dymd q bank balances "${address}" --output json)

    # Check if balances array is empty
    if [ "$(echo "$balance_output" | jq '.balances | length')" -eq 0 ]; then
        echo "| $name | $address | - | this address doesn't have any balance |"
    else
        # If balances exist, loop through each balance and print
        echo "$balance_output" | jq -r --arg name "$name" --arg address "$address" '.balances[] | "| \($name) | \($address) | \(.denom) | \(.amount) |"'
    fi
done
