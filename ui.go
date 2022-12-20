package main

import (
	"fmt"
	"sync"
	"io"
	"os"
        "strings"
        "strconv"
        "time"

	d "github.com/deroholic/derogo"
        "github.com/chzyer/readline"
)

var prompt_mutex sync.Mutex // prompt lock

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
	fmt.Println("addliquidity <pair> [<amount> | max] <symbol>")
	fmt.Println("remliquidity <pair> <percent>")
	fmt.Println("swap <pair> [<amount> | max] <symbol>")
	fmt.Println("status <pair>")
	fmt.Println("quote <symbol1> <symbol2>")
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
	readline.PcItem("quote"),
	readline.PcItem("trade",
		readline.PcItem("help"),
		readline.PcItem("buy"),
		readline.PcItem("sell"),
		readline.PcItem("cancel"),
		readline.PcItem("history"),
		readline.PcItem("orders"),
		readline.PcItem("book"),
	),
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
				displayAddress()
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
			case "quote":
				quote(words[1:])
			case "trade":
				if len(words) > 1 {
					switch words[1] + "" {
					case "help":
						tradeHelp()
					case "buy":
						tradeBuy(words[2:])
					case "sell":
						tradeSell(words[2:])
					case "cancel":
						tradeCancel(words[2:])
					case "history":
						tradeHistory(words[2:])
					case "orders":
						tradeOrders(words[2:])
					case "book":
						tradeBook(words[2:])
					}
				}
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

		network := "MAINNET"
                if testnet {
                        network = "TESTNET"
                }

		p := fmt.Sprintf("%d/%d %s > ", wh, dh, network)
		l.SetPrompt(p)
		l.Refresh()

		prompt_mutex.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}
