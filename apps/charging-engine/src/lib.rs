//! # Charging Engine
//!
//! A high-performance telecom charging engine for real-time usage-based billing.
//!
//! ## Features
//!
//! - Real-time rating and billing
//! - Circuit breaker for fault tolerance
//! - Rate limiting for API protection
//! - Comprehensive monitoring and metrics
//!
//! ## Example
//!
//! ```rust,no_run
//! use charging_engine::ChargingEngine;
//! use charging_engine::charging::types::UsageEvent;
//!
//! #[tokio::main]
//! async fn main() -> Result<(), Box<dyn std::error::Error>> {
//!     let engine = ChargingEngine::new("redis://localhost").await?;
//!
//!     let event = UsageEvent {
//!         imsi: "1234567890".to_string(),
//!         volume: 1000,
//!         timestamp: chrono::Utc::now(),
//!     };
//!
//!     let cost = engine.calculate_cost(&event).await?;
//!     println!("Cost: {}", cost);
//!
//!     Ok(())
//! }
//! ```

pub mod auth;
pub mod charging;
pub mod circuit_breaker;
pub mod errors;
// pub mod rate_limit;
pub mod handlers;
pub mod routes;
pub mod models;
pub mod monitoring;
