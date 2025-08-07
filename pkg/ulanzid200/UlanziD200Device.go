package ulanzid200

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/karalabe/hid"
)

type UlanziD200Device struct {
	device           *hid.Device
	keyPressedChan   chan *KeyPressedEvent
	refreshChan      chan struct{}
	brightness       int
	labelStyle       LabelStyle
	smallWindowMode  SmallWindowMode
	smallWindowData  SmallWindowData
	lastActionTime   time.Time
	iconPath         string
	tmpPath          string
	stopped					 bool
}

const (
	VendorID  = 0x2207
	ProductID = 0x0019

	ButtonCount = 13
	ButtonRows  = 3
	ButtonCols  = 5

	IconWidth  = 196
	IconHeight = 196
)


type CommandProtocol uint16

const (
	OUT_SET_BUTTONS              CommandProtocol = 0x0001
	OUT_PARTIALLY_UPDATE_BUTTONS CommandProtocol = 0x000d
	OUT_SET_SMALL_WINDOW_DATA    CommandProtocol = 0x0006
	OUT_SET_BRIGHTNESS           CommandProtocol = 0x000a
	OUT_SET_LABEL_STYLE          CommandProtocol = 0x000b
	IN_BUTTON                    CommandProtocol = 0x0101
	IN_DEVICE_INFO               CommandProtocol = 0x0303
)

type Packet struct {
	CommandProtocol CommandProtocol
	Length          uint32
	Data            []byte
}

type KeyPressedEvent struct {
	Index   int
}

func BuildPacket(cmd CommandProtocol, length int, data []byte) []byte {
	const PacketSize = 1024
	var buf bytes.Buffer

	buf.Write([]byte{0x7c, 0x7c}) // header

	binary.Write(&buf, binary.BigEndian, uint16(cmd))
	binary.Write(&buf, binary.LittleEndian, uint32(length))

	padded := make([]byte, PacketSize-8)
	copy(padded, data)
	buf.Write(padded)

	return buf.Bytes()
}

func EqualJSON(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return bytes.Equal(aj, bj)
}

func (d *UlanziD200Device) SetBrightness(value int, force bool) {
	if !force && value == d.brightness {
		return
	}
	d.brightness = value
	data := []byte(fmt.Sprintf("%d", value))
	packet := BuildPacket(OUT_SET_BRIGHTNESS, len(data), data)
	d.writePacket(packet)
}

func (d *UlanziD200Device) SetLabelStyle(style LabelStyle, force bool) {
	if !force && EqualJSON(d.labelStyle, style) {
		return
	}

	d.labelStyle = style

	jsonData := style.GetJson()
	packet := BuildPacket(OUT_SET_LABEL_STYLE, len(jsonData), jsonData)
	d.writePacket(packet)
}


func (d *UlanziD200Device) SetSmallWindowData(data SmallWindowData, force bool) {
	data.Mode = d.smallWindowMode
	if !force && EqualJSON(d.smallWindowData, data) {
		return
	}
	d.smallWindowData = data

	packetStr := fmt.Sprintf("%d|%v|%v|%v|%v", data.Mode, data.CPU, data.MEM, data.Time, data.GPU)
	
	packetData := []byte(packetStr)
	packet := BuildPacket(OUT_SET_SMALL_WINDOW_DATA, len(packetData), packetData)
	d.writePacket(packet)
}

func (d *UlanziD200Device) SetButtons(buttons map[int]Button, updateOnly bool) {
	zipPath := d.prepareZip(buttons)
	data, _ := os.ReadFile(zipPath)

	command := OUT_SET_BUTTONS
	if updateOnly {
		command = OUT_PARTIALLY_UPDATE_BUTTONS
	}

	chunkSize := 1024
	chunk := data[:chunkSize-8]
	packet := BuildPacket(command, len(data), chunk)
	d.writePacket(packet)

	for i := chunkSize - 8; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := make([]byte, chunkSize)
		copy(chunk, data[i:end])
		d.writePacket(chunk)
	}
}

