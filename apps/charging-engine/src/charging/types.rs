use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use redis::{FromRedisValue, ToRedisArgs, ToSingleRedisArg};

/// Charging rule for usage authorization
///
/// # Example
///
/// ```rust
/// use charging_engine::charging::types::ChargingRule;
///
/// let rule = ChargingRule::Allowed;
/// assert_eq!(rule.as_str(), "ALLOWED");
/// ```
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ChargingRule {
    Allowed,
    InsufficientCredit,
    DataLimitExceeded,
    VoiceLimitExceeded,
    SmsLimitExceeded,
    UserBlocked,
    Blocked,
}

impl ChargingRule {
    /// Convert charging rule to string representation
    ///
    /// # Example
    ///
    /// ```rust
    /// use charging_engine::charging::types::ChargingRule;
    ///
    /// let rule = ChargingRule::Allowed;
    /// assert_eq!(rule.as_str(), "ALLOWED");
    /// ```
    pub fn as_str(&self) -> &str {
        match self {
            ChargingRule::Allowed => "ALLOWED",
            ChargingRule::InsufficientCredit => "INSUFFICIENT_CREDIT",
            ChargingRule::DataLimitExceeded => "DATA_LIMIT_EXCEEDED",
            ChargingRule::VoiceLimitExceeded => "VOICE_LIMIT_EXCEEDED",
            ChargingRule::SmsLimitExceeded => "SMS_LIMIT_EXCEEDED",
            ChargingRule::UserBlocked => "USER_BLOCKED",
            ChargingRule::Blocked => "BLOCKED",
        }
    }
}

/// Subscriber account information
///
/// # Example
///
/// ```rust
/// use charging_engine::charging::types::{SubscriberAccount, AccountStatus};
/// use chrono::Utc;
///
/// let account = SubscriberAccount {
///     imsi: "1234567890".to_string(),
///     balance: 1000,
///     data_limit: 1000000000,
///     data_used: 0,
///     voice_limit: 1000,
///     voice_used: 0,
///     sms_limit: 100,
///     sms_used: 0,
///     status: AccountStatus::Active,
///     last_updated: Utc::now(),
/// };
/// ```
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SubscriberAccount {
    pub imsi: String,
    pub balance: i64,
    pub data_limit: u64,
    pub data_used: u64,
    pub voice_limit: u64,
    pub voice_used: u64,
    pub sms_limit: u64,
    pub sms_used: u64,
    pub status: AccountStatus,
    pub last_updated: DateTime<Utc>,
}

/// Account status
///
/// # Example
///
/// ```rust
/// use charging_engine::charging::types::AccountStatus;
///
/// let status = AccountStatus::Active;
/// ```
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum AccountStatus {
    Active,
    Suspended,
    Terminated,
    Blocked,
}

/// Usage event for charging
///
/// # Example
///
/// ```rust
/// use charging_engine::charging::types::{UsageEvent, UsageType};
/// use chrono::Utc;
///
/// let event = UsageEvent {
///     imsi: "1234567890".to_string(),
///     session_id: "session123".to_string(),
///     usage_type: UsageType::Data,
///     volume: 1000,
///     timestamp: Utc::now(),
///     rate: 0.01,
///     cost: 10.0,
/// };
/// ```
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UsageEvent {
    pub imsi: String,
    pub session_id: String,
    pub usage_type: UsageType,
    pub volume: u64,
    pub timestamp: DateTime<Utc>,
    pub rate: f64,
    pub cost: f64,
}

/// Usage type classification
///
/// # Example
///
/// ```rust
/// use charging_engine::charging::types::UsageType;
///
/// let usage_type = UsageType::Data;
/// ```
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum UsageType {
    Data,
    Voice,
    SMS,
}

/// Rating plan for subscriber billing
///
/// # Example
///
/// ```rust
/// use charging_engine::charging::types::RatingPlan;
///
/// let plan = RatingPlan {
///     plan_id: "plan1".to_string(),
///     name: "Basic Plan".to_string(),
///     data_rate: 0.01,
///     voice_rate: 0.05,
///     sms_rate: 0.1,
///     monthly_fee: 10.0,
///     data_limit: 1000000000,
///     voice_limit: 1000,
///     sms_limit: 100,
/// };
/// ```
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RatingPlan {
    pub plan_id: String,
    pub name: String,
    pub data_rate: f64,
    pub voice_rate: f64,
    pub sms_rate: f64,
    pub monthly_fee: f64,
    pub data_limit: u64,
    pub voice_limit: u64,
    pub sms_limit: u64,
}

// Redis trait implementations for serialization
impl FromRedisValue for SubscriberAccount {
    fn from_redis_value(v: redis::Value) -> Result<Self, redis::ParsingError> {
        let json: String = redis::from_redis_value(v)?;
        let account: SubscriberAccount = serde_json::from_str(&json)
            .map_err(|e| redis::ParsingError::from(e.to_string()))?;
        Ok(account)
    }
}

impl ToRedisArgs for SubscriberAccount {
    fn write_redis_args<W>(&self, out: &mut W)
    where
        W: redis::RedisWrite + ?Sized,
    {
        let json = serde_json::to_string(self)
            .unwrap_or_else(|_| {
                // Fallback to empty JSON if serialization fails
                r#"{"imsi":"","balance":0,"data_limit":0,"data_used":0,"voice_limit":0,"voice_used":0,"sms_limit":0,"sms_used":0,"status":"Active","last_updated":"1970-01-01T00:00:00Z"}"#.to_string()
            });
        json.write_redis_args(out)
    }
}

impl ToSingleRedisArg for SubscriberAccount {}

impl FromRedisValue for UsageEvent {
    fn from_redis_value(v: redis::Value) -> Result<Self, redis::ParsingError> {
        let json: String = redis::from_redis_value(v)?;
        let event: UsageEvent = serde_json::from_str(&json)
            .map_err(|e| redis::ParsingError::from(e.to_string()))?;
        Ok(event)
    }
}

impl ToRedisArgs for UsageEvent {
    fn write_redis_args<W>(&self, out: &mut W)
    where
        W: redis::RedisWrite + ?Sized,
    {
        let json = serde_json::to_string(self)
            .unwrap_or_else(|_| {
                // Fallback to empty JSON if serialization fails
                r#"{"imsi":"","session_id":"","usage_type":"Data","volume":0,"timestamp":"1970-01-01T00:00:00Z","rate":0.0,"cost":0.0}"#.to_string()
            });
        json.write_redis_args(out)
    }
}

impl ToSingleRedisArg for UsageEvent {}
