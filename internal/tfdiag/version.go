package tfdiag

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrMissingVersion     = errors.New("missing format_version")
	ErrUnsupportedVersion = errors.New("unsupported format_version")
)

func CheckFormatVersion(v string) error {
	if v == "" {
		return ErrMissingVersion
	}
	if major, _, _ := strings.Cut(v, "."); major != "1" {
		return fmt.Errorf("%w %q", ErrUnsupportedVersion, v)
	}
	return nil
}
