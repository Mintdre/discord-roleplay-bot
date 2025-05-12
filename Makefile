.PHONY: all clean rust go run build

# Detect OS for shared library extension
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
    LIB_EXT := so
    RUST_TARGET_DIR := target/release
endif
ifeq ($(UNAME_S),Darwin)
    LIB_EXT := dylib
    RUST_TARGET_DIR := target/release
endif
# Add for Windows if needed:
# ifeq ($(OS),Windows_NT)
#    LIB_EXT := dll
#    RUST_TARGET_DIR := target\release
# endif

RUST_LIB_NAME := libely_rust.$(LIB_EXT)
RUST_LIB_PATH := rust/$(RUST_TARGET_DIR)/$(RUST_LIB_NAME)
GO_BUILD_LIB_DEST := go/$(RUST_LIB_NAME) # For Go build step
GO_BINARY_NAME := elybot
GO_OUTPUT_PATH := ./$(GO_BINARY_NAME) # Output Go binary in the root directory
RUNTIME_LIB_DEST := ./$(RUST_LIB_NAME) # For runtime, next to the Go binary

all: build

build: rust go

rust:
	@echo "Building Rust library..."
	@cd rust && cargo build --release
	@echo "Copying Rust library $(RUST_LIB_PATH) to $(GO_BUILD_LIB_DEST) (for Go build)..."
	@mkdir -p go # Ensure go directory exists for copying
	@cp $(RUST_LIB_PATH) $(GO_BUILD_LIB_DEST)
	@echo "Copying Rust library $(RUST_LIB_PATH) to $(RUNTIME_LIB_DEST) (for runtime)..."
	@cp $(RUST_LIB_PATH) $(RUNTIME_LIB_DEST) # <--- ADDED THIS LINE

go: $(GO_BUILD_LIB_DEST) # Depends on the Rust library being copied for build
	@echo "Building Go application..."
	@cd go && go build -o ../$(GO_BINARY_NAME) . # Output to parent dir (project root)
	@echo "Go application built as $(GO_OUTPUT_PATH)"

$(GO_BUILD_LIB_DEST): rust # If the lib is missing, build rust first

run: build
	@echo "Running Elybot from $(PWD)..." # Show current working directory
	@echo "Ensure $(RUST_LIB_NAME) is in the same directory as $(GO_BINARY_NAME) or in LD_LIBRARY_PATH."
	@./$(GO_BINARY_NAME)

clean:
	@echo "Cleaning up..."
	@cd rust && cargo clean
	@rm -f $(GO_BUILD_LIB_DEST)
	@rm -f $(RUNTIME_LIB_DEST) # <--- ADDED THIS LINE
	@rm -f $(GO_OUTPUT_PATH)
	@rm -rf rust/data/*_memory.json # Be careful with this, it deletes memory files!
	@echo "Cleaned."

# Create dummy data directory if it doesn't exist (useful for first run)
setup_data_dir:
	@mkdir -p rust/data

# Reminder for .env file
env_check:
ifndef DISCORD_BOT_TOKEN
	$(error DISCORD_BOT_TOKEN is not set. Please create a .env file or set environment variables.)
endif
ifndef OPENROUTER_API_KEY
	$(error OPENROUTER_API_KEY is not set. Please create a .env file or set environment variables.)
endif

# Target to ensure .env exists (or remind user)
ensure_env:
	@if [ ! -f .env ]; then \
		echo "Warning: .env file not found. Please create it from .env.example or set environment variables:"; \
		echo "DISCORD_BOT_TOKEN=your_discord_bot_token"; \
		echo "OPENROUTER_API_KEY=your_openrouter_key"; \
		echo "Optional: OPENROUTER_MODEL=mistralai/mistral-7b-instruct (or any other model)"; \
		exit 1; \
	fi
	@echo ".env file found."

# A target that runs all pre-requisites
prepare: ensure_env setup_data_dir
