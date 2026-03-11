package plugin

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

func TestSanitizeAlloyIdentifier(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "uuid with dashes",
			in:   "fe767d4b-2a48-4a8b-add5-60b3cf26dcaa",
			want: "fe767d4b_2a48_4a8b_add5_60b3cf26dcaa",
		},
		{
			name: "starts with digit",
			in:   "123-agent",
			want: "_123_agent",
		},
		{
			name: "only invalid chars",
			in:   "---",
			want: "_",
		},
		{
			name: "trim surrounding whitespace",
			in:   "  agent id  ",
			want: "agent_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeAlloyIdentifier(tt.in)
			if got != tt.want {
				t.Fatalf("sanitizeAlloyIdentifier(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestConfigTemplateUsesSanitizedComponentID(t *testing.T) {
	tpl := template.Must(template.ParseFS(templateFS, "template/*.tpl"))

	rawID := "fe767d4b-2a48-4a8b-add5-60b3cf26dcaa"
	componentID := sanitizeAlloyIdentifier(rawID)
	data := map[string]any{
		"TenantID":    "default",
		"ID":          rawID,
		"ComponentID": componentID,
		"Features": Features{
			LinuxMonitor: true,
			JournalLog:   true,
		},
		"Credentials": map[string]string{
			"mimir_password": "mimir-secret",
			"loki_password":  "loki-secret",
		},
		"LGTMCfg": LGTMCFG{
			Mimir: serviceEndpoint{URL: "http://mimir.example.com"},
			Loki:  serviceEndpoint{URL: "http://loki.example.com"},
		},
	}

	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, configTemplateName, data); err != nil {
		t.Fatalf("ExecuteTemplate(%s): %v", configTemplateName, err)
	}
	rendered := buf.String()

	if strings.Contains(rendered, "unix_exporter_"+rawID) {
		t.Fatalf("rendered config still contains raw ID in unix exporter label: %q", rawID)
	}
	if !strings.Contains(rendered, "prometheus.exporter.unix \"unix_exporter_"+componentID+"\"") {
		t.Fatalf("rendered config is missing sanitized unix exporter label %q", componentID)
	}
	if !strings.Contains(rendered, "prometheus.remote_write.rw_"+componentID+".receiver") {
		t.Fatalf("rendered config is missing sanitized remote_write reference %q", componentID)
	}
	if !strings.Contains(rendered, "username = \""+rawID+"-mimir\"") {
		t.Fatalf("rendered config should keep raw collector ID for mimir username")
	}
	if !strings.Contains(rendered, "username = \""+rawID+"-loki\"") {
		t.Fatalf("rendered config should keep raw collector ID for loki username")
	}
}
