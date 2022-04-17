package channelserver

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Andoryuuta/byteframe"
	"github.com/Solenataris/Erupe/common/bfutil"
	"github.com/Solenataris/Erupe/network/mhfpacket"
	"github.com/Solenataris/Erupe/server/channelserver/compression/deltacomp"
	"github.com/Solenataris/Erupe/server/channelserver/compression/nullcomp"
	"go.uber.org/zap"
)

func handleMsgMhfSavedata(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfSavedata)
	characterSaveData, err := GetCharacterSaveData(s, s.charID)
	if err != nil {
		s.logger.Error("failed to retrieve character save data from db", zap.Error(err), zap.Uint32("charID", s.charID))
		return
	}
	// Var to hold the decompressed savedata for updating the launcher response fields.
	var decompressedData []byte
	if pkt.SaveType == 1 {
		// Diff-based update.
		// diffs themselves are also potentially compressed
		diff, err := nullcomp.Decompress(pkt.RawDataPayload)
		if err != nil {
			s.logger.Fatal("Failed to decompress diff", zap.Error(err))
		}
		// Perform diff.
		s.logger.Info("Diffing...")
		characterSaveData.SetBaseSaveData(deltacomp.ApplyDataDiff(diff, characterSaveData.BaseSaveData()))
	} else {
		// Regular blob update.
		saveData, err := nullcomp.Decompress(pkt.RawDataPayload)
		if err != nil {
			s.logger.Fatal("Failed to decompress savedata from packet", zap.Error(err))
		}
		s.logger.Info("Updating save with blob")
		characterSaveData.SetBaseSaveData(saveData)
	}
	characterSaveData.IsNewCharacter = false
	characterBaseSaveData := characterSaveData.BaseSaveData()
	// Make a copy for updating the launcher fields.
	decompressedData = make([]byte, len(characterBaseSaveData))
	copy(decompressedData, characterBaseSaveData)
	err = characterSaveData.Save(s, nil)
	if err != nil {
		s.logger.Fatal("Failed to update savedata in db", zap.Error(err))
	}
	s.logger.Info("Wrote recompressed savedata back to DB.")
	dumpSaveData(s, pkt.RawDataPayload, "")

	_, err = s.server.db.Exec("UPDATE characters SET weapon_type=$1 WHERE id=$2", uint16(decompressedData[128789]), s.charID)
	if err != nil {
		s.logger.Fatal("Failed to character weapon type in db", zap.Error(err))
	}

	isMale := uint8(decompressedData[80]) // 0x50
	if isMale == 1 {
		_, err = s.server.db.Exec("UPDATE characters SET is_female=true WHERE id=$1", s.charID)
	} else {
		_, err = s.server.db.Exec("UPDATE characters SET is_female=false WHERE id=$1", s.charID)
	}
	if err != nil {
		s.logger.Fatal("Failed to character gender in db", zap.Error(err))
	}

	weaponId := binary.LittleEndian.Uint16(decompressedData[128522:128524]) // 0x1F60A
	_, err = s.server.db.Exec("UPDATE characters SET weapon_id=$1 WHERE id=$2", weaponId, s.charID)
	if err != nil {
		s.logger.Fatal("Failed to character weapon id in db", zap.Error(err))
	}

	hrp := binary.LittleEndian.Uint16(decompressedData[130550:130552]) // 0x1FDF6
	_, err = s.server.db.Exec("UPDATE characters SET hrp=$1 WHERE id=$2", hrp, s.charID)
	if err != nil {
		s.logger.Fatal("Failed to update character hrp in db", zap.Error(err))
	}

	grp := binary.LittleEndian.Uint32(decompressedData[130556:130560]) // 0x1FDFC
	var gr uint16
	if grp > 0 {
		gr = grpToGR(grp)
	} else {
		gr = 0
	}
	_, err = s.server.db.Exec("UPDATE characters SET gr=$1 WHERE id=$2", gr, s.charID)
	if err != nil {
		s.logger.Fatal("Failed to update character gr in db", zap.Error(err))
	}

	characterName := s.clientContext.StrConv.MustDecode(bfutil.UpToNull(decompressedData[88:100]))
	_, err = s.server.db.Exec("UPDATE characters SET name=$1 WHERE id=$2", characterName, s.charID)
	if err != nil {
		s.logger.Fatal("Failed to update character name in db", zap.Error(err))
	}
	doAckSimpleSucceed(s, pkt.AckHandle, []byte{0x00, 0x00, 0x00, 0x00})
}

