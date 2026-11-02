package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StartupConfig holds process-level configuration loaded from environment variables and .env files.
type StartupConfig struct {
	AppEnv          string
	Port            string
	ConfigPath      string
	InternalSecret  string
	TokenTTL        time.Duration
	WatchInterval   time.Duration
	EnforceTLS      bool
	AdminPort       string
	AdminKey        string
	AdminEnabled    bool
	TraceBufferSize int
}

// ServiceConfig defines a logical microservice, its handled path prefixes, and upstream addresses.
type ServiceConfig struct {
	ID        string   `json:"id"`
	Prefixes  []string `json:"prefixes"`
	Upstreams []string `json:"upstreams"`
}

// Config represents the top-level dynamic routing configuration loaded from JSON.
type Config struct {
	AllowedHosts []string        `json:"allowed_hosts"`
	Services     []ServiceConfig `json:"services"`
}

// LoadStartupConfig loads environment variables from .env.gateway or .env, applies defaults, and returns StartupConfig.
func LoadStartupConfig() *StartupConfig {
	loadEnvFiles()

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}

	port := os.Getenv("GATEWAY_PORT")
	if port == "" {
		port = "8000"
	}

	configPath := os.Getenv("GATEWAY_CONFIG_PATH")
	if configPath == "" {
		configPath = filepath.Join("configs", "gateway_routes.json")
	}

	internalSecret := os.Getenv("INTERNAL_SECRET")
	if internalSecret == "" {
		internalSecret = "gym-companion-internal-secret-2026-key"
	}

	tokenTTL := time.Minute
	if ttlStr := os.Getenv("TOKEN_TTL"); ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil && d > 0 {
			tokenTTL = d
		}
	}

	watchInterval := 2 * time.Second
	if intervalStr := os.Getenv("WATCH_INTERVAL"); intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil && d > 0 {
			watchInterval = d
		}
	}

	enforceTLS := strings.EqualFold(os.Getenv("ENFORCE_TLS"), "true")

	adminPort := os.Getenv("GATEWAY_ADMIN_PORT")
	if adminPort == "" {
		adminPort = "8001"
	}

	adminKey := os.Getenv("GATEWAY_ADMIN_KEY")
	if adminKey == "" {
		adminKey = "gym-companion-admin-secret-2026"
	}

	adminEnabled := true
	if enabledStr := os.Getenv("GATEWAY_ADMIN_ENABLED"); enabledStr != "" {
		adminEnabled = !strings.EqualFold(enabledStr, "false")
	}

	traceBufferSize := 2000
	if bufStr := os.Getenv("GATEWAY_TRACE_BUFFER_SIZE"); bufStr != "" {
		var n int
		if _, err := fmt.Sscanf(bufStr, "%d", &n); err == nil && n > 0 {
			traceBufferSize = n
		}
	}

	return &StartupConfig{
		AppEnv:          appEnv,
		Port:            port,
		ConfigPath:      configPath,
		InternalSecret:  internalSecret,
		TokenTTL:        tokenTTL,
		WatchInterval:   watchInterval,
		EnforceTLS:      enforceTLS,
		AdminPort:       adminPort,
		AdminKey:        adminKey,
		AdminEnabled:    adminEnabled,
		TraceBufferSize: traceBufferSize,
	}
}

func loadEnvFiles() {
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		return
	}

	envFile := os.Getenv("GATEWAY_ENV_FILE")
	candidates := []string{".env.gateway", ".env"}
	if envFile != "" {
		candidates = append([]string{envFile}, candidates...)
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if key != "" && os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
			}
		}
		log.Printf("⚙️ [Gateway Config] Loaded environment variables from %s", path)
		break
	}
}

// Manager safely manages dynamic loading and periodic hot-reloading of gateway routing configs.
type Manager struct {
	mu           sync.RWMutex
	filePath     string
	lastModified time.Time
	current      *Config
	onChange     func(*Config)
	stopChan     chan struct{}
}

// NewManager initializes and loads the configuration file.
func NewManager(filePath string, onChange func(*Config)) (*Manager, error) {
	cm := &Manager{
		filePath: filePath,
		onChange: onChange,
		stopChan: make(chan struct{}, 1),
	}

	cfg, modTime, err := cm.readFile()
	if err != nil {
		return nil, fmt.Errorf("read initial gateway config: %w", err)
	}

	cm.current = cfg
	cm.lastModified = modTime

	if cm.onChange != nil {
		cm.onChange(cfg)
	}

	return cm, nil
}

func (cm *Manager) readFile() (*Config, time.Time, error) {
	info, err := os.Stat(cm.filePath)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stat gateway config file: %w", err)
	}

	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("read gateway config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, time.Time{}, fmt.Errorf("unmarshal gateway config: %w", err)
	}

	if extraHosts := os.Getenv("GATEWAY_ALLOWED_HOSTS"); extraHosts != "" {
		for _, h := range strings.Split(extraHosts, ",") {
			h = strings.TrimSpace(h)
			if h != "" {
				cfg.AllowedHosts = append(cfg.AllowedHosts, h)
			}
		}
	}

	return &cfg, info.ModTime(), nil
}

// StartWatcher starts background polling to detect file changes and reload dynamically.
func (cm *Manager) StartWatcher(pollInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-cm.stopChan:
				return
			case <-ticker.C:
				info, err := os.Stat(cm.filePath)
				if err != nil {
					continue
				}

				if info.ModTime().After(cm.lastModified) {
					newCfg, modTime, err := cm.readFile()
					if err != nil {
						log.Printf("[Gateway Watcher] Failed to reload %s: %v", cm.filePath, err)
						continue
					}

					cm.mu.Lock()
					cm.current = newCfg
					cm.lastModified = modTime
					cm.mu.Unlock()

					log.Printf("[Gateway Watcher] Successfully reloaded %d services and %d allowed hosts from %s",
						len(newCfg.Services), len(newCfg.AllowedHosts), cm.filePath)

					if cm.onChange != nil {
						cm.onChange(newCfg)
					}
				}
			}
		}
	}()
}

// Close stops the background file watcher.
func (cm *Manager) Close() {
	select {
	case cm.stopChan <- struct{}{}:
	default:
	}
}

// GetConfig returns a copy of the currently active configuration.
func (cm *Manager) GetConfig() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.current
}
