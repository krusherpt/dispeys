//! dispeys-detector: Active window detection for Linux (X11)
//!
//! Detects the currently focused window's process using xdotool, xprop, and ps.

use std::os::unix::process::CommandExt;
use std::process::Command;
use std::sync::Arc;
use std::time::Duration;

use dispeys_settings::{AppSettings, Button, load_app_settings, save_app_settings};
use log::{error, info};
use tokio::sync::mpsc;

/// Application with its button configuration
pub type Application = dispeys_settings::Application;

/// Active window detector
pub struct AppDetector {
    settings_file_path: String,
    icons_dir_path: String,
    process_changed_chan: mpsc::UnboundedSender<AppSettings>,
}

impl AppDetector {
    /// Create a new AppDetector
    pub fn new(settings_file_path: String, icons_dir_path: String) -> Self {
        AppDetector {
            settings_file_path,
            icons_dir_path,
            process_changed_chan: mpsc::unbounded_channel().0,
        }
    }

    /// Get a sender for process change events
    pub fn process_changed_sender(&self) -> mpsc::UnboundedSender<AppSettings> {
        self.process_changed_chan.clone()
    }

    /// Start the detection loop
    pub fn start(&self) {
        let settings_path = self.settings_file_path.clone();
        let icons_path = self.icons_dir_path.clone();
        let sender = self.process_changed_chan.clone();

        tokio::spawn(async move {
            Self::detection_loop(settings_path, icons_path, sender).await;
        });
    }

    /// Main detection loop
    async fn detection_loop(
        settings_path: String,
        icons_path: String,
        sender: mpsc::UnboundedSender<AppSettings>,
    ) {
        let mut process_name = String::new();
        let mut win_id = String::new();
        let mut last_query_time = tokio::time::Instant::now();

        loop {
            // Throttle: only do full query if >1s since last
            let should_full_query = last_query_time.elapsed() >= Duration::from_secs(1);

            if !should_full_query {
                // Quick check: just get window ID
                if let Ok(current_win_id) = get_active_window_id() {
                    if current_win_id == win_id {
                        tokio::time::sleep(Duration::from_millis(200)).await;
                        continue;
                    }
                }
            }

            // Full query
            match get_active_window_process_name(&win_id) {
                Ok((current_process_name, current_win_id)) => {
                    if current_process_name.is_empty() {
                        tokio::time::sleep(Duration::from_secs(2)).await;
                        continue;
                    }

                    if process_name != current_process_name {
                        process_name = current_process_name.clone();
                        win_id = current_win_id;
                        last_query_time = tokio::time::Instant::now();

                        // Load settings for this process
                        let settings = match load_app_settings(
                            &std::path::Path::new(&settings_path),
                            &std::path::Path::new(&icons_path),
                        ) {
                            Ok(s) => s,
                            Err(e) => {
                                error!("Failed to load settings: {}", e);
                                tokio::time::sleep(Duration::from_secs(2)).await;
                                continue;
                            }
                        };

                        if let Some(app) = settings.get_for_process(&process_name) {
                            info!("process changed to {} done", process_name);
                            let _ = sender.send(settings);
                        }
                    }
                }
                Err(e) => {
                    error!("Detection error: {}", e);
                }
            }

            tokio::time::sleep(Duration::from_secs(2)).await;
        }
    }
}

/// Get the active window ID using xdotool (fast, lightweight)
fn get_active_window_id() -> Result<String, String> {
    let output = Command::new("xdotool")
        .arg("getactivewindow")
        .output()
        .map_err(|e| format!("xdotool failed: {}", e))?;

    Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
}

/// Get the active window's process name using xdotool + xprop + ps
fn get_active_window_process_name(prev_win_id: &str) -> Result<(String, String), String> {
    // Get window ID
    let win_id = get_active_window_id()?;
    if win_id == prev_win_id {
        return Ok((String::new(), win_id));
    }

    // Get PID from window
    let output = Command::new("xprop")
        .args(["-id", &win_id, "_NET_WM_PID"])
        .output()
        .map_err(|e| format!("xprop failed: {}", e))?;

    let line = String::from_utf8_lossy(&output.stdout);
    let parts: Vec<&str> = line.split(" = ").collect();
    if parts.len() != 2 {
        return Err(format!("Unexpected xprop format: {}", line));
    }
    let pid = parts[1].trim();

    // Get process name from PID
    let output = Command::new("ps")
        .args(["-p", pid, "-o", "comm="])
        .output()
        .map_err(|e| format!("ps failed: {}", e))?;

    let process_name = String::from_utf8_lossy(&output.stdout).trim().to_string();

    Ok((process_name, win_id))
}

/// Focus or launch an application
pub fn focus_or_run(program: &str, args: &[&str]) -> Result<(), String> {
    let mut cmd = Command::new(program);
    cmd.args(args);
    cmd.stdin(std::process::Stdio::null());
    cmd.stdout(std::process::Stdio::null());
    cmd.stderr(std::process::Stdio::null());

    // Detach from parent (setsid)
    unsafe {
        cmd.pre_exec(|| {
            libc::setsid();
            Ok(())
        });
    }

    cmd.spawn()
        .map(|_| ())
        .map_err(|e| format!("Failed to start {}: {}", program, e))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_focus_or_run() {
        // Just check it doesn't panic for a non-existent program
        let result = focus_or_run("nonexistent_program_xyz", &[]);
        assert!(result.is_err());
    }
}
