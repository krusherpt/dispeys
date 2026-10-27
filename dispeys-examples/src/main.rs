//! Example: Full dispeys application
//!
//! Connects to Ulanzi D200 device, detects active window, and switches button layouts.

use std::collections::HashMap;
use std::env;
use std::path::PathBuf;
use std::process::Command;

use dispeys_detector::{AppDetector, focus_or_run};
use dispeys_hid::{Button, DeviceConfig, KeyPressedEvent, SmallWindowMode, UlanziD200Device};
use dispeys_monitor::{get_cpu_usage, get_memory_usage};
use dispeys_settings::{AppSettings, load_app_settings, save_app_settings};

fn home_dir() -> PathBuf {
    dirs::home_dir().expect("Cannot determine home directory")
}

fn settings_path() -> PathBuf {
    home_dir().join(".config").join("dispeysController").join("settings.json")
}

fn icons_dir() -> PathBuf {
    home_dir().join(".config").join("dispeysController").join("icons")
}

fn temp_dir() -> PathBuf {
    env::temp_dir().join("dispeysController")
}

#[tokio::main]
async fn main() {
    env_logger::init();

    let settings_path = settings_path();
    let icons_dir = icons_dir();
    let temp_dir = temp_dir();

    // Create default files if they don't exist
    if !settings_path.exists() {
        println!("Creating default settings...");
        dispeys_settings::create_default_files(&settings_path, &icons_dir)
            .expect("Failed to create default files");
    }

    // Ensure temp directory exists
    std::fs::create_dir_all(&temp_dir).expect("Failed to create temp dir");

    // Load settings
    let settings = load_app_settings(&settings_path, &icons_dir)
        .expect("Failed to load settings");

    // Create device
    let config = DeviceConfig {
        small_window_mode: SmallWindowMode::Clock,
        icon_path: icons_dir.clone(),
        tmp_path: temp_dir.clone(),
    };
    let mut device = UlanziD200Device::new(config.clone());

    // Start device
    device.start(config).expect("Failed to start device");

    // Set initial brightness
    let _ = device.set_brightness(100, true);

    // Create app detector
    let detector = AppDetector::new(
        settings_path.to_string_lossy().to_string(),
        icons_dir.to_string_lossy().to_string(),
    );
    detector.start();

    // Set initial default settings
    let default_settings = settings.get_for_process("default");
    if let Some(app) = default_settings {
        let buttons = app
            .buttons
            .iter()
            .enumerate()
            .map(|(i, btn)| {
                (
                    i,
                    Button {
                        name: btn.name.clone(),
                        icon: btn.icon.clone(),
                    },
                )
            })
            .collect::<HashMap<_, _>>();
        let _ = device.set_buttons(buttons, false);
    }

    println!("dispeys running. Press Ctrl+C to exit.");

    // Main loop: handle key presses and update stats
    loop {
        tokio::select! {
            event = device.key_pressed_receiver().recv() => {
                if let Some(event) = event {
                    handle_key_press(&device, &event).await;
                }
            }
            _ = tokio::time::sleep(std::time::Duration::from_millis(500)) => {
                let cpu = get_cpu_usage() as i32;
                let mem = get_memory_usage() as i32;
                let time = chrono::Local::now().format("%H:%M:%S").to_string();

                let data = dispeys_hid::SmallWindowData {
                    mode: SmallWindowMode::Clock,
                    cpu,
                    mem,
                    gpu: 0,
                    time,
                };

                let _ = device.set_small_window_data(data, false);
            }
        }
    }
}

async fn handle_key_press(device: &UlanziD200Device, event: &KeyPressedEvent) {
    println!("Button pressed: index {}", event.index);

    let settings_path = settings_path();
    let settings = match load_app_settings(&settings_path, &icons_dir()) {
        Ok(s) => s,
        Err(e) => {
            log::error!("Failed to load settings: {}", e);
            return;
        }
    };

    let app = match settings.get_for_process("default") {
        Some(a) => a,
        None => return,
    };

    let button = match app.buttons.get(event.index) {
        Some(b) => b,
        None => return,
    };

    if button.command.is_empty() {
        return;
    }

    if button.command.starts_with('@') {
        let target = button.command.trim_start_matches('@').trim();
        if target.is_empty() {
            log::info!("Returning to process detection mode");
            return;
        }
        log::info!("Switching to process config: {}", target);
    } else if button.command.starts_with('$') {
        let program = button.command.trim_start_matches('$').trim();
        if !program.is_empty() {
            log::info!("Launching: {}", program);
            if let Err(e) = focus_or_run(program, &[]) {
                log::error!("Failed to launch {}: {}", program, e);
            }
        }
    } else {
        log::info!("Running command: {}", button.command);
        if let Err(e) = Command::new("sh").arg("-c").arg(&button.command).output() {
            log::error!("Command failed: {}", e);
        }
    }
}
