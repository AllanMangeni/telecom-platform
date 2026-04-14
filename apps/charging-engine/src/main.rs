mod api;
mod charging_new as charging;
mod errors_new as errors;
mod models;

// Include the new modular components
mod charging_types;
mod charging_engine;
mod credit_management;
mod rating_billing;
mod monitoring_sync;
mod sync_operations;
mod monitoring;
mod error_types;
mod error_helpers;

use anyhow::Result;
use std::sync::Arc;
use tracing::info;

use api::create_router;
use charging::ChargingEngine;

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize tracing
    tracing_subscriber::fmt()
        .with_target(false)
        .compact()
        .init();

    // Load environment variables
    dotenv::dotenv().ok();

    // Initialize charging engine
    let redis_url = std::env::var("REDIS_URI").unwrap_or_else(|_| "redis://127.0.0.1/".to_string());
    let sync_interval = std::env::var("SYNC_INTERVAL")
        .unwrap_or_else(|_| "1".to_string())
        .parse::<u64>()
        .unwrap_or(1);

    let charging_engine = Arc::new(ChargingEngine::new(&redis_url, sync_interval)?);
    
    // Test Redis connection
    charging_engine.test_connection().await?;
    info!("Connected to Redis successfully");

    // Create application state
    let state = api::AppState {
        charging_engine,
    };

    // Create router
    let app = create_router(state);

    // Start server
    let port = std::env::var("SERVER_PORT").unwrap_or_else(|_| "8080".to_string());
    let addr = format!("0.0.0.0:{}", port);
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    
    info!("Charging engine listening on {}", addr);
    
    axum::serve(listener, app).await?;

    Ok(())
}
