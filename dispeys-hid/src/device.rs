use std::collections::HashMap;
use std::io::Write;
use std::path::PathBuf;
use std::sync::mpsc;
use std::time::Instant;

use hidapi::{HidApi, HidDevice};
use log::{error, info, warn};
use tokio::sync::mpsc as tokio_mpsc;

use crate::enums::*;
use crate::packet::*;
use crate::zip_layout::*;

/// Configuration for creating a UlanziD200Device
#[derive(Debug, Clone)]
pub struct DeviceConfig {
    pub small_window_mode: SmallWindowMode,
    pub icon_path: PathBuf,
    pub tmp_path: PathBuf,
}

/// Internal command sent from async context to the blocking device task
enum DeviceCommand {
    SetBrightness { value: i32, force: bool },
    SetLabelStyle { style: LabelStyle },
    SetSmallWindowData { data: SmallWindowData, force: bool },
    SetButtons { buttons: HashMap<usize, Button>, update_only: bool },
    Stop,
}

/// Shared state that needs to be mutable across async boundaries
struct SharedState {
    brightness: i32,
    small_window_mode: SmallWindowMode,
    small_window_data: SmallWindowData,
    last_action_time: Instant,
    last_zip_path: Option<PathBuf>,
    last_buttons_hash: u32,
}

/// Ulanzi D200 device interface
pub struct UlanziD200Device {
    cmd_sender: mpsc::Sender<DeviceCommand>,
    key_pressed_receiver: tokio_mpsc::UnboundedReceiver<KeyPressedEvent>,
    refresh_chan: tokio_mpsc::UnboundedSender<()>,
    icon_path: PathBuf,
    tmp_path: PathBuf,
}

impl UlanziD200Device {
    /// Create a new UlanziD200Device instance
    pub fn new(config: DeviceConfig) -> Self {
        let (cmd_sender, _) = mpsc::channel();
        let (key_tx, key_rx) = tokio_mpsc::unbounded_channel();
        UlanziD200Device {
            cmd_sender,
            key_pressed_receiver: key_rx,
            refresh_chan: tokio_mpsc::unbounded_channel().0,
            icon_path: config.icon_path,
            tmp_path: config.tmp_path,
        }
    }

    /// Get a receiver for key press events
    pub fn key_pressed_receiver(&mut self) -> &mut tokio_mpsc::UnboundedReceiver<KeyPressedEvent> {
        &mut self.key_pressed_receiver
    }

    /// Get a sender for refresh events
    pub fn refresh_sender(&self) -> tokio_mpsc::UnboundedSender<()> {
        self.refresh_chan.clone()
    }

    /// Set device brightness (0-100)
    pub fn set_brightness(&self, value: i32, force: bool) -> Result<(), String> {
        self.cmd_sender
            .send(DeviceCommand::SetBrightness { value, force })
            .map_err(|e| format!("Send error: {}", e))
    }

    /// Set label style
    pub fn set_label_style(&self, style: LabelStyle) -> Result<(), String> {
        self.cmd_sender
            .send(DeviceCommand::SetLabelStyle { style })
            .map_err(|e| format!("Send error: {}", e))
    }

    /// Set small window data
    pub fn set_small_window_data(&self, data: SmallWindowData, force: bool) -> Result<(), String> {
        self.cmd_sender
            .send(DeviceCommand::SetSmallWindowData { data, force })
            .map_err(|e| format!("Send error: {}", e))
    }

    /// Set buttons on the device
    pub fn set_buttons(&self, buttons: HashMap<usize, Button>, update_only: bool) -> Result<(), String> {
        self.cmd_sender
            .send(DeviceCommand::SetButtons { buttons, update_only })
            .map_err(|e| format!("Send error: {}", e))
    }

    /// Start the device: connect, spawn blocking device task
    pub fn start(&mut self, config: DeviceConfig) -> Result<(), String> {
        let (cmd_tx, cmd_rx) = mpsc::channel::<DeviceCommand>();
        let key_sender = {
            let (tx, rx) = tokio_mpsc::unbounded_channel();
            // Replace the receiver so the sender goes to the device thread
            self.key_pressed_receiver = rx;
            tx
        };
        let refresh_sender = self.refresh_chan.clone();

        // Swap in the new command sender
        let _ = std::mem::replace(&mut self.cmd_sender, cmd_tx);

        // Spawn the blocking device task
        std::thread::spawn(move || {
            Self::device_loop(config, cmd_rx, key_sender, refresh_sender);
        });

        Ok(())
    }

