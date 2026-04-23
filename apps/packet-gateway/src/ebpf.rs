use aya::{programs::Xdp, Bpf, maps::HashMap};
use aya_log::BpfLogger;
use anyhow::{Context, Result};
use std::net::Ipv4Addr;
use tokio::time::{interval, Duration};
use tracing::{info, warn, error, debug};
use std::collections::HashMap as StdHashMap;
use redis::AsyncCommands;

pub struct EbpfManager {
    bpf: Bpf,
    interface: String,
}

#[derive(Debug, Clone)]
pub struct PacketStats {
    pub ip: u32,
    pub bytes: u64,
}

#[derive(Debug, Clone, Copy)]
pub struct SyncEntry {
    pub ip: u32,
    pub bytes: u64,
    pub credit: i64,
    pub blocked: u8,
    pub valid: u8,
}

#[derive(Debug, Clone)]
pub struct CreditInfo {
    pub ip: u32,
    pub credit: i64,
}

impl EbpfManager {
    pub async fn new(interface: String) -> Result<Self> {
        // Load the compiled eBPF program
        let mut bpf = Bpf::load(include_bytes_aligned!("packet_filter"))?;
        
        // Initialize eBPF logging
        if let Err(e) = BpfLogger::init(&mut bpf) {
            warn!("Failed to initialize eBPF logger: {}", e);
        }
        
        info!("Loaded eBPF program successfully");
        
        Ok(Self { bpf, interface })
    }
    
    pub fn attach(&mut self) -> Result<()> {
        // Get the XDP program
        let program: &mut Xdp = self.bpf.program_mut("packet_filter")
            .ok_or_else(|| anyhow::anyhow!("Program 'packet_filter' not found"))?
            .try_into()?;
        
        // Load the program
        program.load()?;
        info!("Loaded XDP program into kernel");
        
        // Attach to the specified interface
        program.attach(&self.interface, aya::programs::XdpFlags::default())
            .context("Failed to attach XDP program")?;
        
        info!("Attached XDP program to interface: {}", self.interface);
        Ok(())
    }
    
    pub fn get_packet_stats(&self) -> Result<Vec<PacketStats>> {
        let packet_stats_map: &HashMap<_, u32, u64> = self.bpf.map("packet_stats")
            .ok_or_else(|| anyhow::anyhow!("packet_stats map not found"))?
            .try_into()?;
        
        let mut stats = Vec::new();
        
        // Iterate through all entries in the packet_stats map
        for (ip, bytes) in packet_stats_map.iter()? {
            stats.push(PacketStats { ip: *ip, bytes: *bytes });
        }
        
        Ok(stats)
    }
    
    pub fn get_credit_info(&self) -> Result<Vec<CreditInfo>> {
        let user_credits_map: &HashMap<_, u32, i64> = self.bpf.map("user_credits")
            .ok_or_else(|| anyhow::anyhow!("user_credits map not found"))?
            .try_into()?;
        
        let mut credits = Vec::new();
        
        // Iterate through all entries in the user_credits map
        for (ip, credit) in user_credits_map.iter()? {
            credits.push(CreditInfo { ip: *ip, credit: *credit });
        }
        
        Ok(credits)
    }
    
    pub fn update_user_credit(&self, ip: u32, credit: i64) -> Result<()> {
        let user_credits_map: &HashMap<_, u32, i64> = self.bpf.map("user_credits")
            .ok_or_else(|| anyhow::anyhow!("user_credits map not found"))?
            .try_into()?;
        
        user_credits_map.insert(&ip, &credit, 0)?;
        debug!("Updated credit for IP {} to {}", ip, credit);
        
        Ok(())
    }
    
    pub fn block_user(&self, ip: u32, blocked: bool) -> Result<()> {
        let block_list_map: &HashMap<_, u32, u8> = self.bpf.map("block_list")
            .ok_or_else(|| anyhow::anyhow!("block_list map not found"))?
            .try_into()?;
        
        let block_flag: u8 = if blocked { 1 } else { 0 };
        block_list_map.insert(&ip, &block_flag, 0)?;
        
        info!("{} IP {} in eBPF block list", if blocked { "Blocked" } else { "Unblocked" }, ip);
        Ok(())
    }
    
    pub async fn sync_to_redis(&self, redis_conn: &mut redis::aio::MultiplexedConnection) -> Result<()> {
        // Sync packet stats
        let stats = self.get_packet_stats()?;
        for stat in stats {
            let ip_str = self.u32_to_ipv4_string(stat.ip);
            let key = format!("packet_stats:{}", ip_str);
            let _: () = redis_conn.set(&key, stat.bytes).await
                .context("Failed to sync packet stats to Redis")?;
        }
        
        // Sync credit info
        let credits = self.get_credit_info()?;
        for credit in credits {
            let ip_str = self.u32_to_ipv4_string(credit.ip);
            let key = format!("user_credit:{}", ip_str);
            let _: () = redis_conn.set(&key, credit.credit).await
                .context("Failed to sync credit info to Redis")?;
        }
        
        debug!("Synced eBPF maps to Redis");
        Ok(())
    }
    
