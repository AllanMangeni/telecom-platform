use axum::response::{IntoResponse, Response};
use axum::http::StatusCode;
use serde_json::json;
use std::fmt;

#[derive(Debug, Clone)]
pub enum ChargingError {
    RedisConnection(String),
    RedisOperation(String),
    SubscriberNotFound(String),
    RatingPlanNotFound(String),
    InsufficientCredit { available: u64, requested: u64 },
    UsageBlocked(String),
    InvalidInput(String),
    SerializationError(String),
    #[allow(dead_code)]
    ConfigurationError(String),
    InternalError(String),
}

impl fmt::Display for ChargingError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ChargingError::RedisConnection(msg) => write!(f, "Redis connection error: {}", msg),
            ChargingError::RedisOperation(msg) => write!(f, "Redis operation error: {}", msg),
            ChargingError::SubscriberNotFound(imsi) => write!(f, "Subscriber not found: {}", imsi),
            ChargingError::RatingPlanNotFound(plan_id) => write!(f, "Rating plan not found: {}", plan_id),
            ChargingError::InsufficientCredit { available, requested } => {
                write!(f, "Insufficient credit: available={}, requested={}", available, requested)
            }
            ChargingError::UsageBlocked(reason) => write!(f, "Usage blocked: {}", reason),
            ChargingError::InvalidInput(msg) => write!(f, "Invalid input: {}", msg),
            ChargingError::SerializationError(msg) => write!(f, "Serialization error: {}", msg),
            ChargingError::ConfigurationError(msg) => write!(f, "Configuration error: {}", msg),
            ChargingError::InternalError(msg) => write!(f, "Internal error: {}", msg),
        }
    }
}

impl std::error::Error for ChargingError {}

impl IntoResponse for ChargingError {
    fn into_response(self) -> Response {
        let (status, error_message) = match self {
            ChargingError::RedisConnection(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
            ChargingError::RedisOperation(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
            ChargingError::SubscriberNotFound(imsi) => (StatusCode::NOT_FOUND, format!("Subscriber not found: {}", imsi)),
            ChargingError::RatingPlanNotFound(plan_id) => (StatusCode::NOT_FOUND, format!("Rating plan not found: {}", plan_id)),
            ChargingError::InsufficientCredit { available, requested } => {
                (StatusCode::PAYMENT_REQUIRED, format!("Insufficient credit: available={}, requested={}", available, requested))
            }
            ChargingError::UsageBlocked(reason) => (StatusCode::FORBIDDEN, reason),
            ChargingError::InvalidInput(msg) => (StatusCode::BAD_REQUEST, msg),
            ChargingError::SerializationError(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
            ChargingError::ConfigurationError(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
            ChargingError::InternalError(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
        };

        let body = json!({
            "error": error_message
        });

        (status, axum::Json(body)).into_response()
    }
}
