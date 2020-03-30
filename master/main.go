package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"golang.org/x/oauth2/clientcredentials"
	"gopkg.in/ini.v1"
	"io"
	"io/ioutil"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	debug = flag.Bool("debug", false, "Enable debug output.")

	pullerInterval = flag.Duration("puller-interval", 1*time.Second, "Interval used to pull metrics.")

	pusherEnable   = flag.Bool("pusher-enable", false, "Enable metrics push.")
	pusherInterval = flag.Duration("pusher-interval", 1*time.Second, "Interval used to push metrics.")
	pusherUrl      = flag.String("pusher-url", "https://example.com/", "URL to push metrics.")
	pusherAuth     = flag.String("pusher-auth", "basic", "Authentication machanism to use when pushing metrics [basic, oauth].")

	// Basic auth
	pusherUsername = flag.String("pusher-username", os.Getenv("AUTH_USERNAME"), "Basic auth username to use when pushing metrics. The default value is read from the environment variable AUTH_USERNAME.")
	pusherPassword = flag.String("pusher-password", os.Getenv("AUTH_PASSWORD"), "Basic auth password to use when pushing metrics. The default value is read from the environment variable AUTH_PASSWORD.")

	// OAuth2
	pusherClientId     = flag.String("pusher-client-id", os.Getenv("OAUTH_CLIENT_ID"), "OAuth client id to use when pushing metrics. The default value is read from the environment variable OAUTH_CLIENT_ID.")
	pusherClientSecret = flag.String("pusher-client-secret", os.Getenv("OAUTH_CLIENT_SECRET"), "OAuth client secret to use when pushing metrics. The default value is read from the environment variable OAUTH_CLIENT_SECRET.")
	pusherTokenUrl     = flag.String("pusher-token-url", os.Getenv("OAUTH_TOKEN_URL"), "OAuth URL to get token when pushing metrics. The default value is read from the environment variable OAUTH_TOKEN_URL.")
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

// Reset the nodestatus values, for example useful on connection failures.
func (s *NodeStatus) Reset() {
	s.Free = false
	s.Reason = ""
	s.Load1 = 0
	s.Load5 = 0
	s.Load15 = 0
	s.Net = ""
	s.NetUtilization = 0
}

type NodeConfig struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

func readConfiguration(path *string, group *string) ([]NodeConfig, error) {
	fmt.Printf("Configuration file: %s\n", *path)
	cfgIni, err := ini.Load(*path)
	if err != nil {
		fmt.Printf("Failed to read file: %s\n", err.Error())
		os.Exit(2)
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
		Timeout:   3 * time.Second,
		Transport: tr,
	}

	var s NodeStatus
	s.Name = node.Name
	s.Reason = "Initializing"
	status.Store(node.Name, s)

	for {
		sleep := *pullerInterval + time.Duration(rand.Intn(100))*time.Millisecond
		time.Sleep(sleep)

		t0 := time.Now()
		req, err := http.NewRequest(http.MethodGet, node.Url, nil)
		if err != nil {
			fmt.Printf("Puller for %s error: %s\n", node.Name, err.Error())
			s.Reset()
			s.Reason = "Unable to create request"
			status.Store(node.Name, s)
			continue
		}
		req.Header.Set("User-Agent", "NodeStatusPuller/1.0.0")
		req.Header.Set("Accept-Encoding", "gzip")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Puller for %s error: %s\n", node.Name, err.Error())
			s.Reset()
			s.Reason = "Unable to connect to node"
			status.Store(node.Name, s)
			continue
		}

		if *debug {
			fmt.Printf("Puller for %s completed with status %s\n", node.Name, resp.Status)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
			s.Reset()
			s.Reason = "Invalid response code (" + resp.Status + ")"
			status.Store(node.Name, s)
			resp.Body.Close()
			continue
		}

		var reader io.ReadCloser
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(resp.Body)
			if err != nil {
				fmt.Printf("Puller for %s failed to uncompress response: %s\n", node.Name, err.Error())
				s.Reset()
				s.Reason = "Invalid response code (" + resp.Status + ")"
				status.Store(node.Name, s)
				resp.Body.Close()
				continue
			}
		default:
			reader = resp.Body
		}

		body, err := ioutil.ReadAll(reader)
		if err != nil {
			fmt.Printf("Puller for %s error: %s\n", node.Name, err.Error())
			s.Reset()
			s.Reason = "Unable to read response body"
			status.Store(node.Name, s)
			reader.Close()
			continue
		}
		reader.Close()

		elapsed := time.Since(t0).Seconds()
		fmt.Printf("Puller for %s fetched %db in %.2fs\n", node.Name, len(body), elapsed)
		if err := json.Unmarshal(body, &s); err != nil {
			fmt.Printf("Puller for %s error: %s\n", node.Name, err.Error())
			s.Reset()
			s.Reason = "Unable to read status"
			status.Store(node.Name, s)
			continue
		}
		if *debug {
			fmt.Printf("Puller for %s got: %s\n", node.Name, string(body))
		}
		status.Store(node.Name, s)
	}
}

