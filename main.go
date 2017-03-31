package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/net"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type Status struct {
	Load1    float64 `json:"load1"`
	Load5    float64 `json:"load5"`
	Load15   float64 `json:"load15"`
	NetBpsTx uint64  `json:"net-bps-tx"`
	NetBpsRx uint64  `json:"net-bps-rx"`
	Time     int64   `json:"time"`
	Uptime   int     `json:"uptime"`
	Hostname string  `json:"hostname"`
	sync.RWMutex
}

var (
	listenHostFlag     = flag.String("listen-host", "127.0.0.1", "Listen host")
	listenPortFlag     = flag.Int("listen-port", 8080, "Listen port")
	interfaceStatsFlag = flag.String("interface-stats", "all", "Network interface to read stats from")
	intervalFlag       = flag.Int("interval", 1, "Data gather interval in seconds")
	startTime          time.Time
	status             Status
)

func main() {
	flag.Parse()
	if *intervalFlag < 1 {
		fmt.Println("Interval must be higher than 0")
	}
	startTime = time.Now()
	go status.Worker(*interfaceStatsFlag, *intervalFlag)

	log := log.New(os.Stdout, "- ", log.LstdFlags)
	http.HandleFunc("/status", handler)
	http.Handle("/", http.StripPrefix("/", http.FileServer(assetFS())))

	log.Fatal(http.ListenAndServe(*listenHostFlag+":"+strconv.Itoa(*listenPortFlag), nil))
}

func (s *Status) Worker(iface string, interval int) {
	var prevBytesSent uint64
	var prevBytesRecv uint64
	for {
		// Network
		var pernic bool
		if iface == "all" {
			pernic = false
		} else {
			pernic = true
		}

		nics, err := net.IOCounters(pernic)
		if err != nil {
			fmt.Println("Unable to read network stats.")
			break
		}

		var netBpsTx uint64
		var netBpsRx uint64
		for _, nic := range nics {
			if iface == nic.Name {
				if prevBytesSent > 0 {
					netBpsTx = (nic.BytesSent - prevBytesSent) / uint64(interval)
				}
				if prevBytesSent > 0 {
					netBpsRx = (nic.BytesRecv - prevBytesRecv) / uint64(interval)
				}
				prevBytesSent = nic.BytesSent
				prevBytesRecv = nic.BytesRecv
			}
		}

		// Load
		l, err := load.Avg()
		if err != nil {
			fmt.Println("Unable to read load average.")
			break
		}

		// Hostname
		hostname, err := os.Hostname()
		if err != nil {
			fmt.Println("Unable to read hostname.")
			break
		}

		// Time
		now := time.Now()

		s.Lock()
		s.Time = now.Unix()
		s.Uptime = int(now.Sub(startTime).Seconds())
		s.Load1 = l.Load1
		s.Load5 = l.Load5
		s.Load15 = l.Load15
		s.Hostname = hostname
		s.NetBpsTx = netBpsTx
		s.NetBpsRx = netBpsRx
		s.Unlock()

		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=1")

	status.Lock()
	if out, err := json.MarshalIndent(status, "", "    "); err != nil {
		http.Error(w, "Internal Server Error", 503)
	} else {
		fmt.Fprintf(w, string(out))
	}
	status.Unlock()
}
