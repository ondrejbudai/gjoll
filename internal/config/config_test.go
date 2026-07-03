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

func TestParseOutputsCopyFiles(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"copy_files": {"value": [
			{"from": "~/.ssh/id_ed25519"},
			{"from": "~/.anthropic/api_key", "to": "/opt/secrets/key"}
		], "type": ["list", "object"]}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CopyFiles) != 2 {
		t.Fatalf("CopyFiles len = %d, want 2", len(o.CopyFiles))
	}
	if o.CopyFiles[0].From != "~/.ssh/id_ed25519" || o.CopyFiles[0].To != "~/.ssh/id_ed25519" {
		t.Errorf("CopyFiles[0] = %+v, want from=to=~/.ssh/id_ed25519", o.CopyFiles[0])
	}
	if o.CopyFiles[1].From != "~/.anthropic/api_key" || o.CopyFiles[1].To != "/opt/secrets/key" {
		t.Errorf("CopyFiles[1] = %+v", o.CopyFiles[1])
	}
}

func TestParseOutputsOptionalCopyFiles(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CopyFiles) != 0 {
		t.Errorf("CopyFiles len = %d, want 0", len(o.CopyFiles))
	}
}

func TestParseOutputsCopyFilesEmpty(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"copy_files": {"value": [], "type": ["list", "object"]}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CopyFiles) != 0 {
		t.Errorf("CopyFiles len = %d, want 0", len(o.CopyFiles))
	}
}

func TestParseOutputsCopyFilesMissingFrom(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"copy_files": {"value": [
			{"to": "/some/path"},
			{"from": "", "to": "/other/path"},
			{"from": "~/.valid"}
		], "type": ["list", "object"]}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CopyFiles) != 1 {
		t.Fatalf("CopyFiles len = %d, want 1 (only valid entry)", len(o.CopyFiles))
	}
	if o.CopyFiles[0].From != "~/.valid" {
		t.Errorf("CopyFiles[0].From = %q, want %q", o.CopyFiles[0].From, "~/.valid")
	}
}

