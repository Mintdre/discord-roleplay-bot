package main

/*
// Link against libely_rust.so (or .dylib for macOS, .dll for Windows)
#cgo LDFLAGS: -L. -lely_rust
#include <stdlib.h>

char* process_command(char* command_type, char* prompt, char* id, char* api_key);
void free_rust_string(char* s);
*/
import "C"

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	discordToken    string
	googleAPIKey    string
	currentLanguage string
	guildID         = flag.String("guild", "", "Optional: Guild ID for registering commands instantly (dev)")
	httpPort        = flag.String("port", "8080", "Port for the HTTP health check server")
	lang            = flag.String("lang", "en", "Language code for Go logging (e.g., 'en', 'zh-cn')")
	removeCommands  = flag.Bool("remove-commands", false, "Remove all registered slash commands for this bot and exit")
	verbose         = flag.Bool("verbose", false, "Enable detailed (DEBUG) logging in the Rust library") // <-- New Flag
)

var localizedMessages = map[string]map[string]string{
	"en": {
		"envLoadingWarn":       "Warning: No .env file found at %s (or error loading it), relying on environment variables: %v",
		"envTokenMissing":      "FATAL: DISCORD_BOT_TOKEN not set in .env or environment variables",
		"envApiKeyMissing":     "FATAL: GOOGLE_API_KEY not set in .env or environment variables",
		"envLoaded":            "Environment variables loaded.",
		"httpServerStart":      "Go: HTTP server starting on :%s",
		"httpServerFail":       "Go: Failed to start HTTP server: %v",
		"discordSessionFail":   "Go: Error creating Discord session: %v",
		"discordConnFail":      "Go: Error opening Discord connection: %v",
		"discordBotRunning":    "Go: Ely bot is now running (Lang: %s, Verbose: %t). Press CTRL-C to exit.", // Added Verbose status
		"discordBotShutdown":   "Go: Ely bot shutting down.",
		"cmdGuildOnly":         "The `!elyall` command can only be used in a server channel.",
		"cmdPromptMissing":     "Please provide a prompt after the command. Example: `!ely What's happening?`",
		"cmdParsed":            "Go: Parsed command: Type=%s, ID=%s, Prompt='%s'",
		"typingIndicatorFail":  "Go: Error sending typing indicator: %v",
		"ffiCall":              "Go: Calling Rust FFI: Type=%s, ID=%s, PromptLen=%d",
		"ffiReturnNull":        "Go: Rust FFI call returned a NULL pointer. This indicates a severe error in the Rust library.",
		"ffiResponse":          "Go: Received from Rust: ResponseLen=%d",
		"ffiProcessingError":   "Go: Error processing command via Rust: %v",
		"discordSendFail":      "Go: Error sending message to Discord channel %s: %v",
		"discordMsgTruncated":  "(Message truncated due to length)",
		"healthCheckResponse":  "Ely bot is healthy and running!",
		"invalidLang":          "Warning: Invalid language code '%s' provided. Defaulting to 'en'.",
		"slashCmdDeferFail":    "Failed to acknowledge slash command interaction: %v",
		"slashCmdEditFail":     "Failed to edit slash command response: %v",
		"slashCmdElyDesc":      "Send a prompt to Ely using your personal memory.",
		"slashCmdElyAllDesc":   "Send a prompt to Ely using the server's shared memory.",
		"slashCmdPromptDesc":   "The prompt or message for Ely.",
		"slashCmdRegistering":  "Registering slash commands...",
		"slashCmdRegisterOK":   "Successfully registered command: %s",
		"slashCmdRegisterFail": "Failed to register command %s: %v",
		"slashCmdRemoving":     "Removing all registered commands...",
		"slashCmdRemoveOk":     "Successfully removed command: %s",
		"slashCmdRemoveFail":   "Failed to remove commands: %v",
		"slashCmdComplete":     "Command registration/removal complete.",
		"slashCmdElyAllGuild":  "The /elyall command can only be used in a server.",
		"setRustLogLevel":      "Go: Setting Rust log level via ELYBOT_LOG_LEVEL=%s", // New log message
	},
	"zh-cn": {
		"envLoadingWarn":       "警告：在 %s 未找到 .env 文件（或加载错误），将依赖环境变量：%v",
		"envTokenMissing":      "致命错误：未在 .env 文件或环境变量中设置 DISCORD_BOT_TOKEN",
		"envApiKeyMissing":     "致命错误：未在 .env 文件或环境变量中设置 GOOGLE_API_KEY",
		"envLoaded":            "环境变量已加载。",
		"httpServerStart":      "Go: HTTP 服务器正在启动于 :%s",
		"httpServerFail":       "Go: 启动 HTTP 服务器失败：%v",
		"discordSessionFail":   "Go: 创建 Discord 会话时出错：%v",
		"discordConnFail":      "Go: 打开 Discord 连接时出错：%v",
		"discordBotRunning":    "Go: Ely 机器人正在运行 (语言: %s, 详细日志: %t)。按 CTRL-C 退出。", // Added Verbose status
		"discordBotShutdown":   "Go: Ely 机器人正在关闭。",
		"cmdGuildOnly":         "`!elyall` 命令只能在服务器频道中使用。",
		"cmdPromptMissing":     "请在命令后提供提示内容。例如：`!ely 发生了什么？`",
		"cmdParsed":            "Go: 解析命令：类型=%s，ID=%s，提示='%s'",
		"typingIndicatorFail":  "Go: 发送输入状态指示器时出错：%v",
		"ffiCall":              "Go: 调用 Rust FFI：类型=%s，ID=%s，提示长度=%d",
		"ffiReturnNull":        "Go: Rust FFI 调用返回了 NULL 指针。这表明 Rust 库中存在严重错误。",
		"ffiResponse":          "Go: 从 Rust 接收：响应长度=%d",
		"ffiProcessingError":   "Go: 通过 Rust 处理命令时出错：%v",
		"discordSendFail":      "Go: 发送消息到 Discord 频道 %s 时出错：%v",
		"discordMsgTruncated":  "(消息过长，已被截断)",
		"healthCheckResponse":  "Ely 机器人健康运行中！",
		"invalidLang":          "警告：提供了无效的语言代码 '%s'。将使用默认语言 'en'。",
		"slashCmdDeferFail":    "未能确认斜杠命令交互: %v",
		"slashCmdEditFail":     "未能编辑斜杠命令响应: %v",
		"slashCmdElyDesc":      "使用您的个人记忆向 Ely 发送提示。",
		"slashCmdElyAllDesc":   "使用服务器的共享记忆向 Ely 发送提示。",
		"slashCmdPromptDesc":   "给 Ely 的提示或消息。",
		"slashCmdRegistering":  "正在注册斜杠命令...",
		"slashCmdRegisterOK":   "成功注册命令: %s",
		"slashCmdRegisterFail": "注册命令 %s 失败: %v",
		"slashCmdRemoving":     "正在移除所有已注册的命令...",
		"slashCmdRemoveOk":     "成功移除命令: %s",
		"slashCmdRemoveFail":   "移除命令失败: %v",
		"slashCmdComplete":     "命令注册/移除完成。",
		"slashCmdElyAllGuild":  "/elyall 命令只能在服务器中使用。",
		"setRustLogLevel":      "Go: 通过 ELYBOT_LOG_LEVEL=%s 设置 Rust 日志级别", // New log message
	},
}

