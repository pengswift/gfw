package gfw

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pengswift/libonepiece/protocol"
	"github.com/pengswift/protocol/pb"
)

const maxTimeout = time.Hour

var heartbeatBytes = []byte("_heartbeat_")

type protocolV1 struct {
	ctx *context
}

func (p *protocolV1) IOLoop(conn net.Conn) error {
	var err error
	var zeroTime time.Time

	clientID := atomic.AddInt64(&p.ctx.gfw.clientIDSequence, 1)
	client := newClientV1(clientID, conn, p.ctx)

	// 消息主动推送协程
	messagePumpStartedChan := make(chan bool)
	go p.messagePump(client, messagePumpStartedChan)
	<-messagePumpStartedChan

	for {
		if client.HeartbeatInterval > 0 {
			client.SetReadDeadline(time.Now().Add(client.HeartbeatInterval * 2))
		} else {
			client.SetReadDeadline(zeroTime)
		}

		msgid, response, err := p.Exec(client)
		if err != nil {
			ctx := ""
			if parentErr := err.(protocol.ChildErr).Parent(); parentErr != nil {
				ctx = " - " + parentErr.Error()
			}
			p.ctx.gfw.Error("ERROR: [%s] - %s%s", client, err, ctx)

			sendErr := p.Send(client, int32(onepiece.NetMsgID_ERROR), []byte(err.Error()))
			if sendErr != nil {
				break
			}

			if _, ok := err.(*protocol.FatalClientErr); ok {
				break
			}
			continue
		}

		if msgid < 100 {
			err = fmt.Errorf("msgid < 100 kill self")
			break
		}

		if response != nil {
			err = p.Send(client, msgid, response)
			if err != nil {
				err = fmt.Errorf("failed to send response - %s", err)
				break
			}
		}
	}

	p.ctx.gfw.Info("PROTOCOL(V1): [%s] exiting ioloop", client)
	conn.Close()
	close(client.ExitChan)

	return err
}

func (p *protocolV1) messagePump(client *clientV1, startedChan chan bool) {
	var err error
	var clientMsgChan chan *Message

	var flusherChan <-chan time.Time

	outputBufferTicker := time.NewTicker(client.OutputBufferTimeout)
	heartbeatTicker := time.NewTicker(client.HeartbeatInterval)
	heartbeatChan := heartbeatTicker.C

	flushed := true
	// 准备
	close(startedChan)
	for {

		if !client.IsReadyForMessages() {
			clientMsgChan = nil
			flusherChan = nil

			client.Lock()
			err = client.Flush()
			client.Unlock()
			if err != nil {
				goto exit
			}
			flushed = true
		} else if flushed {
			clientMsgChan = client.MsgChan
			flusherChan = nil
		} else {
			clientMsgChan = client.MsgChan
			flusherChan = outputBufferTicker.C
		}

		select {
		case <-flusherChan:
			client.Lock()
			err = client.Flush()
			client.Unlock()
			if err != nil {
				goto exit
			}
			flushed = true
		case <-client.ReadyStateChan:
		case <-heartbeatChan:
			err := p.Send(client, int32(onepiece.NetMsgID_HEART_BEAT), heartbeatBytes)
			if err != nil {
				goto exit
			}
		case msg, ok := <-clientMsgChan:
			if !ok {
				goto exit
			}

			if msg.ID < 100 {
				err = fmt.Errorf("msgid < 100 kill self")
				break
			}

			err := p.Send(client, msg.ID, msg.Body)
			if err != nil {
				goto exit
			}
			flushed = false
		case <-client.ExitChan:
			goto exit
		}
	}

exit:
	p.ctx.gfw.Info("PROTOCOL(V1): [%s] exiting messagePump", client)
	if err != nil {
		p.ctx.gfw.Info("PROTOCOL(V1): [%s] messagePump error - %s", client, err)
	}
}

