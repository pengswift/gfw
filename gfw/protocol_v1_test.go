package gfw

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pengswift/libonepiece/protocol"
	. "github.com/pengswift/libonepiece/test"
	"github.com/pengswift/protocol/pb"
)

func mustStartGFW(opts *Options) (*net.TCPAddr, *GFW) {
	opts.TCPAddress = "127.0.0.1:0"
	if opts.DataPath == "" {
		tmpDir, err := ioutil.TempDir("", fmt.Sprintf("gfw-test-%d", time.Now().UnixNano()))
		if err != nil {
			panic(err)
		}
		opts.DataPath = tmpDir
	}

	gfw := New(opts)
	gfw.Main()
	return gfw.RealTCPAddr(), gfw
}

func mustConnectGFW(tcpAddr *net.TCPAddr) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", tcpAddr.String(), time.Second)
	if err != nil {
		return nil, err
	}
	conn.Write([]byte("  V1"))
	return conn, nil
}

func TestSayHi(t *testing.T) {
	opts := NewOptions()
	opts.ClientTimeout = 60 * time.Second
	tcpAddr, gfw := mustStartGFW(opts)
	defer os.RemoveAll(opts.DataPath)
	defer gfw.Exit()

	conn, err := mustConnectGFW(tcpAddr)
	Equal(t, err, nil)
	defer conn.Close()

	reqSayHi := &onepiece.SayHiRequest{
		Msg: "hi",
	}
	buffer, err := proto.Marshal(reqSayHi)
	_, err = protocol.SendFramedResponse(conn, int32(onepiece.NetMsgID_SAYHI), buffer)
	Equal(t, err, nil)

	msgid, buffer, err := protocol.ReadUnpackedResponse(conn)
	Equal(t, err, nil)
	Equal(t, msgid, int32(onepiece.NetMsgID_SAYHI))

	resSayHi := &onepiece.SayHiResponse{}
	err = proto.Unmarshal(buffer, resSayHi)
	Equal(t, err, nil)
	Equal(t, resSayHi.Msg, "叔叔不约!")
}
