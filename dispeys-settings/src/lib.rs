//! dispeys-settings: Configuration management for Ulanzi D200 button layouts
//!
//! Handles loading, saving, and creating default settings files.

use std::collections::HashMap;
use std::fs;
use std::io;
use std::path::Path;
use std::time::SystemTime;

use include_dir::{include_dir, Dir};

/// Embedded default settings JSON
const DEFAULT_SETTINGS: &str = include_str!("defaults/settings_default.json");

/// Embedded default icons directory
const DEFAULT_ICONS: Dir = include_dir!("$CARGO_MANIFEST_DIR/src/defaults/icons");

/// A button configuration
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct Button {
    pub name: String,
    pub icon: String,
    pub command: String,
}

/// An application's button layout
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct Application {
    pub name: String,
    pub buttons: Vec<Button>,
}

/// Application settings with file tracking
#[derive(Debug, Clone)]
pub struct AppSettings {
    pub applications: HashMap<String, Application>,
    last_modified: SystemTime,
}

impl AppSettings {
    /// Create empty settings
    pub fn empty() -> Self {
        AppSettings {
            applications: HashMap::new(),
            last_modified: SystemTime::UNIX_EPOCH,
        }
    }

    /// Get settings for a process, falling back to "default"
    pub fn get_for_process(&self, process: &str) -> Option<&Application> {
        self.applications
            .get(process)
            .or_else(|| self.applications.get("default"))
    }
}

/// Create default settings file and icons if they don't exist
pub fn create_default_files(path: &Path, icons_target_dir: &Path) -> io::Result<()> {
    // Create directories
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir)?;
    }
    fs::create_dir_all(icons_target_dir)?;

    // Write default settings atomically
    let tmp_path = path.with_extension("json.tmp");
    fs::write(&tmp_path, DEFAULT_SETTINGS)?;
    fs::rename(&tmp_path, path)?;

    // Copy default icons
    for entry in DEFAULT_ICONS.files() {
        let target_path = icons_target_dir.join(entry.path().file_name().unwrap());
        if target_path.exists() {
            continue;
        }
        let tmp_target = target_path.with_extension("png.tmp");
        fs::write(&tmp_target, entry.contents())?;
        fs::rename(&tmp_target, target_path)?;
    }

    Ok(())
}

/// Load settings from a JSON file
pub fn load_app_settings(path: &Path, icon_path: &Path) -> Result<AppSettings, String> {
    let fi = fs::metadata(path).map_err(|e| format!("Cannot stat file: {}", e))?;
    let mod_time = fi.modified().map_err(|e| format!("Cannot get mod time: {}", e))?;

    // Read file
    let data = fs::read_to_string(path).map_err(|e| format!("Cannot read file: {}", e))?;

    let apps: HashMap<String, Application> =
        serde_json::from_str(&data).map_err(|e| format!("JSON parse error: {}", e))?;

    Ok(AppSettings {
        applications: apps,
        last_modified: mod_time,
    })
}

/// Save settings to a JSON file (atomic write)
pub fn save_app_settings(path: &Path, settings: &AppSettings) -> Result<(), String> {
    let data = serde_json::to_string_pretty(&settings.applications)
        .map_err(|e| format!("JSON marshal error: {}", e))?;

    // Create directory if needed
    if let Some(dir) = path.parent() {
        fs::create_dir_all(dir).map_err(|e| format!("Cannot create dir: {}", e))?;
    }

    // Atomic write: write to tmp, then rename
    let tmp_path = path.with_extension("json.tmp");
    fs::write(&tmp_path, &data).map_err(|e| format!("Write temp file error: {}", e))?;
    fs::rename(&tmp_path, path).map_err(|e| format!("Rename temp file error: {}", e))?;

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_load_default_settings() {
        let apps: HashMap<String, Application> =
            serde_json::from_str(DEFAULT_SETTINGS).unwrap();
        assert!(apps.contains_key("default"));
        assert!(apps.contains_key("code"));
        assert!(apps.contains_key("select_app"));
    }

    #[test]
    fn test_save_and_load() {
        let tmp = std::env::temp_dir().join("dispeys_test_settings.json");
        let mut settings = AppSettings::empty();
        settings.applications.insert(
            "test".to_string(),
            Application {
                name: "test".to_string(),
                buttons: vec![Button {
                    name: "Test".to_string(),
                    icon: "test.png".to_string(),
                    command: "echo test".to_string(),
                }],
            },
        );

        save_app_settings(&tmp, &settings).unwrap();
        let loaded = load_app_settings(&tmp, &std::env::temp_dir()).unwrap();
        assert!(loaded.applications.contains_key("test"));

        let _ = fs::remove_file(&tmp);
    }
}
