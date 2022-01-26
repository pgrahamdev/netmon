package messages

import (
	"fmt"
	"strconv"
)

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
type Perf struct {
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
func (pr Perf) Print() {
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
func parseFields(fields []string) (Perf, error) {

	var perfRec Perf
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
func (pr PerfJSON) Print() {
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
const StatusType = "status"

// resultType is the string encoding for the type used for test results
const ResultType = "result"

// initType is the string encoding for the type used to initialize a client
const InitType = "init"

// Result is a structure that wraps a type with the the data to be sent to the
// client. The Type can be a status message, a test results, or an
// initialization message
type Result struct {
	Type string `json:"type"`
	Data string `json:"data"`
}
