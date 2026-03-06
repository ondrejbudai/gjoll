package config

import (
	"testing"
)

func TestParseOutputs(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "fedora", "type": "string"},
		"init_script": {"value": "#!/bin/bash\necho hello", "type": "string"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if o.PublicIP != "1.2.3.4" {
		t.Errorf("PublicIP = %q, want %q", o.PublicIP, "1.2.3.4")
	}
	if o.InstanceID != "i-abc123" {
		t.Errorf("InstanceID = %q, want %q", o.InstanceID, "i-abc123")
	}
	if o.SSHUser != "fedora" {
		t.Errorf("SSHUser = %q, want %q", o.SSHUser, "fedora")
	}
	if o.InitScript != "#!/bin/bash\necho hello" {
		t.Errorf("InitScript = %q, want %q", o.InitScript, "#!/bin/bash\necho hello")
	}
}

func TestParseOutputsOptionalInitScript(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if o.InitScript != "" {
		t.Errorf("InitScript = %q, want empty", o.InitScript)
	}
}

func TestParseOutputsMissingRequired(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"}
	}`)

	_, err := ParseOutputs(data)
	if err == nil {
		t.Fatal("ParseOutputs() expected error for missing fields")
	}
}

func TestParseOutputsInvalidJSON(t *testing.T) {
	_, err := ParseOutputs([]byte(`not json`))
	if err == nil {
		t.Fatal("ParseOutputs() expected error for invalid JSON")
	}
}

func TestParseOutputsCloneSecrets(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"clone_secrets": {"value": [
			{"from": "~/.ssh/id_ed25519"},
			{"from": "~/.anthropic/api_key", "to": "/opt/secrets/key"}
		], "type": ["list", "object"]}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CloneSecrets) != 2 {
		t.Fatalf("CloneSecrets len = %d, want 2", len(o.CloneSecrets))
	}
	if o.CloneSecrets[0].From != "~/.ssh/id_ed25519" || o.CloneSecrets[0].To != "~/.ssh/id_ed25519" {
		t.Errorf("CloneSecrets[0] = %+v, want from=to=~/.ssh/id_ed25519", o.CloneSecrets[0])
	}
	if o.CloneSecrets[1].From != "~/.anthropic/api_key" || o.CloneSecrets[1].To != "/opt/secrets/key" {
		t.Errorf("CloneSecrets[1] = %+v", o.CloneSecrets[1])
	}
}

func TestParseOutputsOptionalCloneSecrets(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CloneSecrets) != 0 {
		t.Errorf("CloneSecrets len = %d, want 0", len(o.CloneSecrets))
	}
}

func TestParseOutputsCloneSecretsEmpty(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"clone_secrets": {"value": [], "type": ["list", "object"]}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CloneSecrets) != 0 {
		t.Errorf("CloneSecrets len = %d, want 0", len(o.CloneSecrets))
	}
}

func TestParseOutputsCloneSecretsMissingFrom(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"clone_secrets": {"value": [
			{"to": "/some/path"},
			{"from": "", "to": "/other/path"},
			{"from": "~/.valid"}
		], "type": ["list", "object"]}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CloneSecrets) != 1 {
		t.Fatalf("CloneSecrets len = %d, want 1 (only valid entry)", len(o.CloneSecrets))
	}
	if o.CloneSecrets[0].From != "~/.valid" {
		t.Errorf("CloneSecrets[0].From = %q, want %q", o.CloneSecrets[0].From, "~/.valid")
	}
}

func TestParseOutputsCloneSecretsDefaultTo(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"clone_secrets": {"value": [
			{"from": "/etc/myconfig"}
		], "type": ["list", "object"]}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CloneSecrets) != 1 {
		t.Fatalf("CloneSecrets len = %d, want 1", len(o.CloneSecrets))
	}
	if o.CloneSecrets[0].To != "/etc/myconfig" {
		t.Errorf("To = %q, want %q (should default to From)", o.CloneSecrets[0].To, "/etc/myconfig")
	}
}

func TestParseOutputsEmptyValue(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "fedora", "type": "string"}
	}`)

	_, err := ParseOutputs(data)
	if err == nil {
		t.Fatal("ParseOutputs() expected error for empty public_ip")
	}
}

func TestParseOutputsProxyGCP(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"proxy": {"value": {
			"target": "https://us-east5-aiplatform.googleapis.com",
			"auth": "gcp",
			"port": 18080
		}, "type": "object"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if o.Proxy == nil {
		t.Fatal("Proxy = nil, want non-nil")
	}
	if o.Proxy.Target != "https://us-east5-aiplatform.googleapis.com" {
		t.Errorf("Proxy.Target = %q, want %q", o.Proxy.Target, "https://us-east5-aiplatform.googleapis.com")
	}
	if o.Proxy.Auth != "gcp" {
		t.Errorf("Proxy.Auth = %q, want %q", o.Proxy.Auth, "gcp")
	}
	if o.Proxy.Port != 18080 {
		t.Errorf("Proxy.Port = %d, want 18080", o.Proxy.Port)
	}
}

func TestParseOutputsProxyAPIKey(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"proxy": {"value": {
			"target": "https://api.anthropic.com",
			"auth": "api-key",
			"api_key_file": "~/.anthropic/api_key",
			"port": 9000
		}, "type": "object"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if o.Proxy == nil {
		t.Fatal("Proxy = nil, want non-nil")
	}
	if o.Proxy.Target != "https://api.anthropic.com" {
		t.Errorf("Proxy.Target = %q, want %q", o.Proxy.Target, "https://api.anthropic.com")
	}
	if o.Proxy.Auth != "api-key" {
		t.Errorf("Proxy.Auth = %q, want %q", o.Proxy.Auth, "api-key")
	}
	if o.Proxy.APIKeyFile != "~/.anthropic/api_key" {
		t.Errorf("Proxy.APIKeyFile = %q, want %q", o.Proxy.APIKeyFile, "~/.anthropic/api_key")
	}
	if o.Proxy.Port != 9000 {
		t.Errorf("Proxy.Port = %d, want 9000", o.Proxy.Port)
	}
}

func TestParseOutputsProxyDefaultPort(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"proxy": {"value": {
			"target": "https://api.anthropic.com",
			"auth": "gcp"
		}, "type": "object"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if o.Proxy == nil {
		t.Fatal("Proxy = nil, want non-nil")
	}
	if o.Proxy.Port != 18080 {
		t.Errorf("Proxy.Port = %d, want 18080 (default)", o.Proxy.Port)
	}
}

func TestParseOutputsOptionalProxy(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if o.Proxy != nil {
		t.Errorf("Proxy = %+v, want nil", o.Proxy)
	}
}

func TestParseOutputsProxyMissingTarget(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"proxy": {"value": {
			"auth": "gcp"
		}, "type": "object"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if o.Proxy != nil {
		t.Errorf("Proxy = %+v, want nil (missing target)", o.Proxy)
	}
}

func TestParseOutputsProxyMissingAuth(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"proxy": {"value": {
			"target": "https://api.anthropic.com"
		}, "type": "object"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if o.Proxy != nil {
		t.Errorf("Proxy = %+v, want nil (missing auth)", o.Proxy)
	}
}