    /// Main device loop running in a blocking thread
    fn device_loop(
        config: DeviceConfig,
        cmd_rx: mpsc::Receiver<DeviceCommand>,
        key_sender: tokio_mpsc::UnboundedSender<KeyPressedEvent>,
        refresh_sender: tokio_mpsc::UnboundedSender<()>,
    ) {
        let mut state = SharedState {
            brightness: 0,
            small_window_mode: config.small_window_mode,
            small_window_data: SmallWindowData {
                mode: config.small_window_mode,
                cpu: 0,
                mem: 0,
                gpu: 0,
                time: String::new(),
            },
            last_action_time: Instant::now(),
            last_zip_path: None,
            last_buttons_hash: 0,
        };

        let mut hid_device: Option<HidDevice> = None;

        // Connect to device
        if !connect_to_device(&mut hid_device) {
            error!("Failed to connect to Ulanzi D200 device");
            return;
        }

        // Small window keepalive timer
        let mut last_keepalive = Instant::now();

        loop {
            // Keepalive: send small window data every 500ms
            if last_keepalive.elapsed().as_millis() >= 500 {
                last_keepalive = Instant::now();

                if let Some(ref dev) = hid_device {
                    let time = chrono::Local::now().format("%H:%M:%S").to_string();
                    let packet_str = format!(
                        "{}|{}|{}|{}|{}",
                        state.small_window_data.mode as u8,
                        state.small_window_data.cpu,
                        state.small_window_data.mem,
                        time,
                        state.small_window_data.gpu
                    );
                    let packet_data = packet_str.into_bytes();
                    let packet = build_packet(
                        CommandProtocol::OutSetSmallWindowData,
                        packet_data.len() as u32,
                        &packet_data,
                    );
                    let _ = dev.write(&packet);
                }
            }

            // Poll for commands (non-blocking with timeout)
            match cmd_rx.recv_timeout(std::time::Duration::from_millis(100)) {
                Ok(cmd) => {
                    match cmd {
                        DeviceCommand::SetBrightness { value, force } => {
                            if !force && value == state.brightness {
                                continue;
                            }
                            state.brightness = value;
                            let data = value.to_string().into_bytes();
                            let packet = build_packet(
                                CommandProtocol::OutSetBrightness,
                                data.len() as u32,
                                &data,
                            );
                            if let Some(ref dev) = hid_device {
                                let _ = dev.write(&packet);
                            }
                        }
                        DeviceCommand::SetLabelStyle { style } => {
                            let json_data = style.to_json_bytes();
                            let packet = build_packet(
                                CommandProtocol::OutSetLabelStyle,
                                json_data.len() as u32,
                                &json_data,
                            );
                            if let Some(ref dev) = hid_device {
                                let _ = dev.write(&packet);
                            }
                        }
                        DeviceCommand::SetSmallWindowData { data, force } => {
                            if !force && state.small_window_data == data {
                                continue;
                            }
                            state.small_window_data = data.clone();
                            let packet_str = format!(
                                "{}|{}|{}|{}|{}",
                                data.mode as u8, data.cpu, data.mem, data.time, data.gpu
                            );
                            let packet_data = packet_str.into_bytes();
                            let packet = build_packet(
                                CommandProtocol::OutSetSmallWindowData,
                                packet_data.len() as u32,
                                &packet_data,
                            );
                            if let Some(ref dev) = hid_device {
                                let _ = dev.write(&packet);
                            }
                        }
                        DeviceCommand::SetButtons { buttons, update_only } => {
                            match prepare_zip(&buttons, &config.icon_path, &config.tmp_path) {
                                Ok(zip_data) => {
                                    // Copy to /tmp for debugging
                                    if let Ok(mut file) =
                                        std::fs::File::create("/tmp/ulanzi_buttons.zip")
                                    {
                                        let _ = file.write_all(&zip_data);
                                        info!("Zip output to /tmp/ulanzi_buttons.zip");
                                    }

                                    let command = if update_only {
                                        CommandProtocol::OutPartiallyUpdateButtons
                                    } else {
                                        CommandProtocol::OutSetButtons
                                    };

                                    let chunk_size = PACKET_SIZE - 8;

                                    // First chunk includes protocol header
                                    let first_chunk = &zip_data[..chunk_size.min(zip_data.len())];
                                    let packet = build_packet(
                                        command,
                                        zip_data.len() as u32,
                                        first_chunk,
                                    );
                                    if let Some(ref dev) = hid_device {
                                        let _ = dev.write(&packet);
                                    }

                                    // Subsequent chunks
                                    for i in (chunk_size..zip_data.len()).step_by(chunk_size) {
                                        let end = (i + chunk_size).min(zip_data.len());
                                        let chunk = &zip_data[i..end];
                                        let mut padded = vec![0u8; PACKET_SIZE];
                                        padded[..chunk.len()].copy_from_slice(chunk);
                                        if let Some(ref dev) = hid_device {
                                            let _ = dev.write(&padded);
                                        }
                                    }
                                }
                                Err(e) => {
                                    error!("SetButtons: failed to prepare ZIP: {}", e);
                                }
                            }
                        }
                        DeviceCommand::Stop => {
                            break;
                        }
                    }
                }
                Err(mpsc::RecvTimeoutError::Timeout) => {
                    // No command, continue to read from device
                }
                Err(mpsc::RecvTimeoutError::Disconnected) => {
                    break;
                }
            }

            // Read from device (non-blocking poll)
            if let Some(ref dev) = hid_device {
                let mut packet = vec![0u8; PACKET_SIZE];
                match dev.read_timeout(&mut packet, 0) {
                    Ok(n) => {
                        if n < 8 {
                            continue;
                        }
                        packet.truncate(n);

                        match parse_input(&packet) {
                            Ok((action, info_opt)) => {
                                if info_opt.is_some() {
                                    // Device info received - refresh and set brightness
                                    let _ = refresh_sender.send(());
                                    let _ = dev.write(&build_packet(
                                        CommandProtocol::OutSetBrightness,
                                        3,
                                        b"100",
                                    ));
                                }

                                if let Some(action) = action {
                                    let i = action.index as usize;
                                    if i < BUTTON_COUNT && action.pressed {
                                        if i == 12 {
                                            state.small_window_mode =
                                                state.small_window_mode.next();
                                        }
                                        let _ =
                                            key_sender.send(KeyPressedEvent { index: i });
                                    } else if i >= BUTTON_COUNT {
                                        warn!("Unknown button index: {}", i);
                                    }
                                }
                            }
                            Err(e) => {
                                warn!("Error parsing input: {}", e);
                            }
                        }
                    }
                    Err(_) => {
                        // Non-blocking read, timeout or error is expected
                        // Try to reconnect
                        hid_device = None;
                        if !connect_to_device(&mut hid_device) {
                            std::thread::sleep(std::time::Duration::from_secs(3));
                        }
                    }
                }
            }
        }
    }

