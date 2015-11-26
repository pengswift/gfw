package gfw

import (
	"crypto/md5"
	"fmt"
	"github.com/pengswift/libonepiece/logger"
	"hash/crc32"
	"io"
	"os"
	"strings"
	"time"
)

type Options struct {
	// basic options
	ID         int64  `flag:"worker-id" cfg:"id"`
	Verbose    bool   `flag:"verbose"`
	TCPAddress string `flag:"tcp-address"`

	// disk options
	DataPath string `flag"data-path"`

	// msg and command options
	MsgTimeout    time.Duration `flag:"msg-timeout" arg:"1ms"`
	ClientTimeout time.Duration

	Logger logger.Logger
}

func getLoggerDataPath() (path string) {
	paths := strings.Split(os.Getenv("GOPATH"), ":")
	for k := range paths {
		path = paths[k] + "/config/logger.xml"
		_, err := os.Lstat(path)
		if err == nil {
			return
		}
	}
	return
}

func NewOptions() *Options {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	h := md5.New()
	io.WriteString(h, hostname)
	defaultID := int64(crc32.ChecksumIEEE(h.Sum(nil)) % 1024)

	l := make(logger.Logger)

	logPath := getLoggerDataPath()
	if logPath == "" {
		fmt.Println("loggerpath does not exist")
		os.Exit(1)
	}

	l.LoadConfiguration(logPath)

	return &Options{
		ID:            defaultID,
		TCPAddress:    "0.0.0.0:6710",
		ClientTimeout: 60 * time.Second,
		Logger:        l,
	}
}
