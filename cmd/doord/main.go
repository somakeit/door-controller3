package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/somakeit/door-controller3/admitter"
	"github.com/somakeit/door-controller3/admitter/led"
	"github.com/somakeit/door-controller3/admitter/strike"
	"github.com/somakeit/door-controller3/auth/hms"
	"github.com/somakeit/door-controller3/contextlogger"
	"github.com/somakeit/door-controller3/guard"
	"github.com/somakeit/door-controller3/guard/nfc"
	"github.com/somakeit/door-controller3/guard/pin"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/mfrc522"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/rpi"
)

func main() {
	flag.Usage = func() {
		fmt.Println("doord [args]")
		fmt.Println("doord is an NFC door controller for So Make It.")
		flag.PrintDefaults()
		fmt.Print(`
Required raspberry pi pins:
  1  - MFRC522_3V3
  6  - MFRC522_Ground
  15 - Door strike/latch
  16 - MFRC522_IRQ
  18 - LED
  19 - MFRC522_MOSI
  21 - MFRC522_MISO
  22 - MFRC522_RST
  23 - MFRC522_SCK
  24 - MFRC522_SDA
`)
	}
	door := flag.Int("door", 0, "Numeric door ID, eg: 1")
	side := flag.String("side", "", "Door side, 'A' or 'B'")
	dsn := flag.String("hms", "", "The DSN for the HMS mysql database as per the Go database/sql package, eg: 'username:password@(host)/database'")
	openTime := flag.Int("opentime", 5, "Number of seconds to open the door for")
	activeHigh := flag.Bool("activehigh", true, "Strike/latch logic level")
	logFile := flag.String("logfile", "/var/log/doord/access.log", "Log file to use or - for STDOUT")
	level := flag.String("loglevel", "info", "log level")
	gain := flag.Int("gain", 5, "Antenna gain 0 to 7")
	flag.Parse()
	logLevel, err := logrus.ParseLevel(*level)
	if err != nil {
		fmt.Println("Invalid log level: ", err)
		flag.Usage()
		os.Exit(2)
	}
	if *door <= 0 {
		fmt.Println("Invalid door ID, a door ID greater than 0 must be provided")
		flag.Usage()
		os.Exit(2)
	}
	if *side != "A" && *side != "B" {
		fmt.Println("Invalid side, a side of either 'A' or 'B' must be provided")
		flag.Usage()
		os.Exit(2)
	}
	if *gain < 0 || *gain > 7 {
		fmt.Println("Invalid antenna gain, must be 0 to 7")
		flag.Usage()
		os.Exit(2)
	}

	log := logrus.StandardLogger()
	log.Level = logLevel
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	if *logFile != "-" {
		file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal("Cannot open log file: ", err)
		}
		defer file.Close()
		log.Out = file
	}
	log.Info("Stating doord")

	if _, err := host.Init(); err != nil {
		log.Fatal("Failed to init host: ", err)
	}

	spi, err := spireg.Open("")
	if err != nil {
		log.Fatal("Failed to open SPI: ", err)
	}

	reader, err := mfrc522.NewSPI(spi, rpi.P1_22, rpi.P1_16)
	if err != nil {
		log.Fatal("Failed to init reader: ", err)
	}
	if err := reader.SetAntennaGain(*gain); err != nil {
		log.Fatal("Failed to set antenna gain: ", err)
	}

	if err := mysql.SetLogger(log); err != nil {
		log.Fatal("Failed to set mysql logger: ", err)
	}
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatal("Failed to open database: ", err)
	}

	ctxLog := &contextlogger.ContextLogger{Logger: log}
	hms.Logger = ctxLog
	strike.Logger = ctxLog

	auth, err := hms.NewClient(db)
	if err != nil {
		log.Fatal("Failed to init hms:, ", err)
	}

	locked := gpio.Low
	if !*activeHigh {
		locked = gpio.High
	}
	if err := rpi.P1_15.Out(locked); err != nil {
		log.Fatal("Failed to pre-lock door: ", err)
	}

	doorStrike := strike.New(rpi.P1_15)
	doorStrike.OpenFor = time.Duration(*openTime) * time.Second
	if !*activeHigh {
		doorStrike.Logic = strike.ActiveLow
	}

	admitters := admitter.Mux{
		doorStrike,
		led.New(rpi.P1_18),
		ctxLog,
	}

	strikeGuard, err := nfc.New(int32(*door), *side, reader, auth, admitters)
	if err != nil {
		log.Fatal("Failed to init guard: ", err)
	}

	pin.Logger = ctxLog
	pinGuard := pin.New(os.Stdin, auth, int32(*door), *side)

	g := guard.Mux{
		strikeGuard,
		pinGuard,
	}

	log.Info("Ready")
	log.Fatal(g.Guard())
}
