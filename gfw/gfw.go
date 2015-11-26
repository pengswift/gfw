package gfw

import (
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pengswift/libonepiece/dirlock"
	"github.com/pengswift/libonepiece/protocol"
	"github.com/pengswift/libonepiece/util"
	"github.com/pengswift/libonepiece/version"
)

type errStore struct {
	err error
}

type GFW struct {
	clientIDSequence int64
	sync.RWMutex

	opts atomic.Value

	dl        *dirlock.DirLock
	errValue  atomic.Value
	startTime time.Time

	tcpListener net.Listener

	exitChan  chan int
	waitGroup util.WaitGroupWrapper
}

func New(opts *Options) *GFW {
	dataPath := opts.DataPath
	if opts.DataPath == "" {
		cwd, _ := os.Getwd()
		dataPath = cwd
	}

	g := &GFW{
		startTime: time.Now(),
		exitChan:  make(chan int),
		dl:        dirlock.New(dataPath),
	}
	g.swapOpts(opts)
	g.errValue.Store(errStore{})

	err := g.dl.Lock()
	if err != nil {
		g.Fatal("FATAL: --data-path=%s in use (possibly by another instance of gfw)", dataPath)
		os.Exit(1)
	}

	if opts.ID < 0 || opts.ID >= 1024 {
		g.Fatal("FATAL: --worker-id must be [0, 1024)")
		os.Exit(1)
	}

	g.Info(version.String("gfw", "0.0.1"))
	g.Info("ID: %d", opts.ID)
	return g
}

func (g *GFW) Debug(f string, args ...interface{}) {
	if g.getOpts().Logger == nil {
		return
	}
	g.getOpts().Logger.Debug(f, args)
}

func (g *GFW) Error(f string, args ...interface{}) {
	if g.getOpts().Logger == nil {
		return
	}
	g.getOpts().Logger.Error(f, args)
}

func (g *GFW) Info(f string, args ...interface{}) {
	if g.getOpts().Logger == nil {
		return
	}
	g.getOpts().Logger.Info(f, args)
}

func (g *GFW) Warn(f string, args ...interface{}) {
	if g.getOpts().Logger == nil {
		return
	}
	g.getOpts().Logger.Warn(f, args)
}

func (g *GFW) Fatal(f string, args ...interface{}) {
	if g.getOpts().Logger == nil {
		return
	}
	g.getOpts().Logger.Fatal(f, args)
}

func (g *GFW) getOpts() *Options {
	return g.opts.Load().(*Options)
}

func (g *GFW) swapOpts(opts *Options) {
	g.opts.Store(opts)
}

func (g *GFW) RealTCPAddr() *net.TCPAddr {
	g.RLock()
	defer g.RUnlock()
	return g.tcpListener.Addr().(*net.TCPAddr)
}

func (g *GFW) SetHealth(err error) {
	g.errValue.Store(errStore{err: err})
}

func (g *GFW) IsHealthy() bool {
	return g.GetError() == nil
}

func (g *GFW) GetError() error {
	errValue := g.errValue.Load()
	return errValue.(errStore).err
}

func (g *GFW) GetHealth() string {
	err := g.GetError()
	if err != nil {
		return fmt.Sprintf("NOK - %s", err)
	}
	return "OK"
}

func (g *GFW) GetStartTime() time.Time {
	return g.startTime
}

func (g *GFW) Main() {
	ctx := &context{g}

	tcpListener, err := net.Listen("tcp", g.getOpts().TCPAddress)
	if err != nil {
		g.Fatal("FATAL: listen (%s) failed - %s", g.getOpts().TCPAddress, err)
		os.Exit(1)
	}
	g.Lock()
	g.tcpListener = tcpListener
	g.Unlock()
	tcpServer := &tcpServer{ctx: ctx}
	g.waitGroup.Wrap(func() {
		protocol.TCPServer(g.tcpListener, tcpServer, g.getOpts().Logger)
	})
}

func (g *GFW) Exit() {
	if g.tcpListener != nil {
		g.tcpListener.Close()
	}

	close(g.exitChan)
	g.waitGroup.Wait()

	g.dl.Unlock()
}
