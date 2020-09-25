package main

import (
	"bytes"
	"flag"
	"fmt"
	config "github.com/D4-project/d4-golang-utils/config"
	"github.com/gomodule/redigo/redis"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	//	_ "github.com/gomodule/redigo/redis"
)

type (
	// Input is a grok - NIFI or Logstash
	redisconfD4 struct {
		redisHost  string
		redisPort  string
		redisDB    int
		redisQueue string
	}
)

// Setting up Flags
var (
	confdir = flag.String("c", "conf.sample", "configuration directory")
	port    = flag.String("port", "80", "http server port")
	buf     bytes.Buffer
	logger  = log.New(&buf, "INFO: ", log.Lshortfile)
	redisD4 redis.Conn
)

func main() {

	// Setting up log file
	f, err := os.OpenFile("pewpew.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	logger.SetOutput(f)
	logger.SetFlags(log.LstdFlags | log.Lshortfile)
	logger.Println("Init")

	// Setting up Graceful killing
	sortie := make(chan os.Signal, 1)
	signal.Notify(sortie, os.Interrupt, os.Kill)
	// Signal goroutine
	go func() {
		<-sortie
		logger.Println("Exiting")
		os.Exit(0)
	}()

	// Usage
	flag.Usage = func() {
		fmt.Printf("d4 - analyzer-d4-pewpew\n")
		fmt.Printf("Serve a realtime pewpew map over HTML / WebSocket\n")
		fmt.Printf("\n")
		fmt.Printf("Usage: analyzer-d4-pewpew -c config_directory\n")
		fmt.Printf("\n")
		fmt.Printf("Configuration\n\n")
		fmt.Printf("The configuration settings are stored in files in the configuration directory\n")
		fmt.Printf("specified with the -c command line switch.\n\n")
		fmt.Printf("Files in the configuration directory\n")
		fmt.Printf("\n")
		fmt.Printf("redis_queue - queueuuid to pop\n")
		fmt.Printf("redis_input - host:port/db\n")
		fmt.Printf("\n")
		flag.PrintDefaults()
	}

	// Parse Flags
	flag.Parse()
	if flag.NFlag() == 0 || *confdir == "" {
		flag.Usage()
		sortie <- os.Kill
	} else {
		*confdir = strings.TrimSuffix(*confdir, "/")
		*confdir = strings.TrimSuffix(*confdir, "\\")
	}

	rd4 := redisconfD4{}
	// Parse Input Redis Config
	tmp := config.ReadConfigFile(*confdir, "redis_input")
	ss := strings.Split(string(tmp), "/")
	if len(ss) <= 1 {
		log.Fatal("Missing Database in Redis input config: should be host:port/database_name")
	}
	rd4.redisDB, _ = strconv.Atoi(ss[1])
	var ret bool
	ret, ss[0] = config.IsNet(ss[0])
	if ret {
		sss := strings.Split(string(ss[0]), ":")
		rd4.redisHost = sss[0]
		rd4.redisPort = sss[1]
	} else {
		log.Fatal("Redis config error.")
	}

	rd4.redisQueue = string(config.ReadConfigFile(*confdir, "redis_queue"))
	// Connect to D4 Redis
	redisD4, err = redis.Dial("tcp", rd4.redisHost+":"+rd4.redisPort, redis.DialDatabase(rd4.redisDB))
	if err != nil {
		logger.Fatal(err)
	}
	defer redisD4.Close()

	logger.Println("Exiting")
}
