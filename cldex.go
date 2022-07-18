package main

import (
	"fmt"
	"os"
	"io"
	"strings"
	"strconv"
	"sync"
	"time"
	"bytes"
	"math"
	"math/big"
	"encoding/hex"
	"io/ioutil"

	d "github.com/deroholic/derogo"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/chzyer/readline"
	"github.com/holiman/uint256"
)

var bridgeRegistry = "d6d3fbe9d3f397d2f576354f54f4aea06bfc8e0ab37f6c9223ddcfdbb20380cd"
var swapRegistry = "e113fe553611142ea7054531abc8c89ad00ad2d4d386b264599b8010f17f53ff"

var testnet bool
var wallet_password string
var wallet_file = "wallet.db"
var daemon_address = "127.0.0.1:10102"

type Token struct {
	contract string
	decimals int
	bridgeFee uint64
	bridgeable bool
	swapable bool
}

type Pair struct {
	contract string
	fee uint64
	val1 uint64
	val2 uint64
	sharesOutstanding uint64
	adds uint64
	rems uint64
	swaps uint64
}

var tokens map[string]Token
var pairs map[string]Pair

var prompt_mutex sync.Mutex // prompt lock
var zerohash crypto.Hash

func parseOpt(param string) {
        s := strings.Split(param, "=")

        if s[0] == "--testnet" {
		testnet = true
        } else if len(s) > 1 && s[0] == "--daemon-address" {
                daemon_address = s[1]
        } else if len(s) > 1 && s[0] == "--wallet" {
		wallet_file = s[1]
        } else if len(s) > 1 && s[0] == "--password" {
                wallet_password = s[1]
        } else if s[0] == "--help" {
                fmt.Printf("wallet [--help] [--wallet=<wallet_file> | <private_key>] [--password=<wallet_password>] [--daemon-address=<127.0.0.1:10102>] [--testnet]\n")
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

func getTokens() {
	tokens = make(map[string]Token)

	// bridgeable tokens
	bridgeVars, bridgeValid := d.DeroGetVars(bridgeRegistry)
	if bridgeValid {
		for key, value := range bridgeVars {
			s := strings.Split(key, ":")
			if s[0] == "s" {
				var tok Token
				tok.contract = value.(string)
				tok.bridgeable = true

				fee_str, _ := d.DeroGetVar(value.(string), "bridgeFee")
				fee, _ := strconv.Atoi(fee_str)
				tok.bridgeFee = uint64(fee)

				dec_str, _ := d.DeroGetVar(value.(string), "decimals")
				tok.decimals, _ = strconv.Atoi(dec_str)

				tokens[s[1]] = tok
			}
		}
	}

	// swappable tokens
	swapVars, swapValid := d.DeroGetVars(swapRegistry)
	if (swapValid) {
		for key, value := range swapVars {
			s := strings.Split(key, ":")
			if s[0] == "t" && s[2] == "c" {
				var tok Token = tokens[s[1]]

				if tok == (Token{}) {
					tok.contract = value.(string)

					dec_str, _ := d.DeroGetVar(swapRegistry, "t:" + s[1] + ":d")
					tok.decimals, _ = strconv.Atoi(dec_str)
				}

				tok.swapable = true
				tokens[s[1]] = tok
			}
		}
	}
}

func getPairs() {
	pairs = make(map[string]Pair)
	swapVars, swapValid := d.DeroGetVars(swapRegistry)

	if (swapValid) {
		for key, value := range swapVars {
			s := strings.Split(key, ":")
			if s[0] == "p" {
				var pair Pair

				pair.contract = value.(string)

				fee_str, _ := d.DeroGetVar(pair.contract, "fee")
				fee, _ := strconv.Atoi(fee_str)
				pair.fee = uint64(fee)

				val1_str, _ := d.DeroGetVar(pair.contract, "val1")
				val1, _ := strconv.Atoi(val1_str)
				pair.val1 = uint64(val1)

				val2_str, _ := d.DeroGetVar(pair.contract, "val2")
				val2, _ := strconv.Atoi(val2_str)
				pair.val2 = uint64(val2)

				adds_str, _ := d.DeroGetVar(pair.contract, "adds")
				adds, _ := strconv.Atoi(adds_str)
				pair.adds = uint64(adds)

				rems_str, _ := d.DeroGetVar(pair.contract, "rems")
				rems, _ := strconv.Atoi(rems_str)
				pair.rems = uint64(rems)

				swaps_str, _ := d.DeroGetVar(pair.contract, "swaps")
				swaps, _ := strconv.Atoi(swaps_str)
				pair.swaps = uint64(swaps)

				shares_str, _ := d.DeroGetVar(pair.contract, "sharesOutstanding")
				shares, _ := strconv.Atoi(shares_str)
				pair.sharesOutstanding = uint64(shares)

				pairs[s[1] + ":" + s[2]] = pair
			}
		}
	}
}

func displayTokens() {
	getTokens()

	fmt.Printf("%-10s %-64s %-7s %-7s %18s\n\n", "TOKEN", "CONTRACT", "SWAP", "BRIDGE", "BALANCE")
	for key, tok := range tokens {
		var bal *big.Float

		bal = d.DeroFormatMoneyPrecision(d.DeroGetSCBal(tok.contract), tok.decimals)
		swap_check := "\u2716"
		bridge_check := "\u2716"
		if (tok.swapable) {
			swap_check = "\u2714"
		}
		if (tok.bridgeable) {
			bridge_check = "\u2714"
		}

		fmt.Printf("%-10s %64s    %s       %s    %18.9f\n", key, tok.contract, swap_check, bridge_check, bal)
	}

	fmt.Printf("\n")
}

func multDiv(a uint64, b uint64, c uint64) (uint64) {
	A := uint256.NewInt(a)
	B := uint256.NewInt(b)
	C := uint256.NewInt(c)

	A = A.Mul(A, B)
	C = A.Div(A, C)

	return C.Uint64()
}

func displayPairs() {
	getPairs()

	fmt.Printf("%-20s %37s %10s %37s\n\n", "PAIR", "TOTAL LIQUIDITY", "OWNERSHIP", "YOUR BALANCE")
	for key, pair := range pairs {
		if pair.sharesOutstanding > 0 {
			s := strings.Split(key, ":")
			tokenA := tokens[s[0]]
			tokenB := tokens[s[1]]

			myShares := d.DeroGetSCBal(pair.contract)
			ownerShip := float32(myShares) / float32(pair.sharesOutstanding) * 100.0;

			bal1_uint64 := multDiv(pair.val1, myShares, pair.sharesOutstanding)
			bal2_uint64 := multDiv(pair.val2, myShares, pair.sharesOutstanding)

			val1 := d.DeroFormatMoneyPrecision(pair.val1, tokenA.decimals)
			val2 := d.DeroFormatMoneyPrecision(pair.val2, tokenB.decimals)

			bal1 := d.DeroFormatMoneyPrecision(bal1_uint64, tokenA.decimals)
			bal2 := d.DeroFormatMoneyPrecision(bal2_uint64, tokenB.decimals)

			fmt.Printf("%-20s %18.9f/%18.9f %9.3f%% %18.9f/%18.9f\n", key, val1, val2, ownerShip, bal1, bal2)
		} else {
			fmt.Printf("%-20s %18.9f/%18.9f %9.3f%% %18.9f/%18.9f\n", key, 0.0, 0.0, 0.0, 0.0, 0.0)
		}
	}
}

func main() {
	walletOpts()

	var err error

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
	d.DeroWalletInit(daemon_address, false, wallet_file, wallet_password)

	fmt.Println("Building lookup tables...")
	if os.Getenv("USE_BIG_TABLE") != "" {
		d.DeroInitLookupTable(1, 1<<24);
	} else {
		d.DeroInitLookupTable(8, 1<<21);
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

func callBridge(scid string, eth_addr string, amount uint64, fee uint64) bool {
        var transfers []rpc.Transfer
        transfers = d.DeroBuildTransfers(transfers, scid, "", 0, amount)
        transfers = d.DeroBuildTransfers(transfers, zerohash.String(), "", 0, fee)

        var args rpc.Arguments
        args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "Bridge"})
        args = append(args, rpc.Argument {"eth_addr", rpc.DataString, eth_addr})

	txid, b := d.DeroCallSC(scid, transfers, args, 0)

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


	token := strings.ToUpper(words[0])
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

func bridge(words []string) {
	if len(words) != 3 {
		fmt.Println("Bridge requires 3 arguments:")
		printHelp()
		return
	}

	token := strings.ToUpper(words[0])
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

	fmt.Printf("Transfer %f %s to Ethereum address %s\n", d.DeroFormatMoneyPrecision(amount, tokens[token].decimals), token, words[1])
	fmt.Printf("Bridge fee %f DERO\n", d.DeroFormatMoneyPrecision(tokens[token].bridgeFee, 5))

	if askContinue() {
		callBridge(scid, words[1], amount, tokens[token].bridgeFee)
	}
}

func addLiquidity(words []string) {
	if len(words) != 3 {
		fmt.Println("addliquidity requires 3 arguments")
		printHelp()
		return
	}

	getPairs()

	words[0] = strings.ToUpper(words[0])
	words[2] = strings.ToUpper(words[2])
	pair := pairs[words[0]]

	if len(pair.contract) == 0 {
		fmt.Printf("pair '%s' is not registered\n", words[0])
		return
	}

	symbols := strings.Split(words[0], ":")
	tok1 := tokens[symbols[0]]
	tok2 := tokens[symbols[1]]

	if words[2] != symbols[0] && words[2] != symbols[1] {
		fmt.Printf("symbol %s is not a member of the swap pair %s\n", words[2], words[0])
		return
	}

	amt_float, err := strconv.ParseFloat(words[1], 64)
	if err != nil {
		fmt.Printf("cannot parse amount '%s'\n", words[1])
		return
	}

	if amt_float <= 0.0 {
		fmt.Println("amount must be > 0.0")
		return
	}

	outstanding_str, _ := d.DeroGetVar(pair.contract, "sharesOutstanding")
	outstanding, _ := strconv.Atoi(outstanding_str)

	var amt1, amt2 uint64
	var float1, float2 float64

	if words[2] == symbols[0] {
		float1 = amt_float
		amt1 = uint64(amt_float * math.Pow(10, float64(tok1.decimals)))
		if outstanding == 0 {
			fmt.Printf("Providing initial liquidity to pair %s with %f %s\n", words[0], amt_float, symbols[0])
			ans := promptInput("Enter equal value of " + symbols[1] + ": ")

			var err error
			float2, err = strconv.ParseFloat(ans, 64)
			if err != nil {
				fmt.Printf("cannot parse amount '%s'\n", ans)
				return
			}
			amt2 = uint64(float2 * math.Pow(10, float64(tok2.decimals)))
		} else {
			amt2 = multDiv(amt1, pair.val2, pair.val1)
			float2_big := d.DeroFormatMoneyPrecision(amt2, tok2.decimals)
			float2, _ = float2_big.Float64()
		}
	} else {
		float2 = amt_float
		amt2 = uint64(amt_float * math.Pow(10, float64(tok2.decimals)))
		if outstanding == 0 {
			fmt.Printf("Providing initial liquidity to pair %s with %f %s\n", words[0], amt_float, symbols[1])
			ans := promptInput("Enter equal value of " + symbols[0] + ": ")

			var err error
			float1, err = strconv.ParseFloat(ans, 64)
			if err != nil {
				fmt.Printf("cannot parse amount '%s'\n", ans)
				return
			}
			amt1 = uint64(float1 * math.Pow(10, float64(tok1.decimals)))
		} else {
			amt1 = multDiv(amt2, pair.val1, pair.val2)
			float1_big := d.DeroFormatMoneyPrecision(amt1, tok1.decimals)
			float1, _ = float1_big.Float64()
		}
	}

	bal1 := d.DeroGetSCBal(tok1.contract)
	bal2 := d.DeroGetSCBal(tok2.contract)

	if amt1 > bal1 {
		fmt.Printf("insufficient funds for %s\n", symbols[0])
		return
	}
	if amt2 > bal2 {
		fmt.Printf("insufficient funds for %s\n", symbols[1])
		return
	}


	fmt.Printf("Adding liquidity to pair %s: %f %s, %f %s\n", words[0], float1, symbols[0], float2, symbols[1])
	if askContinue() == false {
		fmt.Println("aborting...")
		return
	}

	var transfers []rpc.Transfer
	transfers = d.DeroBuildTransfers(transfers, tok1.contract, d.DeroGetRandomAddress(), 0, amt1)
	transfers = d.DeroBuildTransfers(transfers, tok2.contract, d.DeroGetRandomAddress(), 0, amt2)
	var args rpc.Arguments
	args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "AddLiquidity"})

	txid, b := d.DeroCallSC(pair.contract, transfers, args, 0)

	if !b {
		fmt.Println("Transaction failed.")
		return
	}

	fmt.Printf("Transaction submitted: txid = %s\n", txid)
}

func status(words []string) {
	if len(words) != 1 {
		fmt.Println("remliquidity requires 1 arguments")
		printHelp()
		return
	}

	getPairs()

	words[0] = strings.ToUpper(words[0])
	pair := pairs[words[0]]

	if len(pair.contract) == 0 {
		fmt.Printf("pair '%s' is not registered\n", words[0])
		return
	}

	fmt.Printf("%s contract: %s\n\n", words[0], pair.contract)

	symbols := strings.Split(words[0], ":")
	tokenA := tokens[symbols[0]]
	tokenB := tokens[symbols[1]]

	val1_float := float64(pair.val1) / math.Pow(10, float64(tokenA.decimals))
	val2_float := float64(pair.val2) / math.Pow(10, float64(tokenB.decimals))

	fmt.Printf("%s liquidity: %f\n", symbols[0], val1_float)
	fmt.Printf("%s liquidity: %f\n", symbols[1], val2_float)
	fmt.Println()

	if pair.sharesOutstanding > 0 {
		v2in1 := val1_float / val2_float
		v1in2 := val2_float / val1_float
		fmt.Printf("1.0 %s = %f %s\n", symbols[0], v1in2, symbols[1])
		fmt.Printf("1.0 %s = %f %s\n", symbols[1], v2in1, symbols[0])
	} else {
		fmt.Printf("1.0 %s = unknown %s\n", symbols[0], symbols[1])
		fmt.Printf("1.0 %s = unknown %s\n", symbols[1], symbols[0])
	}
	fmt.Println()

	fmt.Printf("Adds / Removes / Swaps (%d / %d / %d)\n", pair.adds, pair.rems, pair.swaps)
}

func swap(words []string) {
	if len(words) != 3 {
		fmt.Println("remliquidity requires 3 arguments")
		printHelp()
		return
	}

	getPairs()

	words[0] = strings.ToUpper(words[0])
	words[2] = strings.ToUpper(words[2])
	pair := pairs[words[0]]

	if len(pair.contract) == 0 {
		fmt.Printf("pair '%s' is not registered\n", words[0])
		return
	}

	symbols := strings.Split(words[0], ":")
	tokenA := tokens[symbols[0]]
	tokenB := tokens[symbols[1]]

	if words[2] != symbols[0] && words[2] != symbols[1] {
		fmt.Printf("symbol %s is not a member of the swap pair %s\n", words[2], words[0])
		return
	}

	amt_float, err := strconv.ParseFloat(words[1], 64)
	if err != nil {
		fmt.Printf("cannot parse amount '%s'\n", words[2])
		return
	}

	if amt_float <= 0.0 {
		fmt.Println("amount must be > 0.0")
		return
	}

	amt := uint64(amt_float * math.Pow(10, float64(tokens[words[2]].decimals)))
	bal := d.DeroGetSCBal(tokens[words[2]].contract)

	if amt > bal {
		fmt.Println("insufficient funds")
		return
	}

	var amt1, amt2 uint64
	var slip float64

	if words[2] == symbols[0] {
		amt1 = amt
		amt2 = 0
		amt_float := float64(amt) / math.Pow(10, float64(tokenA.decimals))
		result := float64(amt) * float64(pair.val2) / float64(pair.val1 + amt)
		result = result * float64(10000-pair.fee) / float64(10000)
		result_float := result / math.Pow(10, float64(tokenB.decimals))
		slip = 100.0 - (1.0 / (1.0 + float64(amt) / float64(pair.val1)) * 100.0)

		fmt.Printf("Swapping %f %s for %f %s fees included (with %f%% slippage)\n", amt_float, words[2], result_float, symbols[1], slip)
	} else {
		amt1 = 0
		amt2 = amt
		amt_float := float64(amt) / math.Pow(10, float64(tokenB.decimals))
		result := float64(amt) * float64(pair.val1) / float64(pair.val2 + amt)
		result = result * float64(10000-pair.fee) / float64(10000)
		result_float := result / math.Pow(10, float64(tokenA.decimals))
		slip = 100.0 - (1.0 / (1.0 + float64(amt) / float64(pair.val2)) * 100.0)

		fmt.Printf("Swapping %f %s for %f %s fees included (with %f%% slippage)\n", amt_float, words[2], result_float, symbols[0], slip)
	}

	if slip > 40.0 {
		fmt.Println("Slippage > 40%, aborting")
		return
	}

	if askContinue() == false {
		fmt.Println("aborting...")
		return
	}

	var transfers []rpc.Transfer
	transfers = d.DeroBuildTransfers(transfers, tokenA.contract, d.DeroGetRandomAddress(), 0, amt1)
	transfers = d.DeroBuildTransfers(transfers, tokenB.contract, d.DeroGetRandomAddress(), 0, amt2)
	var args rpc.Arguments
	args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "Swap"})

	txid, b := d.DeroCallSC(pair.contract, transfers, args, 0)

	if !b {
		fmt.Println("Transaction failed.")
		return
	}

	fmt.Printf("Transaction submitted: txid = %s\n", txid)
}

