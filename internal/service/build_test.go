package service_test

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
	controlapi "github.com/moby/buildkit/api/services/control"

	"github.com/rhajizada/cradle/internal/service"
)

func TestFilterEmpty(t *testing.T) {
	out := service.FilterEmpty([]string{"", "a", " ", "b"})
	if len(out) != 2 || out[0] != "a" || out[1] != "b" {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestStatusEmoji(t *testing.T) {
	if service.StatusEmoji("Pulling") != "📥" {
		t.Fatalf("unexpected emoji for Pulling")
	}
	if service.StatusEmoji("Digest") != "🔍" {
		t.Fatalf("unexpected emoji for Digest")
	}
	if service.StatusEmoji("Status") != "✅" {
		t.Fatalf("unexpected emoji for Status")
	}
	if service.StatusEmoji("Other") != "🔧" {
		t.Fatalf("unexpected default emoji")
	}
}

func TestLabelFor(t *testing.T) {
	if service.LabelFor("", "Status") != "" {
		t.Fatalf("expected empty label for empty id")
	}
	if service.LabelFor("123", "Pulling from") != "" {
		t.Fatalf("expected empty label for numeric id with pulling")
	}
	if service.LabelFor("a1b2c3d4e5f6", "Status") != "layer a1b2c3d4e5f6:" {
		t.Fatalf("unexpected layer label")
	}
	if service.LabelFor("abc", "Status") != "abc:" {
		t.Fatalf("unexpected generic label")
	}
}

func TestLooksNumeric(t *testing.T) {
	if !service.LooksNumeric("123") || service.LooksNumeric("12a") || service.LooksNumeric("") {
		t.Fatalf("looksNumeric behavior unexpected")
	}
}

func TestLooksLayerID(t *testing.T) {
	if !service.LooksLayerID("a1b2c3d4e5f6") {
		t.Fatalf("expected layer id")
	}
	if service.LooksLayerID("short") || service.LooksLayerID("nothexzzzzzz") {
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

	lines, ok := service.DecodeBuildkitTrace(data)
	if !ok || len(lines) == 0 {
		t.Fatalf("expected decoded lines")
	}

	joined := strings.Join(lines, " ")
	if !strings.Contains(joined, "step 1") || !strings.Contains(joined, "hello") || !strings.Contains(joined, "warn") {
		t.Fatalf("unexpected decoded lines: %v", lines)
	}
}

func TestParsePlatformList(t *testing.T) {
	plats, err := service.ParsePlatformList([]string{"linux/amd64", "linux/arm64"})
	if err != nil {
		t.Fatalf("parsePlatformList error: %v", err)
	}
	if len(plats) != 2 {
		t.Fatalf("expected 2 platforms")
	}
}

func TestParsePlatformListInvalid(t *testing.T) {
	if _, err := service.ParsePlatformList([]string{"bad"}); err == nil {
		t.Fatalf("expected error for invalid platform")
	}
}

func TestOutputStyle(_ *testing.T) {
	style := service.OutputStyle(io.Discard)
	_ = style.Line("x", "", "a", "")
}

func TestOutStyleFormatting(t *testing.T) {
	s := service.OutStyle{Color: false}
	if got := s.Prefixed("x"); got != "x" {
		t.Fatalf("unexpected prefixed output: %q", got)
	}
	if got := s.Line("e", "", "a", ""); got != "e a\n" {
		t.Fatalf("unexpected line output: %q", got)
	}

	s = service.OutStyle{Color: true}
	if got := s.Prefixed("x"); !strings.Contains(got, "x") {
		t.Fatalf("expected prefixed to contain text")
	}
	if got := s.Line("e", "green", "a"); !strings.Contains(got, "a") {
		t.Fatalf("expected colored line to contain text")
	}
}
