package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	d "github.com/deroholic/derogo"
	"github.com/deroproject/derohe/rpc"
	"github.com/yourbasic/graph"
)

var swapRegistry string

type Token struct {
	n          int
	contract   string
	decimals   int
	bridgeFee  uint64
	bridgeable bool
	swapable   bool
}

type Pair struct {
	contract          string
	fee               uint64
	val1              uint64
	val2              uint64
	sharesOutstanding uint64
	adds              uint64
	rems              uint64
	swaps             uint64
}

var tokens map[string]Token
var pairs map[string]Pair
var tokenList []string
var tokenGraph *graph.Mutable

func getTokens() {
	tokens = make(map[string]Token)
	n := 0

	// bridgeable tokens
	bridgeVars, bridgeValid := d.DeroGetVars(bridgeRegistry)
	if bridgeValid {
		for key, value := range bridgeVars {
			s := strings.Split(key, ":")
			if s[0] == "s" {
				var tok Token

				tok.n = n
				n++
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
	if swapValid {
		for key, value := range swapVars {
			s := strings.Split(key, ":")
			if s[0] == "t" && s[2] == "c" {
				var tok Token = tokens[s[1]]

				if tok == (Token{}) {
					tok.n = n
					n++
					tok.contract = value.(string)

					dec_str, _ := d.DeroGetVar(swapRegistry, "t:"+s[1]+":d")
					tok.decimals, _ = strconv.Atoi(dec_str)
					tokenList = append(tokenList, s[1])
				}

				tok.swapable = true
				tokens[s[1]] = tok
			}
		}
	}

	// build list
	tokenList = make([]string, len(tokens))
	for k, v := range tokens {
		tokenList[v.n] = k
	}
}

func getPairs() {
	pairs = make(map[string]Pair)
	tokenGraph = graph.New(len(tokens))
	swapVars, swapValid := d.DeroGetVars(swapRegistry)

	if swapValid {
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

				pairs[s[1]+":"+s[2]] = pair

				if pair.val1 > 0 {
					tok1 := tokens[s[1]]
					tok2 := tokens[s[2]]

					val1_float := float64(pair.val1) / math.Pow(10, float64(tok1.decimals))
					val2_float := float64(pair.val2) / math.Pow(10, float64(tok2.decimals))

					tokenGraph.AddCost(tok1.n, tok2.n, int64(val2_float/val1_float*math.Pow(10, 7)))
					tokenGraph.AddCost(tok2.n, tok1.n, int64(val1_float/val2_float*math.Pow(10, 7)))
				}
			}
		}
	}
}

func displayTokens() {
	getTokens()

	fmt.Printf("%-10s %-64s %-7s %-7s %18s\n\n", "TOKEN", "CONTRACT", "SWAP", "BRIDGE", "BALANCE")
	for key, tok := range tokens {
		bal := d.DeroFormatMoneyPrecision(d.DeroGetSCBal(tok.contract), tok.decimals)
		swap_check := "\u2716"
		bridge_check := "\u2716"
		if tok.swapable {
			swap_check = "\u2714"
		}
		if tok.bridgeable {
			bridge_check = "\u2714"
		}

		fmt.Printf("%-10s %64s    %s       %s    %18.7f\n", key, tok.contract, swap_check, bridge_check, bal)
	}

	fmt.Printf("\n")
}

func displayPairs() {
	tlv := float64(0)
	getPairs()

	fmt.Printf("%-20s %36s %10s %36s\n\n", "PAIR", "TOTAL LIQUIDITY", "OWNERSHIP", "YOUR BALANCE")
	for key, pair := range pairs {
		if pair.sharesOutstanding > 0 {
			s := strings.Split(key, ":")
			tokenA := tokens[s[0]]
			tokenB := tokens[s[1]]

			myShares := d.DeroGetSCBal(pair.contract)
			ownerShip := float32(myShares) / float32(pair.sharesOutstanding) * 100.0

			bal1_uint64 := multDiv(pair.val1, myShares, pair.sharesOutstanding)
			bal2_uint64 := multDiv(pair.val2, myShares, pair.sharesOutstanding)

			val1 := d.DeroFormatMoneyPrecision(pair.val1, tokenA.decimals)
			val2 := d.DeroFormatMoneyPrecision(pair.val2, tokenB.decimals)

			bal1 := d.DeroFormatMoneyPrecision(bal1_uint64, tokenA.decimals)
			bal2 := d.DeroFormatMoneyPrecision(bal2_uint64, tokenB.decimals)

			ratio1, _ := conversion(s[0], "DUSDT")
			ratio2, _ := conversion(s[1], "DUSDT")

			val1_float, _ := val1.Float64()
			val2_float, _ := val2.Float64()

			tlv += val1_float * ratio1
			tlv += val2_float * ratio2

			fmt.Printf("%-20s %18.7f/%18.7f %7.3f%% %18.7f/%18.7f\n", key, val1, val2, ownerShip, bal1, bal2)
		} else {
			fmt.Printf("%-20s %18.7f/%18.7f %7.3f%% %18.7f/%18.7f\n", key, 0.0, 0.0, 0.0, 0.0, 0.0)
		}
	}

	fmt.Printf("\n")
	fmt.Printf("TLV: %.2f USDT\n", tlv)
}

func conversion(sym1 string, sym2 string) (ratio float64, path string) {
	if tokens[sym1] == (Token{}) || tokens[sym2] == (Token{}) {
		return
	}

	n1 := tokens[sym1].n
	n2 := tokens[sym2].n

	p, d := graph.ShortestPath(tokenGraph, n1, n2)
	if d == -1 {
		return
	}

	ratio = float64(1.0)

	n := n1
	path = sym1

	for i := 1; i < len(p); i++ {
		ratio *= (float64(tokenGraph.Cost(n, p[i])) / math.Pow(10, 7))
		path += " => " + tokenList[p[i]]
		n = p[i]
	}

	return
}

func quote(words []string) {
	getPairs()

	if len(words) != 2 {
		fmt.Println("quote requires 2 arguments")
		printHelp()
		return
	}

	ratio, path := conversion(words[0], words[1])
	if len(path) == 0 {
		fmt.Printf("Cannot find path between '%s' and '%s'\n", words[0], words[1])
		return
	}

	fmt.Printf("%s\n", path)
	fmt.Printf("1 %s == %0.7f %s\n", words[0], ratio, words[1])
}

func status(words []string) {
	if len(words) != 1 {
		fmt.Println("status requires 1 arguments")
		printHelp()
		return
	}

	getPairs()

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
	fmt.Printf("Bridge fee %4.2f%%\n", float64(pair.fee)/100.0)
	fmt.Println()

	fmt.Printf("Adds / Removes / Swaps (%d / %d / %d)\n", pair.adds, pair.rems, pair.swaps)
}

func swap(words []string) {
	if len(words) != 3 {
		fmt.Println("swap requires 3 arguments")
		printHelp()
		return
	}

	getPairs()

	pair := pairs[words[0]]

	if len(pair.contract) == 0 {
		fmt.Printf("pair '%s' is not registered\n", words[0])
		return
	}

	if pair.val1 == 0 || pair.val2 == 0 {
		fmt.Println("pair has no liquidity")
		return
	}

	symbols := strings.Split(words[0], ":")
	tokenA := tokens[symbols[0]]
	tokenB := tokens[symbols[1]]

	if words[2] != symbols[0] && words[2] != symbols[1] {
		fmt.Printf("symbol %s is not a member of the swap pair %s\n", words[2], words[0])
		return
	}

	bal := d.DeroGetSCBal(tokens[words[2]].contract)

	var amt_float float64
	var err error

	if strings.ToLower(words[1]) == "max" {
		amt_float = float64(bal) / math.Pow(10, float64(tokens[words[2]].decimals))
	} else {
		amt_float, err = strconv.ParseFloat(words[1], 64)
		if err != nil {
			fmt.Printf("cannot parse amount '%s'\n", words[2])
			return
		}
	}

	if amt_float <= 0.0 {
		fmt.Println("amount must be > 0.0")
		return
	}

	amt := uint64(amt_float * math.Pow(10, float64(tokens[words[2]].decimals)))

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
		result := float64(amt) * float64(pair.val2) / float64(pair.val1+amt)
		result = result * float64(10000-pair.fee) / float64(10000)
		result_float := result / math.Pow(10, float64(tokenB.decimals))
		slip = 100.0 - (1.0 / (1.0 + float64(amt)/float64(pair.val1)) * 100.0)

		fmt.Printf("Swapping %f %s for %f %s fees included (with %f%% slippage)\n", amt_float, words[2], result_float, symbols[1], slip)
	} else {
		amt1 = 0
		amt2 = amt
		amt_float := float64(amt) / math.Pow(10, float64(tokenB.decimals))
		result := float64(amt) * float64(pair.val1) / float64(pair.val2+amt)
		result = result * float64(10000-pair.fee) / float64(10000)
		result_float := result / math.Pow(10, float64(tokenA.decimals))
		slip = 100.0 - (1.0 / (1.0 + float64(amt)/float64(pair.val2)) * 100.0)

		fmt.Printf("Swapping %f %s for %f %s fees included (with %f%% slippage)\n", amt_float, words[2], result_float, symbols[0], slip)
	}

	if slip > 40.0 {
		fmt.Println("Slippage > 40%, aborting")
		return
	}

	if !askContinue() {
		fmt.Println("aborting...")
		return
	}

	var transfers []rpc.Transfer
	if amt1 > 0 {
		transfers = d.DeroBuildTransfers(transfers, tokenA.contract, d.DeroGetRandomAddress(), 0, amt1)
	}
	if amt2 > 0 {
		transfers = d.DeroBuildTransfers(transfers, tokenB.contract, d.DeroGetRandomAddress(), 0, amt2)
	}
	var args rpc.Arguments
	args = append(args, rpc.Argument{"entrypoint", rpc.DataString, "Swap"})

	txid, b := d.DeroSafeCallSC(pair.contract, transfers, args)

	if !b {
		fmt.Println("Transaction failed.")
		return
	}

	fmt.Printf("Transaction submitted: txid = %s\n", txid)
}

