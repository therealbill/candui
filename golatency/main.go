package main

import (
	"fmt"
	"log"
	"log/syslog"
	"os"
	"time"

	//"github.com/dmstin/go-humanize"
	"github.com/kelseyhightower/envconfig"
	"github.com/rcrowley/go-metrics"
	//"github.com/codahale/metrics"
	"github.com/therealbill/libredis/client"
)

// LaunchConfig is the configuration msed by the main app
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
		config.Iterations = 1000
	}
}

func doTest(conn *client.Redis) {
	h := metrics.Get("latency:full").(metrics.Histogram)
	cstart := time.Now()
	conn.Ping()
	elapsed := int64(time.Since(cstart) / time.Microsecond)
	h.Update(elapsed)
}

func directLatencyTest() {
	iterations := config.Iterations
	conn, err := client.DialWithConfig(&client.DialConfig{Address: config.RedisConnectionString, Password: config.RedisAuthToken})
	if err != nil {
		logger.Warning("Unable to connect to instance '" + config.RedisConnectionString + "': " + err.Error())
		log.Fatal("No connection, aborting run.")
	}
	fmt.Println("Connected to " + config.RedisConnectionString)
	s := metrics.NewUniformSample(iterations)
	h := metrics.NewHistogram(s)
	metrics.Register("latency:full", h)
	for i := 1; i <= iterations; i++ {
		doTest(conn)
	}
	snap := h.Snapshot()
	avg := snap.Sum() / int64(iterations)
	fmt.Printf("%d iterations over %dus, average %dus/operation\n", iterations, snap.Sum()/1000.0, avg)
	buckets := []float64{0.99, 0.95, 0.9, 0.75, 0.5}
	dist := snap.Percentiles(buckets)
	println("\nPercentile breakout:")
	println("====================")
	for i, b := range buckets {
		if dist[i] == 0 {
			fmt.Printf("%.2f%%: <1us\n", b*100)
		} else {
			fmt.Printf("%.2f%%: %.2fus\n", b*100, dist[i])
		}
	}
	fmt.Printf("\nMin: %dus; Max: %dus; Mean: %.2fus; StdDev: %.2fus\n", snap.Min(), snap.Max(), snap.Mean(), snap.StdDev())
	println("\n\n")
	metrics.WriteJSONOnce(metrics.DefaultRegistry, os.Stderr)

}

func main() {
	directLatencyTest()
}
