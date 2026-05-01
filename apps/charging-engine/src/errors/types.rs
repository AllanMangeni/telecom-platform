use axum::response::{IntoResponse, Response};
use axum::http::StatusCode;
use serde_json::json;
use thiserror::Error;

#[derive(Debug, Error, Clone)]
pub enum ChargingError {
    #[error("Redis connection error: {0}")]
    RedisConnection(String),
    
    #[error("Redis operation error: {0}")]
    RedisOperation(String),
    
    #[error("Database error: {0}")]
    DatabaseError(String),
    
    #[error("Subscriber not found: {0}")]
    SubscriberNotFound(String),
    
    #[error("Rating plan not found: {0}")]
    RatingPlanNotFound(String),
    
    #[error("Insufficient credit: available={available}, requested={requested}")]
    InsufficientCredit { available: u64, requested: u64 },
    
    #[error("Usage blocked: {0}")]
    UsageBlocked(String),
    
    #[error("Invalid input: {0}")]
    InvalidInput(String),
    
    #[error("Serialization error: {0}")]
    SerializationError(String),
    
    #[error("Internal error: {0}")]
    InternalError(String),
}

impl IntoResponse for ChargingError {
    fn into_response(self) -> Response {
        let (status, message) = match &self {
            ChargingError::RedisConnection(_) => (StatusCode::SERVICE_UNAVAILABLE, "Redis connection error"),
            ChargingError::RedisOperation(_) => (StatusCode::INTERNAL_SERVER_ERROR, "Redis operation error"),
            ChargingError::DatabaseError(_) => (StatusCode::INTERNAL_SERVER_ERROR, "Database error"),
            ChargingError::SubscriberNotFound(_) => (StatusCode::NOT_FOUND, "Subscriber not found"),
            ChargingError::RatingPlanNotFound(_) => (StatusCode::NOT_FOUND, "Rating plan not found"),
            ChargingError::InsufficientCredit { .. } => (StatusCode::PAYMENT_REQUIRED, "Insufficient credit"),
            ChargingError::UsageBlocked(_) => (StatusCode::FORBIDDEN, "Usage blocked"),
            ChargingError::InvalidInput(_) => (StatusCode::BAD_REQUEST, "Invalid input"),
            ChargingError::SerializationError(_) => (StatusCode::INTERNAL_SERVER_ERROR, "Serialization error"),
            ChargingError::InternalError(_) => (StatusCode::INTERNAL_SERVER_ERROR, "Internal error"),
        };

        let body = json!({
            "error": message,
            "detail": self.to_string(),
        });

        (status, axum::Json(body)).into_response()
    }
}
