package mhfpacket

import ( 
 "errors" 

 	"github.com/Solenataris/Erupe/network/clientctx"
	"github.com/Solenataris/Erupe/network"
	"github.com/Andoryuuta/byteframe"
)

// MsgSysReserve01 represents the MSG_SYS_reserve01
type MsgSysReserve01 struct{}

// Opcode returns the ID associated with this packet type.
func (m *MsgSysReserve01) Opcode() network.PacketID {
	return network.MSG_SYS_reserve01
}

// Parse parses the packet from binary
func (m *MsgSysReserve01) Parse(bf *byteframe.ByteFrame, ctx *clientctx.ClientContext) error {
	return errors.New("NOT IMPLEMENTED")
}

// Build builds a binary packet from the current data.
func (m *MsgSysReserve01) Build(bf *byteframe.ByteFrame, ctx *clientctx.ClientContext) error {
	return errors.New("NOT IMPLEMENTED")
}