    pub async fn sync_from_redis(&self, redis_conn: &mut redis::aio::MultiplexedConnection) -> Result<()> {
        // Get all credit keys from Redis
        let keys: Vec<String> = redis_conn.keys("user_credit:*").await
            .context("Failed to get credit keys from Redis")?;
        
        for key in keys {
            if let Some(ip_part) = key.strip_prefix("user_credit:") {
                let credit: i64 = redis_conn.get(&key).await
                    .context("Failed to get credit from Redis")?;
                
                if let Ok(ip) = self.ipv4_string_to_u32(ip_part) {
                    self.update_user_credit(ip, credit)?;
                    debug!("Synced credit from Redis for IP {}: {}", ip_part, credit);
                } else {
                    warn!("Invalid IP format in Redis key: {}", key);
                }
            }
        }
        
        // Get blocked users from Redis
        let blocked_keys: Vec<String> = redis_conn.keys("blocked_user:*").await
            .context("Failed to get blocked user keys from Redis")?;
        
        for key in blocked_keys {
            if let Some(ip_part) = key.strip_prefix("blocked_user:") {
                if let Ok(ip) = self.ipv4_string_to_u32(ip_part) {
                    self.block_user(ip, true)?;
                    debug!("Synced block status from Redis for IP: {}", ip_part);
                } else {
                    warn!("Invalid IP format in Redis block key: {}", key);
                }
            }
        }
        
        debug!("Synced data from Redis to eBPF maps");
        Ok(())
    }
    
    fn u32_to_ipv4_string(&self, ip: u32) -> String {
        let ip_addr = Ipv4Addr::from(ip);
        ip_addr.to_string()
    }
    
    fn ipv4_string_to_u32(&self, ip_str: &str) -> Result<u32> {
        let ip_addr: Ipv4Addr = ip_str.parse()
            .context("Invalid IP address format")?;
        Ok(u32::from(ip_addr))
    }
    
    pub fn trigger_sync(&self) -> Result<()> {
        let sync_control_map: &HashMap<_, u32, u64> = self.bpf.map("sync_control")
            .ok_or_else(|| anyhow::anyhow!("sync_control map not found"))?
            .try_into()?;
        
        // Set sync flag to trigger eBPF sync function
        let key = 0u32;
        let sync_flag = 1u64;
        sync_control_map.insert(&key, &sync_flag, 0)?;
        
        debug!("Triggered eBPF sync function");
        Ok(())
    }
    
    pub fn read_sync_buffer(&self) -> Result<Vec<SyncEntry>> {
        let sync_buffer_map: &HashMap<_, u32, SyncEntry> = self.bpf.map("sync_buffer")
            .ok_or_else(|| anyhow::anyhow!("sync_buffer map not found"))?
            .try_into()?;
        
        let mut entries = Vec::new();
        
        // Read from sync buffer (index 0 for now, could expand for batching)
        let key = 0u32;
        if let Some(entry) = sync_buffer_map.get(&key, 0)? {
            if entry.valid == 1 {
                entries.push(*entry);
            }
        }
        
        Ok(entries)
    }
    
    pub fn sync_batch_to_redis(&self, redis_conn: &mut redis::aio::MultiplexedConnection) -> Result<()> {
        // Trigger eBPF sync function
        self.trigger_sync()?;
        
        // Give eBPF program a moment to process
        std::thread::sleep(std::time::Duration::from_millis(10));
        
        // Read batched data from sync buffer
        let entries = self.read_sync_buffer()?;
        
        // Sync each entry to Redis
        for entry in entries {
            let ip_str = self.u32_to_ipv4_string(entry.ip);
            
            // Sync packet stats
            let stats_key = format!("packet_stats:{}", ip_str);
            let _: () = redis_conn.set(&stats_key, entry.bytes).await
                .context("Failed to sync packet stats to Redis")?;
            
            // Sync credit info
            let credit_key = format!("user_credit:{}", ip_str);
            let _: () = redis_conn.set(&credit_key, entry.credit).await
                .context("Failed to sync credit info to Redis")?;
            
            // Sync block status
            if entry.blocked == 1 {
                let block_key = format!("blocked_user:{}", ip_str);
                let _: () = redis_conn.set(&block_key, 1u8).await
                    .context("Failed to sync block status to Redis")?;
            }
            
            debug!("Batch synced IP {}: {} bytes, credit {}, blocked {}", 
                   ip_str, entry.bytes, entry.credit, entry.blocked);
        }
        
        info!("Batch synced {} entries to Redis", entries.len());
        Ok(())
    }
    
    pub fn cleanup_maps(&self) -> Result<()> {
        // Clear packet stats map
        if let Ok(packet_stats_map) = self.bpf.map("packet_stats").and_then(|m| m.try_into()) {
            let map: &HashMap<_, u32, u64> = packet_stats_map;
            let keys: Vec<u32> = map.iter()?.map(|(k, _)| *k).collect();
            for key in keys {
                let _: Result<(), _> = map.delete(&key);
            }
        }
        
        // Clear sync control
        if let Ok(sync_control_map) = self.bpf.map("sync_control").and_then(|m| m.try_into()) {
            let map: &HashMap<_, u32, u64> = sync_control_map;
            let key = 0u32;
            let reset_flag = 0u64;
            let _: Result<(), _> = map.insert(&key, &reset_flag, 0);
        }
        
        info!("Cleared eBPF maps");
        Ok(())
    }
}

// Helper function to include compiled eBPF bytecode
fn include_bytes_aligned(s: &str) -> &'static [u8] {
    // Include the compiled eBPF bytecode
    // The actual path will be generated by the build process
    aya::include_bytes_aligned!(s)
}
