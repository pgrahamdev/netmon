package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{} // use default options

const swVersion = 2

// Indicies used for interpretting the output of speedtest-cli when using the
// --csv output flag (not current used)
const (
	serverNum = iota
	companyName
	location
	date
	distance
	ping
	download
	upload
)

// perf is can be used to store the CSV results from speedtest-cli (not
// currently used)
type perf struct {
	serverNum   int
	companyName string
	location    string
	date        string
	distance    float64
	ping        float64
	download    float64
	upload      float64
}

// A simple print method for the perf type
func (pr perf) print() {
	fmt.Println("---")
	fmt.Println("ServerID:", pr.serverNum)
	fmt.Println("ServerName:", pr.companyName)
	fmt.Println("Location:", pr.location)
	fmt.Println("Date:", pr.date)
	fmt.Println("Distance:", pr.distance)
	fmt.Println("PingLatency:", pr.ping)
	fmt.Println("DownloadRate:", pr.download)
	fmt.Println("UploadRate:", pr.upload)
}

// parseFields can be used to parse the CSV output of speedtest-cli (currently
// unused)
func parseFields(fields []string) (perf, error) {

	var perfRec perf
	var err error
	perfRec.serverNum, err = strconv.Atoi(fields[serverNum])
	if err != nil {
		return perfRec, err
	}
	perfRec.companyName = fields[companyName]
	perfRec.location = fields[location]
	perfRec.date = fields[date]
	perfRec.distance, err = strconv.ParseFloat(fields[distance], 64)
	if err != nil {
		return perfRec, err
	}
	perfRec.ping, err = strconv.ParseFloat(fields[ping], 64)
	if err != nil {
		return perfRec, err
	}
	perfRec.upload, err = strconv.ParseFloat(fields[upload], 64)
	if err != nil {
		return perfRec, err
	}
	perfRec.download, err = strconv.ParseFloat(fields[download], 64)
	return perfRec, err
}

// PerfJSON is used to store the JSON data from the
// speedtest-cli Python program
type PerfJSON struct {
	Server struct {
		ID       string  `json:"id"`
		Sponsor  string  `json:"sponsor"`
		Location string  `json:"name"`
		Country  string  `json:"country"`
		Cc       string  `json:"cc"`
		URL      string  `json:"url"`
		Host     string  `json:"host"`
		Lon      string  `json:"lon"`
		Lat      string  `json:"lat"`
		Distance float64 `json:"d"`
		Latency  float64 `json:"latency"`
		Share    string  `json:"share"`
	} `json:"server"`
	BytesSent     float64 `json:"bytes_sent"`
	BytesReceived float64 `json:"bytes_received"`
	Upload        float64 `json:"upload"`
	Download      float64 `json:"download"`
	Timestamp     string  `json:"timestamp"`
	Ping          float64 `json:"ping"`
}

// A simple print method for the PerfJSON type
func (pr PerfJSON) print() {
	fmt.Println("---")
	fmt.Println("ServerID:", pr.Server.ID)
	fmt.Println("ServerName:", pr.Server.Sponsor)
	fmt.Println("Location:", pr.Server.Location)
	fmt.Println("Date:", pr.Timestamp)
	fmt.Printf("Distance: %.2f km\n", pr.Server.Distance)
	fmt.Printf("PingLatency: %.2f ms\n", pr.Ping)
	fmt.Printf("DownloadRate: %.2f Mb/s\n", pr.Download/1e6)
	fmt.Printf("UploadRate: %.2f Mb/s\n", pr.Upload/1e6)
}

// statusType is the string encoding for the type used for status messages
const statusType = "status"

// resultType is the string encoding for the type used for test results
const resultType = "result"

// initType is the string encoding for the type used to initialize a client
const initType = "init"

// Result is a structure that wraps a type with the the data to be sent to the
// client. The Type can be a status message, a test results, or an
// initialization message
type Result struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// GetSpeedError is an error type for handling getSpeedTestInfo errors.
type GetSpeedError struct {
	ErrorString string
}

// Error is necessary for GetSpeedError's implemention of the error interface
func (gse GetSpeedError) Error() string {
	return gse.ErrorString
}

// getSpeedTestInfo executes speedtest-cli.  If the value of server is > -1,
// then the value is used to query a specific server by server ID.  Otherwise,
// speedtest-cli is run without specifying the server, allowing speedtest-cli to
// determine which remote server to use for the test.
//
// The return value is a PerfJSON structure used to parse the JSON results of
// speedtest-cli and an error.
func getSpeedTestInfo(server int) (PerfJSON, error) {
	var serverID string
	if server > -1 {
		serverID = strconv.Itoa(server)
	} else {
		serverID = ""
	}
	var cmd *exec.Cmd

	var perf PerfJSON
	if serverID != "" {
		cmd = exec.Command("speedtest-cli", "--json", "--server", serverID)
	} else {
		cmd = exec.Command("speedtest-cli", "--json")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return perf, err
	}
	dec := json.NewDecoder(stdout)
	if err = cmd.Start(); err != nil {
		return perf, err
	}
	if err = dec.Decode(&perf); err == io.EOF {
		return perf, GetSpeedError{ErrorString: "No output provided from test."}
	} else if err != nil {
		return perf, err
	}
	if err = cmd.Wait(); err != nil {
		return perf, err
	}
	perf.print()
	return perf, err
}

