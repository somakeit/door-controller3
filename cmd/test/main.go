package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/somakeit/door-controller3/admitter"
	"github.com/somakeit/door-controller3/admitter/led"
	"github.com/somakeit/door-controller3/admitter/strike"
	"github.com/somakeit/door-controller3/auth/staticauth"
	"github.com/somakeit/door-controller3/contextlogger"
	"github.com/somakeit/door-controller3/nfc"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/mfrc522"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/rpi"
)

func main() {
	tags := flag.String("tags", "", "Comma separated list of allowed tags")
	level := flag.String("loglevel", "warn", "log level (default: warn)")
	flag.Parse()
	logLevel, err := logrus.ParseLevel(*level)
	if err != nil {
		log.Fatal("Invalid log level: ", err)
	}

	log := logrus.StandardLogger()
	log.Level = logLevel
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	log.Info("Stating test door")

	if _, err := host.Init(); err != nil {
		log.Fatal("Failed to init host: ", err)
	}

	spi, err := spireg.Open("")
	if err != nil {
		log.Fatal("Failed to open SPI: ", err)
	}

	reader, err := mfrc522.NewSPI(spi, rpi.P1_22, rpi.P1_18)
	if err != nil {
		log.Fatal("Failed to init reader: ", err)
	}

	auth := &staticauth.Static{
		Delay: 2 * time.Second,
		Allow: strings.Split(*tags, ","),
	}

	ctxLog := &contextlogger.ContextLogger{Logger: log}
	strike.Logger = ctxLog

	admitters := admitter.Mux{
		strike.New(rpi.P1_15),
		led.New(rpi.P1_16),
		ctxLog,
	}

	guard, err := nfc.New(1, "A", reader, auth, admitters)
	if err != nil {
		log.Fatal("Failed to init guard: ", err)
	}

	log.Fatal(guard.Guard())
}
