package secrets

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type SecretManager struct {
	secrets map[string]string
	logger  *logrus.Logger
	mutex   sync.RWMutex
	
	// For log masking
	secretValues []string
}

func NewSecretManager(logger *logrus.Logger) *SecretManager {
	return &SecretManager{
		secrets:      make(map[string]string),
		logger:       logger,
		secretValues: make([]string, 0),
	}
}

// LoadSecretsFromFile loads secrets from a .env file
func (sm *SecretManager) LoadSecretsFromFile(filename string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.logger.Debugf("Loading secrets from file: %s", filename)
	
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open secrets file %s: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			sm.logger.Warnf("Invalid format at line %d in %s: %s", lineNum, filename, line)
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
		   (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}
		
		sm.secrets[key] = value
		sm.secretValues = append(sm.secretValues, value)
		
		sm.logger.Debugf("Loaded secret: %s", key)
	}
	
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading secrets file: %w", err)
	}
	
	sm.logger.Infof("Loaded %d secrets from %s", len(sm.secrets), filename)
	return nil
}

// SetSecret sets a secret value
func (sm *SecretManager) SetSecret(key, value string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	sm.secrets[key] = value
	sm.secretValues = append(sm.secretValues, value)
	sm.logger.Debugf("Secret set: %s", key)
}

// GetSecret returns a secret value
func (sm *SecretManager) GetSecret(key string) (string, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	value, exists := sm.secrets[key]
	return value, exists
}

// GetSecrets returns a copy of all secrets
func (sm *SecretManager) GetSecrets() map[string]string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	result := make(map[string]string)
	for k, v := range sm.secrets {
		result[k] = v
	}
	return result
}

// ValidateRequiredSecrets checks if all required secrets are available
func (sm *SecretManager) ValidateRequiredSecrets(requiredSecrets []string) error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	var missing []string
	for _, secret := range requiredSecrets {
		if _, exists := sm.secrets[secret]; !exists {
			missing = append(missing, secret)
		}
	}
	
	if len(missing) > 0 {
		return fmt.Errorf("missing required secrets: %v", missing)
	}
	
	return nil
}

// MaskSecretsInString masks secret values in a string for logging
func (sm *SecretManager) MaskSecretsInString(input string) string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	result := input
	for _, secretValue := range sm.secretValues {
		if secretValue != "" && len(secretValue) > 0 {
			// Replace secret values with [MASKED]
			result = strings.ReplaceAll(result, secretValue, "[MASKED]")
		}
	}
	
	// Also mask common secret patterns
	patterns := []string{
		`(?i)password[=:\s]+[^\s]+`,
		`(?i)key[=:\s]+[^\s]+`,
		`(?i)token[=:\s]+[^\s]+`,
		`(?i)secret[=:\s]+[^\s]+`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "[MASKED]")
	}
	
	return result
}

// ListSecretKeys returns the names of all loaded secrets (not values)
func (sm *SecretManager) ListSecretKeys() []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	keys := make([]string, 0, len(sm.secrets))
	for key := range sm.secrets {
		keys = append(keys, key)
	}
	return keys
}