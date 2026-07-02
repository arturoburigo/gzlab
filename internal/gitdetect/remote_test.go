package gitdetect

import "testing"

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		remote   string
		wantHost string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "scp-like ssh",
			remote:   "git@gitlab.services.betha.cloud:atendimento/protocolo/api-protocolo-cadastros.git",
			wantHost: "https://gitlab.services.betha.cloud",
			wantPath: "atendimento/protocolo/api-protocolo-cadastros",
		},
		{
			name:     "https with .git suffix",
			remote:   "https://gitlab.com/arturoburigo/gitlab-tui.git",
			wantHost: "https://gitlab.com",
			wantPath: "arturoburigo/gitlab-tui",
		},
		{
			name:     "https without .git suffix",
			remote:   "https://gitlab.com/arturoburigo/gitlab-tui",
			wantHost: "https://gitlab.com",
			wantPath: "arturoburigo/gitlab-tui",
		},
		{
			name:     "ssh:// with explicit port is dropped (SSH port, not the API port)",
			remote:   "ssh://git@gitlab.example.com:2222/team/service.git",
			wantHost: "https://gitlab.example.com",
			wantPath: "team/service",
		},
		{
			name:     "https:// with explicit port is preserved (same port as the API)",
			remote:   "https://gitlab.example.com:8443/group/project.git",
			wantHost: "https://gitlab.example.com:8443",
			wantPath: "group/project",
		},
		{
			name:     "http:// scheme is preserved, not forced to https",
			remote:   "http://gitlab.internal/team/service.git",
			wantHost: "http://gitlab.internal",
			wantPath: "team/service",
		},
		{
			name:     "nested subgroups",
			remote:   "git@gitlab.services.betha.cloud:atendimento/protocolo/cadastros/api-protocolo-cadastros-dados.git",
			wantHost: "https://gitlab.services.betha.cloud",
			wantPath: "atendimento/protocolo/cadastros/api-protocolo-cadastros-dados",
		},
		{
			name:    "empty",
			remote:  "",
			wantErr: true,
		},
		{
			name:    "no path",
			remote:  "https://gitlab.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRemoteURL(tt.remote)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRemoteURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", got.Host, tt.wantHost)
			}
			if got.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", got.Path, tt.wantPath)
			}
		})
	}
}

func TestMatchProfileByHost(t *testing.T) {
	hosts := map[string]string{
		"empresa": "https://gitlab.services.betha.cloud",
		"pessoal": "https://gitlab.com/",
	}

	if name, ok := MatchProfileByHost(hosts, "https://gitlab.services.betha.cloud"); !ok || name != "empresa" {
		t.Errorf("MatchProfileByHost() = (%q, %v), want (\"empresa\", true)", name, ok)
	}
	if name, ok := MatchProfileByHost(hosts, "https://GitLab.com"); !ok || name != "pessoal" {
		t.Errorf("MatchProfileByHost() case/slash insensitivity failed: (%q, %v)", name, ok)
	}
	if _, ok := MatchProfileByHost(hosts, "https://unknown.example.com"); ok {
		t.Error("MatchProfileByHost() matched an unconfigured host")
	}
}
