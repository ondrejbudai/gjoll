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
