package main

import (
	"fmt"
	"os"
	"strings"

	d "github.com/deroholic/derogo"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/chzyer/readline"
	"github.com/holiman/uint256"
)

var wallet_password string
var wallet_file = "wallet.db"
var daemon_address = "127.0.0.1:10102"
var testnet = false
var zerohash crypto.Hash

func parseOpt(param string) {
        s := strings.Split(param, "=")

        if len(s) > 1 && s[0] == "--daemon-address" {
                daemon_address = s[1]
        } else if len(s) > 1 && s[0] == "--wallet" {
		wallet_file = s[1]
        } else if len(s) > 1 && s[0] == "--password" {
                wallet_password = s[1]
        } else if s[0] == "--help" {
                fmt.Printf("wallet [--help] [--wallet=<wallet_file> | <private_key>] [--password=<wallet_password>] [--daemon-address=<127.0.0.1:10102>]\n")
                os.Exit(0)
        } else {
                fmt.Printf("invalid argument '%s', skipping\n", param)
        }
}

func walletOpts() {
        for i:= 0; i < len(os.Args[1:]); i++ {
                param := os.Args[i+1]
                if param[0] == '-' && param[1] == '-' {
                        parseOpt(param)
                } else {
                }
        }
}

func multDiv(a uint64, b uint64, c uint64) (uint64) {
	A := uint256.NewInt(a)
	B := uint256.NewInt(b)
	C := uint256.NewInt(c)

	A = A.Mul(A, B)
	C = A.Div(A, C)

	return C.Uint64()
}

func main() {
	walletOpts()

	var err error
	var valid bool

	l, err = readline.NewEx(&readline.Config{
		Prompt:          "\033[31m»\033[0m ",
		HistoryFile:     "",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()

	if len(wallet_password) == 0 && len(wallet_file) != 64 {
		setPasswordCfg := l.GenPasswordConfig()
		setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
			l.SetPrompt(fmt.Sprintf("Enter password(%v): ", len(line)))
			l.Refresh()
			return nil, 0, false
		})

		pwd, _ := l.ReadPasswordWithConfig(setPasswordCfg)
		wallet_password = string(pwd)
	}

	d.DeroInit(daemon_address)

	var mainnet uint64

	mainnet, valid = d.DeroGetKeyUint64("mainnet")
	if !valid {
		panic("Cannot determine network\n")
	}
	bridgeRegistry, valid = d.DeroGetKeyHex("dex.bridge.registry")
	if !valid {
		panic("Cannot find bridge registry contract\n")
	}
	swapRegistry, valid = d.DeroGetKeyHex("dex.swap.registry")
	if !valid {
		panic("Cannot find swap registry contract\n")
	}

	if mainnet == 0 {
		testnet = true
	}
	d.DeroWalletInit(daemon_address, !testnet, wallet_file, wallet_password)

	fmt.Println("Building lookup tables...")
	if os.Getenv("USE_BIG_TABLE") != "" {
		d.DeroInitLookupTable(1, 1<<24);
	} else {
		d.DeroInitLookupTable(2, 1<<21);
	}

	displayTokens()
	commandLoop()
}

func callTransfer(scid string, dero_addr string, amount uint64) bool {
	var transfers []rpc.Transfer

	if scid == zerohash.String() {
		scid = ""
	}

	transfers = d.DeroBuildTransfers(transfers, scid, dero_addr, amount, 0)

	txid, b := d.DeroTransfer(transfers)
	if !b {
		fmt.Println("Transaction failed.")
		return false
	}

	fmt.Printf("Transaction submitted: txid = %s\n", txid)
	return true
}

func transfer(words []string) {
	if len(words) != 3 {
		fmt.Println("Transfer requires 3 arguments:\n")
		printHelp()
		return
	}


	token := words[0]
	tok := tokens[token]
	scid := tok.contract
	decimals := tok.decimals

	if len(scid) == 0 {
		pair := pairs[token]

		if len(pair.contract) > 0 {
			scid = pair.contract
			decimals = 0
		} else {
			fmt.Printf("Token '%s' not found.\n", token)
			return
		}
	}

	amount, err := d.DeroStringToAmount(words[2], decimals)
	if err != nil {
		fmt.Printf("Cannot parse amount '%s'\n", words[2])
		return
	}

	a, err := d.DeroParseValidateAddress(words[1])
        if err != nil {
                fmt.Printf("Cannot parse wallet address '%s'\n", words[1])
                return
        }

	fmt.Printf("Transfer %f %s to %s\n", d.DeroFormatMoneyPrecision(amount, decimals), token, words[1])

	if askContinue() {
		callTransfer(scid, a.String(), amount)
	}
}

func displayAddress() {
	fmt.Printf("Wallet address %s\n", d.DeroGetAddress())
}
