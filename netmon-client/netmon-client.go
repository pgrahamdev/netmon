package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/pgrahamdev/netmon/messages"
)

const swVersion = 1

func receiveResults(conn *websocket.Conn, done chan bool) {
	for {
		select {
		case <-done:
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("Error reading WebSocket:", err.Error())
				return
			}
			var tmpResult messages.Result
			err = json.Unmarshal(message, &tmpResult)
			if err != nil {
				fmt.Println("Error unmarshalling message:", err.Error())
				continue
			}
			switch tmpResult.Type {
			case messages.StatusType:
				fmt.Println("Status:", tmpResult.Data)
			case messages.InitType:
				var perfs []messages.PerfJSON
				err = json.Unmarshal([]byte(tmpResult.Data), &perfs)
				if err != nil {
					fmt.Println("Error unmarshalling init message:", err.Error())
					continue
				}
				for _, value := range perfs {
					value.Print()
				}
			case messages.ResultType:
				var perf messages.PerfJSON
				err = json.Unmarshal([]byte(tmpResult.Data), &perf)
				if err != nil {
					fmt.Println("Error unmarshalling result message:", err.Error())
					continue
				}
				perf.Print()
			}
		}
	}
}

func main() {
	fmt.Println("netmon-client, Version", swVersion)

	ip := flag.String("ip", "localhost", "IP address or name of netmon server")
	port := flag.Int("port", 8080, "TCP port number to use to connect to the netmon server")

	flag.Parse()

	address := *ip + ":" + strconv.Itoa(*port)

	// Create channel related to the interrupt signal, Ctrl-C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: address, Path: "/ws"}
	fmt.Println("Connecting to", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println("Error connecting to server:", err.Error())
	}

	defer conn.Close()

	done := make(chan bool, 1)

	go receiveResults(conn, done)

	stdinReader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-interrupt:
			fmt.Println("Received interrupt.  Quitting...")
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				fmt.Println("Error writing close to WebSocket:", err.Error())
			}
			done <- true
			return
		default:
			_, _, err := stdinReader.ReadRune()
			if err != nil {
				fmt.Println("Error reading value from keyboard:", err.Error())
				done <- true
				return
			}
			fmt.Println("CLI: Requesting test.")
			err = conn.WriteMessage(websocket.TextMessage, []byte("Start-CLI"))
			if err != nil {
				fmt.Println("Error writing request message:", err.Error())
				done <- true
				return
			}
		}
	}
}