func (d *UlanziD200Device) writePacket(packet []byte) {
	if d.device != nil {
		_, err := d.device.Write(packet)
		if err != nil {
			fmt.Println("writePacket error:", err)
		}
	}
}

func (d *UlanziD200Device) readPacket(packet []byte) (n int, err error) {
	if d.device != nil {
		var err error
		n, err = d.device.Read(packet)
		if err != nil {
			err = fmt.Errorf("readPacket error : %w", err)
		}
	}
	return
}


var invalidBytes = [][]byte{
	{0x00},
	// {0x01},
	{0x7c},
}

func randomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func containsInvalidByte(b byte) bool {
	for _, inv := range invalidBytes {
		if b == inv[0] {
			return true
		}
	}
	return false
}

func (d *UlanziD200Device) prepareZip(buttons map[int]Button) string {
	buildPath := filepath.Join(d.tmpPath, ".build")
	pagePath := filepath.Join(buildPath, "page")
	os.RemoveAll(pagePath)
	os.MkdirAll(filepath.Join(pagePath, "icons"), os.ModePerm)
	manifest := make(map[string]interface{})
	icons := []string{}

	for index, btn := range buttons {
		row := index / ButtonCols
		col := index % ButtonCols
		entry := map[string]interface{}{
			"State":     0,
			"ViewParam": []map[string]string{},
		}
		param := map[string]string{}
		if btn.Name != "" {
			param["Text"] = btn.Name
		}
		if btn.Icon != "" {
			icons = append(icons, btn.Icon)
			param["Icon"] = "icons/" + btn.Icon
		}
		entry["ViewParam"] = []map[string]string{param}
		manifest[fmt.Sprintf("%d_%d", col, row)] = entry
	}

	manifestData, _ := json.MarshalIndent(manifest, "", "  ")

	hash := md5.Sum(manifestData)
	hashHex := hex.EncodeToString(hash[:])
	zipPath := filepath.Join(buildPath, hashHex + ".zip")
	
	_, err := os.Stat(zipPath)
	if err == nil || !os.IsNotExist(err) {
		return zipPath
	}

	os.WriteFile(filepath.Join(pagePath, "manifest.json"), manifestData, 0644)

	for _, icon := range icons {
		src := filepath.Join(d.iconPath, icon)
		dst := filepath.Join(pagePath, "icons", icon)
		copyFile(src, dst)
	}

	dummyPath := filepath.Join(pagePath, "dummy.txt")

	var dummyStr string
	dummyRetries := 04
	buildZipPath := filepath.Join(buildPath, ".build.zip")

	for {
		// Если это не первый заход — создаём dummy-файл
		if dummyRetries > 0 {
			fmt.Println("Generating dummy string...")
			dummyStr += randomString(8 * dummyRetries)
			err := os.WriteFile(dummyPath, []byte(dummyStr), 0644)
			if err != nil {
				panic(err)
			}
		}

		// Создаём zip
		err := ZipFolder(pagePath, buildZipPath)
		if err != nil {
			panic(err)
		}

		// Проверяем файл на наличие запрещённых байт в позициях 1016 + n*1024
		fileInfo, err := os.Stat(buildZipPath)
		if err != nil {
			panic(err)
		}

		valid := true
		f, err := os.Open(buildZipPath)
		if err != nil {
			panic(err)
		}

		for i := int64(1016); i < fileInfo.Size(); i += 1024 {
			_, err = f.Seek(i, io.SeekStart)
			if err != nil {
				panic(err)
			}

			buf := make([]byte, 1)
			_, err = f.Read(buf)
			if err != nil && err != io.EOF {
				panic(err)
			}

			if containsInvalidByte(buf[0]) {
				valid = false
				break
			}
		}
		f.Close()

		if valid {
			break
		}

		dummyRetries++
		time.Sleep(50 * time.Millisecond)
	}

	if err := os.Rename(buildZipPath, zipPath); err != nil {
		fmt.Printf("не удалось переименовать %s -> %s: %v", buildZipPath, zipPath, err)
	}

	fmt.Println("ZIP создан и прошёл проверку!")
	return zipPath
}