func getLogMsg(key string, args ...interface{}) string {
	if currentLanguage == "" {
		currentLanguage = "en"
	}
	langMap, langExists := localizedMessages[currentLanguage]
	if !langExists {
		langMap = localizedMessages["en"]
	}
	msgTemplate, keyExists := langMap[key]
	if !keyExists {
		msgTemplate, keyExists = localizedMessages["en"][key]
		if !keyExists {
			return fmt.Sprintf("!!MISSING LOG KEY: %s!!", key)
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(msgTemplate, args...)
	}
	return msgTemplate
}

func loadEnv() {
	exePath, err := os.Executable()
	envPath := ".env"
	if err == nil {
		envPath = filepath.Join(filepath.Dir(exePath), ".env")
	} else {
		log.Printf("Warning: Could not determine executable path: %v", err)
	}
	err = godotenv.Load(envPath)
	if err != nil {
		log.Print(getLogMsg("envLoadingWarn", envPath, err))
	}
	discordToken = os.Getenv("DISCORD_BOT_TOKEN")
	googleAPIKey = os.Getenv("GOOGLE_API_KEY")
	if discordToken == "" {
		log.Fatal(getLogMsg("envTokenMissing"))
	}
	if googleAPIKey == "" {
		log.Fatal(getLogMsg("envApiKeyMissing"))
	}
	log.Print(getLogMsg("envLoaded"))
}

func callRustProcessCommand(commandType, prompt, id, apiKey string) (string, error) {
	cCommandType := C.CString(commandType)
	defer C.free(unsafe.Pointer(cCommandType))
	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))
	cAPIKey := C.CString(apiKey)
	defer C.free(unsafe.Pointer(cAPIKey))
	log.Print(getLogMsg("ffiCall", commandType, id, len(prompt)))
	cResponse := C.process_command(cCommandType, cPrompt, cID, cAPIKey)
	if cResponse == nil {
		log.Print(getLogMsg("ffiReturnNull"))
		return "", fmt.Errorf("critical error: Rust FFI call returned NULL")
	}
	goResponse := C.GoString(cResponse)
	C.free_rust_string(cResponse)
	log.Print(getLogMsg("ffiResponse", len(goResponse)))
	if strings.HasPrefix(goResponse, "Error:") || strings.HasPrefix(goResponse, "Critical Error:") {
		return "", fmt.Errorf(goResponse)
	}
	return goResponse, nil
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	var commandType, prompt, id string
	content := strings.TrimSpace(m.Content)
	if strings.HasPrefix(content, "!ely ") {
		commandType = "user"
		prompt = strings.TrimSpace(strings.TrimPrefix(content, "!ely "))
		id = m.Author.ID
	} else if strings.HasPrefix(content, "!elyall ") {
		if m.GuildID == "" {
			s.ChannelMessageSend(m.ChannelID, getLogMsg("cmdGuildOnly"))
			return
		}
		commandType = "server"
		prompt = strings.TrimSpace(strings.TrimPrefix(content, "!elyall "))
		id = m.GuildID
	} else {
		return
	}
	if prompt == "" {
		s.ChannelMessageSend(m.ChannelID, getLogMsg("cmdPromptMissing"))
		return
	}
	log.Print(getLogMsg("cmdParsed", commandType, id, prompt))
	if typingErr := s.ChannelTyping(m.ChannelID); typingErr != nil {
		log.Print(getLogMsg("typingIndicatorFail", typingErr))
	}
	response, err := callRustProcessCommand(commandType, prompt, id, googleAPIKey)
	if err != nil {
		log.Print(getLogMsg("ffiProcessingError", err))
		userErrorMessage := fmt.Sprintf("Ely encountered a hiccup: %s", err.Error())
		if len(userErrorMessage) > 1900 {
			userErrorMessage = userErrorMessage[:1900] + "..."
		}
		s.ChannelMessageSend(m.ChannelID, userErrorMessage)
		return
	}
	if len(response) > 2000 {
		s.ChannelMessageSend(m.ChannelID, response[:1990]+"...")
		s.ChannelMessageSend(m.ChannelID, getLogMsg("discordMsgTruncated"))
	} else {
		if _, sendErr := s.ChannelMessageSend(m.ChannelID, response); sendErr != nil {
			log.Print(getLogMsg("discordSendFail", m.ChannelID, sendErr))
		}
	}
}

