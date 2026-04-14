use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
    Json,
};
use serde_json::json;
use thiserror::Error;
use tracing::error;

#[derive(Error, Debug)]
pub enum ChargingError {
    #[error("Redis connection failed: {0}")]
    RedisConnection(String),
    
    #[error("Redis operation failed: {0}")]
    RedisOperation(String),
    
    #[error("Account not found: {0}")]
    AccountNotFound(String),
    
    #[error("Insufficient credit: available={available}, requested={requested}")]
    InsufficientCredit { available: i64, requested: u64 },
    
    #[error("Invalid request: {0}")]
    InvalidRequest(String),
    
    #[error("Rating plan not found: {0}")]
    RatingPlanNotFound(String),
    
    #[error("Account operation failed: {0}")]
    AccountOperation(String),
    
    #[error("Serialization error: {0}")]
    Serialization(String),
    
    #[error("Internal server error: {0}")]
    Internal(String),
}

impl IntoResponse for ChargingError {
    fn into_response(self) -> Response {
        let (status, error_message) = match self {
            ChargingError::RedisConnection(ref msg) => {
                error!("Redis connection error: {}", msg);
                (StatusCode::SERVICE_UNAVAILABLE, "Redis connection failed")
            }
            ChargingError::RedisOperation(ref msg) => {
                error!("Redis operation error: {}", msg);
                (StatusCode::INTERNAL_SERVER_ERROR, "Redis operation failed")
            }
            ChargingError::AccountNotFound(ref imsi) => {
                (StatusCode::NOT_FOUND, "Account not found")
            }
            ChargingError::InsufficientCredit { available, requested } => {
                (StatusCode::PAYMENT_REQUIRED, "Insufficient credit")
            }
            ChargingError::InvalidRequest(ref msg) => {
                (StatusCode::BAD_REQUEST, "Invalid request")
            }
            ChargingError::RatingPlanNotFound(ref plan_id) => {
                (StatusCode::NOT_FOUND, "Rating plan not found")
            }
            ChargingError::AccountOperation(ref msg) => {
                error!("Account operation error: {}", msg);
                (StatusCode::INTERNAL_SERVER_ERROR, "Account operation failed")
            }
            ChargingError::Serialization(ref msg) => {
                error!("Serialization error: {}", msg);
                (StatusCode::INTERNAL_SERVER_ERROR, "Data serialization failed")
            }
            ChargingError::Internal(ref msg) => {
                error!("Internal error: {}", msg);
                (StatusCode::INTERNAL_SERVER_ERROR, "Internal server error")
            }
        };

        let body = Json(json!({
            "error": error_message,
            "details": self.to_string(),
            "timestamp": chrono::Utc::now().to_rfc3339()
        }));

        (status, body).into_response()
    }
}

impl From<redis::RedisError> for ChargingError {
    fn from(err: redis::RedisError) -> Self {
        match err.kind() {
            redis::ErrorKind::Io => {
                ChargingError::RedisConnection(err.to_string())
            }
            _ => {
                ChargingError::RedisOperation(err.to_string())
            }
        }
    }
}

impl From<serde_json::Error> for ChargingError {
    fn from(err: serde_json::Error) -> Self {
        ChargingError::Serialization(err.to_string())
    }
}

impl From<anyhow::Error> for ChargingError {
    fn from(err: anyhow::Error) -> Self {
        ChargingError::Internal(err.to_string())
    }
}

pub type ChargingResult<T> = Result<T, ChargingError>;

pub trait ErrorContext<T> {
    fn with_context(self, context: &str) -> ChargingResult<T>;
}

impl<T, E> ErrorContext<T> for Result<T, E>
where
    E: Into<ChargingError>,
{
    fn with_context(self, context: &str) -> ChargingResult<T> {
        self.map_err(|e| {
            let charging_err = e.into();
            match charging_err {
                ChargingError::RedisOperation(ref msg) => {
                    ChargingError::RedisOperation(format!("{}: {}", context, msg))
                }
                ChargingError::AccountOperation(ref msg) => {
                    ChargingError::AccountOperation(format!("{}: {}", context, msg))
                }
                ChargingError::Internal(ref msg) => {
                    ChargingError::Internal(format!("{}: {}", context, msg))
                }
                other => other,
            }
        })
    }
}

// Validation errors
#[derive(Error, Debug)]
pub enum ValidationError {
    #[error("Invalid IP address: {0}")]
    InvalidIP(String),
    
    #[error("Invalid byte value: {0}")]
    InvalidBytes(String),
    
    #[error("Missing required field: {0}")]
    MissingField(String),
    
    #[error("Field too long: {0} (max {1})")]
    FieldTooLong(String, usize),
    
    #[error("Invalid format: {0}")]
    InvalidFormat(String),
}

impl From<ValidationError> for ChargingError {
    fn from(err: ValidationError) -> Self {
        ChargingError::InvalidRequest(err.to_string())
    }
}

// Validation helpers
pub fn validate_ip(ip: &str) -> Result<(), ValidationError> {
    if ip.is_empty() {
        return Err(ValidationError::MissingField("ip".to_string()));
    }
    
    // Basic IP validation - could use std::net::IpAddr for more thorough validation
    if ip.len() > 45 { // IPv6 max length
        return Err(ValidationError::FieldTooLong("ip".to_string(), 45));
    }
    
    // Simple validation for now - could be enhanced
    if ip.contains("..") || ip.starts_with('.') || ip.ends_with('.') {
        return Err(ValidationError::InvalidFormat(ip.to_string()));
    }
    
    Ok(())
}

pub fn validate_bytes(bytes: u64) -> Result<(), ValidationError> {
    if bytes == 0 {
        return Err(ValidationError::InvalidBytes("bytes cannot be zero".to_string()));
    }
    
    if bytes > 1_000_000_000_000 { // 1TB limit
        return Err(ValidationError::InvalidBytes("bytes value too large".to_string()));
    }
    
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_validate_ip() {
        assert!(validate_ip("192.168.1.1").is_ok());
        assert!(validate_ip("10.0.0.1").is_ok());
        
        assert!(validate_ip("").is_err());
        assert!(validate_ip("192.168..1.1").is_err());
        assert!(validate_ip(".192.168.1.1").is_err());
        assert!(validate_ip("192.168.1.1.").is_err());
    }

    #[test]
    fn test_validate_bytes() {
        assert!(validate_bytes(1024).is_ok());
        assert!(validate_bytes(1_000_000_000).is_ok());
        
        assert!(validate_bytes(0).is_err());
        assert!(validate_bytes(2_000_000_000_000).is_err());
    }

    #[test]
    fn test_error_conversion() {
        let redis_err = redis::RedisError::from((
            redis::ErrorKind::Io,
            "Connection failed",
            "Detailed error"
        ));
        let charging_err: ChargingError = redis_err.into();
        
        match charging_err {
            ChargingError::RedisConnection(_) => {}
            _ => panic!("Expected RedisConnection error"),
        }
    }
}
