package main

import (
	"fmt"
	"log"
	"log/syslog"
	"os"
	"strings"
	"time"

	//"github.com/dustin/go-humanize"
	"github.com/kelseyhightower/envconfig"
	"github.com/rcrowley/go-metrics"
	"github.com/therealbill/libredis/client"
	"gopkg.in/mgo.v2"
)

// LaunchConfig is the configuration used by the main app
type LaunchConfig struct {
	RedisConnectionString string
	RedisAuthToken        string
	SentinelConfigFile    string
	LatencyThreshold      int
	Iterations            int
	MongoConnString       string
	MongoDBName           string
	MongoCollectionName   string
	MongoUsername         string
	MongoPassword         string
	UseMongo              bool
}

var config LaunchConfig

// Syslog logging
var logger *syslog.Writer

type Node struct {
	Name       string
	Role       string
	Connection *client.Redis
}

type TestStatsEntry struct {
	Hist      map[string]float64
	Max       int64
	Mean      float64
	Min       int64
	Jitter    float64
	Timestamp int64
	Name      string
	Unit      string
}

var session *mgo.Session

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
	if config.UseMongo || config.MongoConnString > "" {
		fmt.Println("Mongo storage enabled")
		mongotargets := strings.Split(config.MongoConnString, ",")
		fmt.Printf("targets: %+v\n", mongotargets)
		fmt.Print("connecting to mongo...")
		var err error
		session, err = mgo.DialWithInfo(&mgo.DialInfo{Addrs: mongotargets, Username: "latstore", Password: "latstorepass", Database: "redislatency"})
		if err != nil {
			panic(err)
		}
		fmt.Println("done")
		// Optional. Switch the session to a monotonic behavior.
		session.SetMode(mgo.Monotonic, true)
		config.UseMongo = true
	}
}

func main() {
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

	for i := 1; i <= iterations; i++ {
		cstart := time.Now()
		conn.Ping()
		elapsed := int64(time.Since(cstart).Nanoseconds() / 1000)
		h.Update(elapsed)
	}
	snap := h.Snapshot()
	avg := snap.Sum() / int64(iterations)
	fmt.Printf("%d iterations over %dus, average %dus/operation\n", iterations, snap.Sum()/1000.0, avg)
	buckets := []float64{0.99, 0.95, 0.9, 0.75, 0.5}
	dist := snap.Percentiles(buckets)
	println("\nPercentile breakout:")
	println("====================")
	var result TestStatsEntry
	result.Hist = make(map[string]float64)
	result.Name = "test run"
	result.Timestamp = time.Now().Unix()
	for i, b := range buckets {
		fmt.Printf("%.2f%%: %.2fus\n", b*100, dist[i])
		bname := fmt.Sprintf("%.2f", b*100)
		result.Hist[bname] = dist[i]
	}
	fmt.Printf("\nMin: %dus; Max: %dus; Mean: %.2fus; StdDev: %.2fus\n", snap.Min(), snap.Max(), snap.Mean(), snap.StdDev())

	result.Max = snap.Max()
	result.Mean = snap.Mean()
	result.Min = snap.Min()
	result.Jitter = snap.StdDev()
	result.Unit = "us"
	println("\n\n")
	metrics.WriteJSONOnce(metrics.DefaultRegistry, os.Stderr)
	if config.UseMongo {
		coll := session.DB(config.MongoDBName).C(config.MongoCollectionName)
		coll.Insert(&result)
		if err != nil {
			log.Fatal(err)
		}
		println("\nReading dataz from mongo...")
		var previousResults []TestStatsEntry
		iter := coll.Find(nil).Limit(100).Sort("Timestamp").Iter()
		err = iter.All(&previousResults)
		if err != nil {
			println(err)
		}
		for _, test := range previousResults {
			fmt.Printf("%+v\n", test)
			println()
		}
		session.Close()
	}

}
