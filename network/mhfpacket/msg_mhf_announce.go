package mhfpacket

import (
 "errors"

 	"github.com/Solenataris/Erupe/network/clientctx"
	"github.com/Solenataris/Erupe/network"
	"github.com/Andoryuuta/byteframe"
)

// MsgMhfAnnounce represents the MSG_MHF_ANNOUNCE
type MsgMhfAnnounce struct {
  Unk []byte
}

// Opcode returns the ID associated with this packet type.
func (m *MsgMhfAnnounce) Opcode() network.PacketID {
	return network.MSG_MHF_ANNOUNCE
}

// Parse parses the packet from binary
func (m *MsgMhfAnnounce) Parse(bf *byteframe.ByteFrame, ctx *clientctx.ClientContext) error {
  m.Unk = bf.DataFromCurrent()
  bf.Seek(int64(len(bf.Data()) - 2), 0)
	return nil
}

// Build builds a binary packet from the current data.
func (m *MsgMhfAnnounce) Build(bf *byteframe.ByteFrame, ctx *clientctx.ClientContext) error {
	return errors.New("NOT IMPLEMENTED")
}
