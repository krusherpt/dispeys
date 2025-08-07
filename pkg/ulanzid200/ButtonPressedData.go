package ulanzid200

import "fmt"

type ButtonPressedData struct {
	State   byte
	Index   byte
	Pressed bool
}

// Разбор данных кнопки из 3 байтов: state, index, const(0x01)
func ParseButtonPressed(data []byte) (*ButtonPressedData, error) {
	if len(data) < 4 || data[2] != 0x01 {
		return nil, fmt.Errorf("неверный формат ButtonPressedStruct")
	}
	pressed := data[3] == 0x01
	return &ButtonPressedData{
		State:   data[0],
		Index:   data[1],
		Pressed: pressed,
	}, nil
}
