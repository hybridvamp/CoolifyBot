package config

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"coolifymanager/src/coolity"
	_ "github.com/joho/godotenv/autoload"
)

var (
	Coolify    *coolify.Client
	ApiUrl     = os.Getenv("API_URL")
	ApiToken   = os.Getenv("API_TOKEN")
	ApiVersion = os.Getenv("API_VERSION")
	Token      = os.Getenv("TOKEN")
	Port       = os.Getenv("PORT")
	WebhookUrl = os.Getenv("WEBHOOK_URL")
	LogID      = os.Getenv("LOG_ID")
	DebugAPI   = os.Getenv("DEBUG_COOLIFY")
	devList    = os.Getenv("DEV_IDS") // comma-separated
	devIDs     []int64                // parsed slice
	logChatID  int64
	apiVersion string
)

func Init() error {
	if ApiUrl == "" || ApiToken == "" {
		return errors.New("API_URL and API_TOKEN must be set")
	}

	ApiUrl = sanitizeBaseURL(ApiUrl)
	apiVersion = resolveAPIVersion(ApiVersion)

	cacheTTL := 30 * time.Second
	if ttl := os.Getenv("CACHE_TTL_SECONDS"); ttl != "" {
		if sec, err := strconv.Atoi(ttl); err == nil && sec > 0 {
			cacheTTL = time.Duration(sec) * time.Second
		}
	}

	Coolify = coolify.NewClient(
		ApiUrl,
		ApiToken,
		coolify.WithAPIVersion(apiVersion),
		coolify.WithCacheTTL(cacheTTL),
		coolify.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
		coolify.WithDebug(DebugAPI == "1" || strings.ToLower(DebugAPI) == "true"),
	)

	// Parse DEV_IDS
	for _, idStr := range strings.Split(devList, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err == nil {
			devIDs = append(devIDs, id)
		} else {
			log.Printf("Dev ID is not an integer: %s", idStr)
		}
	}

	// Parse LOG_ID
	if LogID != "" {
		if id, err := strconv.ParseInt(LogID, 10, 64); err == nil {
			logChatID = id
		} else {
			log.Printf("LOG_ID is not an integer: %s", LogID)
		}
	}

	return nil
}

// IsDev checks if a given Telegram user ID is in the dev list
func IsDev(userID int64) bool {
	for _, id := range devIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func resolveAPIVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return version
}

func LogChat() int64 {
	return logChatID
}

func sanitizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "/")

	// If user passed a full API URL (e.g. .../api/v4) strip "/api/..."
	apiIndex := strings.Index(strings.ToLower(raw), "/api/")
	if apiIndex != -1 {
		raw = raw[:apiIndex]
	}

	// If user ended with "/api", strip it
	if strings.HasSuffix(strings.ToLower(raw), "/api") {
		raw = raw[:len(raw)-len("/api")]
	}

	return strings.TrimSuffix(raw, "/")
}
