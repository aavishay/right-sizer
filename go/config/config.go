// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package config

import (
	"log"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for resource sizing
type Config struct {
	// Request multipliers - how much to multiply usage to get requests
	CPURequestMultiplier    float64
	MemoryRequestMultiplier float64

	// Limit multipliers - how much to multiply requests to get limits
	CPULimitMultiplier    float64
	MemoryLimitMultiplier float64

	// Maximum caps for resources
	MaxCPULimit    int64 // in millicores
	MaxMemoryLimit int64 // in MB

	// Minimum values for resources
	MinCPURequest    int64 // in millicores
	MinMemoryRequest int64 // in MB

	// Operational configuration
	ResizeInterval time.Duration // How often to check and resize resources
	LogLevel       string        // Log level: debug, info, warn, error
	// Namespace filters
	NamespaceInclude []string // Namespaces to include (from KUBE_NAMESPACE_INCLUDE)
	NamespaceExclude []string // Namespaces to exclude (from KUBE_NAMESPACE_EXCLUDE)
}

// Global config instance
var Global *Config

// Load initializes the configuration from environment variables
func Load() *Config {
	cfg := &Config{
		// Default values
		CPURequestMultiplier:    1.2,
		MemoryRequestMultiplier: 1.2,
		CPULimitMultiplier:      2.0,
		MemoryLimitMultiplier:   2.0,
		MaxCPULimit:             4000,
		MaxMemoryLimit:          8192,
		MinCPURequest:           10,
		MinMemoryRequest:        64,
		ResizeInterval:          30 * time.Second,
		LogLevel:                "info",
	}

	// Load from environment variables with defaults
	if val := os.Getenv("CPU_REQUEST_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.CPURequestMultiplier = f
			log.Printf("CPU_REQUEST_MULTIPLIER set to: %.2f", f)
		} else {
			log.Printf("Warning: Invalid CPU_REQUEST_MULTIPLIER value: %s", val)
		}
	}

	if val := os.Getenv("MEMORY_REQUEST_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.MemoryRequestMultiplier = f
			log.Printf("MEMORY_REQUEST_MULTIPLIER set to: %.2f", f)
		} else {
			log.Printf("Warning: Invalid MEMORY_REQUEST_MULTIPLIER value: %s", val)
		}
	}

	if val := os.Getenv("CPU_LIMIT_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.CPULimitMultiplier = f
			log.Printf("CPU_LIMIT_MULTIPLIER set to: %.2f", f)
		} else {
			log.Printf("Warning: Invalid CPU_LIMIT_MULTIPLIER value: %s", val)
		}
	}

	if val := os.Getenv("MEMORY_LIMIT_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.MemoryLimitMultiplier = f
			log.Printf("MEMORY_LIMIT_MULTIPLIER set to: %.2f", f)
		} else {
			log.Printf("Warning: Invalid MEMORY_LIMIT_MULTIPLIER value: %s", val)
		}
	}

	if val := os.Getenv("MAX_CPU_LIMIT"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MaxCPULimit = i
			log.Printf("MAX_CPU_LIMIT set to: %d millicores", i)
		} else {
			log.Printf("Warning: Invalid MAX_CPU_LIMIT value: %s", val)
		}
	}

	if val := os.Getenv("MAX_MEMORY_LIMIT"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MaxMemoryLimit = i
			log.Printf("MAX_MEMORY_LIMIT set to: %d MB", i)
		} else {
			log.Printf("Warning: Invalid MAX_MEMORY_LIMIT value: %s", val)
		}
	}

	if val := os.Getenv("MIN_CPU_REQUEST"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MinCPURequest = i
			log.Printf("MIN_CPU_REQUEST set to: %d millicores", i)
		} else {
			log.Printf("Warning: Invalid MIN_CPU_REQUEST value: %s", val)
		}
	}

	if val := os.Getenv("MIN_MEMORY_REQUEST"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MinMemoryRequest = i
			log.Printf("MIN_MEMORY_REQUEST set to: %d MB", i)
		} else {
			log.Printf("Warning: Invalid MIN_MEMORY_REQUEST value: %s", val)
		}
	}

	// Load RESIZE_INTERVAL
	if val := os.Getenv("RESIZE_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			cfg.ResizeInterval = duration
			log.Printf("RESIZE_INTERVAL set to: %v", duration)
		} else {
			// Try parsing as seconds if duration parsing fails
			if seconds, err := strconv.Atoi(val); err == nil {
				cfg.ResizeInterval = time.Duration(seconds) * time.Second
				log.Printf("RESIZE_INTERVAL set to: %v", cfg.ResizeInterval)
			} else {
				log.Printf("Warning: Invalid RESIZE_INTERVAL value: %s (use format like '30s', '5m', '1h')", val)
			}
		}
	}

	// Load LOG_LEVEL
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if validLevels[val] {
			cfg.LogLevel = val
		}
	}

	// Load KUBE_NAMESPACE_INCLUDE (CSV)
	if val := os.Getenv("KUBE_NAMESPACE_INCLUDE"); val != "" {
		cfg.NamespaceInclude = parseCSV(val)
		log.Printf("KUBE_NAMESPACE_INCLUDE set to: %v", cfg.NamespaceInclude)
	}

	// Load KUBE_NAMESPACE_EXCLUDE (CSV)
	if val := os.Getenv("KUBE_NAMESPACE_EXCLUDE"); val != "" {
		cfg.NamespaceExclude = parseCSV(val)
		log.Printf("KUBE_NAMESPACE_EXCLUDE set to: %v", cfg.NamespaceExclude)
	}

	Global = cfg
	return cfg
}

// Get returns the global config instance, loading it if necessary
func Get() *Config {
	if Global == nil {
		return Load()
	}
	return Global
}

// parseCSV splits a comma-separated string into a slice, trimming spaces
func parseCSV(s string) []string {
	var out []string
	for _, v := range splitAndTrim(s, ',') {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// splitAndTrim splits by sep and trims spaces
func splitAndTrim(s string, sep rune) []string {
	var res []string
	field := ""
	for _, c := range s {
		if c == sep {
			res = append(res, trimSpace(field))
			field = ""
		} else {
			field += string(c)
		}
	}
	res = append(res, trimSpace(field))
	return res
}

// trimSpace trims leading/trailing spaces
func trimSpace(s string) string {
	i, j := 0, len(s)-1
	for i <= j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j >= i && (s[j] == ' ' || s[j] == '\t') {
		j--
	}
	return s[i : j+1]
}
