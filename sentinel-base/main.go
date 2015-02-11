package main

import (
	"fmt"
	"log/syslog"
	"runtime"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/kelseyhightower/envconfig"
	"github.com/therealbill/libredis/client"
)

// LaunchConfig is the configuration used by the main app
type LaunchConfig struct {
	RedisConnectionString string
	RedisAuthToken        string
	SentinelConfigFile    string
	LatencyThreshold      int
}

var config LaunchConfig

// Syslog logging
var logger *syslog.Writer

type Node struct {
	Name       string
	Pod        SentinelPodConfig
	Role       string
	Connection *client.Redis
}

// containers
var Nodes map[string]*Node

func init() {
	// initialize logging
	logger, _ = syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "candui")
	err := envconfig.Process("candui", &config)
	if err != nil {
		logger.Warning(err.Error())
	}
	sconfig.ManagedPodConfigs = make(map[string]SentinelPodConfig)
	if config.SentinelConfigFile == "" {
		config.SentinelConfigFile = "/etc/redis/sentinel.conf"
	}
	if config.LatencyThreshold == 0 {
		config.LatencyThreshold = 50
	}
}

func loadMastersFromFile() {
	LoadSentinelConfigFile()
	logger.Info(fmt.Sprintf("Loading %d pods ", len(sconfig.ManagedPodConfigs)))
	for _, pod := range sconfig.ManagedPodConfigs {
		nodename := fmt.Sprintf("%s:%d", pod.IP, pod.Port)
		node, exists := Nodes[nodename]
		if !exists {
			conn, err := client.DialWithConfig(&client.DialConfig{Address: nodename, Password: pod.AuthToken})
			if err != nil {
				logger.Warning("Error connecting to node " + nodename)
				continue
			}
			node = &Node{Name: nodename, Pod: pod, Connection: conn}
			if Nodes == nil {
				Nodes = make(map[string]*Node)
			}
			Nodes[nodename] = node
			err = node.Connection.ConfigSetInt("latency-monitor-threshold", config.LatencyThreshold)
			if err != nil {
				logger.Warning("Unable to enable latency on " + nodename)
			}
		}
	}
}

func checkForLatencyOnNodes() {
	loadMastersFromFile()
	latent_nodecount := 0
	nonlatent_nodecount := 0
	logger.Warning(fmt.Sprintf("Checking %d nodes for latency", len(Nodes)))
	for _, node := range Nodes {
		results, err := node.Connection.LatencyHistory("command")
		_ = strings.Contains
		if err != nil {
			logger.Warning(node.Name + " - RR- " + err.Error())
			continue
		}
		history := results.Records
		if len(history) == 0 {
			nonlatent_nodecount++
		} else {
			latent_nodecount++
		}
	}
	logger.Warning(fmt.Sprintf("Found %d nodes with latency spikes", latent_nodecount))
	logger.Warning(fmt.Sprintf("Found %d nodes with NO latency spikes", nonlatent_nodecount))
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)

	logger.Warning(fmt.Sprintf("[Memory Usage] InUse: %s System: %s", humanize.Bytes(memStats.Alloc), humanize.Bytes(memStats.Sys)))

}

func main() {
	for {
		go checkForLatencyOnNodes()
		time.Sleep(time.Second * 60)
	}

}
