package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	config "github.com/D4-project/d4-golang-utils/config"
	"github.com/D4-project/d4-golang-utils/inputreader"
	"github.com/gomodule/redigo/redis"
	"github.com/robfig/cron/v3"
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

	Cmd struct {
		Command string `json:"command"`
	}
)

type DispatchContext struct {
	echo.Context
	// mean of registering
	cu chan chan string
	// mean of getting updates
	c chan string
	// mean of being kickedout
	gtfo chan chan string
}

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

	// Create webserver
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// clientUpdate is a chan of chan string
	clientUpdate := make(chan chan string, 0)
	// gtfoUpdate is a chan of chan string to remove
	gtfoUpdate := make(chan chan string, 4096)
	// store is a channel used to keep a daily log of events
	storeUpdate := make(chan string)

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := &DispatchContext{c, clientUpdate, make(chan string), gtfoUpdate}
			return next(cc)
		}
	})
	e.Static("/", "./build")
	e.File("/map/ne_50m_admin_0_scale_rank.geojson", "./map/ne_50m_admin_0_scale_rank.geojson")

	// Launch LPOP routine
	input := lpoper(src, sortie)

	// Launch the static file routine
	daily := store(storeUpdate, sortie)

	// Launch the dispatch routine
	go dispatch(input, clientUpdate, gtfoUpdate, storeUpdate, sortie)

	e.GET("/ws", send)

	// Set flushing task every day at midnight
	c := cron.New()
	c.AddFunc("@midnight", func() {
		logger.Println("Sending FLUSH command.")
		m := Cmd{Command: "flush"}
		b, _ := json.Marshal(m)
		input <- string(b)
		// emptying daily.json file
		err := daily.Truncate(0)
		if err!= nil {
			logger.Println(err)
		}
		daily.Sync()
		if err!= nil {
			logger.Println(err)
		}
	})
	c.Start()

	// Launch webserver
	e.Logger.Fatal(e.Start("127.0.0.1:1323"))

	logger.Println("Exiting")
}

func lpoper(src io.Reader, sortie chan os.Signal) chan string {
	// Create a scanner on input redis
	rateLimiter := time.Tick(1 * time.Second)
	c := make(chan string)
	go func() {
		for {
			select {
			case <-rateLimiter:
				sc := bufio.NewScanner(src)
				// input routine
				for sc.Scan() {
					c <- sc.Text()
				}
				if err := sc.Err(); err != nil {
					fmt.Sprintln("reading redis:", err)
				}
				// Exit signal
			case <-sortie:
				logger.Println("Exiting")
				os.Exit(0)
			}
		}
	}()
	return c
}

func dispatch(input <-chan string, clientsUpdate chan chan string, gtfoUpdate chan chan string, storeUpdate chan string, sortie chan os.Signal) {
	logger.Println("Loading dispatcher.")
	// client R is a slice containing the clients
	clientR := make([]chan string, 0)
	for {
		select {
		case client := <-clientsUpdate:
			// new client connected, add it to the registry
			clientR = append(clientR, client)
			logger.Println("New client, Number of clients:", len(clientR))
		case i := <-input:
			// input is sent to the connected clients
			for j, c := range clientR {
				logger.Printf("CLIENT: %d", j)
				c <- i
			}
			// input is sent to the static file store
			storeUpdate <- i
		case gtfo := <-gtfoUpdate:
			// Rebuild clientR without the gtfo
			tmp := make([]chan string, 0)
			for _, r := range clientR {
				if r != gtfo {
					tmp = append(tmp, r)
				}
			}
			clientR = tmp
			// Exit signal
		case <-sortie:
			logger.Println("Exiting")
			os.Exit(0)
		}
	}
}

func (d *DispatchContext) Register() {
	println("Registering new client")
	logger.Println("Registering new client %p", d.c)
	d.cu <- d.c
}

func (d *DispatchContext) Remove() {
	println("Removing disconnected client")
	logger.Println("Removing disconnected client %p", d.c)
	d.gtfo <- d.c
}

func send(c echo.Context) error {
	cc := c.(*DispatchContext)
	cc.Register()
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		for msg := range cc.c {
			logger.Printf("Sending %q to %q", msg, c.Request().UserAgent())
			// Write in the websocket
			err := websocket.Message.Send(ws, msg)
			if err != nil {
				// Add client the the gtfo list (disconnected)
				cc.Remove()
				c.Logger().Error(err)
			}
		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func store(storeUpdate chan string, sortie chan os.Signal) *os.File {
	// Create the daily file
	// O_APPEND int = syscall.O_APPEND // append data to the file when writing.
	// needed to move fd @ 0 after daily truncate(0)
	// O_SYNC   int = syscall.O_SYNC   // open for synchronous I/O.
	file, err := os.OpenFile("./build/daily.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0644)
	if err != nil {
		logger.Println("Could not create daily.json", err)
	}
	go func() {
		if err != nil {
			logger.Printf("Could not create static/daily.json")
		}
		defer file.Close()

		w := bufio.NewWriter(file)
		for {
			select {
			case msg := <-storeUpdate:
				// Strip out command from the daily file
				var cmd Cmd
				err := json.Unmarshal([]byte(msg), &cmd)
				if err != nil {
					_, err := w.WriteString(msg)
					if err != nil {
						logger.Println(err)
						break
					}
					err = w.WriteByte('\n')
					if err != nil {
						logger.Println(err)
						break
					}
					err = w.Flush() // we have O_SYNC but older linux kernels are silly
					if err != nil {
						logger.Println(err)
						break
					}
					err = file.Sync()
					if err != nil {
						logger.Println(err)
						break
					}
				}
			case <-sortie:
				logger.Println("Exiting")
				os.Exit(0)
			}
		}
	}()
	return file
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
