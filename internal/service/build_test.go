package service

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	controlapi "github.com/moby/buildkit/api/services/control"
)

func TestFilterEmpty(t *testing.T) {
	out := filterEmpty([]string{"", "a", " ", "b"})
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestStatusEmoji(t *testing.T) {
	if statusEmoji("Pulling") != "📥" {
		t.Fatalf("unexpected emoji for Pulling")
	}
	if statusEmoji("Digest") != "🔍" {
		t.Fatalf("unexpected emoji for Digest")
	}
	if statusEmoji("Status") != "✅" {
		t.Fatalf("unexpected emoji for Status")
	}
	if statusEmoji("Other") != "🔧" {
		t.Fatalf("unexpected default emoji")
	}
}

func TestLabelFor(t *testing.T) {
	if labelFor("", "Status") != "" {
		t.Fatalf("expected empty label for empty id")
	}
	if labelFor("123", "Pulling from") != "" {
		t.Fatalf("expected empty label for numeric id with pulling")
	}
	if labelFor("a1b2c3d4e5f6", "Status") != "layer a1b2c3d4e5f6:" {
		t.Fatalf("unexpected layer label")
	}
	if labelFor("abc", "Status") != "abc:" {
		t.Fatalf("unexpected generic label")
	}
}

func TestLooksNumeric(t *testing.T) {
	if !looksNumeric("123") || looksNumeric("12a") || looksNumeric("") {
		t.Fatalf("looksNumeric behavior unexpected")
	}
}

func TestLooksLayerID(t *testing.T) {
	if !looksLayerID("a1b2c3d4e5f6") {
		t.Fatalf("expected layer id")
	}
	if looksLayerID("short") || looksLayerID("nothexzzzzzz") {
		t.Fatalf("expected invalid layer id")
	}
}

func TestDecodeBuildkitTrace(t *testing.T) {
	sr := &controlapi.StatusResponse{
		Vertexes: []*controlapi.Vertex{
			{Name: "step 1"},
		},
		Logs: []*controlapi.VertexLog{
			{Msg: []byte("hello\nworld\n")},
		},
		Warnings: []*controlapi.VertexWarning{
			{Short: []byte("warn")},
		},
	}

	raw, err := proto.Marshal(sr)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal raw: %v", err)
	}

	lines, ok := decodeBuildkitTrace(data)
	if !ok || len(lines) == 0 {
		t.Fatalf("expected decoded lines")
	}

	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "step 1") || !strings.Contains(joined, "hello") || !strings.Contains(joined, "warn") {
		t.Fatalf("unexpected decoded lines: %v", lines)
	}
}

func TestParsePlatformList(t *testing.T) {
	plats, err := parsePlatformList([]string{"linux/amd64", "linux/arm64"})
	if err != nil {
		t.Fatalf("parsePlatformList error: %v", err)
	}
	if len(plats) != 2 {
		t.Fatalf("expected 2 platforms")
	}
}

func TestParsePlatformListInvalid(t *testing.T) {
	if _, err := parsePlatformList([]string{"bad"}); err == nil {
		t.Fatalf("expected error for invalid platform")
	}
}

func TestOutputStyle(t *testing.T) {
	style := outputStyle(io.Discard)
	_ = style.line("x", "", "a", "")
}

func TestOutStyleFormatting(t *testing.T) {
	s := outStyle{color: false}
	if got := s.prefixed("x"); got != "x" {
		t.Fatalf("unexpected prefixed output: %q", got)
	}
	if got := s.line("e", "", "a", ""); got != "e a\n" {
		t.Fatalf("unexpected line output: %q", got)
	}

	s = outStyle{color: true}
	if got := s.prefixed("x"); !strings.Contains(got, "x") {
		t.Fatalf("expected prefixed to contain text")
	}
	if got := s.line("e", colorGreen, "a"); !strings.Contains(got, "a") {
		t.Fatalf("expected colored line to contain text")
	}
}