func (p *protocolV1) Exec(client *clientV1) (int32, []byte, error) {
	bodyLen, err := readLen(client.Reader, client.lenSlice)
	if err != nil {
		if err == io.EOF {
			return 0, nil, nil
		}
		return 0, nil, protocol.NewFatalClientErr(nil, "E_INVALID", fmt.Sprintf("failed to read message body size %s", err))
	}

	// body 必须包含4个字节的消息ID
	if bodyLen < 4 {
		return 0, nil, protocol.NewFatalClientErr(nil, "E_INVALID", fmt.Sprintf("invalid message body size %d", bodyLen))
	}

	fullContent := make([]byte, bodyLen)

	_, err = io.ReadFull(client.Reader, fullContent)
	if err != nil {
		return 0, nil, protocol.NewFatalClientErr(nil, "E_INVALID", fmt.Sprintf("invalid message body size %d", len(fullContent)))
	}

	msgid := binary.BigEndian.Uint32(fullContent[:4])
	msgBody := fullContent[4:]

	switch onepiece.NetMsgID(msgid) {
	case onepiece.NetMsgID_SAYHI:
		return p.SayHi(client, msgBody)
	case onepiece.NetMsgID_HEART_BEAT:
		return p.HeartBeat(client, msgBody)
	case onepiece.NetMsgID_REGISTER:
		return p.Register(client, msgBody)
	case onepiece.NetMsgID_AUTO_REGISTER:
		return p.AutoRegister(client, msgBody)
	case onepiece.NetMsgID_LOGIN:
		return p.Login(client, msgBody)
	case onepiece.NetMsgID_LOGIN_WORLD:
		return p.LoginWorld(client, msgBody)
	case onepiece.NetMsgID_RANDOM_ROLE_NAME:
		return p.RandomRoleName(client, msgBody)
	case onepiece.NetMsgID_CREATE_ROLE:
		return p.CreateRole(client, msgBody)
	case onepiece.NetMsgID_DELETE_ROLE:
		return p.DeleteRole(client, msgBody)
	case onepiece.NetMsgID_CHOOSE_ROLE:
		return p.ChooseRole(client, msgBody)
	}

	return 0, nil, protocol.NewFatalClientErr(nil, "E_INVALID", fmt.Sprintf("invalid command by netmessageid %d", msgid))

}

func readLen(r io.Reader, tmp []byte) (int32, error) {
	_, err := io.ReadFull(r, tmp)
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(tmp)), nil
}

func (p *protocolV1) Send(client *clientV1, frameType int32, data []byte) error {
	client.Lock()
	var zeroTime time.Time
	if client.HeartbeatInterval > 0 {
		client.SetWriteDeadline(time.Now().Add(client.HeartbeatInterval))
	} else {
		client.SetWriteDeadline(zeroTime)
	}

	_, err := protocol.SendFramedResponse(client.Writer, frameType, data)
	if err != nil {
		client.Unlock()
		return err
	}

	// TODO 此处代议
	if frameType == int32(onepiece.NetMsgID_ERROR) {
		err = client.Flush()
	}

	client.Unlock()

	return err
}

func (p *protocolV1) SayHi(client *clientV1, body []byte) (int32, []byte, error) {
	if len(body) <= 0 {
		return 0, nil, protocol.NewClientErr(nil, "E_INVALID", fmt.Sprintf("invalid message body size %d in sayhi", len(body)))
	}

	sayHiReq := &onepiece.SayHiRequest{}
	err := proto.Unmarshal(body, sayHiReq)
	if err != nil {
		return 0, nil, protocol.NewFatalClientErr(nil, "E_INVALID", fmt.Sprintf("invalid unmarshaling err in sayhi"))

	}

	if sayHiReq.Msg != "hi" {
		return 0, nil, protocol.NewFatalClientErr(nil, "E_INVALID", fmt.Sprintf("invalid message body content in sayhi"))
	}

	sayHiRes := &onepiece.SayHiResponse{
		Msg: "叔叔不约!",
	}

	buffer, err := proto.Marshal(sayHiRes)
	if err != nil {
		return 0, nil, protocol.NewFatalClientErr(nil, "E_INVALID", fmt.Sprintf("invalid marshaling err in sayhi"))
	}

	return int32(onepiece.NetMsgID_SAYHI), buffer, nil
}

func (p *protocolV1) HeartBeat(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}

func (p *protocolV1) Register(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}

func (p *protocolV1) AutoRegister(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}

func (p *protocolV1) Login(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}

func (p *protocolV1) LoginWorld(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}

func (p *protocolV1) RandomRoleName(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}

func (p *protocolV1) CreateRole(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}

func (p *protocolV1) DeleteRole(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}

func (p *protocolV1) ChooseRole(client *clientV1, body []byte) (int32, []byte, error) {
	return 0, nil, nil
}
