package protocol

import (
	"bytes"
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"movie.night.ws.server/hub/protocol/protobuf"
	"movie.night.ws.server/hub/protocol/protobuf/enums"
)

// Represents an incoming, partially unread message.
type Packet struct {
	EMsg        enums.EMSG
	IsProto     bool
	Data        []byte
}

func NewPacket(data []byte) (*Packet, error) {

	var rawEMsg uint32
	err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &rawEMsg)
	if err != nil {
		return nil, err
	}

	eMsg := protobuf.NewEMsg(rawEMsg)
	buf := bytes.NewReader(data)

	if protobuf.IsProto(rawEMsg) {

		header := protobuf.NewMsgHdrProtoBuf()
		header.Msg = eMsg
		err = header.Deserialize(buf)
		if err != nil {
			return nil, err
		}

		return &Packet{
			EMsg:        eMsg,
			IsProto:     true,
			Data:        data,
		}, nil
	}

	header := protobuf.NewMsgHdrProtoBuf()
	header.Msg = eMsg
	err = header.Deserialize(buf)
	if err != nil {
		return nil, err
	}
	return &Packet{
		EMsg:        eMsg,
		IsProto:     false,
		Data:        data,
	}, nil
}

func (p *Packet) ReadProtoMsg(body proto.Message) error {
	header := protobuf.NewMsgHdrProtoBuf()
	buf := bytes.NewBuffer(p.Data)
	if err := header.Deserialize(buf); err != nil {
		return err
	}
	if err := proto.Unmarshal(buf.Bytes(), body); err != nil {
		return err
	}
	return nil
}

//
//func (p *Packet) ReadClientMsg(body MessageBody) *ClientMsg {
//	header := NewExtendedClientMsgHdr()
//	buf := bytes.NewReader(p.Data)
//	header.Deserialize(buf)
//	body.Deserialize(buf)
//	payload := make([]byte, buf.Len())
//	buf.Read(payload)
//	return &ClientMsg{
//		Header:  header,
//		Body:    body,
//		Payload: payload,
//	}
//}
//
//func (p *Packet) ReadMsg(body MessageBody) *Msg {
//	header := NewMsgHdr()
//	buf := bytes.NewReader(p.Data)
//	header.Deserialize(buf)
//	body.Deserialize(buf)
//	payload := make([]byte, buf.Len())
//	buf.Read(payload)
//	return &Msg{
//		Header:  header,
//		Body:    body,
//		Payload: payload,
//	}
//}
