package main

import (
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync"
	//"os"
	//"strings"
	"encoding/json"
	"gopkg.in/ini.v1"
	"time"
)

var (
	pullerInterval = flag.Int("puller-interval", 1000, "Interval in milliseconds to pull metrics")
	pusherInterval = flag.Int("pusher-interval", 1000, "Interval in milliseconds to push metrics")
	pusherUrl      = flag.String("pusher-url", "https://example.com/", "URL to push metrics")
	pusherUsername = flag.String("pusher-username", "", "Username to use when pushing metrics")
	pusherSecret   = flag.String("pusher-secret", "", "Secret to use when pushing metrics")
	pusherEnable   = flag.Bool("pusher-enable", false, "Enable metrics push")
)

type NodeStatus struct {
	Free           bool    `json:"free"`
	Reason         string  `json:"reason"`
	Load1          float64 `json:"load1,omitempty"`
	Load5          float64 `json:"load5,omitempty"`
	Load15         float64 `json:"load15,omitempty"`
	Net            string  `json:"net,omitempty"`
	NetThreshold   string  `json:"net-threshold,omitempty"`
	NetUtilization uint64  `json:"net-utilization,omitempty"`
	Time           int64   `json:"time,omitempty"`
	Uptime         int     `json:"uptime,omitempty"`
	Name           string  `json:"name"`
	Hostname       string  `json:"hostname,omitempty"`
}

type NodeConfig struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

func readConfiguration(path *string, group *string) ([]NodeConfig, error) {
	log.Println("Configuration file:", *path)
	cfgIni, err := ini.Load(*path)
	if err != nil {
		log.Fatal("Failed to read file:", err)
	}

	var nodes []NodeConfig
	for _, section := range cfgIni.Sections() {
		for _, entry := range cfgIni.Section(section.Name()).Keys() {
			var node NodeConfig
			node.Url = entry.Value()
			node.Name = entry.Name()
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func StatusPuller(node NodeConfig, status *sync.Map) {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   4 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:        5,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 2 * time.Second,
		DisableCompression:  false,
	}
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: tr,
	}

	var s NodeStatus
	s.Name = node.Name
	s.Reason = "Initializing"
	status.Store(node.Name, s)

	for {
		sleep := time.Duration(*pullerInterval+rand.Intn(100)) * time.Millisecond
		//log.Println("Puller for "+node.Name+" sleeping in", sleep.String())
		time.Sleep(sleep)

		t0 := time.Now()
		req, err := http.NewRequest(http.MethodGet, node.Url, nil)
		if err != nil {
			log.Println("Puller for "+node.Name+" error:", err)
			s.Reason = "Unable to create request"
			status.Store(node.Name, s)
			continue
		}
		req.Header.Set("User-Agent", "NodeStatusPuller/1.0.0")
		resp, err := client.Do(req)
		if err != nil {
			log.Println("Puller for "+node.Name+" error:", err)
			s.Reason = "Unable to connect to node"
			status.Store(node.Name, s)
			continue
		}
		elapsed := time.Since(t0).Seconds()
		log.Println("Puller for "+node.Name+" completed in", elapsed)
		if resp.StatusCode != http.StatusOK {
			s.Reason = "Invalid response code"
			status.Store(node.Name, s)
			continue
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("Puller for "+node.Name+" error:", err)
			s.Reason = "Invalid response code"
			status.Store(node.Name, s)
			continue
		}

		if err := json.Unmarshal(body, &s); err != nil {
			log.Println("Puller for "+node.Name+" error:", err)
			s.Reason = "Unable to read status"
			status.Store(node.Name, s)
			continue
		}
		status.Store(node.Name, s)
	}
}

func StatusPusher(nodes []NodeConfig, status *sync.Map) {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   4 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:        5,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 2 * time.Second,
		DisableCompression:  false,
	}
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: tr,
	}

	for {
		sleep := time.Duration(*pusherInterval+rand.Intn(100)) * time.Millisecond
		//log.Println("Pusher sleeping in", sleep.String())
		time.Sleep(sleep)

		var all []NodeStatus
		for _, node := range nodes {
			if s, loaded := status.Load(node.Name); loaded {
				all = append(all, s.(NodeStatus))
			}
		}

		out, err := json.MarshalIndent(all, "", "    ")
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println(string(out))

		t0 := time.Now()
		req, err := http.NewRequest(http.MethodPost, *pusherUrl, bytes.NewBuffer(out))
		if err != nil {
			log.Println("Pusher error:", err)
			continue
		}
		req.Header.Set("User-Agent", "NodeStatusPusher/1.0.0")
		if *pusherUsername != "" || *pusherSecret != "" {
			req.SetBasicAuth(*pusherUsername, *pusherSecret)
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Println("Pusher error:", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.Println("Pusher error, got invalid response code:", resp.StatusCode)
			continue
		}

		if _, err := io.Copy(ioutil.Discard, resp.Body); err != nil {
			log.Println("Pusher error:", err)
			continue
		}

		defer resp.Body.Close()

		elapsed := time.Since(t0).Seconds()
		log.Println("Pusher completed in", elapsed)
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	var cfgFile = flag.String("config", "/etc/nodestatus/nodes.ini", "Path to configuration file")
	var group = flag.String("group", "nodes", "Group to read from the configuration file")
	flag.Parse()

	nodes, err := readConfiguration(cfgFile, group)
	if err != nil {
		log.Fatal(err)
	}

	status := new(sync.Map)
	for _, node := range nodes {
		go StatusPuller(node, status)
	}

	if *pusherEnable {
		go StatusPusher(nodes, status)
	}

	// Block here
	select {}
}
