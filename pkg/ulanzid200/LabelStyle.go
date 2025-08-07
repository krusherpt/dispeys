package ulanzid200

import (
	"encoding/json"
	"strconv"
)

func hexToInt(hexStr string) int64 {
	val, _ := strconv.ParseInt(hexStr, 16, 64)
	return val
}

type LabelStyle struct {
	Align     string `json:"Align"`
	Color     string `json:"-"`
	IntColor  int64  `json:"Color"`
	FontName  string `json:"FontName"`
	ShowTitle bool   `json:"ShowTitle"`
	Size      int    `json:"Size"`
	Weight    int    `json:"Weight"`
}

func NewLabelStyle(style map[string]interface{}) LabelStyle {
	if _, ok := style["align"]; !ok {
		style["align"] = "bottom"
	}
	if _, ok := style["color"]; !ok {
		style["color"] = "FFFFFF"
	}
	if _, ok := style["font_name"]; !ok {
		style["font_name"] = "Roboto"
	}
	if _, ok := style["show_title"]; !ok {
		style["show_title"] = true
	}
	if _, ok := style["size"]; !ok {
		style["size"] = 10
	}
	if _, ok := style["weight"]; !ok {
		style["weight"] = 80
	}
	
	return LabelStyle{
		Align:     style["align"].(string),
		Color:     style["color"].(string),
		FontName:  style["font_name"].(string),
		ShowTitle: style["show_title"].(bool),
		Size:      style["size"].(int),
		Weight:    style["weight"].(int),
	}
}

func (s *LabelStyle) GetJson() []byte {
	s.IntColor = hexToInt(s.Color)
	jsonData, _ := json.Marshal(s)
	return jsonData
}