package cmd

import (
	"reflect"
	"testing"
)

func TestReverseTunnelArgs(t *testing.T) {
	tests := []struct {
		name  string
		specs []string
		want  []string
	}{
		{
			name:  "empty",
			specs: nil,
			want:  nil,
		},
		{
			name:  "single",
			specs: []string{"8080:localhost:3000"},
			want:  []string{"-R", "8080:localhost:3000"},
		},
		{
			name:  "multiple",
			specs: []string{"8080:localhost:3000", "9090:localhost:80"},
			want:  []string{"-R", "8080:localhost:3000", "-R", "9090:localhost:80"},
		},
		{
			name:  "with bind address",
			specs: []string{"0.0.0.0:8080:localhost:3000"},
			want:  []string{"-R", "0.0.0.0:8080:localhost:3000"},
		},
		{
			name:  "dynamic port",
			specs: []string{"0:localhost:3000"},
			want:  []string{"-R", "0:localhost:3000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reverseTunnelArgs(tt.specs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reverseTunnelArgs(%v) = %v, want %v", tt.specs, got, tt.want)
			}
		})
	}
}

func TestProxyCmdHasRFlag(t *testing.T) {
	f := proxyCmd.Flags().Lookup("reverse")
	if f == nil {
		t.Fatal("proxy command missing --reverse flag")
	}
	if f.Shorthand != "R" {
		t.Errorf("proxy --reverse shorthand = %q, want %q", f.Shorthand, "R")
	}
}

func TestSSHCmdHasRFlag(t *testing.T) {
	f := sshCmd.Flags().Lookup("reverse")
	if f == nil {
		t.Fatal("ssh command missing --reverse flag")
	}
	if f.Shorthand != "R" {
		t.Errorf("ssh --reverse shorthand = %q, want %q", f.Shorthand, "R")
	}
}
