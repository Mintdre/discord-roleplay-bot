use crate::memory::Message;
use log::{debug, error, info, warn};
use reqwest::blocking::Client;
use serde::{Deserialize, Serialize};
use std::env;
use std::io::{self, Write};

#[derive(Serialize, Deserialize, Debug, Clone)]
struct GeminiPart {
    text: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
struct GeminiContent {
    role: String,
    parts: Vec<GeminiPart>,
}

#[derive(Serialize, Debug, Default)]
struct GeminiGenerationConfig {
    #[serde(skip_serializing_if = "Option::is_none")]
    temperature: Option<f32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    max_output_tokens: Option<u32>,
}

#[derive(Serialize, Debug)]
struct GeminiSafetySetting {
    category: String,
    threshold: String,
}

#[derive(Serialize, Debug, Default)]
struct GeminiRequest {
    contents: Vec<GeminiContent>,
    #[serde(skip_serializing_if = "Option::is_none")]
    generation_config: Option<GeminiGenerationConfig>,
    #[serde(skip_serializing_if = "Option::is_none")]
    safety_settings: Option<Vec<GeminiSafetySetting>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    system_instruction: Option<GeminiContent>,
}

#[derive(Deserialize, Debug)]
struct GeminiCandidate {
    content: GeminiContent,
    #[serde(default)]
    safety_ratings: Vec<GeminiSafetyRating>,
    #[serde(default)]
    finish_reason: Option<String>,
}

#[derive(Deserialize, Debug, Default)]
struct GeminiSafetyRating {
    category: String,
    probability: String,
}

#[derive(Deserialize, Debug)]
struct GeminiResponse {
    #[serde(default)]
    candidates: Vec<GeminiCandidate>,
}

#[derive(Deserialize, Debug)]
struct GeminiErrorDetail {
    #[serde(rename = "@type")]
    error_type: Option<String>,
    reason: Option<String>,
    message: Option<String>,
}

#[derive(Deserialize, Debug)]
struct GeminiError {
    code: Option<u16>,
    message: Option<String>,
    status: Option<String>,
    #[serde(default)]
    details: Vec<GeminiErrorDetail>,
}

#[derive(Deserialize, Debug)]
struct GeminiErrorResponse {
    error: GeminiError,
}

fn convert_to_gemini_contents(messages: &[Message]) -> (Vec<GeminiContent>, Option<GeminiContent>) {
    let mut contents = Vec::new();
    let mut system_instruction: Option<GeminiContent> = None;
    let mut last_role = "";

    for message in messages {
        if message.role == "system" {
            system_instruction = Some(GeminiContent {
                role: "system".to_string(),
                parts: vec![GeminiPart {
                    text: message.content.clone(),
                }],
            });
            continue;
        }

        let current_role = match message.role.as_str() {
            "user" => "user",
            "assistant" | "model" => "model",
            _ => {
                warn!(
                    "Unknown role found in history: '{}'. Skipping.",
                    message.role
                );
                continue;
            }
        };

        if current_role == last_role {
            warn!(
                "Consecutive messages found with the same role: '{}'. Sending as is.",
                current_role
            );
        }

        contents.push(GeminiContent {
            role: current_role.to_string(),
            parts: vec![GeminiPart {
                text: message.content.clone(),
            }],
        });
        last_role = current_role;
    }

    (contents, system_instruction)
}

pub fn call_generative_api(api_key: &str, messages: &[Message]) -> Result<String, String> {
    let model_name =
        env::var("GEMINI_MODEL").unwrap_or_else(|_| "gemini-1.5-flash-latest".to_string());
    info!("Using Gemini model: {}", model_name);

    let (gemini_contents, system_instruction) = convert_to_gemini_contents(messages);

    if gemini_contents.is_empty() && system_instruction.is_none() {
        return Err("No valid user messages found to send to API.".to_string());
    }

    let request_payload = GeminiRequest {
        contents: gemini_contents,
        system_instruction,
        generation_config: Some(GeminiGenerationConfig {
            temperature: Some(0.7),
            max_output_tokens: Some(1024),
        }),
        safety_settings: Some(vec![
            GeminiSafetySetting {
                category: "HARM_CATEGORY_HARASSMENT".to_string(),
                threshold: "BLOCK_MEDIUM_AND_ABOVE".to_string(),
            },
            GeminiSafetySetting {
                category: "HARM_CATEGORY_HATE_SPEECH".to_string(),
                threshold: "BLOCK_MEDIUM_AND_ABOVE".to_string(),
            },
            GeminiSafetySetting {
                category: "HARM_CATEGORY_SEXUALLY_EXPLICIT".to_string(),
                threshold: "BLOCK_MEDIUM_AND_ABOVE".to_string(),
            },
            GeminiSafetySetting {
                category: "HARM_CATEGORY_DANGEROUS_CONTENT".to_string(),
                threshold: "BLOCK_MEDIUM_AND_ABOVE".to_string(),
            },
        ]),
    };

    let client = Client::new();
    let api_url = format!(
        "https://generativelanguage.googleapis.com/v1beta/models/{}:generateContent?key={}",
        model_name, api_key
    );

    debug!(
        "Gemini Request Payload: {:?}",
        serde_json::to_string(&request_payload).unwrap_or_default()
    );

    let response_result = client
        .post(&api_url)
        .header("Content-Type", "application/json")
        .json(&request_payload)
        .send();

    match response_result {
        Ok(res) => {
            let status = res.status();
            match res.text() {
                Ok(raw_text) => {
                    println!("--- BEGIN RAW GEMINI RESPONSE (Status: {}) ---", status);
                    println!("{}", raw_text);
                    println!("--- END RAW GEMINI RESPONSE ---");
                    io::stdout()
                        .flush()
                        .unwrap_or_else(|e| eprintln!("Error flushing stdout: {}", e));

                    debug!(
                        "Gemini RAW Response (Status: {}): [See raw printout above/below]",
                        status
                    );

                    if status.is_success() {
                        match serde_json::from_str::<GeminiResponse>(&raw_text) {
                            Ok(parsed_response) => {
                                if let Some(candidate) = parsed_response.candidates.get(0) {
                                    if let Some(part) = candidate.content.parts.get(0) {
                                        info!("Successfully received and parsed response from Gemini.");
                                        debug!("Finish Reason: {:?}", candidate.finish_reason);
                                        debug!("Safety Ratings: {:?}", candidate.safety_ratings);
                                        return Ok(part.text.clone());
                                    }
                                }
                                warn!("Gemini response successful, but no valid text content found in candidates. Raw Text: {}", raw_text);
                                Err("No valid response content from Gemini (possibly blocked or empty).".to_string())
                            }
                            Err(e) => {
                                error!("Failed to parse successful Gemini JSON response. Error: {}. Raw Text: {}", e, raw_text);
                                Err(format!("Failed to parse Gemini JSON response: {}", e))
                            }
                        }
                    } else {
                        match serde_json::from_str::<GeminiErrorResponse>(&raw_text) {
                            Ok(error_response) => {
                                error!(
                                    "Gemini API Error (Status {}): Code={:?}, Message='{}', Status='{:?}', Details={:?}",
                                    status, error_response.error.code, error_response.error.message.as_deref().unwrap_or("N/A"), error_response.error.status, error_response.error.details
                                );
                                Err(format!(
                                    "Gemini API Error ({}): {}",
                                    status,
                                    error_response
                                        .error
                                        .message
                                        .unwrap_or_else(|| "Unknown error".to_string())
                                ))
                            }
                            Err(_) => {
                                error!("Gemini API Error (Status {}). Failed to parse error response. Raw Response: {}", status, raw_text);
                                Err(format!("Gemini API Error ({}): {}", status, raw_text))
                            }
                        }
                    }
                }
                Err(e) => {
                    error!("Failed to read Gemini response body as text: {}", e);
                    Err(format!("Failed to read Gemini response body: {}", e))
                }
            }
        }
        Err(e) => {
            error!("Failed to send request to Gemini API: {}", e);
            Err(format!("Failed to send request to Gemini API: {}", e))
        }
    }
}
