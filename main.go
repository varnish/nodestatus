package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/net"
)

type Status struct {
	Free           bool    `json:"free"`
	Reason         string  `json:"reason"`
	Load1          float64 `json:"load1"`
	Load5          float64 `json:"load5"`
	Load15         float64 `json:"load15"`
	Net            string  `json:"net"`
	NetThreshold   string  `json:"net-threshold"`
	NetUtilization uint64  `json:"net-utilization"`
	Time           int64   `json:"time"`
	Uptime         int     `json:"uptime"`
	Hostname       string  `json:"hostname"`
	sync.RWMutex
}

var (
	maintenanceFilePathFlag = flag.String("maintenance", "/etc/varnish/maintenance", "File in the file system indicating maintenance mode")
	listenHostFlag          = flag.String("listen-host", "127.0.0.1", "Listen host")
	listenPortFlag          = flag.Int("listen-port", 8080, "Listen port")
	netThresholdFlag            = flag.String("net-threshold", "128 Mbps", "Network bandwidth threshold (units bps, Kbps, Mbps, Gbps and Tbps)")
	netDeviceFlag           = flag.String("net-dev", "all", "Network interface to read stats from")
	intervalFlag            = flag.Int("interval", 1, "Data gather interval in seconds")
	status                 Status
	netThreshold           uint64
)

func main() {
	var err error
	flag.Parse()

	// Validate command line flags
	if *intervalFlag < 1 {
		log.Fatalln("Interval must be higher than 0")
	}

	netThreshold, err = ParseBit(*netThresholdFlag)
	if err != nil {
		log.Fatalln("Unable to parse network threshold:", err)
	}
	log.Println("Network threshold set to " + HumanizeBit(netThreshold))

	// Goroutine to collect metrics and calculate utilization
	go status.Worker(*netDeviceFlag, *intervalFlag)

	http.HandleFunc("/", statusHandler)

	log.Println("Listening at " + *listenHostFlag + ":" + strconv.Itoa(*listenPortFlag))
	log.Fatal(http.ListenAndServe(*listenHostFlag+":"+strconv.Itoa(*listenPortFlag), nil))
}

func (s *Status) Worker(iface string, interval int) {
	var prevBytesSent uint64
	var prevBytesRecv uint64
	var bps uint64
	startTime := time.Now()

	var pernic bool
	var maintenance bool

	if iface == "all" {
		pernic = false
	} else {
		pernic = true
	}

	for {
		// Network
		nics, err := net.IOCounters(pernic)
		if err != nil {
			log.Fatalln("Unable to read network stats:", err)
		}

		var netBpsTx uint64
		var netBpsRx uint64
		for _, nic := range nics {
			if iface == nic.Name {
				if prevBytesSent > 0 {
					netBpsTx = (nic.BytesSent - prevBytesSent) / uint64(interval) * 8
				}
				if prevBytesSent > 0 {
					netBpsRx = (nic.BytesRecv - prevBytesRecv) / uint64(interval) * 8
				}
				prevBytesSent = nic.BytesSent
				prevBytesRecv = nic.BytesRecv

				// If the receive bandwidth is higher than transmit bandwidth,
				// report receive bandwidth instead.
				if netBpsTx > netBpsRx {
					bps = netBpsTx
				} else {
					bps = netBpsRx
				}
			}
		}

		// Load
		l, err := load.Avg()
		if err != nil {
			log.Fatalln("Unable to read load average:", err)
		}

		// Hostname
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalln("Unable to read hostname:", err)
		}

		// Time
		now := time.Now()

		// Maintenance mode
		// If the file exists: Maintenance mode
		// If the file does not exist: Not maintenance mode
		if _, err := os.Stat(*maintenanceFilePathFlag); err == nil {
			maintenance = true
		} else {
			maintenance = false
		}

		s.Lock()
		s.Time = now.Unix()

		// Uptime of this process
		s.Uptime = int(now.Sub(startTime).Seconds())

		s.Load1 = l.Load1
		s.Load5 = l.Load5
		s.Load15 = l.Load15
		s.Hostname = hostname

		s.Net = HumanizeBit(bps)
		s.NetThreshold = HumanizeBit(netThreshold)
		s.NetUtilization = 100 * bps / netThreshold

		// Assume normal operation before checking readings
		s.Free = true
		s.Reason = "Normal operation"

		// Set free to false if network is is utilized
		if bps >= netThreshold {
			s.Free = false
			s.Reason = "Network fully utilizied"
		}

		// Set free to false if in maintenance mode
		if maintenance {
			s.Free = false
			s.Reason = "Maintenance mode"
		}
		s.Unlock()

		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=1, stale-while-revalidate=1")

	status.Lock()
	if out, err := json.MarshalIndent(status, "", "    "); err != nil {
		http.Error(w, "Internal Server Error", 503)
	} else {
		fmt.Fprintf(w, string(out))
	}
	status.Unlock()
}
