package ulanzid200

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type ParsedPacket struct {
	CommandProtocol CommandProtocol
	Data            interface{}
}

// Разбор IncomingStruct
func ParseIncomingStruct(inp []byte) (*ParsedPacket, error) {
	if len(inp) < 8 {
		return nil, fmt.Errorf("входной пакет слишком короткий")
	}
	if inp[0] != 0x7c || inp[1] != 0x7c {
		return nil, fmt.Errorf("неверная сигнатура")
	}

	cmd := CommandProtocol(binary.BigEndian.Uint16(inp[2:4]))
	length := binary.LittleEndian.Uint32(inp[4:8])
	if int(length)+8 > len(inp) {
		return nil, fmt.Errorf("длина данных превышает размер пакета")
	}
	data := inp[8 : 8+length]

	switch cmd {
	case 0x0101: // IN_BUTTON
		buttonData, err := ParseButtonPressed(data)
		if err != nil {
			return nil, err
		}
		return &ParsedPacket{
			CommandProtocol: cmd,
			Data:            buttonData,
		}, nil

	case 0x0303: // IN_DEVICE_INFO
		text := string(bytes.Trim(data, "\x00"))
		return &ParsedPacket{
			CommandProtocol: cmd,
			Data:            text,
		}, nil

	default:
		return nil, fmt.Errorf("неизвестный протокол команды: 0x%04x", cmd)
	}
}