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
var connMap = make(map[*websocket.Conn]bool)
var perfChan = make(chan int, 2)

func ws(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	fmt.Println("New web socket connection.")
	connMap[c] = true
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			connMap[c] = false
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			connMap[c] = false
			break
		}
	}
}

func oldmain() {
	flag.Parse()
	log.SetFlags(0)

}

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

//"encoding/csv"

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

const statusType = "status"
const resultType = "result"
const initType = "init"

// Result contains the resulting data from an operation.  Either an actual
// performance result or a status message.
type Result struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

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

func getSpeedTestInfo(server int) PerfJSON {
	var serverID string
	if server > -1 {
		serverID = strconv.Itoa(server)
	} else {
		serverID = ""
	}
	var cmd *exec.Cmd
	// for iter := 0; iter < *iterations; iter++ {
	// 	if iter != 0 {
	// 		time.Sleep(time.Minute * time.Duration(*period))
	// 	}
	// 	if *csvFlag {
	// 		if serverID != "" {
	// 			cmd = exec.Command("speedtest-cli", "--csv", "--server", serverID)
	// 		} else {
	// 			cmd = exec.Command("speedtest-cli", "--csv")
	// 		}
	// 		stdout, err := cmd.StdoutPipe()
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		if err = cmd.Start(); err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		csvReader := csv.NewReader(stdout)
	// 		fields, err := csvReader.Read()
	// 		perfRec, err := parseFields(fields)
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		if err = cmd.Wait(); err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		perfRec.print()
	// 	} else {
	var perf PerfJSON
	if serverID != "" {
		cmd = exec.Command("speedtest-cli", "--json", "--server", serverID)
	} else {
		cmd = exec.Command("speedtest-cli", "--json")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	dec := json.NewDecoder(stdout)
	if err = cmd.Start(); err != nil {
		log.Fatal(err)
	}
	if err = dec.Decode(&perf); err == io.EOF {
		fmt.Println("Found end of file.")
	} else if err != nil {
		fmt.Println("Error decoding JSON.")
		log.Fatal(err)
	}
	if err = cmd.Wait(); err != nil {
		log.Fatal(err)
	}
	perf.print()
	return perf
}

// HandlerContext provides a context for the WebSockets.
type HandlerContext struct {
	mtx     sync.Mutex
	perfs   []PerfJSON
	reqChan chan bool
	wsMap   map[*websocket.Conn]bool
}

// NewHandlerContext creaees a new HandlerContext struct
func NewHandlerContext() *HandlerContext {
	return &HandlerContext{reqChan: make(chan bool), wsMap: make(map[*websocket.Conn]bool)}
}

// WsHandler uses the context information to handle WebSocket requests
func (ctx *HandlerContext) WsHandler(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	fmt.Println("New web socket connection.")
	ctx.wsMap[c] = true
	defer c.Close()
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
		tmpData = []byte("[]")
	}
	err = c.WriteJSON(Result{Type: initType, Data: string(tmpData)})
	if err != nil {
		log.Println("write:", err)
		ctx.wsMap[c] = false
		return
	}

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			ctx.wsMap[c] = false
			break
		}
		log.Printf("recv: %s", message)
		ctx.reqChan <- true
	}
}

// Need to pass perfs by reference so we can add to the slice.
func speedtestHandler(server int, req chan bool, wsMap map[*websocket.Conn]bool, perfs *[]PerfJSON) {

	var perf PerfJSON
	for {
		// Wait for a request
		<-req
		// Send out a status message to all WebSockets
		for conn, v := range wsMap {
			// We have a dead WebSocket.  Clean it up
			if v == false {
				conn.Close()
				delete(wsMap, conn)
				// Otherwise, let's try to use it
			} else {
				err := conn.WriteJSON(Result{Type: statusType, Data: "Request made. Waiting for response."})
				if err != nil {
					log.Println("write:", err)
					conn.Close()
					delete(wsMap, conn)
				}
			}
		}
		// Request speedTest data
		perf = getSpeedTestInfo(server)
		// Add to the perfs array for future reference
		*perfs = append(*perfs, perf)
		// Marshal the latest value for sending
		tmpData, err := json.Marshal(perf)
		if err != nil {
			log.Println("Error encoding speedtest data.")
			continue
		}
		// Send out the result
		for conn, v := range wsMap {
			// We have a dead WebSocket.  Clean it up
			if v == false {
				conn.Close()
				delete(wsMap, conn)
			} else {
				err = conn.WriteJSON(Result{Type: resultType, Data: string(tmpData)})
				if err != nil {
					log.Println("write:", err)
					conn.Close()
					delete(wsMap, conn)
				}
			}
		}
	}
}

func speedtestTimer(req chan bool, period int) {
	for {
		req <- true
		time.Sleep(time.Minute * time.Duration(period))
	}
}

func main() {
	server := flag.Int("server", -1, "The server ID to use for speedtest-cli")
	// csvFlag := flag.Bool("csv", false, "Enable CSV mode for speedtest-cli")
	period := flag.Int("period", 60, "The period between calls to speedtest-cli")
	// iterations := flag.Int("iterations", 24, "The number of times to execute speedtest-cli")
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
