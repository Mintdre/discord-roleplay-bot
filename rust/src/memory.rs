use chrono;
use lazy_static::lazy_static;
use log::{debug, error, info, log, warn, Level, LevelFilter};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::env;
use std::fs::{self, File, OpenOptions};
use std::io::{BufReader, BufWriter, ErrorKind};
use std::path::{Path, PathBuf};
use std::sync::Mutex;

const MEMORY_DIR: &str = "data";
const MAX_HISTORY_LEN: usize = 20;

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Message {
    pub role: String,
    pub content: String,
}

#[derive(Serialize, Deserialize, Debug, Clone, Default)]
pub struct ChatMemory {
    pub messages: Vec<Message>,
}

lazy_static! {
    static ref USER_MEMORIES: Mutex<HashMap<String, ChatMemory>> = Mutex::new(HashMap::new());
    static ref SERVER_MEMORIES: Mutex<HashMap<String, ChatMemory>> = Mutex::new(HashMap::new());
}

fn get_memory_file_path(id: &str, memory_type: &str) -> PathBuf {
    let dir = PathBuf::from(MEMORY_DIR);
    if !dir.exists() {
        fs::create_dir_all(&dir).expect("Failed to create memory directory");
    }
    dir.join(format!("{}_{}_memory.json", id, memory_type))
}

fn load_memory_from_file(path: &Path) -> Result<ChatMemory, std::io::Error> {
    if !path.exists() {
        return Ok(ChatMemory::default());
    }
    let file = File::open(path)?;
    let reader = BufReader::new(file);
    let memory: ChatMemory = serde_json::from_reader(reader)
        .map_err(|e| std::io::Error::new(ErrorKind::InvalidData, e))?;
    Ok(memory)
}

fn save_memory_to_file(path: &Path, memory: &ChatMemory) -> Result<(), std::io::Error> {
    let file = OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .open(path)?;
    let writer = BufWriter::new(file);
    serde_json::to_writer_pretty(writer, memory)
        .map_err(|e| std::io::Error::new(ErrorKind::Other, e))?;
    Ok(())
}

pub fn get_memory(id: &str, memory_type: &str) -> ChatMemory {
    let mut memories_map = match memory_type {
        "user" => USER_MEMORIES.lock().unwrap(),
        "server" => SERVER_MEMORIES.lock().unwrap(),
        _ => panic!("Invalid memory type"),
    };
    if let Some(memory) = memories_map.get(id) {
        info!("Loaded memory for {} {} from cache", memory_type, id);
        return memory.clone();
    }
    let path = get_memory_file_path(id, memory_type);
    match load_memory_from_file(&path) {
        Ok(mut memory) => {
            if memory.messages.len() > MAX_HISTORY_LEN {
                let start_index = memory.messages.len() - MAX_HISTORY_LEN;
                memory.messages = memory.messages.split_off(start_index);
                info!(
                    "Truncated loaded memory for {} {} to {} messages",
                    memory_type, id, MAX_HISTORY_LEN
                );
            }
            info!(
                "Loaded memory for {} {} from file: {:?}",
                memory_type, id, path
            );
            memories_map.insert(id.to_string(), memory.clone());
            memory
        }
        Err(e) => {
            warn!(
                "Failed to load memory for {} {}: {}, creating new.",
                memory_type, id, e
            );
            let new_memory = ChatMemory::default();
            memories_map.insert(id.to_string(), new_memory.clone());
            new_memory
        }
    }
}

pub fn update_memory(id: &str, memory_type: &str, user_prompt: &str, assistant_response: &str) {
    let mut memories_map_guard = match memory_type {
        "user" => USER_MEMORIES.lock().unwrap(),
        "server" => SERVER_MEMORIES.lock().unwrap(),
        _ => panic!("Invalid memory type for update"),
    };
    let memory_entry = memories_map_guard.entry(id.to_string()).or_insert_with(|| {
        warn!(
            "Memory not found in cache for update of {} {}, attempting to load or create new.",
            memory_type, id
        );
        let path = get_memory_file_path(id, memory_type);
        load_memory_from_file(&path).unwrap_or_else(|e| {
            warn!(
                "Failed to load memory from file for update of {} {}: {}, creating new.",
                memory_type, id, e
            );
            ChatMemory::default()
        })
    });
    memory_entry.messages.push(Message {
        role: "user".to_string(),
        content: user_prompt.to_string(),
    });
    memory_entry.messages.push(Message {
        role: "assistant".to_string(),
        content: assistant_response.to_string(),
    });
    if memory_entry.messages.len() > MAX_HISTORY_LEN {
        let start_index = memory_entry.messages.len() - MAX_HISTORY_LEN;
        memory_entry.messages = memory_entry.messages.split_off(start_index);
        info!(
            "Truncated memory for {} {} during update to {} messages",
            memory_type, id, MAX_HISTORY_LEN
        );
    }
    let path = get_memory_file_path(id, memory_type);
    if let Err(e) = save_memory_to_file(&path, &memory_entry) {
        error!("Failed to save memory for {} {}: {}", memory_type, id, e);
    } else {
        info!(
            "Saved memory for {} {} to file: {:?}",
            memory_type, id, path
        );
    }
}

pub fn setup_rust_logging() {
    // --- Read log level from environment variable ---
    let log_level_str = env::var("ELYBOT_LOG_LEVEL").unwrap_or_else(|_| "info".to_string());
    let (log_level_filter, log_level_name) = match log_level_str.to_lowercase().as_str() {
        "debug" | "verbose" | "trace" => (LevelFilter::Debug, "DEBUG"),
        _ => (LevelFilter::Info, "INFO"), // Default to Info
    };

    fern::Dispatch::new()
        .format(|out, message, record| {
            out.finish(format_args!(
                "{}[{}][{}] {}",
                chrono::Local::now().format("[%Y-%m-%d][%H:%M:%S]"),
                record.target(),
                record.level(),
                message
            ))
        })
        .level(log_level_filter)
        .chain(std::io::stdout())
        .apply()
        .expect("Failed to initialize logger");

    log!(target: "ely_rust::memory", log_level_filter.to_level().unwrap_or(Level::Info), "Rust logging configured to {} level.", log_level_name);
}