var commands = []*discordgo.ApplicationCommand{
	{Name: "ely", Description: getLogMsg("slashCmdElyDesc"), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "prompt", Description: getLogMsg("slashCmdPromptDesc"), Required: true}}},
	{Name: "elyall", Description: getLogMsg("slashCmdElyAllDesc"), Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "prompt", Description: getLogMsg("slashCmdPromptDesc"), Required: true}}},
}

func interactionHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	data := i.ApplicationCommandData()
	var commandType, prompt, id string
	promptOption, ok := data.Options[0].Value.(string)
	if !ok || promptOption == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Content: "Error: Prompt is missing."}})
		return
	}
	prompt = promptOption
	switch data.Name {
	case "ely":
		commandType = "user"
		if i.User != nil {
			id = i.User.ID
		} else if i.Member != nil && i.Member.User != nil {
			id = i.Member.User.ID
		} else {
			log.Printf("Error: Could not get user ID from interaction. GuildID: %s, User: %+v, Member: %+v", i.GuildID, i.User, i.Member)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Content: "Error: Could not identify user."}})
			return
		}
	case "elyall":
		if i.GuildID == "" {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Content: getLogMsg("slashCmdElyAllGuild")}})
			return
		}
		commandType = "server"
		id = i.GuildID
	default:
		return
	}
	log.Print(getLogMsg("cmdParsed", commandType, id, prompt))
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource})
	if err != nil {
		log.Print(getLogMsg("slashCmdDeferFail", err))
		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: "Error: Failed to properly acknowledge command."})
		return
	}
	response, err := callRustProcessCommand(commandType, prompt, id, googleAPIKey)
	var responseContent string
	if err != nil {
		log.Print(getLogMsg("ffiProcessingError", err))
		responseContent = fmt.Sprintf("Ely encountered a hiccup: %s", err.Error())
		if len(responseContent) > 1900 {
			responseContent = responseContent[:1900] + "..."
		}
	} else {
		responseContent = response
	}
	finalContent := responseContent
	extraMessage := ""
	if len(responseContent) > 2000 {
		finalContent = responseContent[:1990] + "..."
		extraMessage = "\n" + getLogMsg("discordMsgTruncated")
	}
	_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &finalContent})
	if err != nil {
		log.Print(getLogMsg("slashCmdEditFail", err))
		_, errFollowup := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: finalContent})
		if errFollowup != nil {
			log.Printf("Failed to send followup message after edit failure: %v", errFollowup)
		} else if extraMessage != "" {
			_, errExtra := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: extraMessage})
			if errExtra != nil {
				log.Printf("Failed to send truncation follow-up message after edit failure: %v", errExtra)
			}
			extraMessage = ""
		}
	}
	if extraMessage != "" && err == nil {
		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: extraMessage})
		if err != nil {
			log.Printf("Failed to send truncation follow-up message: %v", err)
		}
	}
}

