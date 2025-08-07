package ulanzid200

import (
	"encoding/json"
	"fmt"
	"time"
)

type ButtonAction struct {
	Index   byte
	Pressed bool
	State   byte
}

type DeviceInfo struct {
	Dversion     string `json:"Dversion"`
	SerialNumber string `json:"SerialNumber"`
	Error        string `json:"error"`
}

func ParseInput(dev *UlanziD200Device, inp []byte) (action *ButtonAction, info *DeviceInfo, err error) {
	parsed, err := ParseIncomingStruct(inp)
	if err != nil {
		return nil, nil, err
	}

	if parsed.Data == nil {
		return nil, nil, nil
	}

	switch parsed.CommandProtocol {
	case IN_DEVICE_INFO:
		var info DeviceInfo
		err := json.Unmarshal([]byte(parsed.Data.(string)), &info)
		if err != nil {
			fmt.Println("Ошибка парсинга:", err)
			return nil, nil, err
		}
		return nil, &info, err

	case IN_BUTTON:
		dev.lastActionTime = time.Now()
		btn, ok := parsed.Data.(*ButtonPressedData)
		if !ok {
			return nil, nil, fmt.Errorf("unexpected data type for button: %T", parsed.Data)
		}
		return &ButtonAction{
			Index:   btn.Index,
			Pressed: btn.Pressed,
			State:   btn.State,
		}, nil, nil
	}

	return nil, nil, nil
}