func grpToGR(n uint32) uint16 {
	var gr uint16
	gr = 1
	switch grp := int(n); {
	case grp < 208750: // Up to 50
		i := 0
		for {
			grp -= 500
			if grp <= 500 {
				if grp < 0 {
					i--
				}
				break
			} else {
				i++
				for j := 0; j < i; j++ {
					grp -= 150
				}
			}
		}
		gr = uint16(i + 2)
		break
	case grp < 593400: // 51-99
		grp -= 208750
		i := 51
		for {
			if grp < 7850 {
				break
			}
			i++
			grp -= 7850
		}
		gr = uint16(i)
		break
	case grp < 993400: // 100-149
		grp -= 593400
		i := 100
		for {
			if grp < 8000 {
				break
			}
			i++
			grp -= 8000
		}
		gr = uint16(i)
		break
	case grp < 1400900: // 150-199
		grp -= 993400
		i := 150
		for {
			if grp < 8150 {
				break
			}
			i++
			grp -= 8150
		}
		gr = uint16(i)
		break
	case grp < 2315900: // 200-299
		grp -= 1400900
		i := 200
		for {
			if grp < 9150 {
				break
			}
			i++
			grp -= 9150
		}
		gr = uint16(i)
		break
	case grp < 3340900: // 300-399
		grp -= 2315900
		i := 300
		for {
			if grp < 10250 {
				break
			}
			i++
			grp -= 10250
		}
		gr = uint16(i)
		break
	case grp < 4505900: // 400-499
		grp -= 3340900
		i := 400
		for {
			if grp < 11650 {
				break
			}
			i++
			grp -= 11650
		}
		gr = uint16(i)
		break
	case grp < 5850900: // 500-599
		grp -= 4505900
		i := 500
		for {
			if grp < 13450 {
				break
			}
			i++
			grp -= 13450
		}
		gr = uint16(i)
		break
	case grp < 7415900: // 600-699
		grp -= 5850900
		i := 600
		for {
			if grp < 15650 {
				break
			}
			i++
			grp -= 15650
		}
		gr = uint16(i)
		break
	case grp < 9230900: // 700-799
		grp -= 7415900
		i := 700
		for {
			if grp < 18150 {
				break
			}
			i++
			grp -= 18150
		}
		gr = uint16(i)
		break
	case grp < 11345900: // 800-899
		grp -= 9230900
		i := 800
		for {
			if grp < 21150 {
				break
			}
			i++
			grp -= 21150
		}
		gr = uint16(i)
		break
	default: // 900+
		grp -= 11345900
		i := 900
		for {
			if grp < 23950 {
				break
			}
			i++
			grp -= 23950
		}
		gr = uint16(i)
		break
	}
	return gr
}

func dumpSaveData(s *Session, data []byte, suffix string) {
	if !s.server.erupeConfig.DevModeOptions.SaveDumps.Enabled {
		return
	} else {
		dir := filepath.Join(s.server.erupeConfig.DevModeOptions.SaveDumps.OutputDir, fmt.Sprintf("%s_", s.Name))
		path := filepath.Join(s.server.erupeConfig.DevModeOptions.SaveDumps.OutputDir, fmt.Sprintf("%s_", s.Name), fmt.Sprintf("%d_%s_%s%s.bin", s.charID, s.Name, Time_Current().Format("2006-01-02_15.04.05"), suffix))

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			os.Mkdir(dir, os.ModeDir)
		}
		err := ioutil.WriteFile(path, data, 0644)
		if err != nil {
			s.logger.Fatal("Error dumping savedata", zap.Error(err))
		}
	}
}

func handleMsgMhfLoaddata(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfLoaddata)
	overrideFile := filepath.Join(".", "bin", "save_override.bin")
	var data []byte

	if _, err := os.Stat(overrideFile); err == nil {
		file, err := os.Open(overrideFile)
		if err != nil {
			panic(err)
		}
		data, err := ioutil.ReadAll(file)
		if err != nil {
			panic(err)
		}
		doAckBufSucceed(s, pkt.AckHandle, data)
	}

	err := s.server.db.QueryRow("SELECT savedata FROM characters WHERE id = $1", s.charID).Scan(&data)
	if err != nil {
		s.logger.Fatal("Failed to get savedata from db", zap.Error(err))
	}
	doAckBufSucceed(s, pkt.AckHandle, data)
}

func handleMsgMhfSaveScenarioData(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfSaveScenarioData)

	_, err := s.server.db.Exec("UPDATE characters SET scenariodata = $1 WHERE characters.id = $2", pkt.RawDataPayload, int(s.charID))
	if err != nil {
		s.logger.Fatal("Failed to update scenario data in db", zap.Error(err))
	}

	// Do this ack manually because it uses a non-(0|1) error code
	s.QueueSendMHF(&mhfpacket.MsgSysAck{
		AckHandle:        pkt.AckHandle,
		IsBufferResponse: false,
		ErrorCode:        0x40,
		AckData:          []byte{0x00, 0x00, 0x00, 0x40},
	})
}

