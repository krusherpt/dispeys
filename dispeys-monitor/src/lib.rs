//! dispeys-monitor: Hardware monitoring (CPU, memory, GPU)

use std::process::Command;
use std::sync::Mutex;

use sysinfo::System;

lazy_static::lazy_static! {
    static ref SYSTEM: Mutex<System> = Mutex::new({
        let mut sys = System::new_all();
        sys.refresh_cpu_all();
        sys
    });
}

/// Get CPU usage percentage
pub fn get_cpu_usage() -> f64 {
    let sys = match SYSTEM.lock() {
        Ok(s) => s,
        Err(_) => return 0.0,
    };

    sys.global_cpu_usage() as f64
}

/// Get memory usage percentage
pub fn get_memory_usage() -> f64 {
    let sys = match SYSTEM.lock() {
        Ok(s) => s,
        Err(_) => return 0.0,
    };

    let used_percent = sys.used_memory() as f64 / sys.total_memory() as f64 * 100.0;
    used_percent
}

/// Get GPU usage percentage via nvidia-smi
pub fn get_gpu_usage() -> Result<f64, String> {
    let output = Command::new("nvidia-smi")
        .args(["--query-gpu=utilization.gpu", "--format=csv,noheader,nounits"])
        .output()
        .map_err(|e| format!("nvidia-smi not found or not installed: {}", e))?;

    if !output.status.success() {
        return Err(format!("nvidia-smi failed with exit code: {:?}", output.status.code()));
    }

    let str = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let value: f64 = str
        .parse()
        .map_err(|e| format!("Failed to parse nvidia-smi output: {}", e))?;

    Ok(value)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cpu_usage() {
        let cpu = get_cpu_usage();
        assert!(cpu >= 0.0 && cpu <= 100.0);
    }

    #[test]
    fn test_memory_usage() {
        let mem = get_memory_usage();
        assert!(mem >= 0.0 && mem <= 100.0);
    }
}
