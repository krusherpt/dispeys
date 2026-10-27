/// USB Vendor ID for Ulanzi D200
pub const VENDOR_ID: u16 = 0x2207;

/// USB Product ID for Ulanzi D200
pub const PRODUCT_ID: u16 = 0x0019;

/// Total number of buttons on the device
pub const BUTTON_COUNT: usize = 13;

/// Number of button rows
pub const BUTTON_ROWS: usize = 3;

/// Number of button columns
pub const BUTTON_COLS: usize = 5;

/// Icon width in pixels
pub const ICON_WIDTH: usize = 196;

/// Icon height in pixels
pub const ICON_HEIGHT: usize = 196;

/// HID packet size
pub const PACKET_SIZE: usize = 1024;

/// Command protocols for device communication
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u16)]
pub enum CommandProtocol {
    /// Set all buttons with full ZIP payload
    OutSetButtons = 0x0001,

    /// Partially update buttons
    OutPartiallyUpdateButtons = 0x000d,

    /// Set small window data (clock/stats)
    OutSetSmallWindowData = 0x0006,

    /// Set brightness
    OutSetBrightness = 0x000a,

    /// Set label style
    OutSetLabelStyle = 0x000b,

    /// Button press event from device
    InButton = 0x0101,

    /// Device info response
    InDeviceInfo = 0x0303,

    /// Brightness acknowledgement
    InBrightnessResponse = 0x030a,
}

/// Small window display mode (button 12 cycles through these)
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum SmallWindowMode {
    Stats = 0,
    Clock = 1,
    Background = 2,
}

impl SmallWindowMode {
    /// Get the next mode in the cycle (skipping Stats)
    pub fn next(&self) -> Self {
        let next_val = ((*self as u8) + 2) % 3;
        match next_val {
            0 => SmallWindowMode::Stats,
            1 => SmallWindowMode::Clock,
            2 => SmallWindowMode::Background,
            _ => unreachable!(),
        }
    }
}

/// Data displayed in the small window area
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SmallWindowData {
    pub mode: SmallWindowMode,
    pub cpu: i32,
    pub mem: i32,
    pub gpu: i32,
    pub time: String,
}

/// A button on the device
#[derive(Debug, Clone)]
pub struct Button {
    pub name: String,
    pub icon: String,
}

/// Label styling for button text
#[derive(Debug, Clone, serde::Serialize)]
pub struct LabelStyle {
    #[serde(rename = "Align")]
    pub align: String,

    #[serde(rename = "Color")]
    pub int_color: i64,

    #[serde(rename = "FontName")]
    pub font_name: String,

    #[serde(rename = "ShowTitle")]
    pub show_title: bool,

    pub size: i32,

    pub weight: i32,

    #[serde(skip)]
    pub color: String,
}

impl LabelStyle {
    /// Create a new LabelStyle with defaults matching the Go version
    pub fn new() -> Self {
        LabelStyle {
            align: "bottom".to_string(),
            color: "FFFFFF".to_string(),
            int_color: hex_to_int("FFFFFF"),
            font_name: "Roboto".to_string(),
            show_title: true,
            size: 10,
            weight: 80,
        }
    }

    /// Override a field from a key-value map
    pub fn with(mut self, key: &str, value: &str) -> Self {
        match key {
            "align" => self.align = value.to_string(),
            "color" => {
                self.color = value.to_string();
                self.int_color = hex_to_int(value);
            }
            "font_name" => self.font_name = value.to_string(),
            "show_title" => self.show_title = value.parse::<bool>().unwrap_or(true),
            "size" => self.size = value.parse::<i32>().unwrap_or(10),
            "weight" => self.weight = value.parse::<i32>().unwrap_or(80),
            _ => {}
        }
        self
    }

    /// Serialize to JSON bytes for sending to device
    pub fn to_json_bytes(&self) -> Vec<u8> {
        let mut clone = self.clone();
        clone.int_color = hex_to_int(&self.color);
        serde_json::to_vec(&clone).unwrap_or_default()
    }
}

/// Device information returned by the device
#[derive(Debug, Clone, serde::Deserialize)]
pub struct DeviceInfo {
    #[serde(rename = "Dversion")]
    pub dversion: String,

    #[serde(rename = "SerialNumber")]
    pub serial_number: String,

    pub error: String,
}

/// Raw button press data from device (3 bytes: state, index, const 0x01)
#[derive(Debug, Clone)]
pub struct ButtonPressedData {
    pub state: u8,
    pub index: u8,
    pub pressed: bool,
}

/// Actionable button event (after parsing)
#[derive(Debug, Clone)]
pub struct ButtonAction {
    pub index: u8,
    pub pressed: bool,
    pub state: u8,
}

/// Event emitted when a button is pressed
#[derive(Debug, Clone)]
pub struct KeyPressedEvent {
    pub index: usize,
}

/// Parsed incoming packet
#[derive(Debug)]
pub struct ParsedPacket {
    pub command_protocol: CommandProtocol,
    pub data: PacketData,
}

/// Data contained in a parsed packet
#[derive(Debug)]
pub enum PacketData {
    Button(ButtonPressedData),
    DeviceInfo(String),
}

fn hex_to_int(hex_str: &str) -> i64 {
    i64::from_str_radix(hex_str, 16).unwrap_or(0)
}
