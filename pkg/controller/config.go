package controller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
)

type Config struct {
	GracePeriodSeconds  int64              `json:"gracePeriodSeconds"`
	DrainTimeoutSeconds int64              `json:"drainTimeoutSeconds"`
	NamespaceSelector   *NamespaceSelector `json:"namespaceSelector,omitempty"`
}

type NamespaceSelector struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

func (ns *NamespaceSelector) Matches(namespace string) bool {
	if ns == nil {
		return true
	}

	if len(ns.Exclude) > 0 {
		for _, excluded := range ns.Exclude {
			if excluded == namespace {
				return false
			}
		}
	}

	if len(ns.Include) > 0 {
		for _, included := range ns.Include {
			if included == namespace {
				return true
			}
		}
		return false
	}

	return true
}

func NewDefaultConfig() *Config {
	return &Config{
		GracePeriodSeconds:  30,
		DrainTimeoutSeconds: 300,
		NamespaceSelector:   nil,
	}
}

func ParseConfig(configMap *corev1.ConfigMap) (*Config, error) {
	if configMap == nil {
		return nil, fmt.Errorf("configMap cannot be nil")
	}

	config := NewDefaultConfig()

	if configMap.Data == nil {
		return config, nil
	}

	if gracePeriodStr, exists := configMap.Data["gracePeriodSeconds"]; exists {
		if gracePeriod, err := strconv.ParseInt(gracePeriodStr, 10, 64); err == nil {
			if gracePeriod < 0 {
				return nil, fmt.Errorf("gracePeriodSeconds must be non-negative, got: %d", gracePeriod)
			}
			if gracePeriod > 3600 {
				return nil, fmt.Errorf("gracePeriodSeconds must be less than 3600 (1 hour), got: %d", gracePeriod)
			}
			config.GracePeriodSeconds = gracePeriod
		} else {
			return nil, fmt.Errorf("invalid gracePeriodSeconds: %v", err)
		}
	}

	if drainTimeoutStr, exists := configMap.Data["drainTimeoutSeconds"]; exists {
		if drainTimeout, err := strconv.ParseInt(drainTimeoutStr, 10, 64); err == nil {
			if drainTimeout <= 0 {
				return nil, fmt.Errorf("drainTimeoutSeconds must be positive, got: %d", drainTimeout)
			}
			if drainTimeout > 7200 {
				return nil, fmt.Errorf("drainTimeoutSeconds must be less than 7200 (2 hours), got: %d", drainTimeout)
			}
			if drainTimeout < config.GracePeriodSeconds {
				return nil, fmt.Errorf("drainTimeoutSeconds (%d) must be greater than gracePeriodSeconds (%d)", drainTimeout, config.GracePeriodSeconds)
			}
			config.DrainTimeoutSeconds = drainTimeout
		} else {
			return nil, fmt.Errorf("invalid drainTimeoutSeconds: %v", err)
		}
	}

	if namespaceSelectorStr, exists := configMap.Data["namespaceSelector"]; exists {
		var namespaceSelector NamespaceSelector
		if err := json.Unmarshal([]byte(namespaceSelectorStr), &namespaceSelector); err != nil {
			return nil, fmt.Errorf("invalid namespaceSelector JSON: %v", err)
		}
		config.NamespaceSelector = &namespaceSelector
	}

	return config, nil
}

func (c *Config) GetGracePeriod() time.Duration {
	return time.Duration(c.GracePeriodSeconds) * time.Second
}

func (c *Config) GetDrainTimeout() time.Duration {
	return time.Duration(c.DrainTimeoutSeconds) * time.Second
}
