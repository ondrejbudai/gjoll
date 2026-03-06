package config

import (
	"encoding/json"
	"fmt"
)

// SecretMapping defines a file to copy from the local machine to the VM.
type SecretMapping struct {
	From string
	To   string
}

// ProxyConfig defines an HTTP reverse proxy with credential injection.
type ProxyConfig struct {
	Target     string `json:"target"`       // upstream URL (e.g., https://api.anthropic.com)
	Auth       string `json:"auth"`         // "gcp" or "api-key"
	APIKeyFile string `json:"api_key_file"` // local path to API key file (for api-key auth)
	Port       int    `json:"port"`         // remote tunnel port (default 18080)
}

// Outputs holds the parsed values from `tofu output -json`.
type Outputs struct {
	PublicIP     string
	InstanceID   string
	SSHUser      string
	InitScript   string          // optional
	CloneSecrets []SecretMapping // optional
	Proxy        *ProxyConfig    // optional
}

// tofuOutput is the structure of a single output value from `tofu output -json`.
type tofuOutput struct {
	Value     any    `json:"value"`
	Type      any    `json:"type"`
	Sensitive bool   `json:"sensitive"`
}

// ParseOutputs parses the JSON output from `tofu output -json` and validates
// that all required fields are present.
func ParseOutputs(data []byte) (*Outputs, error) {
	var raw map[string]tofuOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing tofu output: %w", err)
	}

	getString := func(key string) string {
		if out, ok := raw[key]; ok {
			if s, ok := out.Value.(string); ok {
				return s
			}
		}
		return ""
	}

	o := &Outputs{
		PublicIP:   getString("public_ip"),
		InstanceID: getString("instance_id"),
		SSHUser:    getString("ssh_user"),
		InitScript: getString("init_script"),
	}

	// Parse optional clone_secrets list
	if out, ok := raw["clone_secrets"]; ok {
		if list, ok := out.Value.([]any); ok {
			for _, item := range list {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				from, _ := m["from"].(string)
				if from == "" {
					continue
				}
				to, _ := m["to"].(string)
				if to == "" {
					to = from
				}
				o.CloneSecrets = append(o.CloneSecrets, SecretMapping{From: from, To: to})
			}
		}
	}

	// Parse optional proxy config
	if out, ok := raw["proxy"]; ok {
		if m, ok := out.Value.(map[string]any); ok {
			target, _ := m["target"].(string)
			auth, _ := m["auth"].(string)
			if target != "" && auth != "" {
				proxy := &ProxyConfig{
					Target: target,
					Auth:   auth,
					Port:   18080, // default
				}
				if apiKeyFile, ok := m["api_key_file"].(string); ok {
					proxy.APIKeyFile = apiKeyFile
				}
				if port, ok := m["port"].(float64); ok {
					proxy.Port = int(port)
				}
				o.Proxy = proxy
			}
		}
	}

	var missing []string
	if o.PublicIP == "" {
		missing = append(missing, "public_ip")
	}
	if o.InstanceID == "" {
		missing = append(missing, "instance_id")
	}
	if o.SSHUser == "" {
		missing = append(missing, "ssh_user")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required outputs: %v", missing)
	}

	return o, nil
}
