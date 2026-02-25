package remote

import (
	"testing"
)

func TestCopyValidation(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		dest    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "both remote",
			src:     ":/remote/a",
			dest:    ":/remote/b",
			wantErr: true,
			errMsg:  "both source and destination cannot be remote",
		},
		{
			name:    "neither remote",
			src:     "./local/a",
			dest:    "./local/b",
			wantErr: true,
			errMsg:  "one of source or destination must be remote (prefix with :)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Copy("/dev/null", "test", tt.src, tt.dest)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if err.Error() != tt.errMsg {
					t.Errorf("error = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