func handleMsgMhfLoadScenarioData(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfLoadScenarioData)
	var scenarioData []byte
	bf := byteframe.NewByteFrame()
	err := s.server.db.QueryRow("SELECT scenariodata FROM characters WHERE characters.id = $1", int(s.charID)).Scan(&scenarioData)
	if err != nil {
		s.logger.Fatal("Failed to get scenario data contents in db", zap.Error(err))
	} else {
		if len(scenarioData) == 0 {
			bf.WriteUint32(0x00)
		} else {
			bf.WriteBytes(scenarioData)
		}
	}
	doAckBufSucceed(s, pkt.AckHandle, bf.Data())
}

func handleMsgMhfGetPaperData(s *Session, p mhfpacket.MHFPacket) {
	// if the game gets bad responses for this it breaks the ability to save
	pkt := p.(*mhfpacket.MsgMhfGetPaperData)
	var data []byte
	var err error
	if pkt.Unk2 == 4 {
		data, err = hex.DecodeString("0A218EAD000000000000000000000000")
	} else if pkt.Unk2 == 5 {
		data, err = hex.DecodeString("0A218EAD00000000000000000000003403E900010000000000000000000003E900020000000000000000000003EB00010064006400C80064000003EB00020096006400F00064000003EC000A270F002800000000000003ED000A01F4000000000000000003EF00010000000000000000000003F000C801900BB801900BB8000003F200010FA0000000000000000003F200020FA0000000000000000003F3000117703A984E2061A8753003F3000217703A984E2061A8753003F400011F40445C57E46B6C791803F400021F40445C57E46B6C791803F700010010001000100000000003F7000200100010001000000000044D000107E001F4000000000000044D000207E001F4000000000000044F0001000000000BB800000BB8044F0002000000000BB800000BB804500001000A270F00280000000004500002000A270F00280000000004510001000A01F400000000000004510002000A01F400000000000007D100010011003A0000000602BC07D100010014003A0000000300C807D100010016003A0000000700FA07D10001001B003A00000001006407D100010035003A0000000803E807D100010043003A0000000901F407D100010044003A00000002009607D10001004A003A0000000400C807D10001004B003A0000000501F407D10001004C003A0000000A032007D100010050003A0000000B038407D100010059003A0000000C025807D100020011003C0000000602BC07D100020014003C0000000300C807D100020016003C00000007015E07D10002001B003C00000001006407D100020027003C0000000D00C807D100020028003C0000000F025807D100020035003C0000000803E807D100020043003C0000000201F407D100020044003C00000009009607D10002004A003C0000000400C807D10002004B003C0000000501F407D10002004C003C0000000A032007D100020050003C0000000B038407D100020051003C0000000E038407D100020059003C0000000C025807D10002005E003C0000001003E8")
	} else if pkt.Unk2 == 6 {
		data, err = hex.DecodeString("0A218EAD0000000000000000000001A503EA00640000000000000000000003EE00012710271000000000000003EE000227104E2000000000000003F100140000000000000000000003F5000100010001006400C8012C03F5000100010002006400C8012C03F5000100020001012C006400C803F5000100020002012C006400C803F500010003000100C8012C006403F500010003000200C8012C006403F5000200010001012C006400C803F5000200010002012C006400C803F500020002000100C8012C006403F500020002000200C8012C006403F5000200030001006400C8012C03F5000200030002006400C8012C03F500030001000100C8012C006403F500030001000200C8012C006403F5000300020001006400C8012C03F5000300020002006400C8012C03F5000300030001012C006400C803F5000300030002012C006400C803F800010001005000000000000003F800010002005000000000000003F800010003005000000000000003F800020001005000000000000003F800020002005000000000000003F800020003005000000000000004B10001003C003200000000000004B10002003C003200000000000004B200010000000500320000000004B2000100060014003C0000000004B200010015002800460000000004B200010029007800500000000004B20001007900A0005A0000000004B2000100A100FA00640000000004B2000100FB01F400640000000004B2000101F5270F00640000000004B200020000006400640000000004B20002006500C800640000000004B2000200C901F400960000000004B2000201F5270F00960000000004B3000100000005000A0000000004B300010006000A00140000000004B30001000B001E001E0000000004B30001001F003C00280000000004B30001003D007800320000000004B3000100790082003C0000000004B300010083008C00460000000004B30001008D009600500000000004B30001009700A000550000000004B3000100A100C800640000000004B3000100C901F400640000000004B3000101F5270F00640000000004B300020000007800460000000004B30002007901F400780000000004B3000201F5270F00780000000004B4000100000005000F0000000004B400010006000A00140000000004B40001000B000F00190000000004B4000100100014001B0000000004B4000100150019001E0000000004B40001001A001E00200000000004B40001001F002800230000000004B400010029003200250000000004B400010033003C00280000000004B40001003D0046002B0000000004B4000100470050002D0000000004B400010051005A002F0000000004B40001005B006400320000000004B400010065006E003C0000000004B40001006F007800460000000004B4000100790082004B0000000004B400010083008C00520000000004B40001008D00A000550000000004B4000100A100C800640000000004B4000100C901F400640000000004B4000101F5270F00640000000004B400020000007800460000000004B40002007901F400780000000004B4000201F5270F0078000000000FA10001000000000000000000000FA10002000029AB0005000000010FA10002000029AB0005000000010FA10002000029AB0005000000010FA10002000029AB0005000000010FA10002000029AC0002000000010FA10002000029AC0002000000010FA10002000029AC0002000000010FA10002000029AC0002000000010FA10002000029AD0001000000010FA10002000029AD0001000000010FA10002000029AD0001000000010FA10002000029AD0001000000010FA10002000029AF0003000000010FA10002000029AF0003000000010FA10002000029AF0003000000010FA10002000029AF0003000000010FA10002000028900001000000010FA10002000028900001000000010FA10002000029AE0002000000010FA10002000029AE0002000000010FA10002000029BA0002000000010FA10002000029BB0002000000010FA10002000029B60001000000010FA10002000029B60001000000010FA5000100002B970001138800010FA5000100002B9800010D1600010FA5000100002B99000105DC00010FA5000100002B9A0001006400010FA5000100002B9B0001003200010FA5000200002B970002070800010FA5000200002B98000204B000010FA5000200002B99000201F400010FA5000200002B9A0001003200010FA5000200002B1D0001009600010FA5000200002B1E0001009600010FA5000200002B240001009600010FA5000200002B310001009600010FA5000200002B330001009600010FA5000200002B470001009600010FA5000200002B5A0001009600010FA5000200002B600001009600010FA5000200002B6D0001009600010FA5000200002B780001009600010FA5000200002B7D0001009600010FA5000200002B810001009600010FA5000200002B870001009600010FA5000200002B7C0001009600010FA5000200002B1F0001009600010FA5000200002B200001009600010FA5000200002B290001009600010FA5000200002B350001009600010FA5000200002B370001009600010FA5000200002B450001009600010FA5000200002B5B0001009600010FA5000200002B610001009600010FA5000200002B790001009600010FA5000200002B7A0001009600010FA5000200002B7B0001009600010FA5000200002B830001009600010FA5000200002B890001009600010FA5000200002B580001009600010FA5000200002B210001009600010FA5000200002B270001009600010FA5000200002B2E0001009600010FA5000200002B390001009600010FA5000200002B3C0001009600010FA5000200002B430001009600010FA5000200002B5C0001009600010FA5000200002B620001009600010FA5000200002B6F0001009600010FA5000200002B7F0001009600010FA5000200002B800001009600010FA5000200002B820001009600010FA5000200002B500001009600010FA50002000028820001009600010FA50002000028800001009600010FA6000100002B970001138800010FA6000100002B9800010D1600010FA6000100002B99000105DC00010FA6000100002B9A0001006400010FA6000100002B9B0001003200010FA6000200002B970002070800010FA6000200002B98000204B000010FA6000200002B99000201F400010FA6000200002B9A0001003200010FA6000200002B1D0001009600010FA6000200002B1E0001009600010FA6000200002B240001009600010FA6000200002B310001009600010FA6000200002B330001009600010FA6000200002B470001009600010FA6000200002B5A0001009600010FA6000200002B600001009600010FA6000200002B6D0001009600010FA6000200002B780001009600010FA6000200002B7D0001009600010FA6000200002B810001009600010FA6000200002B870001009600010FA6000200002B7C0001009600010FA6000200002B1F0001009600010FA6000200002B200001009600010FA6000200002B290001009600010FA6000200002B350001009600010FA6000200002B370001009600010FA6000200002B450001009600010FA6000200002B5B0001009600010FA6000200002B610001009600010FA6000200002B790001009600010FA6000200002B7A0001009600010FA6000200002B7B0001009600010FA6000200002B830001009600010FA6000200002B890001009600010FA6000200002B580001009600010FA6000200002B210001009600010FA6000200002B270001009600010FA6000200002B2E0001009600010FA6000200002B390001009600010FA6000200002B3C0001009600010FA6000200002B430001009600010FA6000200002B5C0001009600010FA6000200002B620001009600010FA6000200002B6F0001009600010FA6000200002B7F0001009600010FA6000200002B800001009600010FA6000200002B820001009600010FA6000200002B500001009600010FA60002000028820001009600010FA60002000028800001009600010FA7000100002B320001004600010FA7000100002B340001004600010FA7000100002B360001004600010FA7000100002B380001004600010FA7000100002B3A0001004600010FA7000100002B6E0001004600010FA7000100002B700001004600010FA7000100002B660001004600010FA7000100002B680001004600010FA7000100002B6A0001004600010FA7000100002B220001004600010FA7000100002B230001004600010FA7000100002B420001004600010FA7000100002B840001004600010FA7000100002B3B0001004600010FA7000100002B280001004600010FA7000100002B260001004600010FA7000100002B5F0001004600010FA7000100002B630001004600010FA7000100002B640001004600010FA7000100002B710001004600010FA7000100002B7E0001004600010FA7000100002B4C0001004600010FA7000100002B4D0001004600010FA7000100002B4E0001004600010FA7000100002B4F0001004600010FA7000100002B560001004600010FA7000100002B570001004600010FA70001000028860001004600010FA70001000028870001004600010FA70001000028880001004600010FA70001000028890001004600010FA700010000288A0001004600010FA7000100002B3D0001002D00010FA7000100002B3F0001002D00010FA7000100002B410001002D00010FA7000100002B440001002D00010FA7000100002B460001002D00010FA7000100002B6C0001002D00010FA7000100002B730001002D00010FA7000100002B770001002D00010FA7000100002B860001002D00010FA7000100002B300001002D00010FA7000100002B520001002D00010FA7000100002B590001002D00010FA700010000287F0001002D00010FA70001000028830001002D00010FA70001000028850001002D00010FA7000100002B480001000F00010FA7000100002B490001000F00010FA7000100002B4B0001000F00010FA7000100002B750001000F00010FA7000100002B550001000E00010FA7000100002B2D0001000A00010FA7000100002B8B0001000A00010FA70001000028840001000500010FA70001000028810001000100010FA7000100002B9B0001009600010FA7000100002CC90001003200010FA7000100002CCA0001001900010FA7000100002CCB000100C800010FA7000100002CCC0001019000010FA7000100002CCD0001009600010FA7000100002B1D0001005C00010FA7000100002B1E0001005C00010FA7000100002B240001005C00010FA7000100002B310001005C00010FA7000100002B330001005C00010FA7000100002B470001005C00010FA7000100002B5A0001005C00010FA7000100002B600001005C00010FA7000100002B6D0001005C00010FA7000100002B7D0001005C00010FA7000100002B810001005C00010FA7000100002B870001005C00010FA7000100002B7C0001005C00010FA7000100002B1F0001005C00010FA7000100002B200001005C00010FA7000100002B290001005C00010FA7000100002B350001005C00010FA7000100002B370001005C00010FA7000100002B450001005C00010FA7000100002B5B0001005C00010FA7000100002B610001005C00010FA7000100002B790001005C00010FA7000100002B7A0001005C00010FA7000100002B7B0001005C00010FA7000100002B830001005C00010FA7000100002B890001005B00010FA7000100002B580001005B00010FA7000100002B210001005B00010FA7000100002B270001005B00010FA7000100002B2E0001005B00010FA7000100002B390001005B00010FA7000100002B3C0001005B00010FA7000100002B430001005B00010FA7000100002B5C0001005B00010FA7000100002B620001005B00010FA7000100002B6F0001005B00010FA7000100002B7F0001005B00010FA7000100002B800001005B00010FA7000100002B820001005B00010FA7000100002B500001005B00010FA70001000028820001005B00010FA70001000028800001005B00010FA7000100002B250001005B00010FA7000100002B3E0001005B00010FA7000100002B5D0001005B00010FA7000100002B650001005B00010FA7000100002B720001005B00010FA7000100002B850001005B00010FA7000100002B2B0001005B00010FA7000100002B5E0001005B00010FA7000100002B740001005B00010FA7000100002B400001005B00010FA7000100002B4A0001005B00010FA7000100002B6B0001005B00010FA7000100002B880001005B00010FA7000100002B510001005B00010FA7000100002B530001005B00010FA7000100002B540001005B00010FA7000100002B2A0001005B00010FA7000100002B670001005B00010FA7000100002B690001005B00010FA7000100002B760001005B00010FA7000100002B2F0001005B00010FA7000100002B2C0001005B00010FA7000100002B8A0001005B00010FA7000200002B320001005A00010FA7000200002B340001005A00010FA7000200002B360001005A00010FA7000200002B380001005A00010FA7000200002B3A0001005A00010FA7000200002B6E0001005A00010FA7000200002B700001005A00010FA7000200002B660001005A00010FA7000200002B680001005A00010FA7000200002B6A0001005A00010FA7000200002B220001005A00010FA7000200002B230001005A00010FA7000200002B420001005A00010FA7000200002B840001005A00010FA7000200002B3B0001005A00010FA7000200002B280001005A00010FA7000200002B260001005A00010FA7000200002B5F0001005A00010FA7000200002B630001005A00010FA7000200002B640001005A00010FA7000200002B710001005A00010FA7000200002B7E0001005A00010FA7000200002B4C0001005A00010FA7000200002B4D0001005A00010FA7000200002B4E0001005A00010FA7000200002B4F0001005A00010FA7000200002B560001005A00010FA7000200002B570001005A00010FA70002000028860001005A00010FA70002000028870001005A00010FA70002000028880001005A00010FA70002000028890001005A00010FA700020000288A0001005A00010FA7000200002B3D0001005000010FA7000200002B3F0001005000010FA7000200002B410001005000010FA7000200002B440001005000010FA7000200002B460001005000010FA7000200002B6C0001005000010FA7000200002B730001005000010FA7000200002B770001005000010FA7000200002B860001005000010FA7000200002B300001005000010FA7000200002B520001005000010FA7000200002B590001005000010FA700020000287F0001005000010FA70002000028830001005000010FA70002000028850001005000010FA7000200002B480001001600010FA7000200002B490001001600010FA7000200002B4B0001001600010FA7000200002B750001001600010FA7000200002B550001001600010FA7000200002B2D0001000F00010FA7000200002B8B0001000F00010FA70002000028840001000800010FA70002000028810001000200010FA7000200002B97000304C400010FA7000200002B980003028A00010FA7000200002B99000300A000010FA7000200002D8D0001032000010FA7000200002D8E0001032000010FA7000200002B9B000101F400010FA7000200002B9A0001022600010FA7000200002CC90001003200010FA7000200002CCA0001001900010FA7000200002CCB000100FA00010FA7000200002CCC000101F400010FA7000200002CCD000100AF0001106A000100002B9B000117700001106A000100002CC9000100C80001106A000100002CCA000100640001106A000100002CCB000103E80001106A000100002CCC000107D00001106A000100002CCD000102BC0001106A000200002D8D000103200001106A000200002D8E000103200001106A000200002B9B000101900001106A000200002CC9000101900001106A000200002CCA000100C80001106A000200002CCB000107D00001106A000200002CCC00010FA00001106A000200002CCD000105780001")
	} else if pkt.Unk2 == 6001 {
		data, err = hex.DecodeString("0A218EAD0000000000000000000000052B97010113882B9801010D162B99010105DC2B9A010100642B9B01010032")
	} else if pkt.Unk2 == 6002 {
		data, err = hex.DecodeString("0A218EAD00000000000000000000002F2B97020107082B98020104B02B99020101F42B9A010100322B1D010100962B1E010100962B24010100962B31010100962B33010100962B47010100962B5A010100962B60010100962B6D010100962B78010100962B7D010100962B81010100962B87010100962B7C010100962B1F010100962B20010100962B29010100962B35010100962B37010100962B45010100962B5B010100962B61010100962B79010100962B7A010100962B7B010100962B83010100962B89010100962B58010100962B21010100962B27010100962B2E010100962B39010100962B3C010100962B43010100962B5C010100962B62010100962B6F010100962B7F010100962B80010100962B82010100962B5001010096288201010096288001010096")
	} else if pkt.Unk2 == 6010 {
		data, err = hex.DecodeString("0A218EAD00000000000000000000000B2B9701010E742B9801010B542B99010105142CBD010100FA2CBE010100FA2F17010100FA2F21010100FA2F1A010100FA2F24010100FA2DFE010100C82DFD01010190")
	} else if pkt.Unk2 == 6011 {
		data, err = hex.DecodeString("0A218EAD00000000000000000000000B2B9701010E742B9801010B542B99010105142CBD010100FA2CBE010100FA2F17010100FA2F21010100FA2F1A010100FA2F24010100FA2DFE010100C82DFD01010190")
	} else if pkt.Unk2 == 6012 {
		data, err = hex.DecodeString("0A218EAD00000000000000000000000D2B9702010DAC2B9802010B542B990201051430DC010101902CBD010100C82CBE010100C82F17010100C82F21010100C82F1A010100C82F24010100C82DFF010101902E00010100C82E0101010064")
	} else if pkt.Unk2 == 7001 {
		data, err = hex.DecodeString("0A218EAD00000000000000000000009D2B1D010101222B1E0101010E2B240101010E2B31010101222B33010101222B47010101222B5A010101182B600101012C2B6D010101182B78010101222B7D010101222B810101012C2B87010101222B7C0101010E2B220101002F2B250101002F2B380101002F2B360101002F2B3E010100302B5D0101002F2B640101002F2B650101002F2B700101002F2B720101002F2B7E0101002F2B850101002F2B4C0101002F2B4F0101002F2B560101002F28860101002F28870101002F2B2B010100112B3F010100102B44010100102B5E010100112B74010100112B52010100112B97010104B02B970201028A2B98010103202B980201012C2B99010100642B99020100322B9C010100642B9A010100642B9B010100642B960101012C2CC70101012C2C5C0101012C2CC80101012C2C5D010101F42B1F0102012C2B200102010E2B290102012C2B35010201222B37010201222B45010201222B5B010201182B610102012C2B79010200FA2B7A0102012C2B7B010201182B83010201222B89010201042B580102012C2B260102002F2B3A0102002F2B3B0102002F2B400102002F2B4A0102002F2B5F0102002F2B660102002F2B680102002F2B6A0102002F2B6B0102002F2B710102002F2B88010200302B4D0102002F2B510102002F2B530102002F28880102002F28890102002F2B77010200112B3D010200112B86010200112B46010200112B30010200102B54010200102B97010204B02B970202028A2B98010203202B980202012C2B99010200642B99020200322B9C010200642B9A010200642B9B010200642B960102012C2CC70102012C2C5C0102012C2CC80102012C2C5D010201F42B210103010A2B270103010A2B2E0103010A2B390103010A2B3C0103010A2B430103010A2B5C0103010A2B620103010A2B6F0103010A2B7F0103010C2B800103010C2B820103010C2B500103010C28820103010A28800103010C2B23010300322B28010300322B2A010300322B32010300322B34010300322B42010300322B63010300322B67010300322B69010300322B6E010300322B76010300322B84010300322B4E010300322B57010300322B2F01030032288A010300322B2C0103000F2B410103000F2B8A0103000F2B6C0103000F2B730103000F2B590103000F287F0103000F28830103000F28850103000F2A1A010301772BC9010301772A3D010301772C7D010301772B97010303E82B97020300FA2B98010302BC2B98020300AF2B990103012C2B990203004B2CC9010300352CCA0103001B2CCB0103010A2CCC010302152CCD010300BA")
	} else if pkt.Unk2 == 7002 {
		data, err = hex.DecodeString("0A218EAD0000000000000000000000B92B1D010100642B1E010100642B24010100642B31010100642B33010100642B47010100642B5A010100642B60010100642B6D010100642B78010100642B7D010100642B81010100642B87010100642B7C010100642B220101003C2B250101003C2B380101003C2B360101003C2B3E0101003C2B5D0101003C2B640101003C2B650101003C2B700101003C2B720101003C2B7E0101003C2B850101003C2B4C0101003C2B4F0101003C2B560101003C28860101003C28870101003C2B2B010100142B3F010100142B44010100142B5E010100142B74010100142B52010100142B9C010101902B9A010100C82B9B010100C82CC7010100642CC80101009628730101009630DA010100C830DB0101012C30DC01010384353D0101015E353C010100C82C5C010100642C5D010100962EEE010100FA2EF0010101902EEF0101019A2B97020101F42B97040101F42B97060101F42B98020101902B98040101902B98060101902B99020100642B99040100642B99060100642B1F010200642B20010200642B29010200642B35010200642B37010200642B45010200642B5B010200642B61010200642B79010200642B7A010200642B7B010200642B83010200642B89010200642B58010200642B260102003C2B3A0102003C2B3B0102003C2B400102003C2B4A0102003C2B5F0102003C2B660102003C2B680102003C2B6A0102003C2B6B0102003C2B710102003C2B880102003C2B4D0102003C2B510102003C2B530102003C28880102003C28890102003C2B77010200142B3D010200142B86010200142B46010200142B30010200142B54010200142B9C010201902B9A010200C82B9B010200C82CC7010200FA2CC80102015E30DA0102009630DB010200C830DC0102015E353D010200FA353C010200C82873010201902B96010200642C5C010200642C5D010200642EEE0102012C2EF0010201C22EEF010201CC2B97020201F42B97040201F42B97060201F42B98020201902B98040201902B98060201902B99020200642B99040200642B99060200642B21010300782B27010300782B2E010300782B39010300782B3C010300782B43010300782B5C010300782B62010300782B6F010300782B7F010300782B80010300782B82010300782B50010300782882010300782880010300782B23010300412B28010300412B2A010300412B32010300412B34010300412B42010300412B63010300412B67010300412B69010300412B6E010300412B76010300412B84010300412B4E010300412B57010300412B2F01030041288A010300412B2C0103000F2B410103000F2B8A0103000F2B6C0103000F2B730103000F2B590103000F287F0103000F28830103000F28850103000F2A1A030301EA2BC9030301EA2A3D030301EA2C7D030301EA2F0E030301F430D7030301F42B97020301F42B97040301F42B97060301F42B98020301902B98040301902B98060301902B99020300642B99040300642B99060300642CC9010300352CCA0103001B2CCB0103010A2CCC010302152CCD010300BA")
	} else if pkt.Unk2 == 7011 {
		data, err = hex.DecodeString("0A218EAD00000000000000000000009D2B1D010101222B1E0101010E2B240101010E2B31010101222B33010101222B47010101222B5A010101182B600101012C2B6D010101182B78010101222B7D010101222B810101012C2B87010101222B7C0101010E2B220101002F2B250101002F2B380101002F2B360101002F2B3E010100302B5D0101002F2B640101002F2B650101002F2B700101002F2B720101002F2B7E0101002F2B850101002F2B4C0101002F2B4F0101002F2B560101002F28860101002F28870101002F2B2B010100112B3F010100102B44010100102B5E010100112B74010100112B52010100112B97010104B02B970201028A2B98010103202B980201012C2B99010100642B99020100322B9C010100642B9A010100642B9B010100642B960101012C2CC70101012C2C5C0101012C2CC80101012C2C5D010101F42B1F0102012C2B200102010E2B290102012C2B35010201222B37010201222B45010201222B5B010201182B610102012C2B79010200FA2B7A0102012C2B7B010201182B83010201222B89010201042B580102012C2B260102002F2B3A0102002F2B3B0102002F2B400102002F2B4A0102002F2B5F0102002F2B660102002F2B680102002F2B6A0102002F2B6B0102002F2B710102002F2B88010200302B4D0102002F2B510102002F2B530102002F28880102002F28890102002F2B77010200112B3D010200112B86010200112B46010200112B30010200102B54010200102B97010204B02B970202028A2B98010203202B980202012C2B99010200642B99020200322B9C010200642B9A010200642B9B010200642B960102012C2CC70102012C2C5C0102012C2CC80102012C2C5D010201F42B210103010A2B270103010A2B2E0103010A2B390103010A2B3C0103010A2B430103010A2B5C0103010A2B620103010A2B6F0103010A2B7F0103010C2B800103010C2B820103010C2B500103010C28820103010A28800103010C2B23010300322B28010300322B2A010300322B32010300322B34010300322B42010300322B63010300322B67010300322B69010300322B6E010300322B76010300322B84010300322B4E010300322B57010300322B2F01030032288A010300322B2C0103000F2B410103000F2B8A0103000F2B6C0103000F2B730103000F2B590103000F287F0103000F28830103000F28850103000F2A1A010301772BC9010301772A3D010301772C7D010301772B97010303E82B97020300FA2B98010302BC2B98020300AF2B990103012C2B990203004B2CC9010300352CCA0103001B2CCB0103010A2CCC010302152CCD010300BA")
	} else if pkt.Unk2 == 7012 {
		data, err = hex.DecodeString("0A218EAD00000000000000000000009D2B1D010101222B1E0101010E2B240101010E2B31010101222B33010101222B47010101222B5A010101182B600101012C2B6D010101182B78010101222B7D010101222B810101012C2B87010101222B7C0101010E2B220101002F2B250101002F2B380101002F2B360101002F2B3E010100302B5D0101002F2B640101002F2B650101002F2B700101002F2B720101002F2B7E0101002F2B850101002F2B4C0101002F2B4F0101002F2B560101002F28860101002F28870101002F2B2B010100112B3F010100102B44010100102B5E010100112B74010100112B52010100112B97010104B02B970201028A2B98010103202B980201012C2B99010100642B99020100322B9C010100642B9A010100642B9B010100642B960101012C2CC70101012C2C5C0101012C2CC80101012C2C5D010101F42B1F0102012C2B200102010E2B290102012C2B35010201222B37010201222B45010201222B5B010201182B610102012C2B79010200FA2B7A0102012C2B7B010201182B83010201222B89010201042B580102012C2B260102002F2B3A0102002F2B3B0102002F2B400102002F2B4A0102002F2B5F0102002F2B660102002F2B680102002F2B6A0102002F2B6B0102002F2B710102002F2B88010200302B4D0102002F2B510102002F2B530102002F28880102002F28890102002F2B77010200112B3D010200112B86010200112B46010200112B30010200102B54010200102B97010204B02B970202028A2B98010203202B980202012C2B99010200642B99020200322B9C010200642B9A010200642B9B010200642B960102012C2CC70102012C2C5C0102012C2CC80102012C2C5D010201F42B210103010A2B270103010A2B2E0103010A2B390103010A2B3C0103010A2B430103010A2B5C0103010A2B620103010A2B6F0103010A2B7F0103010C2B800103010C2B820103010C2B500103010C28820103010A28800103010C2B23010300322B28010300322B2A010300322B32010300322B34010300322B42010300322B63010300322B67010300322B69010300322B6E010300322B76010300322B84010300322B4E010300322B57010300322B2F01030032288A010300322B2C0103000F2B410103000F2B8A0103000F2B6C0103000F2B730103000F2B590103000F287F0103000F28830103000F28850103000F2A1A010301772BC9010301772A3D010301772C7D010301772B97010303E82B97020300FA2B98010302BC2B98020300AF2B990103012C2B990203004B2CC9010300352CCA0103001B2CCB0103010A2CCC010302152CCD010300BA")
	} else {
		data = []byte{0x00, 0x00, 0x00, 0x00}
		s.logger.Info("GET_PAPER request for unknown type")
	}
	if err != nil {
		panic(err)
	}
	doAckBufSucceed(s, pkt.AckHandle, data)
}

func handleMsgSysAuthData(s *Session, p mhfpacket.MHFPacket) {}