func remLiquidity(words []string) {
	if len(words) != 2 {
		fmt.Println("remliquidity requires 2 arguments")
		printHelp()
		return
	}

	getPairs()

	words[0] = strings.ToUpper(words[0])
	pair := pairs[words[0]]

	if len(pair.contract) == 0 {
		fmt.Printf("pair '%s' is not registered\n", words[0])
		return
	}

	percent, err := strconv.ParseFloat(words[1], 64)
	if err != nil {
		fmt.Printf("cannot parse percentage '%s'\n", words[2])
		return
	}

	if percent <= 0.0 || percent > 100.0 {
		fmt.Println("amount must be > 0.0 and <= 100.0")
		return
	}

	myShares := d.DeroGetSCBal(pair.contract)
	if myShares <= 0 {
		fmt.Println("You own no liquidity of pair %s\n", words[0])
		return
	}

	symbols := strings.Split(words[0], ":")
	tokenA := tokens[symbols[0]]
	tokenB := tokens[symbols[1]]

	bal1_uint64 := multDiv(pair.val1, myShares,  pair.sharesOutstanding)
	bal2_uint64 := multDiv(pair.val2, myShares,  pair.sharesOutstanding)

	bal1_float := float64(bal1_uint64) / math.Pow(10, float64(tokenA.decimals))
	bal2_float := float64(bal2_uint64) / math.Pow(10, float64(tokenB.decimals))

	rem1_float := bal1_float * percent / 100.0
	rem2_float := bal2_float * percent / 100.0

	remShares := uint64(float64(myShares) * percent / 100.0)

	fmt.Printf("Your liquidity for pair %s is %f %s, %f %s\n", words[0], bal1_float, symbols[0], bal2_float, symbols[1])
	fmt.Printf("Remove %f%% (%f %s, %f %s)\n", percent, rem1_float, symbols[0], rem2_float, symbols[1])

	if askContinue() == false {
		fmt.Println("aborting...")
		return
	}

	var transfers []rpc.Transfer
	transfers = d.DeroBuildTransfers(transfers, pair.contract, d.DeroGetRandomAddress(), 0, remShares)
	var args rpc.Arguments
	args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "RemoveLiquidity"})

	txid, b := d.DeroCallSC(pair.contract, transfers, args, 0)

	if !b {
		fmt.Println("Transaction failed.")
		return
	}

	fmt.Printf("Transaction submitted: txid = %s\n", txid)
}

