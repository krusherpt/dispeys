use std::collections::HashMap;
use std::fs;
use std::io::{Cursor, Read, Write};
use std::path::Path;

use crate::enums::*;

/// Hash button configuration to detect changes (same algorithm as Go)
fn buttons_hash(buttons: &HashMap<usize, Button>) -> u32 {
    let mut h: u32 = 0;
    for (i, btn) in buttons {
        h ^= (*i as u32).wrapping_mul(2654435761);
        for c in btn.name.chars() {
            h ^= (c as u32).wrapping_mul(2654435761);
        }
        for c in btn.icon.chars() {
            h ^= (c as u32).wrapping_mul(2654435761);
        }
    }
    h
}

/// Build the ZIP archive for button update.
/// Matches the Go implementation: manifest.json + icons + dummy padding for ZIP validity.
pub fn prepare_zip(buttons: &HashMap<usize, Button>, icon_path: &Path, tmp_path: &Path) -> Result<Vec<u8>, String> {
    let build_path = tmp_path.join(".build");
    let page_path = build_path.join("page");

    // Clean and recreate
    if page_path.exists() {
        fs::remove_dir_all(&page_path).map_err(|e| format!("Failed to remove build dir: {}", e))?;
    }
    fs::create_dir_all(page_path.join("icons")).map_err(|e| format!("Failed to create build dir: {}", e))?;

    // Build manifest
    let mut manifest: HashMap<String, serde_json::Value> = HashMap::new();
    let mut icons: Vec<String> = Vec::new();

    for (index, btn) in buttons {
        let row = index / BUTTON_COLS;
        let col = index % BUTTON_COLS;
        let key = format!("{}_{}", col, row);

        let mut entry = serde_json::Map::new();
        entry.insert("State".to_string(), serde_json::json!(0));
        let mut params: Vec<serde_json::Value> = Vec::new();
        let mut param = serde_json::Map::new();

        if !btn.name.is_empty() {
            param.insert("Text".to_string(), serde_json::json!(btn.name));
        }
        if !btn.icon.is_empty() {
            icons.push(btn.icon.clone());
            param.insert("Icon".to_string(), serde_json::json!(format!("icons/{}", btn.icon)));
        }
        params.push(serde_json::Value::Object(param));
        entry.insert("ViewParam".to_string(), serde_json::json!(params));

        manifest.insert(key, serde_json::Value::Object(entry));
    }

    let manifest_data = serde_json::to_string_pretty(&manifest)
        .map_err(|e| format!("Failed to serialize manifest: {}", e))?;

    // Write manifest
    fs::write(
        page_path.join("manifest.json"),
        manifest_data.as_bytes(),
    ).map_err(|e| format!("Failed to write manifest: {}", e))?;

    // Copy icons
    for icon in &icons {
        let src = icon_path.join(icon);
        let dst = page_path.join("icons").join(icon);
        if src.exists() {
            fs::copy(&src, &dst).map_err(|e| format!("Failed to copy icon {}: {}", icon, e))?;
        }
    }

    // Build ZIP with deterministic padding to avoid invalid bytes at packet boundaries
    let max_retries = 10;
    for attempt in 0..=max_retries {
        // Write dummy padding file if needed
        if attempt > 0 {
            let mut padding = vec![0u8; attempt * 64];
            for (i, p) in padding.iter_mut().enumerate() {
                *p = ((i * 7 + 3) % 256) as u8;
                if *p == 0x00 || *p == 0x7c {
                    *p = 0x41;
                }
            }
            fs::write(page_path.join("dummy.txt"), &padding)
                .map_err(|e| format!("Failed to write dummy: {}", e))?;
        }

        let zip_data = zip_folder(&page_path).map_err(|e| format!("Failed to create ZIP: {}", e))?;

        if crate::packet::zip_is_valid(&zip_data) {
            return Ok(zip_data);
        }

        // Small delay before retry (not really needed in sync code, but matching Go)
        std::thread::sleep(std::time::Duration::from_millis(10));
    }

    Err("Failed to create valid ZIP after retries".to_string())
}

/// Recursively zip a directory
fn zip_folder(src_dir: &Path) -> Result<Vec<u8>, String> {
    let mut cursor = Cursor::new(Vec::new());

    {
        let mut zip = zip::ZipWriter::new(&mut cursor);
        let options: zip::write::FileOptions<'_, ()> = zip::write::FileOptions::default()
            .compression_method(zip::CompressionMethod::Deflated)
            .unix_permissions(0o644);

        let mut buffer = Vec::new();
        for entry in walkdir(src_dir) {
            let entry = entry.map_err(|e| format!("Walk error: {}", e))?;
            let path = &entry.path;
            let stripped = path.strip_prefix(src_dir).map_err(|e| format!("Strip prefix error: {}", e))?;

            if path == src_dir {
                continue;
            }

            if entry.file_type.is_dir() {
                zip.add_directory(
                    stripped.to_string_lossy(),
                    options,
                ).map_err(|e| format!("Add directory error: {}", e))?;
            } else {
                zip.start_file(
                    stripped.to_string_lossy(),
                    options,
                ).map_err(|e| format!("Start file error: {}", e))?;

                let mut f = fs::File::open(path).map_err(|e| format!("Open file error: {}", e))?;
                f.read_to_end(&mut buffer).map_err(|e| format!("Read file error: {}", e))?;
                zip.write_all(&buffer).map_err(|e| format!("Write to zip error: {}", e))?;
                buffer.clear();
            }
        }

        zip.finish().map_err(|e| format!("Finish zip error: {}", e))?;
    }

    Ok(cursor.into_inner())
}

/// Simple recursive directory walker (avoids external dependency)
fn walkdir(dir: &Path) -> impl Iterator<Item = Result<DirEntry, String>> {
    WalkDir::new(dir)
}

struct WalkDir {
    stack: Vec<WalkEntry>,
}

struct WalkEntry {
    path: std::path::PathBuf,
    entries: std::fs::ReadDir,
}

struct DirEntry {
    path: std::path::PathBuf,
    file_type: std::fs::FileType,
}

impl WalkDir {
    fn new(dir: &Path) -> Self {
        let entries = fs::read_dir(dir).unwrap_or_else(|_| {
            std::fs::read_dir(dir).expect("WalkDir: initial read_dir failed")
        });
        WalkDir {
            stack: vec![WalkEntry {
                path: dir.to_path_buf(),
                entries,
            }],
        }
    }
}

impl Iterator for WalkDir {
    type Item = Result<DirEntry, String>;

    fn next(&mut self) -> Option<Self::Item> {
        loop {
            let top = self.stack.last_mut()?;
            match top.entries.next() {
                Some(Ok(entry)) => {
                    let path = entry.path();
                    let ft = entry.file_type().ok()?;
                    if ft.is_dir() {
                        let sub_entries = fs::read_dir(&path).ok()?;
                        self.stack.push(WalkEntry {
                            path: path.clone(),
                            entries: sub_entries,
                        });
                    }
                    return Some(Ok(DirEntry {
                        path,
                        file_type: ft,
                    }));
                }
                Some(Err(e)) => return Some(Err(format!("ReadDir error: {}", e))),
                None => { self.stack.pop(); }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_buttons_hash() {
        let mut buttons = HashMap::new();
        buttons.insert(0, Button {
            name: "Test".to_string(),
            icon: "test.png".to_string(),
        });

        let hash = buttons_hash(&buttons);
        assert_ne!(hash, 0);

        // Same config should produce same hash
        let hash2 = buttons_hash(&buttons);
        assert_eq!(hash, hash2);
    }
}
