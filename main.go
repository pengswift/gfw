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

	"github.com/pengswift/gfw"
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

}

func main() {

}
