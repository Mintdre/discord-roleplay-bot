package main

/*
// Link against libely_rust.so (or .dylib for macOS, .dll for Windows)
// The library is expected to be in the same directory as the Go executable during runtime,
// or in a standard library path. The Makefile copies it to the go/ directory before build.
#cgo LDFLAGS: -L. -lely_rust
#include <stdlib.h> // For C.free

// Forward declarations of Rust functions
char* process_command(char* command_type, char* prompt, char* id, char* api_key);
void free_rust_string(char* s);
*/
import "C"

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unsafe"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	discordToken     string
	openRouterAPIKey string
	guildID          string
)

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	discordToken = os.Getenv("DISCORD_BOT_TOKEN")
	openRouterAPIKey = os.Getenv("OPENROUTER_API_KEY")

	if discordToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN not set in .env or environment variables")
	}
	if openRouterAPIKey == "" {
		log.Fatal("OPENROUTER_API_KEY not set in .env or environment variables")
	}
	log.Println("Environment variables loaded.")
}

func main() {
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Ely bot is healthy!")
		})
		log.Println("HTTP server listening on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
		return
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
		return
	}

	log.Println("Ely bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
	log.Println("Ely bot shutting down.")
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	var commandType, prompt, id string

	if strings.HasPrefix(m.Content, "!ely ") {
		commandType = "user"
		prompt = strings.TrimSpace(strings.TrimPrefix(m.Content, "!ely "))
		id = m.Author.ID
		log.Printf("User command: UserID=%s, Prompt='%s'", id, prompt)
	} else if strings.HasPrefix(m.Content, "!elyall ") {
		if m.GuildID == "" {
			s.ChannelMessageSend(m.ChannelID, "The `!elyall` command can only be used in a server channel.")
			return
		}
		commandType = "server"
		prompt = strings.TrimSpace(strings.TrimPrefix(m.Content, "!elyall "))
		id = m.GuildID
		log.Printf("Server command: GuildID=%s, Prompt='%s'", id, prompt)
	} else {
		return
	}

	if prompt == "" {
		s.ChannelMessageSend(m.ChannelID, "Please provide a prompt after the command.")
		return
	}

	s.ChannelTyping(m.ChannelID)

	cCommandType := C.CString(commandType)
	defer C.free(unsafe.Pointer(cCommandType))

	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))

	cAPIKey := C.CString(openRouterAPIKey)
	defer C.free(unsafe.Pointer(cAPIKey))

	log.Printf("Calling Rust: Type=%s, ID=%s, Prompt=%s", commandType, id, prompt)
	cResponse := C.process_command(cCommandType, cPrompt, cID, cAPIKey)

	goResponse := C.GoString(cResponse)
	C.free_rust_string(cResponse)

	log.Printf("Go received from Rust: '%s'", goResponse)

	if len(goResponse) > 2000 {
		s.ChannelMessageSend(m.ChannelID, goResponse[:1990]+"...")
		s.ChannelMessageSend(m.ChannelID, "... (message truncated)")
	} else {
		s.ChannelMessageSend(m.ChannelID, goResponse)
	}
}
