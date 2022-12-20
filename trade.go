package main

import (
	"fmt"
	"strings"
	"strconv"
	"encoding/hex"
	"math"
	"sort"
	"time"

	d "github.com/deroholic/derogo"
	"github.com/deroproject/derohe/rpc"
)

type Hist struct {
        timestamp uint64
        v1 uint64
        v2 uint64
}

type Order struct {
        order uint64
        n uint64
        o1, o2 uint64
        v1, v2 uint64
        t string
        s string
}

type TradePair struct {
        contract string
        fee uint64
        buy_root uint64
        sell_root uint64
        o1, o2 uint64
        hist []Hist
        orders map[uint64]Order
        prices map[uint64]uint64
}

var tradePairs map[string]TradePair

func getTradePairs() {
        tradePairs = make(map[string]TradePair)
	tradeVars, tradeValid := d.DeroGetVars(swapRegistry)

        if (tradeValid) {
                for key, value := range tradeVars {
                        s := strings.Split(key, ":")
                        if s[0] == "c" {
                                var tradePair TradePair

                                tradePair.contract = value.(string)
                                tradePair.orders = make(map[uint64]Order)
                                tradePair.prices = make(map[uint64]uint64)

                                pairVars, pairVars_valid := d.DeroGetVars(tradePair.contract)
                                if pairVars_valid {
                                        for k, v := range pairVars {
                                                sk := strings.Split(k, ":")

                                                var v_64, k1_64 uint64
                                                var v_str string

                                                switch t := v.(type) {
                                                        case string:
                                                                v_byte, _ := hex.DecodeString(t)
                                                                v_str = string(v_byte)
                                                        case float64:
                                                                v_64 = uint64(t)
                                                }

                                                if len(sk) > 1 {
                                                        k1_int, _ := strconv.Atoi(string(sk[1]))
                                                        k1_64 = uint64(k1_int)
                                                }
                                                order := tradePair.orders[k1_64]

                                                switch sk[0] {
                                                case "fee":
                                                        tradePair.fee = v_64
                                                case "buy":
                                                        tradePair.buy_root = v_64
                                                case "sell":
                                                        tradePair.sell_root = v_64
                                                case "o1":
                                                        tradePair.o1 = v_64
                                                case "o2":
                                                        tradePair.o2 = v_64
                                                case "h":
                                                        h := strings.Split(string(v_str), ":")
                                                        h0, _ := strconv.Atoi(h[1])
                                                        h1, _ := strconv.Atoi(h[2])
                                                        h2, _ := strconv.Atoi(h[3])
                                                        tradePair.hist =  append(tradePair.hist, Hist{uint64(h0), uint64(h1), uint64(h2)})
                                                case "tn":
                                                        order.n = v_64
                                                        order.order = k1_64
                                                        tradePair.orders[k1_64] = order
                                                case "to1":
                                                        order.o1 = v_64
                                                        tradePair.orders[k1_64] = order
                                                case "to2":
                                                        order.o2 = v_64
                                                        tradePair.orders[k1_64] = order
                                                case "tv1":
                                                        order.v1 = v_64
                                                        tradePair.orders[k1_64] = order
                                                case "tv2":
                                                        order.v2 = v_64
                                                        tradePair.orders[k1_64] = order
                                                case "tt":
                                                        order.t = v_str
                                                        tradePair.orders[k1_64] = order
                                                case "ts":
                                                        order.s = v.(string)
                                                        tradePair.orders[k1_64] = order
                                                case "nk":
                                                        tradePair.prices[k1_64] = v_64
                                                }

                                        }
                                }

                                tradePairs[s[1] + ":" + s[2]] = tradePair
                        }
                }
        }
}

func tradeSell(words []string) {
        if len(words) != 3 {
                fmt.Println("sell requires 3 arguments")
                tradeHelp()
                return
        }

        getTradePairs()

        pair := tradePairs[words[0]]

        if len(pair.contract) == 0 {
                fmt.Printf("pair '%s' is not registered\n", words[0])
                return
        }

        symbols := strings.Split(words[0], ":")
        tokenA := tokens[symbols[0]]

        amt_float, err := strconv.ParseFloat(words[1], 64)
        if err != nil {
                fmt.Printf("cannot parse amount '%s'\n", words[1])
                return
        }

        price_float, err := strconv.ParseFloat(words[2], 64)
        if err != nil {
                fmt.Printf("cannot parse amount '%s'\n", words[2])
                return
        }

        if amt_float <= 0.0 || price_float <= 0.0 {
                fmt.Println("amounts must be > 0.0")
                return
        }

        amt_64 := uint64(amt_float * math.Pow10(tokenA.decimals))
        price_64 := uint64(price_float * 10000000.0)

        var transfers []rpc.Transfer
        transfers = d.DeroBuildTransfers(transfers, tokenA.contract, d.DeroGetRandomAddress(), 0, amt_64)

        var args rpc.Arguments
        args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "Sell"})
        args = append(args, rpc.Argument {"price", rpc.DataUint64, price_64})

        ge, ge_valid := d.DeroEstimateGas(pair.contract, transfers, args, 0)
        if !ge_valid || ge.Status != "OK" {
                fmt.Printf("Error: %+s\n", ge.Status)
                return
        }

        fmt.Printf("Sell limit order %f %s @ %f %s\n", amt_float, symbols[0], price_float, symbols[1])
        if askContinue() == false {
                fmt.Println("aborting...")
                return
        }

        txid, b := d.DeroSafeCallSC(pair.contract, transfers, args)