func swapRegToken(words []string) {
	if len(words) != 1 {
		fmt.Println("swapregtoken requires 1 arguments")
		return
	}

	sym, sym_valid := d.DeroGetVar(words[0], "symbol")
	if sym == "" {
		fmt.Printf("Cannot find SCID '%s'\n", words[0])
		return
	}

	symbol, _ := hex.DecodeString(sym)

	if tokens[string(symbol)].swapable {
		fmt.Println("token already registered")
		return
	}

	dec_str, dec_valid := d.DeroGetVar(words[0], "decimals")
	dec, _ := strconv.Atoi(dec_str)

	if sym_valid && dec_valid {
		var transfers []rpc.Transfer
		var args rpc.Arguments
		args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "RegisterToken"})
		args = append(args, rpc.Argument {"symbol", rpc.DataString, string(symbol)})
		args = append(args, rpc.Argument {"scid", rpc.DataString, words[0]})
		args = append(args, rpc.Argument {"decimals", rpc.DataUint64, uint64(dec)})

		txid, b := d.DeroCallSC(swapRegistry, transfers, args, 0)

		if !b {
			fmt.Println("Transaction failed.")
			return
		}

		fmt.Printf("Transaction submitted: txid = %s\n", txid)
	} else {
		fmt.Println("Cannot find valid token")
	}

}

