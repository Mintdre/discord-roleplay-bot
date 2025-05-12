use crate::memory::Message;
use log::{debug, error, info};
use reqwest::blocking::Client;
use serde::{Deserialize, Serialize};

#[derive(Serialize)]
struct OpenRouterRequest<'a> {
    model: &'a str,
    messages: &'a [Message],
    #[serde(skip_serializing_if = "Option::is_none")]
    temperature: Option<f32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    max_tokens: Option<u32>,
}

#[derive(Deserialize, Debug)]
struct OpenRouterChoice {
    message: Message,
}

#[derive(Deserialize, Debug)]
struct OpenRouterResponse {
    choices: Vec<OpenRouterChoice>,
}

pub fn call_openrouter_api(api_key: &str, prompt_messages: &[Message]) -> Result<String, String> {
    let client = Client::new();
    let model_name = std::env::var("OPENROUTER_MODEL")
        .unwrap_or_else(|_| "mistralai/mistral-7b-instruct".to_string());
    info!("Using OpenRouter model: {}", model_name);

    let request_payload = OpenRouterRequest {
        model: &model_name,
        messages: prompt_messages,
        temperature: Some(0.7),
        max_tokens: Some(1024),
    };

    debug!(
        "OpenRouter Request Payload: {:?}",
        serde_json::to_string_pretty(&request_payload).unwrap_or_default()
    );

    let response = client
        .post("https://openrouter.ai/api/v1/chat/completions")
        .bearer_auth(api_key)
        .header("Content-Type", "application/json")
        .json(&request_payload)
        .send();

    match response {
        Ok(res) => {
            if res.status().is_success() {
                match res.json::<OpenRouterResponse>() {
                    Ok(parsed_response) => {
                        if let Some(choice) = parsed_response.choices.get(0) {
                            info!("Successfully received response from OpenRouter.");
                            debug!("OpenRouter Full Response: {:?}", parsed_response);
                            Ok(choice.message.content.clone())
                        } else {
                            error!("OpenRouter response successful, but no choices found.");
                            Err("No response choices from OpenRouter.".to_string())
                        }
                    }
                    Err(e) => {
                        error!("Failed to parse OpenRouter JSON response: {}", e);
                        Err(format!("Failed to parse OpenRouter JSON response: {}", e))
                    }
                }
            } else {
                let status = res.status();
                let error_body = res
                    .text()
                    .unwrap_or_else(|_| "Could not read error body".to_string());
                error!("OpenRouter API Error ({}): {}", status, error_body);
                Err(format!("OpenRouter API Error ({}): {}", status, error_body))
            }
        }
        Err(e) => {
            error!("Failed to send request to OpenRouter: {}", e);
            Err(format!("Failed to send request to OpenRouter: {}", e))
        }
    }
}
