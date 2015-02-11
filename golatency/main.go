package main

import (
	"fmt"
	"log"
	"log/syslog"
	"os"
	"strings"
	"time"

	//"github.com/dmstin/go-humanize"
	"github.com/kelseyhightower/envconfig"
	"github.com/rcrowley/go-metrics"
	"github.com/therealbill/libredis/client"
	"gopkg.in/mgo.v2"
)

// LaunchConfig is the configuration msed by the main app
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
	JSONOut               bool
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
	Max       float64
	Mean      float64
	Min       float64
	Jitter    float64
	Timestamp int64
	Name      string
	Unit      string
}

var session *mgo.Session

func init() {
	// initialize logging
	logger, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "golatency")
	if err != nil {
		fmt.Printf("Unable to connect to syslog, err: %s", err.Error())
	}
	err = envconfig.Process("golatency", &config)
	if err != nil {
		logger.Warning(err.Error())
	}
	if config.Iterations == 0 {
		config.Iterations = 1000
	}
	if config.UseMongo || config.MongoConnString > "" {
		fmt.Println("Mongo storage enabled")
		mongotargets := strings.Split(config.MongoConnString, ",")
		fmt.Printf("targets: %+v\n", mongotargets)
		fmt.Print("connecting to mongo...")
		var err error
		session, err = mgo.DialWithInfo(&mgo.DialInfo{Addrs: mongotargets, Username: config.MongoUsername, Password: config.MongoPassword, Database: config.MongoDBName})
		if err != nil {
			config.UseMongo = false
			panic(err)
		}
		fmt.Println("done")
		// Optional. Switch the session to a monotonic behavior.
		session.SetMode(mgo.Monotonic, true)
		config.UseMongo = true
	}
}

func doTest(conn *client.Redis) {
	h := metrics.Get("latency:full").(metrics.Histogram)
	cstart := time.Now()
	conn.Ping()
	elapsed := int64(time.Since(cstart).Nanoseconds())
	h.Update(elapsed)
}

func main() {
	iterations := config.Iterations
	conn, err := client.DialWithConfig(&client.DialConfig{Address: config.RedisConnectionString, Password: config.RedisAuthToken})
	if err != nil {
		if logger != nil {
			logger.Warning("Unable to connect to instance '" + config.RedisConnectionString + "': " + err.Error())
		}
		log.Fatal("No connection, aborting run.")
	}
	//fmt.Println("Connected to " + config.RedisConnectionString)
	s := metrics.NewUniformSample(iterations)
	h := metrics.NewHistogram(s)
	metrics.Register("latency:full", h)
	for i := 1; i <= iterations; i++ {
		doTest(conn)
	}
	snap := h.Snapshot()
	avg := snap.Sum() / int64(iterations)
	if !config.JSONOut {
		fmt.Printf("%d iterations over %s, average %s/operation\n", iterations, time.Duration(snap.Sum()), time.Duration(avg))
	}
	buckets := []float64{0.99, 0.95, 0.9, 0.75, 0.5}
	dist := snap.Percentiles(buckets)
	if !config.JSONOut {
		println("\nPercentile breakout:")
		println("====================")
	}
	var result TestStatsEntry
	result.Hist = make(map[string]float64)
	result.Name = "test run"
	result.Timestamp = time.Now().Unix()
	min := time.Duration(snap.Min())
	max := time.Duration(snap.Max())
	mean := time.Duration(snap.Mean())
	stddev := time.Duration(snap.StdDev())
	if !config.JSONOut {
		fmt.Printf("\nMin: %s\nMax: %s\nMean: %s\nJitter: %s\n", min, max, mean, stddev)
	}
	for i, b := range buckets {
		d := time.Duration(dist[i])
		if !config.JSONOut {
			fmt.Printf("%.2f%%: %v\n", b*100, d)
		}
		bname := fmt.Sprintf("%.2f", b*100)
		result.Hist[bname] = dist[i]
	}

	result.Max = float64(snap.Max())
	result.Mean = snap.Mean()
	result.Min = float64(snap.Min())
	result.Jitter = snap.StdDev()
	result.Unit = "ns"
	if config.JSONOut {
		metrics.WriteJSONOnce(metrics.DefaultRegistry, os.Stdout)
	} else {
		println("\n\n")
	}
	if config.UseMongo {
		coll := session.DB(config.MongoDBName).C(config.MongoCollectionName)
		coll.Insert(&result)
		if err != nil {
			log.Fatal(err)
		}
		println("\nReading dataz from mongo...")
		var previousResults []TestStatsEntry
		iter := coll.Find(nil).Limit(25).Sort("-Timestamp").Iter()
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