func swapRegPair(words []string) {
	if len(words) != 2 {
		fmt.Println("swapregpair requires 2 arguments")
		return
	}

	tokenA := strings.ToUpper(words[0])
	tokenB := strings.ToUpper(words[1])

	if tokens[tokenA].contract == "" || tokens[tokenB].contract == "" {
		fmt.Println("both tokens must be registered")
		return
	}

	pair1, _ := d.DeroGetVar(swapRegistry, tokenA + ":" + tokenB)
	pair2, _ := d.DeroGetVar(swapRegistry, tokenB + ":" + tokenA)

	if len(pair1) > 0 || len(pair2) > 0 {
		fmt.Println("pair already registered")
		return
	}

	fee_str, _ := d.DeroGetVar(swapRegistry, "fee")

        src, err := ioutil.ReadFile("ps_pair.bas")
        if err != nil {
                fmt.Println("Cannot open ps_swap.bas: %s", err)
		return
        }

        src = bytes.Replace(src, []byte("${asset1}"), []byte(tokens[tokenA].contract), -1)
        src = bytes.Replace(src, []byte("${asset2}"), []byte(tokens[tokenB].contract), -1)
        src = bytes.Replace(src, []byte("${fee}"), []byte(fee_str), -1)

        txid, res := d.DeroDeploySC(src)

	if !res {
		fmt.Println("Transaction failed.")
		return
	}

	fmt.Printf("Install pair contract transaction submitted: txid = %s\n", txid)
	fmt.Printf("Waiting")

	var result string
	for true {
		result = d.DeroConfirmTx(txid, 0)
		if result != "pending" {
			break
		}
		fmt.Printf(".")
		time.Sleep(5*time.Second)
	}
	fmt.Printf("\n")

	if result != "confirmed" {
		fmt.Println("Txn failed")
		return
	}

	var transfers []rpc.Transfer
	var args rpc.Arguments
	args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "RegisterPair"})
	args = append(args, rpc.Argument {"tokenA", rpc.DataString, tokenA})
	args = append(args, rpc.Argument {"tokenB", rpc.DataString, tokenB})
	args = append(args, rpc.Argument {"scid", rpc.DataString, txid})

	txid, b := d.DeroCallSC(swapRegistry, transfers, args, 0)

	if !b {
		fmt.Println("Transaction failed.")
		return
	}

	fmt.Printf("Pair registration transaction submitted: txid = %s\n", txid)
}

func printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("")
	fmt.Println("help")
	fmt.Println("quit")
	fmt.Println("address")
	fmt.Println("bridge <token> <eth_address> <amount>")
	fmt.Println("transfer <token> <dero_wallet> <amount>")
	fmt.Println("balance")
	fmt.Println("pairs")
	fmt.Println("addliqudity <pair> <amount> <symbol>")
	fmt.Println("remliqudity <pair> <percent>")
	fmt.Println("swap <pair> <symbol> <amount>")
	fmt.Println("status <pair>")
}

var completer = readline.NewPrefixCompleter(
	readline.PcItem("mode",
		readline.PcItem("vi"),
		readline.PcItem("emacs"),
	),
	readline.PcItem("balance"),
	readline.PcItem("address"),
	readline.PcItem("bye"),
	readline.PcItem("exit"),
	readline.PcItem("quit"),
	readline.PcItem("help"),
	readline.PcItem("balance"),
	readline.PcItem("transfer"),
	readline.PcItem("bridge"),
	readline.PcItem("pairs"),
	readline.PcItem("addliquidity"),
	readline.PcItem("remliquidity"),
	readline.PcItem("swap"),
	readline.PcItem("status"),
)

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

var l *readline.Instance

func promptInput(prompt string) (string) {
	prompt_mutex.Lock()
	l.SetPrompt(prompt)
	str, err := l.Readline()
	prompt_mutex.Unlock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	return str
}

