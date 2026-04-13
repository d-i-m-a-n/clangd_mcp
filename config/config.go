package config

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const (
	DefaultPort = 7878

	EnvClangdPath = "CLANGD_MCP_CLANGD_PATH"
	EnvPort       = "CLANGD_MCP_PORT"
)

// Config holds the resolved application configuration.
type Config struct {
	ClangdPath string // absolute path to clangd binary
	Port       int    // MCP server port
}

// fileConfig mirrors the JSON config file structure.
type fileConfig struct {
	ClangdPath string `json:"clangd_path,omitempty"`
	Port       int    `json:"port,omitempty"`
}

// Load resolves configuration with priority: env > config file > defaults.
func Load() Config {
	fc := loadFile()
	return Config{
		ClangdPath: resolveClangdPath(fc.ClangdPath),
		Port:       resolvePort(fc.Port),
	}
}

// loadFile reads clangd-mcp.cfg from the same directory as the executable.
func loadFile() fileConfig {
	exe, err := os.Executable()
	if err != nil {
		return fileConfig{}
	}
	cfgPath := filepath.Join(filepath.Dir(exe), "clangd-mcp.cfg")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fileConfig{}
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		log.Printf("clangd-mcp: %s parse error: %v", cfgPath, err)
		return fileConfig{}
	}
	log.Printf("clangd-mcp: loaded config from %s", cfgPath)
	return fc
}

// resolveClangdPath: env > config file > PATH lookup.
func resolveClangdPath(fromFile string) string {
	if v := os.Getenv(EnvClangdPath); v != "" {
		return v
	}
	if fromFile != "" {
		return fromFile
	}
	path, err := exec.LookPath("clangd")
	if err != nil {
		return "clangd"
	}
	return path
}

// resolvePort: env > config file > default.
func resolvePort(fromFile int) int {
	if v := os.Getenv(EnvPort); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			return port
		}
		log.Printf("clangd-mcp: invalid %s=%q, using default", EnvPort, v)
	}
	if fromFile > 0 {
		return fromFile
	}
	return DefaultPort
}
