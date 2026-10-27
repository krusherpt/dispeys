use crate::enums::*;

/// Build a 1024-byte HID packet with header, command, length, and data
pub fn build_packet(cmd: CommandProtocol, length: u32, data: &[u8]) -> Vec<u8> {
    let mut buf = Vec::with_capacity(PACKET_SIZE);

    // Header
    buf.push(0x7c);
    buf.push(0x7c);

    // Command protocol (BigEndian)
    buf.extend_from_slice(&(cmd as u16).to_be_bytes());

    // Data length (LittleEndian)
    buf.extend_from_slice(&length.to_le_bytes());

    // Data padded to PACKET_SIZE - 8
    let padded_size = PACKET_SIZE - 8;
    let mut padded = vec![0u8; padded_size];
    let copy_len = data.len().min(padded_size);
    padded[..copy_len].copy_from_slice(&data[..copy_len]);
    buf.extend_from_slice(&padded);

    buf
}

/// Invalid bytes that cannot appear at packet-boundary positions in ZIP
const INVALID_BYTES: [u8; 2] = [0x00, 0x7c];

fn contains_invalid_byte(b: u8) -> bool {
    INVALID_BYTES.contains(&b)
}

/// Check that no invalid bytes appear at packet-boundary positions (1016 + n*1024) in the ZIP file
pub fn zip_is_valid(zip_data: &[u8]) -> bool {
    let size = zip_data.len();
    for i in (1016usize..size).step_by(PACKET_SIZE) {
        if contains_invalid_byte(zip_data[i]) {
            return false;
        }
    }
    true
}

/// Parse incoming device packet
pub fn parse_incoming(inp: &[u8]) -> Result<Option<ParsedPacket>, String> {
    if inp.len() < 8 {
        return Err("Packet too short".to_string());
    }

    if inp[0] != 0x7c || inp[1] != 0x7c {
        return Err("Invalid signature".to_string());
    }

    let cmd = u16::from_be_bytes([inp[2], inp[3]]);
    let length = u32::from_le_bytes([inp[4], inp[5], inp[6], inp[7]]);

    if (length as usize) + 8 > inp.len() {
        return Err("Data length exceeds packet size".to_string());
    }

    let data = &inp[8..(8 + length as usize)];

    match cmd {
        0x0101 => {
            let button_data = parse_button_pressed(data)?;
            Ok(Some(ParsedPacket {
                command_protocol: CommandProtocol::InButton,
                data: PacketData::Button(button_data),
            }))
        }
        0x0303 => {
            let text = data.iter().take_while(|&&b| b != 0).map(|&b| b as char).collect::<String>();
            Ok(Some(ParsedPacket {
                command_protocol: CommandProtocol::InDeviceInfo,
                data: PacketData::DeviceInfo(text),
            }))
        }
        0x030a => {
            // Brightness response - ignore
            Ok(None)
        }
        0x0103 => {
            // Unknown input - ignore
            Ok(None)
        }
        0x0104 => {
            // Another unknown input - ignore
            Ok(None)
        }
        _ => Err(format!("Unknown command protocol: 0x{:04x}", cmd)),
    }
}

/// Parse button press data from 3 bytes: state, index, const(0x01)
fn parse_button_pressed(data: &[u8]) -> Result<ButtonPressedData, String> {
    if data.len() < 4 || data[2] != 0x01 {
        return Err("Invalid ButtonPressedData format".to_string());
    }

    let pressed = data[3] == 0x01;

    Ok(ButtonPressedData {
        state: data[0],
        index: data[1],
        pressed,
    })
}

/// Parse a complete input packet into actionable events
pub fn parse_input(inp: &[u8]) -> Result<(Option<ButtonAction>, Option<DeviceInfo>), String> {
    let parsed = parse_incoming(inp)?;

    if parsed.is_none() {
        return Ok((None, None));
    }

    let parsed = parsed.unwrap();

    match parsed.command_protocol {
        CommandProtocol::InDeviceInfo => {
            let text = match parsed.data {
                PacketData::DeviceInfo(t) => t,
                _ => return Err("DeviceInfo type error".to_string()),
            };
            let info: DeviceInfo = serde_json::from_str(&text).map_err(|e| format!("DeviceInfo parse error: {}", e))?;
            Ok((None, Some(info)))
        }
        CommandProtocol::InButton => {
            let btn = match parsed.data {
                PacketData::Button(b) => b,
                _ => return Err("Button type error".to_string()),
            };
            Ok((Some(ButtonAction {
                index: btn.index,
                pressed: btn.pressed,
                state: btn.state,
            }), None))
        }
        _ => Ok((None, None)),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_build_packet_size() {
        let packet = build_packet(CommandProtocol::OutSetBrightness, 3, b"100");
        assert_eq!(packet.len(), PACKET_SIZE);
    }

    #[test]
    fn test_build_packet_header() {
        let packet = build_packet(CommandProtocol::OutSetBrightness, 3, b"100");
        assert_eq!(packet[0], 0x7c);
        assert_eq!(packet[1], 0x7c);
        assert_eq!(u16::from_be_bytes([packet[2], packet[3]]), 0x000a);
        assert_eq!(u32::from_le_bytes([packet[4], packet[5], packet[6], packet[7]]), 3);
    }

    #[test]
    fn test_parse_button_pressed() {
        let data = &[0x00, 0x05, 0x01, 0x01]; // state=0, index=5, pressed=true
        let result = parse_button_pressed(data).unwrap();
        assert_eq!(result.index, 5);
        assert!(result.pressed);
    }

    #[test]
    fn test_zip_is_valid() {
        let mut data = vec![0x41u8; 2048]; // Safe bytes
        assert!(zip_is_valid(&data));

        // Place invalid byte at boundary
        data[1016] = 0x7c;
        assert!(!zip_is_valid(&data));
    }
}