func askContinue() bool {
	str := promptInput("Continue (N/y) ? ")

	if len(str) > 0 {
		if  str[0:1] == "y" || str[0:1] == "Y" {
			return true
		}
	}

	fmt.Println("Cancelled.")
	return false
}

func commandLoop() {
	go update_prompt()

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		words := strings.Fields(line)

		if len(words) > 0 {
			switch strings.ToLower(words[0]) {
			case "mode":
				if len(words) > 1 {
					switch words[1] + "" {
					case "vi":
						l.SetVimMode(true)
					case "emacs":
						l.SetVimMode(false)
					default:
						println("invalid mode:", line[5:])
					}
				}
			case "help", "?":
				printHelp()
			case "address":
				fmt.Printf("Wallet address %s\n", d.DeroGetAddress())
			case "bridge":
				bridge(words[1:])
			case "transfer":
				transfer(words[1:])
			case "balance":
				displayTokens()
			case "pairs":
				displayPairs()
			case "addliquidity":
				addLiquidity(words[1:])
			case "remliquidity":
				remLiquidity(words[1:])
			case "swap":
				swap(words[1:])
			case "status":
				status(words[1:])
			case "swapregtoken":
				swapRegToken(words[1:])
			case "swapregpair":
				swapRegPair(words[1:])
			case "exit", "quit", "q", "bye":
				goto exit;
			case "":
			default:
				fmt.Println("unknown command: ", strconv.Quote(line))
			}
		}
	}
exit:
}

func update_prompt() {
	for {
		prompt_mutex.Lock()

		dh := d.DeroGetHeight()
		wh := d.DeroGetWalletHeight()

		p := fmt.Sprintf("%d/%d > ", wh, dh)
		l.SetPrompt(p)
		l.Refresh()

		prompt_mutex.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}
