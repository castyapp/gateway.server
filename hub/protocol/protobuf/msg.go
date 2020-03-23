package protobuf

import (
	"bytes"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/golang/protobuf/proto"
	"io"
	"net"
)

type IMsg interface {
	IsProto() bool
	GetMsgType() enums.EMSG
}

// Represents a protobuf backed client message with session data.
type ClientMsgProtobuf struct {
	Header *MsgHdrProtoBuf
	Body   proto.Message
}

func NewMsgProtobuf(eMsg enums.EMSG, body proto.Message) (buffer *bytes.Buffer, err error) {
	buffer = new(bytes.Buffer)
	msg   := NewClientMsgProtobuf(eMsg, body)
	if err := msg.Serialize(buffer); err != nil {
		return nil, err
	}
	return buffer,nil
}

func BrodcastMsgProtobuf(conn net.Conn, eMsg enums.EMSG, body proto.Message) (err error) {
	var (
		msg    = NewClientMsgProtobuf(eMsg, body)
		buffer = new(bytes.Buffer)
	)
	if err = msg.Serialize(buffer); err != nil {
		return
	}
	if err = wsutil.WriteServerMessage(conn, ws.OpBinary, buffer.Bytes()); err != nil {
		return
	}
	return
}

func NewClientMsgProtobuf(eMsg enums.EMSG, body proto.Message) *ClientMsgProtobuf {
	hdr := NewMsgHdrProtoBuf()
	hdr.Msg = eMsg
	return &ClientMsgProtobuf{
		Header: hdr,
		Body:   body,
	}
}

func (c *ClientMsgProtobuf) IsProto() bool {
	return true
}

func (c *ClientMsgProtobuf) GetMsgType() enums.EMSG {
	return NewEMsg(uint32(c.Header.Msg))
}

func (c *ClientMsgProtobuf) Serialize(w io.Writer) error {
	err := c.Header.Serialize(w)
	if err != nil {
		return err
	}
	if c.Body != nil {
		body, err := proto.Marshal(c.Body)
		if err != nil {
			return err
		}
		_, err = w.Write(body)
	}
	return err
}

