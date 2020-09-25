package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	config "github.com/D4-project/d4-golang-utils/config"
	"github.com/D4-project/d4-golang-utils/inputreader"
	"github.com/gomodule/redigo/redis"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"golang.org/x/net/websocket"
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
	confdir        = flag.String("c", "conf.sample", "configuration directory")
	port           = flag.String("port", "80", "http server port")
	buf            bytes.Buffer
	logger         = log.New(&buf, "INFO: ", log.Lshortfile)
	redisCon       redis.Conn
	redisInputPool *redis.Pool
	src            io.Reader
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

	// Create a new redis connection pool
	redisInputPool = newPool(rd4.redisHost+":"+rd4.redisPort, 16)
	redisCon, err = redisInputPool.Dial()
	if err != nil {
		logger.Fatal("Could not connect to d4 Redis")
	}
	// Create the Redis Reader from which we will get the data from
	src, err = inputreader.NewLPOPReader(&redisCon, rd4.redisDB, rd4.redisQueue)
	if err != nil {
		logger.Fatal("Could not create d4 Redis Descriptor %q \n", err)
	}

	// Create the dispatching channel
	dispatch := make(chan string)
	// Create a scanner
	scanner := bufio.NewScanner(src)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Static("/", "./public")
	e.GET("/ws", send(dispatch))

	// Launch LPOP routine
	go func(co chan<- string, sc *bufio.Scanner) {
		for sc.Scan() {
			fmt.Print(sc.Text())
			co <- sc.Text()
		}
	}(dispatch, scanner)

	e.Logger.Fatal(e.Start(":1323"))
	logger.Println("Exiting")
}

func send(co chan string) echo.HandlerFunc {
	return func(c echo.Context) error {
		websocket.Handler(func(ws *websocket.Conn) {
			defer ws.Close()
			for {
				msg := <-co
				fmt.Print(msg)
				// Write
				err := websocket.Message.Send(ws, msg)
				if err != nil {
					c.Logger().Error(err)
				}
			}
		}).ServeHTTP(c.Response(), c.Request())
		return nil
	}
}

func newPool(addr string, maxconn int) *redis.Pool {
	return &redis.Pool{
		MaxActive:   maxconn,
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		// Dial or DialContext must be set. When both are set, DialContext takes precedence over Dial.
		Dial: func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}
}
