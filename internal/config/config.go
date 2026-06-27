package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	defaultListenAddr     = "127.0.0.1:17890"
	defaultBaseURL        = "https://ilinkai.weixin.qq.com"
	defaultCDNBaseURL     = "https://novac2c.cdn.weixin.qq.com/c2c"
	defaultChannelVersion = "2.0.1"
	defaultPollTimeout    = 35 * time.Second
)

type Config struct {
	ListenAddr     string
	StateDir       string
	MediaDir       string
	DBPath         string
	SettingsPath   string
	DefaultBaseURL string
	CDNBaseURL     string
	ChannelVersion string
	PollTimeout    time.Duration
	LogLevelText   string
	OpenBrowser    bool
	WebhookURL     string

	WeComCorpID         string
	WeComCorpSecret     string
	WeComAgentID        int
	WeComCallbackToken  string
	WeComCallbackAESKey string
	WeComAPIBaseURL     string
	WeComAutoReply      bool
	WeComWebhookURL     string

	AgentEnabled      bool
	AgentDefaultMode  string
	AgentMaxIterations int
	AgentSessionTTL   time.Duration
	LLMBaseURL        string
	LLMAPIKey         string
	LLMModel          string
	LLMTemperature    float64
	LLMMaxTokens      int
	FetchMaxContent   int
}

func Load() Config {
	stateDir := envOrDefault("WCFLINK_STATE_DIR", defaultStateDir())
	mediaDir := envOrDefault("WCFLINK_MEDIA_DIR", filepath.Join(stateDir, "media"))
	dbPath := envOrDefault("WCFLINK_DB_PATH", filepath.Join(stateDir, "wcfLink.db"))
	settingsPath := filepath.Join(stateDir, "settings.json")
	fileSettings := loadFileSettings(settingsPath)
	return Config{
		ListenAddr:     envOrDefault("WCFLINK_LISTEN_ADDR", valueOrDefault(fileSettings.ListenAddr, defaultListenAddr)),
		StateDir:       stateDir,
		MediaDir:       mediaDir,
		DBPath:         dbPath,
		SettingsPath:   settingsPath,
		DefaultBaseURL: envOrDefault("WCFLINK_BASE_URL", defaultBaseURL),
		CDNBaseURL:     envOrDefault("WCFLINK_CDN_BASE_URL", defaultCDNBaseURL),
		ChannelVersion: envOrDefault("WCFLINK_CHANNEL_VERSION", defaultChannelVersion),
		PollTimeout:    envDurationOrDefault("WCFLINK_POLL_TIMEOUT", defaultPollTimeout),
		LogLevelText:   envOrDefault("WCFLINK_LOG_LEVEL", "INFO"),
		OpenBrowser:    envBoolOrDefault("WCFLINK_OPEN_BROWSER", false),
		WebhookURL:     envOrDefault("WCFLINK_WEBHOOK_URL", fileSettings.WebhookURL),

		WeComCorpID:         envOrDefault("WCFLINK_WECOM_CORP_ID", ""),
		WeComCorpSecret:     envOrDefault("WCFLINK_WECOM_CORP_SECRET", ""),
		WeComAgentID:        envIntOrDefault("WCFLINK_WECOM_AGENT_ID", 0),
		WeComCallbackToken:  envOrDefault("WCFLINK_WECOM_CALLBACK_TOKEN", ""),
		WeComCallbackAESKey: envOrDefault("WCFLINK_WECOM_CALLBACK_AES_KEY", ""),
		WeComAPIBaseURL:     envOrDefault("WCFLINK_WECOM_API_BASE_URL", ""),
		WeComAutoReply:      envBoolOrDefault("WCFLINK_WECOM_AUTO_REPLY", false),
		WeComWebhookURL:     envOrDefault("WCFLINK_WECOM_WEBHOOK_URL", ""),

		AgentEnabled:      envBoolOrDefault("WCFLINK_AGENT_ENABLED", false),
		AgentDefaultMode:  envOrDefault("WCFLINK_AGENT_DEFAULT_MODE", "icemark"),
		AgentMaxIterations: envIntOrDefault("WCFLINK_AGENT_MAX_ITERATIONS", 10),
		AgentSessionTTL:   envDurationOrDefault("WCFLINK_AGENT_SESSION_TTL", 168*time.Hour),
		LLMBaseURL:        envOrDefault("WCFLINK_LLM_BASE_URL", "https://api.deepseek.com"),
		LLMAPIKey:         envOrDefault("WCFLINK_LLM_API_KEY", ""),
		LLMModel:          envOrDefault("WCFLINK_LLM_MODEL", "deepseek-chat"),
		LLMTemperature:    envFloatOrDefault("WCFLINK_LLM_TEMPERATURE", 0.7),
		LLMMaxTokens:      envIntOrDefault("WCFLINK_LLM_MAX_TOKENS", 4096),
		FetchMaxContent:   envIntOrDefault("WCFLINK_FETCH_MAX_CONTENT_LENGTH", 8000),
	}
}

type FileSettings struct {
	ListenAddr string `json:"listen_addr"`
	WebhookURL string `json:"webhook_url"`
}

func SaveFileSettings(path string, settings FileSettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c Config) LogLevel() slog.Level {
	switch c.LogLevelText {
	case "DEBUG", "debug":
		return slog.LevelDebug
	case "WARN", "warn", "WARNING", "warning":
		return slog.LevelWarn
	case "ERROR", "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func defaultStateDir() string {
	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		return filepath.Join(filepath.Dir(exePath), "data")
	}
	return filepath.Join(".", "data")
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func envBoolOrDefault(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func envFloatOrDefault(key string, fallback float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func loadFileSettings(path string) FileSettings {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileSettings{}
	}
	var settings FileSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return FileSettings{}
	}
	return settings
}

func valueOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