func main() {
	flag.Parse()

	if _, ok := localizedMessages[*lang]; ok {
		currentLanguage = *lang
	} else {
		log.Printf(getLogMsg("invalidLang", *lang))
		currentLanguage = "en"
	}

	rustLogLevel := "info"
	if *verbose {
		rustLogLevel = "debug"
	}
	err := os.Setenv("ELYBOT_LOG_LEVEL", rustLogLevel)
	if err != nil {
		log.Printf("Warning: Failed to set ELYBOT_LOG_LEVEL environment variable: %v", err)
	} else {
		log.Print(getLogMsg("setRustLogLevel", rustLogLevel))
	}

	loadEnv() // Load .env AFTER setting potential log level env var

	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, getLogMsg("healthCheckResponse")) })
		log.Print(getLogMsg("httpServerStart", *httpPort))
		if err := http.ListenAndServe(":"+*httpPort, nil); err != nil {
			log.Fatal(getLogMsg("httpServerFail", err))
		}
	}()

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatal(getLogMsg("discordSessionFail", err))
	}
	dg.AddHandler(messageCreate)
	dg.AddHandler(interactionHandler)
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages
	err = dg.Open()
	if err != nil {
		log.Fatal(getLogMsg("discordConnFail", err))
	}

	if *removeCommands {
		log.Print(getLogMsg("slashCmdRemoving"))
		registeredCommands, err := dg.ApplicationCommands(dg.State.User.ID, *guildID)
		if err != nil {
			log.Fatal(getLogMsg("slashCmdRemoveFail", err))
		}
		for _, cmd := range registeredCommands {
			err := dg.ApplicationCommandDelete(dg.State.User.ID, *guildID, cmd.ID)
			if err != nil {
				log.Fatalf("Cannot delete command %q: %v", cmd.Name, err)
			} else {
				log.Print(getLogMsg("slashCmdRemoveOk", cmd.Name))
			}
		}
		log.Print(getLogMsg("slashCmdComplete"))
		dg.Close()
		os.Exit(0)
	} else {
		log.Print(getLogMsg("slashCmdRegistering"))
		registeredCmds := make([]*discordgo.ApplicationCommand, len(commands))
		for i, cmd := range commands {
			registeredCmd, err := dg.ApplicationCommandCreate(dg.State.User.ID, *guildID, cmd)
			if err != nil {
				log.Print(getLogMsg("slashCmdRegisterFail", cmd.Name, err))
			} else {
				log.Print(getLogMsg("slashCmdRegisterOK", cmd.Name))
				registeredCmds[i] = registeredCmd
			}
		}
		log.Print(getLogMsg("slashCmdComplete"))
	}

	log.Print(getLogMsg("discordBotRunning", currentLanguage, *verbose))
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
	log.Print(getLogMsg("discordBotShutdown"))
}
