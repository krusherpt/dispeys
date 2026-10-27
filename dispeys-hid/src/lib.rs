//! dispeys-hid: Rust library for communicating with Ulanzi D200 stream deck devices
//!
//! This crate provides low-level HID communication with the Ulanzi D200 device,
//! including button management, small window display, and hardware monitoring.

pub mod device;
pub mod enums;
pub mod packet;
pub mod zip_layout;

pub use device::{DeviceConfig, UlanziD200Device};
pub use enums::*;
