package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/somakeit/door-controller3/admitter"
	"github.com/somakeit/door-controller3/admitter/led"
	"github.com/somakeit/door-controller3/admitter/strike"
	"github.com/somakeit/door-controller3/auth/staticauth"
	"github.com/somakeit/door-controller3/contextlogger"
	"github.com/somakeit/door-controller3/guard"
	"github.com/somakeit/door-controller3/guard/nfc"
	"github.com/somakeit/door-controller3/guard/pin"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/mfrc522"
	"periph.io/x/host/v3"
	"periph.io/x/host/v3/rpi"
)

func main() {
	tags := flag.String("tags", "", "Comma separated list of allowed tags")
	level := flag.String("loglevel", "debug", "log level")
	gain := flag.Int("gain", 7, "antenna gain 0 to 7")
	delay := flag.Duration("delay", 2*time.Second, "artificial auth delay")
	flag.Parse()
	logLevel, err := logrus.ParseLevel(*level)
	if err != nil {
		log.Fatal("Invalid log level: ", err)
	}

	if *gain < 0 || *gain > 7 {
		log.Fatal("Invalid antenna gain.")
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

	reader, err := mfrc522.NewSPI(spi, rpi.P1_22, rpi.P1_16)
	if err != nil {
		log.Fatal("Failed to init reader: ", err)
	}
	if err := reader.SetAntennaGain(*gain); err != nil {
		log.Fatal("Failed to set antenna gain: ", err)
	}

	auth := &staticauth.Static{
		Delay: *delay,
		Allow: strings.Split(*tags, ","),
	}

	ctxLog := &contextlogger.ContextLogger{Logger: log}
	strike.Logger = ctxLog

	admitters := admitter.Mux{
		strike.New(rpi.P1_15),
		led.New(rpi.P1_18),
		ctxLog,
	}

	strikeGuard, err := nfc.New(1, "A", reader, auth, admitters)
	if err != nil {
		log.Fatal("Failed to init guard: ", err)
	}

	pin.Logger = ctxLog
	pinGuard := pin.New(os.Stdin, auth, 1, "A")

	log.Fatal(guard.Mux{
		strikeGuard,
		pinGuard,
	}.Guard())
}
