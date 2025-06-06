use std::env;
use std::ffi::{CStr, CString};
use std::os::raw::c_char;
use std::panic;
use std::sync::Once;

mod gemini_client;
mod memory;

use gemini_client::call_generative_api;
use log::{error, info};
use memory::{get_memory, setup_rust_logging, update_memory, Message};

static INIT_LOGGING: Once = Once::new();

fn init_logging_once() {
    INIT_LOGGING.call_once(|| {
        setup_rust_logging();
    });
}

#[no_mangle]
pub extern "C" fn process_command(
    command_type_ptr: *const c_char,
    prompt_ptr: *const c_char,
    id_ptr: *const c_char,
    api_key_ptr: *const c_char,
) -> *mut c_char {
    init_logging_once();

    let result = panic::catch_unwind(|| {
        let command_type = unsafe {
            CStr::from_ptr(command_type_ptr)
                .to_str()
                .unwrap_or("error_type")
        };
        let prompt = unsafe {
            CStr::from_ptr(prompt_ptr)
                .to_str()
                .unwrap_or("error_prompt")
        };
        let id = unsafe { CStr::from_ptr(id_ptr).to_str().unwrap_or("error_id") };
        let api_key = unsafe { CStr::from_ptr(api_key_ptr).to_str().unwrap_or("") };

        if api_key.is_empty() {
            error!("Google API key is missing!");
            return CString::new("Error: Google API key is missing.")
                .unwrap()
                .into_raw();
        }

        info!(
            "Rust: Processing command: Type={}, ID={}, Prompt='{}'",
            command_type, id, prompt
        );

        let historical_memory = get_memory(id, command_type);

        let system_prompt_content = env::var("SYSTEM_PROMPT")
            .unwrap_or_else(|_| "You are Elysia from the game Honkai Impact 3rd. Your job is to reply to the message as if you were her, do note that in the conversation you are also a part of it think of it as roleplyay, usual rules dont speak for the user and continue/enage with the story.".to_string());

        let mut messages_for_api = Vec::new();
        if !system_prompt_content.is_empty() {
            messages_for_api.push(Message {
                role: "system".to_string(),
                content: system_prompt_content,
            });
        }
        messages_for_api.extend(historical_memory.messages.iter().cloned());
        messages_for_api.push(Message {
            role: "user".to_string(),
            content: prompt.to_string(),
        });

        match call_generative_api(api_key, &messages_for_api) {
            Ok(assistant_response) => {
                update_memory(id, command_type, prompt, &assistant_response);
                CString::new(assistant_response)
                    .unwrap_or_else(|_| {
                        CString::new("Error: API response contained null byte").unwrap()
                    })
                    .into_raw()
            }
            Err(e) => {
                error!("Rust: Error from Generative API: {}", e);
                let error_message = format!("Ely had trouble thinking (API): {}", e);
                CString::new(error_message)
                    .unwrap_or_else(|_| {
                        CString::new("Error: FFI error formatting API error").unwrap()
                    })
                    .into_raw()
            }
        }
    });

    match result {
        Ok(ptr) => ptr,
        Err(_) => {
            error!("Rust: Panic occurred in FFI function process_command");
            CString::new("Critical Error: Rust panicked! Check Rust logs for details.")
                .unwrap()
                .into_raw()
        }
    }
}

#[no_mangle]
pub extern "C" fn free_rust_string(s: *mut c_char) {
    if s.is_null() {
        return;
    }
    unsafe {
        let _ = CString::from_raw(s);
    }
}