func ZipFolder(srcDir, zipFile string) error {
	outFile, err := os.Create(zipFile)
	if err != nil {
		return fmt.Errorf("не удалось создать архив: %w", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	fmt.Println(src, dst)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func New(mode SmallWindowMode, IconPath, TmpPath string) *UlanziD200Device {
	return &UlanziD200Device{
		smallWindowMode: mode,
		iconPath: IconPath,
		tmpPath: TmpPath,
		keyPressedChan: make(chan *KeyPressedEvent),
		refreshChan : make(chan struct{}),
	}
}

func (d *UlanziD200Device) KeyPressedChan() chan *KeyPressedEvent {
	return d.keyPressedChan
}

func (d *UlanziD200Device) RefreshChan() chan struct{} {
	return d.refreshChan
}

func (d *UlanziD200Device) Start() {
	d.connectToDevice()
	go func() {
		for {
			if d.device != nil {
				d.SetSmallWindowData(NewSmallWindowData(map[string]interface{}{}), false)
				time.Sleep(500*time.Millisecond)
			}
			
			if d.stopped {
				break
			}
		}
	}()
	go func() {
		defer d.device.Close()
		packet := make([]byte, 1024)
		for {
			if d.device == nil {
				if !d.connectToDevice() {
					time.Sleep(3 * time.Second)
					continue
				}
			}
			plen, err := d.readPacket(packet)

			if d.stopped {
				break
			}

			if err != nil || plen < 8 {
				fmt.Printf("  Error read packet: %v\n", err)
				d.connectToDevice()
				continue
			}
			
			buttonAction, info, err := ParseInput(d, packet[:plen])
			if err != nil {
				fmt.Printf("  Error parsing input: %v\n", err)
				continue
			}
			if info != nil {
				d.refreshChan <- struct{}{}
				d.SetBrightness(100, true)
			}
			if buttonAction != nil {
				i := int(buttonAction.Index)
				if buttonAction.Pressed && i == 13 {
					d.smallWindowMode = GetNextMode(d.smallWindowMode)
				} else if !buttonAction.Pressed {
					d.keyPressedChan <- &KeyPressedEvent{
						Index: i,
					}
				}
			}
		}
	}()
}

func (d *UlanziD200Device) connectToDevice() bool {
	if !hid.Supported() {
		fmt.Println("HID not supported")
		return false
	}
	succcess := false
	if d.device != nil {
		d.device.Close()
		time.Sleep(3 * time.Second)
	}

	hids := hid.Enumerate(0, 0)
	for i := 0; i < len(hids); i++ {
		for j := i + 1; j < len(hids); j++ {
			if hids[i].Path > hids[j].Path {
				hids[i], hids[j] = hids[j], hids[i]
			}
		}
	}
	for i, hid := range hids {
		if hid.VendorID != VendorID || hid.ProductID != ProductID || hid.Interface != 0 {
			continue
		}
		fmt.Printf("HID #%d\n", i)
		fmt.Printf("  OS Path:      %s\n", hid.Path)
		fmt.Printf("  Vendor ID:    %#04x\n", hid.VendorID)
		fmt.Printf("  Product ID:   %#04x\n", hid.ProductID)
		fmt.Printf("  Release:      %d\n", hid.Release)
		fmt.Printf("  Serial:       %s\n", hid.Serial)
		fmt.Printf("  Manufacturer: %s\n", hid.Manufacturer)
		fmt.Printf("  Product:      %s\n", hid.Product)
		fmt.Printf("  Usage Page:   %#04x\n", hid.UsagePage)
		fmt.Printf("  Usage:        %d\n", hid.Usage)
		fmt.Printf("  Interface:    %d\n", hid.Interface)
		hidDevice, err := hid.Open()
		if err != nil {
			fmt.Printf("  Error opening device: %v\n", err)
			continue
		}
		fmt.Printf("  Device opened successfully.\n")
		d.device = hidDevice
		succcess = true
		break
	}
	return succcess
}

func (d *UlanziD200Device) Stop() {
	d.stopped = true
}

