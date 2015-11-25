package gfw

import (
	"io"
	"net"

	"github.com/pengswift/libonepiece/protocol"
	"github.com/pengswift/protocol/pb"
)

type tcpServer struct {
	ctx *context
}

func (p *tcpServer) Handle(clientConn net.Conn) {
	p.ctx.gfw.Info("TCP: new client(%s)", clientConn.RemoteAddr())

	buf := make([]byte, 4)
	_, err := io.ReadFull(clientConn, buf)
	if err != nil {
		p.ctx.gfw.Error("ERROR: failed to read protocol version - %s", err)
	}
	protocolMagic := string(buf)

	p.ctx.gfw.Info("CLIENT(%s): desired protocol magic '%s'",
		clientConn.RemoteAddr(), protocolMagic)

	var prot protocol.Protocol
	switch protocolMagic {
	case "  V1":
		prot = &protocolV1{ctx: p.ctx}
	default:
		protocol.SendFramedResponse(clientConn, int32(onepiece.NetMsgID_ERROR), []byte("E_BAD_PROTOCOL"))
		clientConn.Close()
		p.ctx.gfw.Info("ERROR: client(%s) bad protocol magic '%s'",
			clientConn.RemoteAddr(), protocolMagic)
		return
	}

	err = prot.IOLoop(clientConn)
	if err != nil {
		p.ctx.gfw.Error("ERROR: client(%s) - %s", clientConn.RemoteAddr(), err)
		return
	}
}
