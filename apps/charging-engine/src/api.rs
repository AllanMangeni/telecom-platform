use axum::{
    extract::{Path, State},
    response::IntoResponse,
    routing::{get, post},
    Json, Router,
};
use tower_http::cors::{Any, CorsLayer};
use tracing::{info, warn};

use crate::errors::{ChargingError, ChargingResult, validate_ip, validate_bytes, ErrorContext, log_error, validate_amount, validate_session_id};
use crate::models::*;

#[derive(Clone)]
pub struct AppState {
    pub charging_engine: std::sync::Arc<crate::charging::ChargingEngine>,
}

pub fn create_router(state: AppState) -> Router {
    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any);

    Router::new()
        .route("/v1/credit/:ip/check", post(check_credit))
        .route("/v1/credit/:ip/deduct", post(deduct_credit))
        .route("/v1/credit/:ip/add", post(add_credit))
        .route("/v1/credit/:ip/balance", get(get_balance))
        .route("/v1/subscriber/:imsi", get(get_subscriber))
        .route("/v1/usage", post(record_usage))
        .route("/health", get(health_check))
        .layer(cors)
        .with_state(state)
}

/// POST /v1/credit/:ip/check
/// Check if user has enough credit for data usage
pub async fn check_credit(
    Path(ip): Path<String>,
    State(state): State<AppState>,
    Json(req): Json<CreditCheckRequest>,
) -> ChargingResult<Json<CreditCheckResponse>> {
    // Validate input
    validate_ip(&ip)?;
    validate_bytes(req.bytes_requested)?;

    let allowed = state.charging_engine.check_credit(&ip, req.bytes_requested).await
        .with_context("Failed to check credit")
        .map_err(|e| {
            log_error(&e);
            e
        })?;
    
    let remaining = state.charging_engine.get_balance(&ip).await
        .with_context("Failed to get balance")?;
    
    info!(
        "Credit check for {}: {} bytes requested, {} bytes available, allowed: {}",
        ip, req.bytes_requested, remaining, allowed
    );

    Ok(Json(CreditCheckResponse {
        allowed,
        remaining_bytes: remaining as i64,
    }))
}

/// POST /v1/credit/:ip/deduct
/// Deduct bytes from user's credit balance
pub async fn deduct_credit(
    Path(ip): Path<String>,
    State(state): State<AppState>,
    Json(req): Json<DeductRequest>,
) -> ChargingResult<()> {
    // Validate input
    validate_ip(&ip)?;
    validate_bytes(req.bytes_used)?;
    validate_amount(req.bytes_used as f64)?;

    let new_balance = state.charging_engine.deduct_credit(&ip, req.bytes_used).await
        .with_context("Failed to deduct credit")?;
    
    info!(
        "User {} deducted {} bytes, remaining: {}",
        ip, req.bytes_used, new_balance
    );

    Ok(())
}

/// POST /v1/credit/:ip/add
/// Add bytes to user's credit balance
pub async fn add_credit(
    Path(ip): Path<String>,
    State(state): State<AppState>,
    Json(req): Json<AddCreditRequest>,
) -> ChargingResult<()> {
    // Validate input
    validate_ip(&ip)?;
    validate_bytes(req.bytes_to_add)?;
    validate_amount(req.bytes_to_add as f64)?;

    let new_balance = state.charging_engine.add_credit(&ip, req.bytes_to_add).await
        .with_context("Failed to add credit")?;
    
    info!(
        "User {} added {} bytes, new balance: {}",
        ip, req.bytes_to_add, new_balance
    );

    Ok(())
}

/// GET /v1/credit/:ip/balance
/// Get current credit balance
pub async fn get_balance(
    Path(ip): Path<String>,
    State(state): State<AppState>,
) -> ChargingResult<Json<BalanceResponse>> {
    // Validate input
    validate_ip(&ip)?;

    let balance = state.charging_engine.get_balance(&ip).await
        .with_context("Failed to get balance")?;

    info!("Retrieved balance for IP: {}", ip);

    Ok(Json(BalanceResponse {
        ip: ip.clone(),
        balance_bytes: balance as i64,
    }))
}

/// GET /v1/subscriber/:imsi
/// Get subscriber account by IMSI
pub async fn get_subscriber(
    Path(imsi): Path<String>,
    State(state): State<AppState>,
) -> ChargingResult<Json<serde_json::Value>> {
    let account = state.charging_engine.get_subscriber_account(&imsi).await
        .with_context("Failed to get subscriber account")?;

    match account {
        Some(acc) => Ok(Json(serde_json::json!({
            "imsi": acc.imsi,
            "balance": acc.balance,
            "data_used": acc.data_used,
            "data_limit": acc.data_limit,
            "voice_used": acc.voice_used,
            "voice_limit": acc.voice_limit,
            "sms_used": acc.sms_used,
            "sms_limit": acc.sms_limit,
        }))),
        None => Err(ChargingError::SubscriberNotFound(imsi)),
    }
}

/// POST /v1/usage
/// Record usage event
pub async fn record_usage(
    State(state): State<AppState>,
    Json(req): Json<serde_json::Value>,
) -> ChargingResult<Json<serde_json::Value>> {
    let imsi = req.get("imsi").and_then(|v| v.as_str()).ok_or_else(|| {
        ChargingError::InvalidInput("Missing IMSI".to_string())
    })?;
    let session_id = req.get("session_id").and_then(|v| v.as_str()).ok_or_else(|| {
        ChargingError::InvalidInput("Missing session_id".to_string())
    })?;
    
    validate_session_id(session_id)?;

    // Create and record usage event
    let event = crate::charging::types::UsageEvent {
        imsi: imsi.to_string(),
        session_id: session_id.to_string(),
        volume: req.get("volume").and_then(|v| v.as_u64()).unwrap_or(0),
        cost: req.get("cost").and_then(|v| v.as_f64()).unwrap_or(0.0),
        rate: req.get("rate").and_then(|v| v.as_f64()).unwrap_or(0.0),
        usage_type: crate::charging::types::UsageType::Data,
        timestamp: chrono::Utc::now(),
    };

    state.charging_engine.record_usage_event(&event).await
        .with_context("Failed to record usage event")?;

    Ok(Json(serde_json::json!({
        "status": "recorded",
        "imsi": imsi,
        "session_id": session_id,
    })))
}

/// GET /health
/// Health check endpoint
pub async fn health_check() -> ChargingResult<Json<HealthResponse>> {
    Ok(Json(HealthResponse::default()))
}
