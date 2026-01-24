package service

import (
	"fmt"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const platformVariantParts = 3

func ParsePlatform(s string) (*ocispec.Platform, error) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 || len(parts) > platformVariantParts {
		return nil, fmt.Errorf("invalid platform %q (expected os/arch[/variant])", s)
	}
	p := &ocispec.Platform{
		OS:           parts[0],
		Architecture: parts[1],
	}
	if len(parts) == platformVariantParts {
		p.Variant = parts[2]
	}
	return p, nil
}
