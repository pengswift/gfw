package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mreiferson/go-options"

	"github.com/pengswift/gfw/gfw"
	"github.com/pengswift/libonepiece/version"
)

func gfwFlagset() *flag.FlagSet {
	flagSet := flag.NewFlagSet("gfw", flag.ExitOnError)

	// basic options
	flagSet.String("config", "", "path to config file")
	flagSet.Bool("version", false, "print version string")
	flagSet.Bool("verbose", false, "enable verbose logging")
	flagSet.Int64("worker-id", 0, "unique seed for message ID generation (int) in range [0, 4096) (will default to hash of hostname)")
	flagSet.String("tcp-address", "0.0.0.0:6100", "<addr>:<port> to listen on for TCP clients")

	// disk options
	flagSet.String("data-path", "", "path to store log")

	// msg and command options
	flagSet.String("msg-timeout", "60s", "duration to wait before auto-requeing a message")

	return flagSet
}

type config map[string]interface{}

func main() {
	flagSet := gfwFlagset()
	flagSet.Parse(os.Args[1:])

	rand.Seed(time.Now().UTC().UnixNano())

	if flagSet.Lookup("version").Value.(flag.Getter).Get().(bool) {
		fmt.Println(version.String("gfw", "0.0.1"))
		return
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	var cfg config
	configFile := flagSet.Lookup("config").Value.String()
	if configFile != "" {
		_, err := toml.DecodeFile(configFile, &cfg)
		if err != nil {
			log.Fatalf("ERROR: failed to load config file %s - %s", configFile, err.Error())
		}
	}

	opts := gfw.NewOptions()
	options.Resolve(opts, flagSet, cfg)
	gfw := gfw.New(opts)

	gfw.Main()
	<-signalChan
	gfw.Exit()
}