func TestParseOutputsCopyFilesDefaultTo(t *testing.T) {
	data := []byte(`{
		"public_ip": {"value": "1.2.3.4", "type": "string"},
		"instance_id": {"value": "i-abc123", "type": "string"},
		"ssh_user": {"value": "ubuntu", "type": "string"},
		"copy_files": {"value": [
			{"from": "/etc/myconfig"}
		], "type": ["list", "object"]}
	}`)

	o, err := ParseOutputs(data)
	if err != nil {
		t.Fatalf("ParseOutputs() error: %v", err)
	}
	if len(o.CopyFiles) != 1 {
		t.Fatalf("CopyFiles len = %d, want 1", len(o.CopyFiles))
	}
	if o.CopyFiles[0].To != "/etc/myconfig" {
		t.Errorf("To = %q, want %q (should default to From)", o.CopyFiles[0].To, "/etc/myconfig")
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

func TestParseOutputsProxies(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    []ProxyConfig
		wantLen int
	}{
		{
			name: "GCP proxy",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "ubuntu", "type": "string"},
				"proxies": {"value": [
					{"name": "vertex", "target": "https://us-east5-aiplatform.googleapis.com", "auth": "gcp", "port": 18080}
				], "type": ["list", "object"]}
			}`,
			want: []ProxyConfig{
				{Name: "vertex", Target: "https://us-east5-aiplatform.googleapis.com", Auth: "gcp", Port: 18080},
			},
		},
		{
			name: "API key proxy",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "ubuntu", "type": "string"},
				"proxies": {"value": [
					{"name": "anthropic", "target": "https://api.anthropic.com", "auth": "api-key", "api_key_file": "~/.anthropic/api_key", "port": 9000}
				], "type": ["list", "object"]}
			}`,
			want: []ProxyConfig{
				{Name: "anthropic", Target: "https://api.anthropic.com", Auth: "api-key", APIKeyFile: "~/.anthropic/api_key", Port: 9000},
			},
		},
		{
			name: "no-auth proxy",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "ubuntu", "type": "string"},
				"proxies": {"value": [
					{"name": "internal", "target": "https://internal.api.com", "port": 9000}
				], "type": ["list", "object"]}
			}`,
			want: []ProxyConfig{
				{Name: "internal", Target: "https://internal.api.com", Port: 9000},
			},
		},
		{
			name: "default port",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "ubuntu", "type": "string"},
				"proxies": {"value": [
					{"name": "vertex", "target": "https://api.example.com", "auth": "gcp"}
				], "type": ["list", "object"]}
			}`,
			want: []ProxyConfig{
				{Name: "vertex", Target: "https://api.example.com", Auth: "gcp", Port: 18080},
			},
		},
		{
			name: "multiple proxies",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "ubuntu", "type": "string"},
				"proxies": {"value": [
					{"name": "vertex", "target": "https://us-east5-aiplatform.googleapis.com", "auth": "gcp", "port": 18080},
					{"name": "anthropic", "target": "https://api.anthropic.com", "auth": "api-key", "api_key_file": "~/.anthropic/api_key", "port": 18081}
				], "type": ["list", "object"]}
			}`,
			want: []ProxyConfig{
				{Name: "vertex", Target: "https://us-east5-aiplatform.googleapis.com", Auth: "gcp", Port: 18080},
				{Name: "anthropic", Target: "https://api.anthropic.com", Auth: "api-key", APIKeyFile: "~/.anthropic/api_key", Port: 18081},
			},
		},
		{
			name: "local llm string port",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "fedora", "type": "string"},
				"proxies": {"value": [
					{"name": "llm", "target": "http://127.0.0.1:11434", "port": "11434"}
				], "type": ["list", "object"]}
			}`,
			want: []ProxyConfig{
				{Name: "llm", Target: "http://127.0.0.1:11434", Port: 11434},
			},
		},
		{
			name: "no proxies output",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "ubuntu", "type": "string"}
			}`,
			wantLen: 0,
		},
		{
			name: "skip entry missing name",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "ubuntu", "type": "string"},
				"proxies": {"value": [
					{"target": "https://api.example.com", "auth": "gcp"},
					{"name": "valid", "target": "https://api.example.com"}
				], "type": ["list", "object"]}
			}`,
			want: []ProxyConfig{
				{Name: "valid", Target: "https://api.example.com", Port: 18080},
			},
		},
		{
			name: "skip entry missing target",
			data: `{
				"public_ip": {"value": "1.2.3.4", "type": "string"},
				"instance_id": {"value": "i-abc123", "type": "string"},
				"ssh_user": {"value": "ubuntu", "type": "string"},
				"proxies": {"value": [
					{"name": "bad", "auth": "gcp"}
				], "type": ["list", "object"]}
			}`,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := ParseOutputs([]byte(tt.data))
			if err != nil {
				t.Fatalf("ParseOutputs() error: %v", err)
			}

			if tt.want != nil {
				if len(o.Proxies) != len(tt.want) {
					t.Fatalf("Proxies len = %d, want %d", len(o.Proxies), len(tt.want))
				}
				for i, want := range tt.want {
					got := o.Proxies[i]
					if got.Name != want.Name {
						t.Errorf("Proxies[%d].Name = %q, want %q", i, got.Name, want.Name)
					}
					if got.Target != want.Target {
						t.Errorf("Proxies[%d].Target = %q, want %q", i, got.Target, want.Target)
					}
					if got.Auth != want.Auth {
						t.Errorf("Proxies[%d].Auth = %q, want %q", i, got.Auth, want.Auth)
					}
					if got.APIKeyFile != want.APIKeyFile {
						t.Errorf("Proxies[%d].APIKeyFile = %q, want %q", i, got.APIKeyFile, want.APIKeyFile)
					}
					if got.Port != want.Port {
						t.Errorf("Proxies[%d].Port = %d, want %d", i, got.Port, want.Port)
					}
				}
			} else {
				if len(o.Proxies) != tt.wantLen {
					t.Errorf("Proxies len = %d, want %d", len(o.Proxies), tt.wantLen)
				}
			}
		})
	}
}
