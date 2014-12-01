package main

import (
	"fmt"
	"log"
	"log/syslog"
	"os"

	"time"

	//"github.com/dustin/go-humanize"
	"github.com/kelseyhightower/envconfig"
	"github.com/rcrowley/go-metrics"
	//"github.com/codahale/metrics"
	"github.com/therealbill/libredis/client"
)

// LaunchConfig is the configuration used by the main app
type LaunchConfig struct {
	RedisConnectionString string
	RedisAuthToken        string
	SentinelConfigFile    string
	LatencyThreshold      int
	Iterations            int
}

var config LaunchConfig

// Syslog logging
var logger *syslog.Writer

type Node struct {
	Name       string
	Role       string
	Connection *client.Redis
}

func init() {
	// initialize logging
	logger, _ = syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "candui.golatency")
	err := envconfig.Process("candui", &config)
	if err != nil {
		logger.Warning(err.Error())
	}
	if config.Iterations == 0 {
		config.Iterations = 100000
	}
	/*
		sconfig.ManagedPodConfigs = make(map[string]SentinelPodConfig)
		if config.SentinelConfigFile == "" {
			config.SentinelConfigFile = "/etc/redis/sentinel.conf"
		}
		if config.LatencyThreshold == 0 {
			config.LatencyThreshold = 50
		}
	*/
}

func directLatencyTest() {
	iterations := config.Iterations
	conn, err := client.DialWithConfig(&client.DialConfig{Address: config.RedisConnectionString, Password: config.RedisAuthToken})
	if err != nil {
		logger.Warning("Unable to connect to instance: " + err.Error())
		log.Fatal("No connection, aborting run.")
	}
	fmt.Println("Connected to " + config.RedisConnectionString)
	s := metrics.NewUniformSample(iterations)
	h := metrics.NewHistogram(s)
	metrics.Register("latency:full", h)
	//w, _ := syslog.Dial("unixgram", "/dev/log", syslog.LOG_INFO, "metrics")
	//go metrics.Syslog(metrics.DefaultRegistry, time.Millisecond*3000, w)

	//go metrics.WriteJSON(metrics.DefaultRegistry, time.Millisecond*1000, os.Stderr)
	//loopstart := time.Now()
	for i := 1; i <= iterations; i++ {
		cstart := time.Now()
		conn.Ping()
		elapsed := int64(time.Since(cstart).Nanoseconds() / 1000)
		h.Update(elapsed)
	}
	//loopelapsed := time.Since(loopstart).Nanoseconds()
	//duration_us := loopelapsed / 1000.0
	//avg_us := float64(avg / 1000.0)
	snap := h.Snapshot()
	avg := snap.Sum() / int64(iterations)
	fmt.Printf("%d iterations over %dus, average %dus/operation\n", iterations, snap.Sum()/1000.0, avg)
	buckets := []float64{0.99, 0.95, 0.9, 0.75, 0.5}
	dist := snap.Percentiles(buckets)
	println("\nPercentile breakout:")
	println("====================")
	for i, b := range buckets {
		fmt.Printf("%.2f%%: %.2fus\n", b*100, dist[i])
	}
	fmt.Printf("\nMin: %dus; Max: %dus; Mean: %.2fus; StdDev: %.2fus\n", snap.Min(), snap.Max(), snap.Mean(), snap.StdDev())
	println("\n\n")
	metrics.WriteJSONOnce(metrics.DefaultRegistry, os.Stderr)

}

func main() {
	directLatencyTest()
}