//      txid, b := d.DeroCallSC(pair.contract, transfers, args, 300)

        if !b {
                fmt.Println("Transaction failed.")
                return
        }

        fmt.Printf("Transaction submitted: txid = %s, fees = %d\n", txid, ge.GasStorage)
}

func tradeBuy(words []string) {
        if len(words) != 3 {
                fmt.Println("buy requires 3 arguments")
                tradeHelp()
                return
        }

        getTradePairs()

        pair := tradePairs[words[0]]

        if len(pair.contract) == 0 {
                fmt.Printf("pair '%s' is not registered\n", words[0])
                return
        }

        symbols := strings.Split(words[0], ":")
        tokenA := tokens[symbols[0]]
        tokenB := tokens[symbols[1]]

        amt_float, err := strconv.ParseFloat(words[1], 64)
        if err != nil {
                fmt.Printf("cannot parse amount '%s'\n", words[1])
                return
        }

        price_float, err := strconv.ParseFloat(words[2], 64)
        if err != nil {
                fmt.Printf("cannot parse amount '%s'\n", words[2])
                return
        }

        if amt_float <= 0.0 || price_float <= 0.0 {
                fmt.Println("amounts must be > 0.0")
                return
        }

        amt1_64 := uint64(amt_float * math.Pow10(tokenA.decimals))
        price_64 := uint64(price_float * 10000000.0)
        amt2_64 := uint64(amt_float * price_float * math.Pow10(tokenB.decimals)) + 1

        var transfers []rpc.Transfer
        transfers = d.DeroBuildTransfers(transfers, tokenB.contract, d.DeroGetRandomAddress(), 0, amt2_64)

        var args rpc.Arguments
        args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "Buy"})
        args = append(args, rpc.Argument {"o1", rpc.DataUint64, amt1_64})
        args = append(args, rpc.Argument {"price", rpc.DataUint64, price_64})

        ge, ge_valid := d.DeroEstimateGas(pair.contract, transfers, args, 0)
        if !ge_valid || ge.Status != "OK" {
                fmt.Printf("Error: %+s\n", ge.Status)
                return
        }

        fmt.Printf("Buy limit order %f %s @ %f %s\n", amt_float, symbols[0], price_float, symbols[1])
        if askContinue() == false {
                fmt.Println("aborting...")
                return
        }

        txid, b := d.DeroSafeCallSC(pair.contract, transfers, args)
//      txid, b := d.DeroCallSC(pair.contract, transfers, args, 300)

        if !b {
                fmt.Println("Transaction failed.")
                return
        }

        fmt.Printf("Transaction submitted: txid = %s, fees = %d\n", txid, ge.GasStorage)
}

func tradeCancel(words []string) {
        if len(words) != 2 {
                fmt.Println("cancel requires 2 arguments")
                tradeHelp()
                return
        }

        getTradePairs()

        pair := tradePairs[words[0]]

        if len(pair.contract) == 0 {
                fmt.Printf("pair '%s' is not registered\n", words[0])
                return
        }

        tx, err := strconv.Atoi(words[1])
        if err != nil {
                fmt.Println("invalid transaction number")
                return
        }

        var transfers []rpc.Transfer
        var args rpc.Arguments
        args = append(args, rpc.Argument {"entrypoint", rpc.DataString, "Cancel"})
        args = append(args, rpc.Argument {"tx", rpc.DataUint64, uint64(tx)})

        ge, ge_valid := d.DeroEstimateGas(pair.contract, transfers, args, 0)
        if !ge_valid || ge.Status != "OK" {
                fmt.Printf("Error: %+s\n", ge.Status)
                return
        }

        fmt.Printf("Cancel %s order %d\n", words[0], tx)
        if askContinue() == false {
                fmt.Println("aborting...")
                return
        }

        txid, b := d.DeroSafeCallSC(pair.contract, transfers, args)

        if !b {
                fmt.Println("Transaction failed.")
                return
        }

        fmt.Printf("Transaction submitted: txid = %s, fees = %d\n", txid, ge.GasStorage)
}

type ordSum struct {
        price uint64
        amount uint64
        total uint64
}

func tradeBookSum(orders []ordSum, dir string) (out []ordSum) {
        if dir == "fwd" {
                sort.Slice(orders, func(i, j int) bool { return orders[i].price < orders[j].price })
        } else {
                sort.Slice(orders, func(i, j int) bool { return orders[i].price > orders[j].price })
        }

        if len(orders) < 1 {
                return
        }

        p := orders[0].price
        a := uint64(0)
        t := uint64(0)
        for _, o := range orders {
                if o.price != p {
                        out = append(out, ordSum{p, a, t})
                        a = 0
                }
                p = o.price
                a += o.amount
                t += o.amount
        }
        out = append(out, ordSum{p, a, t})

        return
}

