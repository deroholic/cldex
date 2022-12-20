package main

import (
	"fmt"
	"strings"

	d "github.com/deroholic/derogo"
	"github.com/deroproject/derohe/rpc"
)

var bridgeRegistry string

func callBridge(scid string, eth_addr string, amount uint64, fee uint64) bool {
        var transfers []rpc.Transfer
        transfers = d.DeroBuildTransfers(transfers, scid, "", 0, amount)
        transfers = d.DeroBuildTransfers(transfers, zerohash.String(), "", 0, fee)

        var args rpc.Arguments
        args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "Bridge"})
        args = append(args, rpc.Argument {"eth_addr", rpc.DataString, eth_addr})

	txid, b := d.DeroSafeCallSC(scid, transfers, args)

	if !b {
		fmt.Println("Transaction failed.")
		return false
	}

	fmt.Printf("Transaction submitted: txid = %s\n", txid)
	return true
}

func bridge(words []string) {
	if len(words) != 3 {
		fmt.Println("Bridge requires 3 arguments:")
		printHelp()
		return
	}

	token := words[0]
	if token == "DERO" {
		fmt.Println("Cannot bridge DERO (yet).")
		return
	}

	scid := tokens[token].contract
	if scid == "" {
		fmt.Printf("Token '%s' not found.\n", token)
		return
	}

	amount, err := d.DeroStringToAmount(words[2], tokens[token].decimals)
	if err != nil {
		fmt.Printf("Cannot parse amount '%s'\n", words[2])
		return
	}

	if words[1] == strings.ToLower(words[1]) || words[1] == strings.ToUpper(words[1]) {
		fmt.Printf("Ethereum address must be in CamelCase (mixed case) not all lower or all upper.\n")
		fmt.Printf("Please check and try again with a different address format.\n")
		return
	}

	fmt.Printf("Transfer %f %s to Ethereum address %s\n", d.DeroFormatMoneyPrecision(amount, tokens[token].decimals), token, words[1])
	fmt.Printf("Bridge fee %f DERO\n", d.DeroFormatMoneyPrecision(tokens[token].bridgeFee, 5))

	if askContinue() {
		callBridge(scid, words[1], amount, tokens[token].bridgeFee)
	}
}
