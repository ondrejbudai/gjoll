package config

import (
	"encoding/json"
	"fmt"
)

// Outputs holds the parsed values from `tofu output -json`.
type Outputs struct {
	PublicIP   string
	InstanceID string
	SSHUser    string
	InitScript string // optional
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
