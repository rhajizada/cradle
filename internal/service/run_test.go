package service

import (
	"testing"

	"github.com/rhajizada/cradle/internal/config"

	mobynet "github.com/moby/moby/api/types/network"
)

func TestParsePorts(t *testing.T) {
	exposed, bindings, err := parsePorts([]string{
		"80",
		"8080:80",
		"127.0.0.1:2222:22",
		"[::1]:2223:22",
	})
	if err != nil {
		t.Fatalf("parsePorts error: %v", err)
	}

	p80, _ := mobynet.ParsePort("80")
	p22, _ := mobynet.ParsePort("22")

	if _, ok := exposed[p80]; !ok {
		t.Fatalf("expected port 80 exposed")
	}
	if _, ok := exposed[p22]; !ok {
		t.Fatalf("expected port 22 exposed")
	}

	if len(bindings[p80]) != 1 || bindings[p80][0].HostPort != "8080" {
		t.Fatalf("expected port 80 to map to host 8080")
	}
	if len(bindings[p22]) != 2 {
		t.Fatalf("expected two bindings for port 22")
	}
	if bindings[p22][0].HostPort != "2222" {
		t.Fatalf("expected host port 2222")
	}
	if bindings[p22][1].HostPort != "2223" {
		t.Fatalf("expected host port 2223")
	}
}

func TestParsePortsInvalid(t *testing.T) {
	if _, _, err := parsePorts([]string{"bad:format:too:many"}); err == nil {
		t.Fatalf("expected error for invalid port mapping")
	}
}

func TestRunFingerprintDeterministic(t *testing.T) {
	run := config.RunSpec{
		Env: map[string]string{
			"B": "2",
			"A": "1",
		},
		Cmd: []string{"sh", "-lc", "echo ok"},
	}

	first, err := runFingerprint("alias", "name", "img:tag", "imgid", run, true, true, false)
	if err != nil {
		t.Fatalf("runFingerprint error: %v", err)
	}
	second, err := runFingerprint("alias", "name", "img:tag", "imgid", run, true, true, false)
	if err != nil {
		t.Fatalf("runFingerprint error: %v", err)
	}
	if first != second {
		t.Fatalf("expected deterministic fingerprint")
	}

	run.Env["A"] = "changed"
	third, err := runFingerprint("alias", "name", "img:tag", "imgid", run, true, true, false)
	if err != nil {
		t.Fatalf("runFingerprint error: %v", err)
	}
	if first == third {
		t.Fatalf("expected fingerprint to change when env changes")
	}
}
