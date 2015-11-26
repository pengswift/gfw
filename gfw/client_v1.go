package gfw

import (
	"bufio"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const defaultBufferSize = 16 * 1024

const (
	stateInit = iota
	stateLogined
	stateEncrypted
	statePlanetLogined
	statePlanetLoginout
	stateClosing
)

type clientV1 struct {
	sync.RWMutex

	ID  int64
	ctx *context

	net.Conn

	Reader *bufio.Reader
	Writer *bufio.Writer

	OutputBufferSize    int
	OutputBufferTimeout time.Duration

	HeartbeatInterval time.Duration

	MsgTimeout time.Duration

	State          int32
	ConnectTime    time.Time
	ReadyStateChan chan int
	ExitChan       chan int

	Hostname string

	MsgChan chan *Message

	// re-usable buffer for reading the 4-byte lengths off the wire
	lenBuf   [4]byte
	lenSlice []byte
}

func newClientV1(id int64, conn net.Conn, ctx *context) *clientV1 {
	var identifier string
	if conn != nil {
		// TODO  同一路由传来的两个客户端 ip是否相等
		identifier, _, _ = net.SplitHostPort(conn.RemoteAddr().String())
	}

	c := &clientV1{
		ID:  id,
		ctx: ctx,

		Conn: conn,

		Reader: bufio.NewReaderSize(conn, defaultBufferSize),
		Writer: bufio.NewWriterSize(conn, defaultBufferSize),

		OutputBufferSize:    defaultBufferSize,
		OutputBufferTimeout: 250 * time.Millisecond,

		// TODO  异步发送消息的 timeout
		MsgTimeout: ctx.gfw.getOpts().MsgTimeout,

		ReadyStateChan: make(chan int, 1),
		ExitChan:       make(chan int),
		ConnectTime:    time.Now(),
		State:          stateInit,

		Hostname: identifier,

		MsgChan: make(chan *Message, 1),

		HeartbeatInterval: ctx.gfw.getOpts().ClientTimeout / 2,
	}
	c.lenSlice = c.lenBuf[:]
	return c
}

func (c *clientV1) String() string {
	return c.RemoteAddr().String()
}

func (c *clientV1) IsReadyForMessages() bool {
	// 登陆到星球后，方可回传消息
	if atomic.LoadInt32(&c.State) == statePlanetLogined {
		return true
	}
	return false
}

func (c *clientV1) tryUpdateReadState() {
	select {
	case c.ReadyStateChan <- 1:
	default:
	}
}

func (c *clientV1) StartClose() {
	atomic.StoreInt32(&c.State, stateClosing)
}

func (c *clientV1) Pause() {
	c.tryUpdateReadState()
}

func (c *clientV1) UnPause() {
	c.tryUpdateReadState()
}

// TODO 此处代议， 心跳周期内是否有效
func (c *clientV1) Flush() error {
	var zeroTime time.Time
	if c.HeartbeatInterval > 0 {
		c.SetWriteDeadline(time.Now().Add(c.HeartbeatInterval))
	} else {
		c.SetWriteDeadline(zeroTime)
	}

	err := c.Writer.Flush()
	if err != nil {
		return err
	}

	return nil
}
