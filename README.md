
# Elybot - Hybrid Go/Rust Discord Roleplay Bot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT) <!-- Or choose another license -->

A Discord bot designed for roleplaying interactions, built with a hybrid Go and Rust architecture. Go handles Discord API interactions and command parsing, while Rust manages memory, core logic, and communication with the Google Gemini API.

## Features

*   **Hybrid Architecture:** Combines Go (for network I/O, Discord API) and Rust (for safety, performance, core logic) via FFI (shared library).
*   **AI Integration:** Uses the Google Gemini API (via Rust `reqwest`) for generating roleplay responses.
*   **Persistent Memory:**
    *   **User-specific memory:** Remembers conversation history on a per-user basis (`!ely`, `/ely`).
    *   **Server-wide memory:** Remembers conversation history for the entire server (`!elyall`, `/elyall`).
    *   Memory is stored as JSON files in `rust/data/` and cached in memory.
    *   Memory history is truncated to prevent excessive growth (`MAX_HISTORY_LEN` in Rust).
*   **Dual Command Support:** Responds to both traditional prefix commands (`!ely`, `!elyall`) and Discord Slash Commands (`/ely`, `/elyall`).
*   **Configurable Logging:**
    *   Rust logs can be set to INFO (default) or DEBUG (`--verbose` flag).
    *   Go logs can be localized (`--lang` flag, currently supports `en` and `zh-cn`).
*   **Configuration via `.env`:** API keys and model settings are managed securely.
*   **Easy Build Process:** Includes a `Makefile` for simple compilation of both Go and Rust components.
*   **Health Check:** Basic HTTP endpoint (`/health`) for monitoring.

## Project Structure

```
elybot/
├── Makefile # Build script
├── .env # Local environment variables (ignored by git)
├── .env.example # Example environment file
├── .gitignore # Files to ignore in git
├── go/ # Go application source
│   ├── main.go # Main Go application (Discord, HTTP, FFI calls)
│   ├── go.mod # Go module definition
│   ├── go.sum # Go module checksums
│   └── libely_rust.so # Compiled Rust library (copied by Makefile)
├── rust/ # Rust library source
│   ├── Cargo.toml # Rust package definition
│   ├── src/
│   │   ├── lib.rs # FFI interface, main Rust logic orchestration
│   │   ├── memory.rs # Memory management (load, save, update)
│   │   └── gemini_client.rs # Google Gemini API interaction logic
│   └── data/ # Persistent storage for memory files
│       └── example_*.json # Example memory files (ignored by git)
├── elybot # Compiled Go executable (output by Makefile)
├── libely_rust.so # Compiled Rust library (copied for runtime)
└── README.md # This file
```

## Prerequisites

*   **Go:** Version 1.18 or later. ([Installation Guide](https://golang.org/doc/install))
*   **Rust:** Stable toolchain. ([Installation Guide](https://www.rust-lang.org/tools/install))
*   **Git:** For cloning and version control. ([Installation Guide](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git))
*   **Discord Bot Token:**
    *   Create a bot application on the [Discord Developer Portal](https://discord.com/developers/applications).
    *   Enable the **Message Content Intent** under Privileged Gateway Intents.
    *   Copy the bot token.
*   **Google API Key:**
    *   Create an API key enabled for the "Generative Language API" (Gemini) from the [Google AI Developer site](https://aistudio.google.com/app/apikey) or Google Cloud Console.
*   **(Optional) `make`:** For using the Makefile (standard on Linux/macOS).

## Configuration

1.  Copy the example environment file:
    ```bash
    cp .env.example .env
    ```
2.  Edit the `.env` file and fill in your actual credentials and desired settings:

    ```env
    # --- Discord Settings ---
    DISCORD_BOT_TOKEN="YOUR_DISCORD_BOT_TOKEN_HERE"

    # --- Google Gemini Settings ---
    GOOGLE_API_KEY="YOUR_GOOGLE_GEMINI_API_KEY_HERE"
    # Model examples: gemini-1.5-flash-latest, gemini-1.5-pro-latest, gemini-1.0-pro
    GEMINI_MODEL="gemini-1.5-flash-latest" 

    # --- System Prompt (Optional, used by Rust) ---
    # Defines the initial personality/instructions for the AI
    SYSTEM_PROMPT="You are Ely, a friendly and creative roleplaying assistant. Engage with the user, continue the story, and be imaginative."
    ```

## Building

The project uses a `Makefile` to simplify the build process.

1.  **Prepare (First time):** Checks for `.env` and creates `rust/data` directory.
    ```bash
    make prepare
    ```
2.  **Build All:** Compiles the Rust library and the Go executable.
    ```bash
    make
    # Or: make build
    ```
    This will:
    *   Compile the Rust library (`libely_rust.so`, `.dylib`, or `.dll`) in release mode (`rust/target/release/`).
    *   Copy the Rust library to `go/` (for Go build linking) and to the project root (`./`) (for runtime linking).
    *   Compile the Go application into an executable named `elybot` in the project root.

3.  **Clean:** Removes build artifacts and copied libraries.
    ```bash
    make clean
    ```

## Running

1.  **Build the bot** using `make`.
2.  **Run the executable:**
    ```bash
    ./elybot [flags]
    ```

**Command-line Flags:**

*   `--lang <code>`: Sets the language for Go's console logs (e.g., `--lang en`, `--lang zh-cn`). Default: `en`.
*   `--verbose`: Enables detailed DEBUG level logging in the Rust library (shows API payloads, raw responses, etc.). Default: `false` (INFO level).
*   `--port <number>`: Sets the port for the HTTP health check server. Default: `8080`.
*   `--guild <id>`: (Development) Registers slash commands *only* to this specific Guild ID for instant updates, instead of globally.
*   `--remove-commands`: If present, the bot will remove all its registered slash commands (globally, or in the specified `--guild`) and then exit. Useful for cleanup.

**Example:** Run with Chinese logs and detailed Rust logging:
```bash
./elybot --lang zh-cn --verbose
```

## Inviting the Bot

1. Go to your bot application on the Discord Developer Portal.
2. Navigate to "OAuth2" -> "URL Generator".
3. Select the bot scope.
4. Under "Bot Permissions", grant:
    - Send Messages
    - Read Message History
    - (Implicitly needs View Channels)
5. Copy the generated URL and open it in your browser to invite the bot to your desired server(s).

## Usage (Commands)

Interact with the bot using either prefix or slash commands:

**User Memory:**

- `!ely <your message>`
- `/ely prompt:<your message>`

Uses memory specific to your Discord User ID.

**Server Memory:**

- `!elyall <your message>` (Must be used in a server channel)
- `/elyall prompt:<your message>` (Must be used in a server channel)

Uses memory shared across the entire Discord Server (Guild ID).

## Notes

- **Memory:** Memory files are stored in `rust/data/` named `<id>_<type>_memory.json`. They are simple JSON arrays of message objects. History is currently limited by `MAX_HISTORY_LEN` in `rust/src/memory.rs`.
- **FFI:** Communication between Go and Rust happens via a C Foreign Function Interface (FFI). Go loads the compiled Rust shared library (`libely_rust.so` or equivalent) and calls exported C functions defined in `rust/src/lib.rs`.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