// HandlerContext provides a context for the WebSockets.  The state for
// WebSocket handling and the slice of test results are stored here.  These
// could have been global variables for the program, but they are all stored in
// a single structure for this implementation.
type HandlerContext struct {
	mtx     sync.Mutex
	perfs   []PerfJSON
	reqChan chan bool
	wsMap   map[*websocket.Conn]bool
}

// NewHandlerContext creates a new HandlerContext struct
func NewHandlerContext() *HandlerContext {
	return &HandlerContext{reqChan: make(chan bool), wsMap: make(map[*websocket.Conn]bool)}
}

// WsHandler uses the context information to handle WebSocket requests
func (ctx *HandlerContext) WsHandler(w http.ResponseWriter, r *http.Request) {
	// This defines a CheckOrigin function that accepts any request.  This
	// probably should be revisited for security purposes, but was done to
	// ease the initial implementation.
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	// Actually upgrade the HTTP connection to the WebSocket protocol, if
	// possible.
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	fmt.Println("New web socket connection.")
	// Add the connection as active to the WebSocket map
	ctx.wsMap[c] = true
	// Make sure we close the connection when we exit the function
	defer c.Close()

	// Used to hold the current list of performance test results
	var tmpData []byte
	// Marshalling an empty slice returns "null",
	// so check for the case and handle it
	if len(ctx.perfs) > 0 {
		// Send the current data
		tmpData, err = json.Marshal(ctx.perfs)
		if err != nil {
			log.Println("Error encoding speedtest data.")
			return
		}
	} else {
		// Provide an empty array otherwise
		tmpData = []byte("[]")
	}
	// Wrap the initial state in a Result struct with type initType
	err = c.WriteJSON(Result{Type: initType, Data: string(tmpData)})
	if err != nil {
		log.Println("write:", err)
		// If the connection has a problem, mark the WebSocket as inactive in
		// the WebSocket map and return (we only want the sendWebSocketData
		// function to clean up wsMap, otherwise craziness will ensue)
		ctx.wsMap[c] = false
		return
	}

	// Listen to requests forever
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			// Mark the WebSocket as inactive so it can be cleaned up by
			// sendWebSocketData
			ctx.wsMap[c] = false
			break
		}
		log.Printf("recv: %s", message)
		// Send the reqChan a message to initiate a run of speedtest-cli
		ctx.reqChan <- true
	}
}

// sendWebSocketData takes a map of WebSocket connection pointers and will send
// a message to all active WebSockets.  If a WebSocket is inactive, it will
// remove that WebSocket connection pointer from the map.  Only
// sendWebSocketData can remove entries from the map for safety's sake.
func sendWebSocketData(wsMap map[*websocket.Conn]bool, messageType string, data string) {
	// Send out a status message to all WebSockets
	for conn, v := range wsMap {
		// We have a dead WebSocket.  Clean it up
		if v == false {
			conn.Close()
			delete(wsMap, conn)
			// Otherwise, let's try to use it
		} else {
			err := conn.WriteJSON(Result{Type: messageType, Data: data})
			if err != nil {
				log.Println("write:", err)
				conn.Close()
				delete(wsMap, conn)
			}
		}
	}
}

// speedtestHandler waits for a request on the req channel, sends a status
// message to the clients, runs speedtest-clie via getSpeedTestInfo, adds the
// results to the results slice (perfs), and then sends the incremental result
// to the clients.  We are passing perfs by reference so we can add to the slice.
func speedtestHandler(server int, req chan bool, wsMap map[*websocket.Conn]bool, perfs *[]PerfJSON) {

	var perf PerfJSON
	var err error
	for {
		// Wait for a request
		<-req
		// Send out a status message to all WebSockets
		sendWebSocketData(wsMap, statusType, "Request made. Waiting for response.")
		// Request speedTest data
		perf, err = getSpeedTestInfo(server)
		if err != nil {
			log.Println("Error trying to get SpeedTest info.\n" + err.Error())
			// Send out a status message to all WebSockets
			sendWebSocketData(wsMap, statusType, "Error executing SpeedTest.")
			continue
		}
		// Add to the perfs array for future reference
		*perfs = append(*perfs, perf)
		// Marshal the latest value for sending
		tmpData, err := json.Marshal(perf)
		if err != nil {
			log.Println("Error encoding speedtest data.")
			continue
		}
		// Send out the result
		sendWebSocketData(wsMap, resultType, string(tmpData))
	}
}

// speedtestTimer initiates a test request at regular intervals.  The interval
// is defined as "period" number of minutes.
func speedtestTimer(req chan bool, period int) {
	for {
		req <- true
		time.Sleep(time.Minute * time.Duration(period))
	}
}

// main initializes the main HandlerContext struct, starts the threads for
// handling test requests and initiating period requests, registers the file
// server and WebSocket handlers, and, finally, starts the web server.
func main() {
	fmt.Println("netmon, Version", swVersion)
	server := flag.Int("server", -1, "The server ID to use for speedtest-cli. "+
		"If -1 is provided,\nspeedtest-cli will choose the 'best' server.")
	period := flag.Int("period", 60, "The period (in minutes) between calls to speedtest-cli")
	addr := flag.String("addr", ":8080", "http service address")

	flag.Parse()

	ctx := NewHandlerContext()

	// Run go routine that actually requests data
	go speedtestHandler(*server, ctx.reqChan, ctx.wsMap, &(ctx.perfs))

	// provides a request every *period minutes
	go speedtestTimer(ctx.reqChan, *period)
	// var serverID string
	http.HandleFunc("/ws", ctx.WsHandler)
	http.Handle("/", http.FileServer(http.Dir("www")))

	log.Fatal(http.ListenAndServe(*addr, nil))
}
