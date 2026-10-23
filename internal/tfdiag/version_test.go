package tfdiag

import (
	"errors"
	"testing"
)

func TestCheckFormatVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		version string
		want    error
		msg     string
	}{
		{"1.0", nil, ""},
		{"1.2", nil, ""},
		{"1", nil, ""},
		{"", ErrMissingVersion, "missing format_version"},
		{"2.0", ErrUnsupportedVersion, `unsupported format_version "2.0"`},
		{"10.1", ErrUnsupportedVersion, `unsupported format_version "10.1"`},
	}
	for _, tt := range tests {
		err := CheckFormatVersion(tt.version)
		if !errors.Is(err, tt.want) || (err != nil && err.Error() != tt.msg) {
			t.Errorf("CheckFormatVersion(%q) = %v, want %v with message %q", tt.version, err, tt.want, tt.msg)
		}
	}
}
