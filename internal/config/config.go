package config

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// FileMapping defines a file to copy from the local machine to the VM.
type FileMapping struct {
	From string
	To   string
}

// ProxyConfig defines an HTTP reverse proxy with optional credential injection.
type ProxyConfig struct {
	Name       string `json:"name"`         // proxy name (must be unique per instance)
	Target     string `json:"target"`       // upstream URL (e.g., https://api.anthropic.com)
	Auth       string `json:"auth"`         // "gcp", "api-key", or "" (no auth)
	APIKeyFile string `json:"api_key_file"` // local path to API key file (for api-key auth)
	Port       int    `json:"port"`         // remote tunnel port (default 18080)
}

// Outputs holds the parsed values from `tofu output -json`.
type Outputs struct {
	PublicIP   string
	InstanceID string
	SSHUser    string
	InitScript string        // optional
	CopyFiles  []FileMapping // optional
	Proxies    []ProxyConfig // optional
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
		PublicIP:    getString("public_ip"),
		InstanceID:  getString("instance_id"),
		SSHUser:     getString("ssh_user"),
		InitScript:  getString("init_script"),
	}

	// Parse optional copy_files list
	if out, ok := raw["copy_files"]; ok {
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
				o.CopyFiles = append(o.CopyFiles, FileMapping{From: from, To: to})
			}
		}
	}

	// Parse optional proxies list
	if out, ok := raw["proxies"]; ok {
		if list, ok := out.Value.([]any); ok {
			for _, item := range list {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				name, _ := m["name"].(string)
				target, _ := m["target"].(string)
				if name == "" || target == "" {
					continue
				}
				p := ProxyConfig{
					Name:   name,
					Target: target,
					Port:   18080, // default
				}
				if auth, ok := m["auth"].(string); ok {
					p.Auth = auth
				}
				if apiKeyFile, ok := m["api_key_file"].(string); ok {
					p.APIKeyFile = apiKeyFile
				}
				switch v := m["port"].(type) {
				case float64:
					p.Port = int(v)
				case string:
					if n, err := strconv.Atoi(v); err == nil {
						p.Port = n
					}
				}
				o.Proxies = append(o.Proxies, p)
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