    /// Stop the device loops
    pub fn stop(&self) {
        let _ = self.cmd_sender.send(DeviceCommand::Stop);
    }
}

/// Connect to the Ulanzi D200 device
fn connect_to_device(hid_device: &mut Option<HidDevice>) -> bool {
    if hid_device.is_some() {
        hid_device.take();
        std::thread::sleep(std::time::Duration::from_secs(3));
    }

    let hid_api = match HidApi::new() {
        Ok(api) => api,
        Err(e) => {
            error!("Failed to create HidApi: {}", e);
            return false;
        }
    };

    let mut devices: Vec<_> = hid_api.device_list().collect();
    devices.sort_by(|a, b| a.path().cmp(b.path()));

    for (i, device_info) in devices.iter().enumerate() {
        if device_info.vendor_id() != VENDOR_ID
            || device_info.product_id() != PRODUCT_ID
            || device_info.interface_number() != 0
        {
            continue;
        }

        info!("HID #{}", i);
        info!("  Vendor ID:    #{:04x}", device_info.vendor_id());
        info!("  Product ID:   #{:04x}", device_info.product_id());
        info!("  Release:      {}", device_info.release_number());
        info!(
            "  Serial:       {}",
            device_info.serial_number().unwrap_or("<none>")
        );
        info!(
            "  Manufacturer: {}",
            device_info.manufacturer_string().unwrap_or("<none>")
        );
        info!(
            "  Product:      {}",
            device_info.product_string().unwrap_or("<none>")
        );
        info!("  Usage Page:   #{:04x}", device_info.usage_page());
        info!("  Usage:        {}", device_info.usage());
        info!("  Interface:    {}", device_info.interface_number());

        match device_info.open_device(&hid_api) {
            Ok(dev) => {
                info!("  Device opened successfully.");
                *hid_device = Some(dev);
                return true;
            }
            Err(e) => {
                error!("  Error opening device: {}", e);
            }
        }
    }

    false
}