func tradeBook(words []string) {
        if len(words) != 1 {
                fmt.Println("book requires 1 arguments")
                tradeHelp()
                return
        }

        getTradePairs()

        pair := tradePairs[words[0]]

        if len(pair.contract) == 0 {
                fmt.Printf("pair '%s' is not registered\n", words[0])
                return
        }

        symbols := strings.Split(words[0], ":")

        // get last trade
        sort.Slice(pair.hist, func(i, j int) bool { return pair.hist[i].timestamp > pair.hist[j].timestamp })

        var sell []ordSum
        var buy []ordSum
        for _, order := range pair.orders {
                rec := ordSum{pair.prices[order.n], order.o1, 0}
                if order.t == "sell" {
                        sell = append(sell, rec)
                } else {
                        buy = append(buy, rec)
                }
        }

        buy = tradeBookSum(buy, "rev")
        sell = tradeBookSum(sell, "fwd")

        fmt.Printf("TYPE %19s %19s %19s\n", "PRICE", "AMOUNT", "TOTAL")
        fmt.Printf("\n")

        if len(sell) > 0 {
                for i := len(sell)-1; i >= 0; i-- {
                        fmt.Printf("SELL %19f %19f %19f\n",
                                float64(sell[i].price) / 10000000.0,
                                float64(sell[i].amount) / math.Pow10(tokens[symbols[0]].decimals),
                                float64(sell[i].total) / math.Pow10(tokens[symbols[0]].decimals))
                }
        }

	if len(pair.hist) > 0 {
		fmt.Printf("\n")
		fmt.Printf("LAST %19f %19f\n",
			float64(pair.hist[0].v2) / 10000000.0,
			float64(pair.hist[0].v1) / math.Pow10(tokens[symbols[0]].decimals))
		fmt.Printf("\n")
	}

	if len(buy) > 0 {
		for i := 0; i < len(buy); i++ {
			fmt.Printf("BUY  %19f %19f %19f\n",
				float64(buy[i].price) / 10000000.0,
				float64(buy[i].amount) / math.Pow10(tokens[symbols[0]].decimals),
				float64(buy[i].total) / math.Pow10(tokens[symbols[0]].decimals))
		}
	}
}

func tradeOrders(words []string) {
        if len(words) != 1 {
                fmt.Println("orders requires 1 arguments")
                tradeHelp()
                return
        }

        getTradePairs()

        pair := tradePairs[words[0]]

        if len(pair.contract) == 0 {
                fmt.Printf("pair '%s' is not registered\n", words[0])
                return
        }

        symbols := strings.Split(words[0], ":")

        pubKey := hex.EncodeToString(d.DeroGetPub())

        keys := make([]uint64, 0, len(pair.orders))
        for k := range pair.orders {
                if pair.orders[k].s == pubKey {
                        keys = append(keys, k)
                }
        }
        sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

        fmt.Printf("Open orders:\n\n")
        fmt.Printf("%5s %4s %19s %19s %19s %19s\n\n", "ORDER", "TYPE", "PRICE", "AMOUNT", "FILLED", "UNFILLED")
        for _, k := range keys {
                o := pair.orders[uint64(k)]
                o1 := float64(o.o1) / math.Pow10(tokens[symbols[0]].decimals)
                v1 := float64(o.v1) / math.Pow10(tokens[symbols[0]].decimals)
                price := float64(pair.prices[o.n]) / 10000000.0
                fmt.Printf("%5d %4s %19f %19f %19f %19f\n", o.order, o.t, price, o1, o1-v1, v1)
        }
}

func tradeHistory(words []string) {
        if len(words) != 1 {
                fmt.Println("history requires 1 arguments")
                tradeHelp()
                return
        }

        getTradePairs()

        pair := tradePairs[words[0]]

        if len(pair.contract) == 0 {
                fmt.Printf("pair '%s' is not registered\n", words[0])
                return
        }

        symbols := strings.Split(words[0], ":")

        sort.Slice(pair.hist, func(i, j int) bool { return pair.hist[i].timestamp > pair.hist[j].timestamp })

        fmt.Printf("Trade History:\n\n")
        fmt.Printf("%-29s %19s %19s\n\n", "TIME", symbols[0], "PRICE")

        for _, h := range pair.hist {
                t := time.Unix(int64(h.timestamp), 0)
                amt1 := float64(h.v1) / math.Pow10(tokens[symbols[0]].decimals)
                price := float64(h.v2) / 10000000.0

                fmt.Printf("%29s %19f %19f\n", t, amt1, price)
        }

}

func tradeHelp() {
        fmt.Println("dero trade buy <pair> <amount> <price>")
        fmt.Println("dero trade sell <pair> <amount> <price>")
        fmt.Println("dero trade cancel <orderId>")
        fmt.Println("dero trade history <pair>")
        fmt.Println("dero trade orders <pair>")
        fmt.Println("dero trade book <pair>")
}