func StatusPusherWrapper(nodes []NodeConfig, status *sync.Map) {
	for {
		err := StatusPusher(nodes, status)
		if err != nil {
			fmt.Printf("Restarting pusher due to: %s\n", err.Error())
		}
	}
}

func StatusPusher(nodes []NodeConfig, status *sync.Map) error {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 12 * time.Second,
		}).DialContext,
		MaxIdleConns:        5,
		IdleConnTimeout:     12 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableCompression:  false,
	}
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: tr,
	}

	if *pusherAuth == "oauth" {
		conf := clientcredentials.Config{
			ClientID:     *pusherClientId,
			ClientSecret: *pusherClientSecret,
			TokenURL:     *pusherTokenUrl,
		}
		client = conf.Client(context.Background())
	}

	for {
		sleep := *pullerInterval + time.Duration(rand.Intn(100))*time.Millisecond
		time.Sleep(sleep)

		var all []NodeStatus
		for _, node := range nodes {
			if s, loaded := status.Load(node.Name); loaded {
				all = append(all, s.(NodeStatus))
			}
		}

		out, err := json.MarshalIndent(all, "", "    ")
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			continue
		}
		if *debug {
			fmt.Printf("Pusher will send: %s\n", string(out))
		}

		t0 := time.Now()
		req, err := http.NewRequest(http.MethodPost, *pusherUrl, bytes.NewBuffer(out))
		if err != nil {
			fmt.Printf("Pusher error: %s\n", err.Error())
			continue
		}
		req.Header.Set("User-Agent", "NodeStatusPusher/1.0.0")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")

		if *pusherAuth == "basic" {
			req.SetBasicAuth(*pusherUsername, *pusherPassword)
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Pusher error: %s\n", err.Error())
			return err
		}

		if *debug {
			fmt.Printf("Pusher completed with status: %s\n", resp.Status)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Pusher error: %s\n", err.Error())
			resp.Body.Close()
			return err
		}
		resp.Body.Close()

		if resp.StatusCode == 429 {
			fmt.Printf("Pusher was rate limited. Sleeping for 15 seconds\n")
			time.Sleep(15 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Pusher error, got invalid response code: %s\n", resp.Status)
			fmt.Printf("Got response body: %s\n", string(body))
			return err
		}

		elapsed := time.Since(t0).Seconds()
		fmt.Printf("Pusher sent %db in %.2fs\n", len(out), elapsed)
	}
}

func main() {
	rand.Seed(time.Now().Unix())
	var cfgFile = flag.String("config", "/etc/nodestatus/nodes.ini", "Path to configuration file")
	var group = flag.String("group", "nodes", "Group to read from the configuration file")
	flag.Parse()

	nodes, err := readConfiguration(cfgFile, group)
	if err != nil {
		fmt.Printf(err.Error())
		os.Exit(2)
	}

	status := new(sync.Map)
	for _, node := range nodes {
		go StatusPuller(node, status)
	}

	if *pusherEnable {
		go StatusPusherWrapper(nodes, status)
	}

	// Block here
	select {}
}
