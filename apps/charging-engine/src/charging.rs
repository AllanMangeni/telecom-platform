use chrono::{DateTime, Utc};
use redis::AsyncCommands;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::time::Duration;
use tokio::time::{interval, sleep};
use tracing::{info, warn, error, debug};

use crate::errors::{ChargingError, ChargingResult, ErrorContext};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SubscriberAccount {
    pub imsi: String,
    pub balance: i64,        // in smallest currency unit (e.g., cents)
    pub data_limit: u64,    // bytes
    pub data_used: u64,     // bytes
    pub voice_limit: u64,   // seconds
    pub voice_used: u64,    // seconds
    pub sms_limit: u64,     // count
    pub sms_used: u64,      // count
    pub status: AccountStatus,
    pub last_updated: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum AccountStatus {
    Active,
    Suspended,
    Terminated,
    Blocked,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UsageEvent {
    pub imsi: String,
    pub session_id: String,
    pub usage_type: UsageType,
    pub volume: u64,        // bytes, seconds, or count
    pub timestamp: DateTime<Utc>,
    pub rate: f64,          // cost per unit
    pub cost: f64,          // total cost
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum UsageType {
    Data,
    Voice,
    SMS,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RatingPlan {
    pub plan_id: String,
    pub name: String,
    pub data_rate: f64,     // cost per MB
    pub voice_rate: f64,    // cost per minute
    pub sms_rate: f64,      // cost per SMS
    pub monthly_fee: f64,
    pub data_limit: u64,    // bytes
    pub voice_limit: u64,   // seconds
    pub sms_limit: u64,     // count
}

pub struct ChargingEngine {
    redis_client: redis::Client,
    rating_plans: HashMap<String, RatingPlan>,
    sync_interval: Duration,
}

impl ChargingEngine {
    pub fn new(redis_url: &str, sync_interval_secs: u64) -> ChargingResult<Self> {
        let redis_client = redis::Client::open(redis_url)
            .context("Failed to create Redis client")?;

        let mut rating_plans = HashMap::new();
        
        // Default rating plans
        rating_plans.insert("basic".to_string(), RatingPlan {
            plan_id: "basic".to_string(),
            name: "Basic Plan".to_string(),
            data_rate: 0.01,      // $0.01 per MB
            voice_rate: 0.05,     // $0.05 per minute
            sms_rate: 0.10,       // $0.10 per SMS
            monthly_fee: 10.0,    // $10 monthly
            data_limit: 1_000_000_000,    // 1GB
            voice_limit: 300,             // 300 minutes
            sms_limit: 100,                // 100 SMS
        });

        rating_plans.insert("premium".to_string(), RatingPlan {
            plan_id: "premium".to_string(),
            name: "Premium Plan".to_string(),
            data_rate: 0.005,     // $0.005 per MB
            voice_rate: 0.02,     // $0.02 per minute
            sms_rate: 0.05,       // $0.05 per SMS
            monthly_fee: 25.0,    // $25 monthly
            data_limit: 5_000_000_000,    // 5GB
            voice_limit: 1000,            // 1000 minutes
            sms_limit: 500,                // 500 SMS
        });

        Ok(Self {
            redis_client,
            rating_plans,
            sync_interval: Duration::from_secs(sync_interval_secs),
        })
    }

    pub async fn start(&self) -> ChargingResult<()> {
        info!("Starting charging engine");
        
        let mut conn = self.redis_client.get_multiplexed_async_connection().await
            .context("Failed to get Redis connection")?;

        // Test connection
        let _: () = redis::cmd("PING").query_async(&mut conn).await?;
        info!("Connected to Redis successfully");

        // Start sync loop
        let mut ticker = interval(self.sync_interval);
        
        loop {
            ticker.tick().await;
            
            if let Err(e) = self.sync_usage_data(&mut conn).await {
                error!("Failed to sync usage data: {}", e);
            }
            
            if let Err(e) = self.check_credit_limits(&mut conn).await {
                error!("Failed to check credit limits: {}", e);
            }
        }
    }

    async fn sync_usage_data(&self, conn: &mut redis::aio::MultiplexedConnection) -> Result<()> {
        debug!("Syncing usage data from packet gateway");
        
        // Get all usage keys from packet gateway
        let pattern = "usage:*";
        let keys: Vec<String> = redis::cmd("KEYS")
            .arg(pattern)
            .query_async(conn)
            .await
            .unwrap_or_default();

        for key in keys {
            if let Some(ip) = key.strip_prefix("usage:") {
                // Get IMSI mapping from IP -> IMSI
                let imsi_key = format!("ip2imsi:{}", ip);
                if let Ok(imsi): Result<String, _> = redis::cmd("GET")
                    .arg(&imsi_key)
                    .query_async(conn)
                    .await 
                {
                    // Get usage data
                    let usage: u64 = redis::cmd("GET")
                        .arg(&key)
                        .query_async(conn)
                        .await
                        .unwrap_or(0);

                    if usage > 0 {
                        // Update subscriber usage
                        self.update_subscriber_usage(conn, &imsi, usage).await?;
                        
                        // Clear processed usage
                        let _: () = redis::cmd("DEL").arg(&key).query_async(conn).await?;
                    }
                }
            }
        }

        Ok(())
    }

    async fn update_subscriber_usage(
        &self, 
        conn: &mut redis::aio::MultiplexedConnection,
        imsi: &str,
        data_bytes: u64
    ) -> Result<()> {
        let account_key = format!("account:{}", imsi);
        
        // Get current account
        let account_data: Option<String> = redis::cmd("GET")
            .arg(&account_key)
            .query_async(conn)
            .await?;

        if let Some(data) = account_data {
            let mut account: SubscriberAccount = serde_json::from_str(&data)
                .context("Failed to deserialize account")?;

            // Update data usage
            account.data_used += data_bytes;
            account.last_updated = Utc::now();

            // Calculate cost
            if let Some(plan) = self.rating_plans.get(&account.imsi) {
                let mb_used = data_bytes as f64 / (1024.0 * 1024.0);
                let cost = mb_used * plan.data_rate;
                
                // Deduct from balance
                account.balance -= (cost * 100.0) as i64; // Convert to cents
                
                // Create usage event
                let usage_event = UsageEvent {
                    imsi: imsi.to_string(),
                    session_id: "default".to_string(), // TODO: Get actual session
                    usage_type: UsageType::Data,
                    volume: data_bytes,
                    timestamp: Utc::now(),
                    rate: plan.data_rate,
                    cost,
                };

                // Store usage event
                let event_key = format!("events:{}:{}", imsi, Utc::now().timestamp());
                let event_json = serde_json::to_string(&usage_event)?;
                let _: () = redis::cmd("SET")
                    .arg(&event_key)
                    .arg(&event_json)
                    .arg("EX")
                    .arg(86400) // Expire in 24 hours
                    .query_async(conn)
                    .await?;

                // Check if over limit
                if account.data_used > account.data_limit {
                    warn!("Subscriber {} exceeded data limit: {} > {}", 
                          imsi, account.data_used, account.data_limit);
                    
                    // Block subscriber if significantly over limit
                    if account.data_used > account.data_limit * 110 / 100 {
                        account.status = AccountStatus::Blocked;
                        
                        // Update block list in packet gateway
                        let ip_key = format!("imsi2ip:{}", imsi);
                        if let Ok(ip): Result<String, _> = redis::cmd("GET")
                            .arg(&ip_key)
                            .query_async(conn)
                            .await 
                        {
                            let block_key = format!("block:{}", ip);
                            let _: () = redis::cmd("SET")
                                .arg(&block_key)
                                .arg(1)
                                .query_async(conn)
                                .await?;
                        }
                    }
                }

                // Save updated account
                let account_json = serde_json::to_string(&account)?;
                let _: () = redis::cmd("SET")
                    .arg(&account_key)
                    .arg(&account_json)
                    .query_async(conn)
                    .await?;

                info!("Updated usage for {}: {} bytes, cost: ${:.4}, balance: ${:.2}", 
                      imsi, data_bytes, cost, account.balance as f64 / 100.0);
            }
        }

        Ok(())
    }

    async fn check_credit_limits(&self, conn: &mut redis::aio::MultiplexedConnection) -> Result<()> {
        debug!("Checking credit limits");
        
        // Get all account keys
        let pattern = "account:*";
        let keys: Vec<String> = redis::cmd("KEYS")
            .arg(pattern)
            .query_async(conn)
            .await
            .unwrap_or_default();

        for key in keys {
            if let Some(imsi) = key.strip_prefix("account:") {
                let account_data: Option<String> = redis::cmd("GET")
                    .arg(&key)
                    .query_async(conn)
                    .await?;

                if let Some(data) = account_data {
                    let account: SubscriberAccount = serde_json::from_str(&data)
                        .context("Failed to deserialize account")?;

                    // Check if balance is negative
                    if account.balance < 0 {
                        warn!("Subscriber {} has negative balance: ${:.2}", 
                              imsi, account.balance as f64 / 100.0);

                        // Block if significantly negative
                        if account.balance < -1000 { // -$10
                            account.status = AccountStatus::Blocked;
                            
                            // Update block list
                            let ip_key = format!("imsi2ip:{}", imsi);
                            if let Ok(ip): Result<String, _> = redis::cmd("GET")
                                .arg(&ip_key)
                                .query_async(conn)
                                .await 
                            {
                                let block_key = format!("block:{}", ip);
                                let _: () = redis::cmd("SET")
                                    .arg(&block_key)
                                    .arg(1)
                                    .query_async(conn)
                                    .await?;
                            }

                            // Save updated account
                            let account_json = serde_json::to_string(&account)?;
                            let _: () = redis::cmd("SET")
                                .arg(&key)
                                .arg(&account_json)
                                .query_async(conn)
                                .await?;
                        }
                    }
                }
            }
        }

        Ok(())
    }

    pub async fn create_account(&self, imsi: &str, plan_id: &str, initial_balance: i64) -> Result<()> {
        let mut conn = self.redis_client.get_multiplexed_async_connection().await?;
        
        let plan = self.rating_plans.get(plan_id)
            .ok_or_else(|| anyhow::anyhow!("Rating plan '{}' not found", plan_id))?;

        let account = SubscriberAccount {
            imsi: imsi.to_string(),
            balance: initial_balance,
            data_limit: plan.data_limit,
            data_used: 0,
            voice_limit: plan.voice_limit,
            voice_used: 0,
            sms_limit: plan.sms_limit,
            sms_used: 0,
            status: AccountStatus::Active,
            last_updated: Utc::now(),
        };

        let account_key = format!("account:{}", imsi);
        let account_json = serde_json::to_string(&account)?;
        
        let _: () = redis::cmd("SET")
            .arg(&account_key)
            .arg(&account_json)
            .query_async(&mut conn)
            .await?;

        info!("Created account for {} with plan {} and balance ${:.2}", 
              imsi, plan_id, initial_balance as f64 / 100.0);

        Ok(())
    }

    pub async fn get_account(&self, imsi: &str) -> Result<Option<SubscriberAccount>> {
        let mut conn = self.redis_client.get_multiplexed_async_connection().await?;
        
        let account_key = format!("account:{}", imsi);
        let account_data: Option<String> = redis::cmd("GET")
            .arg(&account_key)
            .query_async(&mut conn)
            .await?;

        if let Some(data) = account_data {
            let account: SubscriberAccount = serde_json::from_str(&data)
                .context("Failed to deserialize account")?;
            Ok(Some(account))
        } else {
            Ok(None)
        }
    }

    pub async fn top_up_balance(&self, imsi: &str, amount: i64) -> Result<()> {
        let mut conn = self.redis_client.get_multiplexed_async_connection().await?;
        
        let account_key = format!("account:{}", imsi);
        let account_data: Option<String> = redis::cmd("GET")
            .arg(&account_key)
            .query_async(&mut conn)
            .await?;

        if let Some(data) = account_data {
            let mut account: SubscriberAccount = serde_json::from_str(&data)
                .context("Failed to deserialize account")?;

            account.balance += amount;
            account.last_updated = Utc::now();

            // Unblock if balance is restored and status was blocked
            if account.status == AccountStatus::Blocked && account.balance > 0 {
                account.status = AccountStatus::Active;
                
                // Remove from block list
                let ip_key = format!("imsi2ip:{}", imsi);
                if let Ok(ip): Result<String, _> = redis::cmd("GET")
                    .arg(&ip_key)
                    .query_async(&mut conn)
                    .await 
                {
                    let block_key = format!("block:{}", ip);
                    let _: () = redis::cmd("DEL").arg(&block_key).query_async(&mut conn).await?;
                }
            }

            let account_json = serde_json::to_string(&account)?;
            let _: () = redis::cmd("SET")
                .arg(&account_key)
                .arg(&account_json)
                .query_async(&mut conn)
                .await?;

            info!("Topped up balance for {} by ${:.2}, new balance: ${:.2}", 
                  imsi, amount as f64 / 100.0, account.balance as f64 / 100.0);
        } else {
            return Err(anyhow::anyhow!("Account not found for IMSI: {}", imsi));
        }

        Ok(())
    }

    // API methods for HTTP endpoints

    pub async fn test_connection(&self) -> Result<()> {
        let mut conn = self.redis_client.get_multiplexed_async_connection().await
            .context("Failed to get Redis connection")?;
        
        let _: () = redis::cmd("PING").query_async(&mut conn).await?;
        Ok(())
    }

    pub async fn check_credit(&self, ip: &str, bytes_requested: u64) -> Result<bool> {
        let mut conn = self.redis_client.get_multiplexed_async_connection().await
            .context("Failed to get Redis connection")?;

        let key = format!("credit:{}", ip);
        let credit: i64 = conn.get(&key).await.unwrap_or(0);

        Ok(credit >= bytes_requested as i64)
    }

    pub async fn get_balance(&self, ip: &str) -> Result<i64> {
        let mut conn = self.redis_client.get_multiplexed_async_connection().await
            .context("Failed to get Redis connection")?;

        let key = format!("credit:{}", ip);
        let balance: i64 = conn.get(&key).await.unwrap_or(0);
        Ok(balance)
    }

    pub async fn deduct_credit(&self, ip: &str, bytes_used: u64) -> Result<i64> {
        let mut conn = self.redis_client.get_multiplexed_async_connection().await
            .context("Failed to get Redis connection")?;

        let key = format!("credit:{}", ip);
        let new_balance: i64 = conn.decr(&key, bytes_used).await.unwrap_or(0);
        Ok(new_balance)
    }

    pub async fn add_credit(&self, ip: &str, bytes_to_add: u64) -> Result<i64> {
        let mut conn = self.redis_client.get_multiplexed_async_connection().await
            .context("Failed to get Redis connection")?;

        let key = format!("credit:{}", ip);
        let new_balance: i64 = conn.incr(&key, bytes_to_add).await.unwrap_or(0);
        Ok(new_balance)
    }
}