func addLiquidity(words []string) {
	if len(words) != 3 {
		fmt.Println("addliquidity requires 3 arguments")
		printHelp()
		return
	}

	getPairs()

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

	var amt_float float64
	var err error

	if strings.ToLower(words[1]) == "max" {
		bal := d.DeroGetSCBal(tokens[words[2]].contract)
		amt_float = float64(bal) / math.Pow(10, float64(tokens[words[2]].decimals))
	} else {
		amt_float, err = strconv.ParseFloat(words[1], 64)
		if err != nil {
			fmt.Printf("cannot parse amount '%s'\n", words[1])
			return
		}
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
	if !askContinue() {
		fmt.Println("aborting...")
		return
	}

	var transfers []rpc.Transfer
	transfers = d.DeroBuildTransfers(transfers, tok1.contract, d.DeroGetRandomAddress(), 0, amt1)
	transfers = d.DeroBuildTransfers(transfers, tok2.contract, d.DeroGetRandomAddress(), 0, amt2)
	var args rpc.Arguments
	args = append(args, rpc.Argument{"entrypoint", rpc.DataString, "AddLiquidity"})

	txid, b := d.DeroSafeCallSC(pair.contract, transfers, args)

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
		fmt.Printf("You own no liquidity of pair %s\n", words[0])
		return
	}

	symbols := strings.Split(words[0], ":")
	tokenA := tokens[symbols[0]]
	tokenB := tokens[symbols[1]]

	bal1_uint64 := multDiv(pair.val1, myShares, pair.sharesOutstanding)
	bal2_uint64 := multDiv(pair.val2, myShares, pair.sharesOutstanding)

	bal1_float := float64(bal1_uint64) / math.Pow(10, float64(tokenA.decimals))
	bal2_float := float64(bal2_uint64) / math.Pow(10, float64(tokenB.decimals))

	rem1_float := bal1_float * percent / 100.0
	rem2_float := bal2_float * percent / 100.0

	remShares := uint64(float64(myShares) * percent / 100.0)

	fmt.Printf("Your liquidity for pair %s is %f %s, %f %s\n", words[0], bal1_float, symbols[0], bal2_float, symbols[1])
	fmt.Printf("Remove %f%% (%f %s, %f %s)\n", percent, rem1_float, symbols[0], rem2_float, symbols[1])

	if !askContinue() {
		fmt.Println("aborting...")
		return
	}

	var transfers []rpc.Transfer
	transfers = d.DeroBuildTransfers(transfers, pair.contract, d.DeroGetRandomAddress(), 0, remShares)
	var args rpc.Arguments
	args = append(args, rpc.Argument{"entrypoint", rpc.DataString, "RemoveLiquidity"})

	txid, b := d.DeroSafeCallSC(pair.contract, transfers, args)

	if !b {
		fmt.Println("Transaction failed.")
		return
	}

	fmt.Printf("Transaction submitted: txid = %s\n", txid)
}
