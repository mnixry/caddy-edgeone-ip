package caddy_edgeone_ip

import (
	"os"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func TestUnmarshal_MissingArgs(t *testing.T) {
	cases := []string{
		`edgeone`,
		`edgeone zone-1`,
		`edgeone zone-1 secret-id`,
		`edgeone { }`,
	}
	for _, input := range cases {
		d := caddyfile.NewTestDispenser(input)

		r := EdgeOneIPRange{}
		if err := r.UnmarshalCaddyfile(d); err == nil {
			t.Errorf("expected error for %q, got nil", input)
		}
	}
}

func TestUnmarshal_FromEnv(t *testing.T) {
	t.Setenv("EONE_ZONE_ID", "zone-123")
	t.Setenv("EONE_SECRET_ID", "sid-456")
	t.Setenv("EONE_SECRET_KEY", "skey-789")

	raw := `edgeone ${EONE_ZONE_ID} ${EONE_SECRET_ID} ${EONE_SECRET_KEY}`
	input := os.ExpandEnv(raw)

	d := caddyfile.NewTestDispenser(input)
	r := EdgeOneIPRange{}
	if err := r.UnmarshalCaddyfile(d); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if r.ZoneID != "zone-123" {
		t.Errorf("zone id: expected %q, got %q", "zone-123", r.ZoneID)
	}
	if r.SecretID != "sid-456" {
		t.Errorf("secret id: expected %q, got %q", "sid-456", r.SecretID)
	}
	if r.SecretKey != "skey-789" {
		t.Errorf("secret key: expected %q, got %q", "skey-789", r.SecretKey)
	}
}

// Simulates being nested in another block and parses optional arguments
func TestUnmarshalNested_WithOptional(t *testing.T) {
	input := `{
		edgeone zone-1 sid-1 skey-1 {
			interval 1.5h
			timeout 30s
			api_endpoint teo.tencentcloudapi.com
		}
		other_module 10h
	}`

	d := caddyfile.NewTestDispenser(input)

	// Enter the outer block.
	d.Next()
	d.NextBlock(d.Nesting())

	r := EdgeOneIPRange{}
	if err := r.UnmarshalCaddyfile(d); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if got, want := r.Interval, caddy.Duration(90*time.Minute); got != want {
		t.Errorf("interval: expected %v, got %v", want, got)
	}
	if got, want := r.Timeout, caddy.Duration(30*time.Second); got != want {
		t.Errorf("timeout: expected %v, got %v", want, got)
	}
	if r.APIEndpoint != "teo.tencentcloudapi.com" {
		t.Errorf("api_endpoint: expected %q, got %q", "teo.tencentcloudapi.com", r.APIEndpoint)
	}

	// Ensure the dispenser cursor advanced to the next item after the block
	d.Next()
	if d.Val() != "other_module" {
		t.Errorf("cursor at unexpected position, expected 'other_module', got %v", d.Val())
	}
}
