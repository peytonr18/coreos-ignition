//go:build linux

// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	configErrors "github.com/coreos/ignition/v2/config/shared/errors"
	cfgutil "github.com/coreos/ignition/v2/config/util"
	"github.com/coreos/ignition/v2/config/v3_6_experimental/types"
	"github.com/coreos/ignition/v2/internal/log"
	"github.com/coreos/ignition/v2/internal/resource"
	"github.com/vincent-petithory/dataurl"
)

type stubFetcher struct {
	resource.Fetcher
	responses map[string][]byte
}

func newStubFetcher() *stubFetcher {
	l := log.New(true)
	return &stubFetcher{
		Fetcher:   resource.Fetcher{Logger: &l},
		responses: make(map[string][]byte),
	}
}

func (f *stubFetcher) expect(url string, payload []byte) {
	f.responses[url] = payload
}

func (f *stubFetcher) FetchToBuffer(u url.URL, opts resource.FetchOptions) ([]byte, error) {
	if data, ok := f.responses[u.String()]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("unexpected URL %s", u.String())
}

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	logger := log.New(true)
	t.Cleanup(func() {
		logger.Close()
	})
	return &logger
}

func fileByPath(t *testing.T, files []types.File, path string) *types.File {
	t.Helper()
	for i := range files {
		if files[i].Node.Path == path {
			return &files[i]
		}
	}
	t.Fatalf("file %s not found", path)
	return nil
}

func dataURLContents(t *testing.T, src string) string {
	t.Helper()
	du, err := dataurl.DecodeString(src)
	if err != nil {
		t.Fatalf("failed to decode data URL: %v", err)
	}
	return string(du.Data)
}

func TestParseProvisioningConfig(t *testing.T) {
	raw := []byte(`
<wa:ProvisioningSection xmlns:wa="http://schemas.microsoft.com/windowsazure">
  <LinuxProvisioningConfigurationSet>
    <HostName>myhost</HostName>
    <UserName>azureuser</UserName>
    <UserPassword>password</UserPassword>
    <DisableSshPasswordAuthentication>false</DisableSshPasswordAuthentication>
    <SSH>
      <PublicKeys>
        <PublicKey>
          <Value>ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCu</Value>
        </PublicKey>
      </PublicKeys>
    </SSH>
  </LinuxProvisioningConfigurationSet>
</wa:ProvisioningSection>`)

	cfg, err := parseProvisioningConfig(raw)
	if err != nil {
		t.Fatalf("parseProvisioningConfig() err = %v", err)
	}
	if cfg.UserName != "azureuser" {
		t.Fatalf("expected username azureuser, got %s", cfg.UserName)
	}
	if len(cfg.SSH.PublicKeys) != 1 {
		t.Fatalf("expected 1 ssh key, got %d", len(cfg.SSH.PublicKeys))
	}
}

func TestParseProvisioningConfigErrors(t *testing.T) {
	tests := []struct {
		name string
		xml  []byte
	}{
		{
			name: "malformed XML",
			xml: []byte(`<wa:ProvisioningSection xmlns:wa="http://schemas.microsoft.com/windowsazure">
				<LinuxProvisioningConfigurationSet>
					<UserName>testuser
				</LinuxProvisioningConfigurationSet>
			</wa:ProvisioningSection>`),
		},
		{
			name: "empty XML",
			xml:  []byte(``),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseProvisioningConfig(tt.xml)
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
		})
	}
}

func TestBuildGeneratedConfig(t *testing.T) {
	meta := &instanceMetadata{
		Compute: instanceComputeMetadata{
			Hostname: "example",
			OSProfile: instanceOSProfile{
				AdminUsername: "meta-user",
			},
			PublicKeys: []instancePublicKey{
				{KeyData: "ssh-rsa AAAAB3Nza meta"},
			},
		},
	}
	prov := &linuxProvisioningConfigurationSet{
		UserName: "prov-user",
		SSH: sshSection{
			PublicKeys: []sshPublicKey{
				{Value: "ssh-ed25519 AAAAC3Nza prov"},
			},
		},
		UserPassword: "plaintext",
	}

	cfg, err := buildGeneratedConfig(testLogger(t), meta, prov)
	if err != nil {
		t.Fatalf("buildGeneratedConfig() err = %v", err)
	}

	if len(cfg.Passwd.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(cfg.Passwd.Users))
	}
	user := cfg.Passwd.Users[0]
	if user.Name != "meta-user" {
		t.Fatalf("expected user meta-user, got %s", user.Name)
	}
	if len(user.SSHAuthorizedKeys) != 2 {
		t.Fatalf("expected 2 ssh keys, got %d", len(user.SSHAuthorizedKeys))
	}
	// Password should be hashed (starts with $6$ for SHA-512)
	if user.PasswordHash == nil {
		t.Fatalf("expected password hash to be set")
	}
	if !strings.HasPrefix(*user.PasswordHash, "$6$") {
		t.Fatalf("expected password hash to be SHA-512 (start with $6$), got %s", *user.PasswordHash)
	}

	if len(cfg.Storage.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(cfg.Storage.Files))
	}
}

func TestBuildGeneratedConfigWithPrehashedPassword(t *testing.T) {
	// Test that pre-hashed passwords are not double-hashed
	prehashedPassword := "$6$rounds=5000$saltsalt$hashedvalue"
	meta := &instanceMetadata{
		Compute: instanceComputeMetadata{
			OSProfile: instanceOSProfile{
				AdminUsername: "testuser",
			},
		},
	}
	prov := &linuxProvisioningConfigurationSet{
		UserPassword: prehashedPassword,
	}

	cfg, err := buildGeneratedConfig(testLogger(t), meta, prov)
	if err != nil {
		t.Fatalf("buildGeneratedConfig() err = %v", err)
	}

	user := cfg.Passwd.Users[0]
	if user.PasswordHash == nil || *user.PasswordHash != prehashedPassword {
		t.Fatalf("expected pre-hashed password to be preserved, got %v", user.PasswordHash)
	}
}

func TestBuildGeneratedConfigErrors(t *testing.T) {
	meta := &instanceMetadata{}
	prov := &linuxProvisioningConfigurationSet{}
	if _, err := buildGeneratedConfig(testLogger(t), meta, prov); err == nil {
		t.Fatalf("expected error when username missing")
	}
}

func TestBuildGeneratedConfigUsernamePriority(t *testing.T) {
	// Test that IMDS AdminUsername takes priority over OVF UserName
	meta := &instanceMetadata{
		Compute: instanceComputeMetadata{
			OSProfile: instanceOSProfile{
				AdminUsername: "imds-admin",
			},
		},
	}
	prov := &linuxProvisioningConfigurationSet{
		UserName: "ovf-user",
	}

	cfg, err := buildGeneratedConfig(testLogger(t), meta, prov)
	if err != nil {
		t.Fatalf("buildGeneratedConfig() err = %v", err)
	}
	if cfg.Passwd.Users[0].Name != "imds-admin" {
		t.Fatalf("expected IMDS username 'imds-admin' to take priority, got %s", cfg.Passwd.Users[0].Name)
	}
}

func TestBuildGeneratedConfigUsernameFallback(t *testing.T) {
	// Test fallback to OVF UserName when IMDS AdminUsername is empty
	meta := &instanceMetadata{
		Compute: instanceComputeMetadata{
			OSProfile: instanceOSProfile{
				AdminUsername: "",
			},
		},
	}
	prov := &linuxProvisioningConfigurationSet{
		UserName: "ovf-user",
	}

	cfg, err := buildGeneratedConfig(testLogger(t), meta, prov)
	if err != nil {
		t.Fatalf("buildGeneratedConfig() err = %v", err)
	}
	if cfg.Passwd.Users[0].Name != "ovf-user" {
		t.Fatalf("expected OVF username 'ovf-user' as fallback, got %s", cfg.Passwd.Users[0].Name)
	}
}

func TestBuildGeneratedConfigNoPassword(t *testing.T) {
	meta := &instanceMetadata{
		Compute: instanceComputeMetadata{
			OSProfile: instanceOSProfile{
				AdminUsername: "testuser",
			},
		},
	}
	prov := &linuxProvisioningConfigurationSet{}

	cfg, err := buildGeneratedConfig(testLogger(t), meta, prov)
	if err != nil {
		t.Fatalf("buildGeneratedConfig() err = %v", err)
	}
	if cfg.Passwd.Users[0].PasswordHash != nil {
		t.Fatalf("expected nil password hash when no password provided, got %v", *cfg.Passwd.Users[0].PasswordHash)
	}
}

func TestCollectSSHPublicKeysDedup(t *testing.T) {
	meta := &instanceMetadata{
		Compute: instanceComputeMetadata{
			PublicKeys: []instancePublicKey{
				{KeyData: "ssh-rsa AAAA"},
				{KeyData: "ssh-rsa AAAA"},
			},
		},
	}
	prov := &linuxProvisioningConfigurationSet{
		SSH: sshSection{
			PublicKeys: []sshPublicKey{
				{Value: "ssh-rsa BBBB"},
				{Value: "ssh-rsa AAAA"},
			},
		},
	}
	keys := collectSSHPublicKeys(meta, prov)
	if len(keys) != 2 {
		t.Fatalf("expected 2 unique keys, got %d", len(keys))
	}
}

func TestPasswordAuthDisabledParsing(t *testing.T) {
	trueCases := []string{"true", "TRUE", "1", " yes ", "YES"}
	for _, tc := range trueCases {
		prov := linuxProvisioningConfigurationSet{DisableSshPasswordAuthentication: tc}
		if !prov.passwordAuthDisabled() {
			t.Fatalf("expected %q to disable password auth", tc)
		}
	}
	falseCases := []string{"false", "0", "no", "", "NO", "False"}
	for _, tc := range falseCases {
		prov := linuxProvisioningConfigurationSet{DisableSshPasswordAuthentication: tc}
		if prov.passwordAuthDisabled() {
			t.Fatalf("expected %q to allow password auth", tc)
		}
	}
}

func TestHashPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() err = %v", err)
	}

	// Verify hash format
	if !strings.HasPrefix(hash, "$6$") {
		t.Fatalf("expected SHA-512 hash prefix $6$, got %s", hash)
	}

	// Verify hash has expected structure: $6$<salt>$<hash>
	parts := strings.Split(hash, "$")
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts in hash, got %d: %s", len(parts), hash)
	}
	if parts[1] != "6" {
		t.Fatalf("expected algorithm identifier '6', got %s", parts[1])
	}
	if len(parts[2]) != 16 {
		t.Fatalf("expected 16 character salt, got %d: %s", len(parts[2]), parts[2])
	}
	if len(parts[3]) != 86 {
		t.Fatalf("expected 86 character hash, got %d: %s", len(parts[3]), parts[3])
	}
}

func TestIsPasswordHashed(t *testing.T) {
	tests := []struct {
		password string
		expected bool
	}{
		{"$6$salt$hash", true},
		{"$5$salt$hash", true},
		{"$y$salt$hash", true},
		{"$2a$10$hash", true},
		{"$2b$10$hash", true},
		{"$2y$10$hash", true},
		{"$1$salt$hash", true},
		{"plaintext", false},
		{"$invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		result := IsPasswordHashed(tt.password)
		if result != tt.expected {
			t.Errorf("IsPasswordHashed(%q) = %v, expected %v", tt.password, result, tt.expected)
		}
	}
}

func TestApplyExtensionsNoKnobs(t *testing.T) {
	cfg := types.Config{
		Ignition: types.Ignition{
			Version: "3.6.0",
		},
	}

	fetcher := newStubFetcher()
	out, err := applyExtensions(&fetcher.Fetcher, cfg)
	if err != nil {
		t.Fatalf("applyExtensions() err = %v", err)
	}
	if !reflect.DeepEqual(cfg, out) {
		t.Fatalf("expected config to be unchanged when no knobs enabled")
	}
}

func TestApplyExtensionsKnobsEnabled(t *testing.T) {
	t.Cleanup(func() {
		fetchInstanceMetadataFunc = fetchInstanceMetadata
		readOvfEnvironmentFunc = readOvfEnvironment
	})

	fetchInstanceMetadataFunc = func(f *resource.Fetcher) (*instanceMetadata, error) {
		return &instanceMetadata{
			Compute: instanceComputeMetadata{
				OSProfile: instanceOSProfile{AdminUsername: "imds-user"},
				PublicKeys: []instancePublicKey{
					{KeyData: "ssh-rsa AAAA"},
				},
			},
		}, nil
	}
	ovf := []byte(`<wa:ProvisioningSection xmlns:wa="http://schemas.microsoft.com/windowsazure">
  <LinuxProvisioningConfigurationSet>
    <UserName>ovf-user</UserName>
    <UserPassword>password</UserPassword>
    <DisableSshPasswordAuthentication>true</DisableSshPasswordAuthentication>
    <CustomData>ZWNobyBoZWxsbwo=</CustomData>
    <SSH>
      <PublicKeys>
        <PublicKey><Value>ssh-ed25519 BBBB</Value></PublicKey>
      </PublicKeys>
    </SSH>
  </LinuxProvisioningConfigurationSet>
</wa:ProvisioningSection>`)
	readOvfEnvironmentFunc = func(f *resource.Fetcher, _ []string) ([]byte, error) {
		return ovf, nil
	}

	cfg := types.Config{
		Ignition: types.Ignition{
			Version: "3.6.0",
			Extensions: types.Extensions{
				Azure: types.AzureExtensions{
					UserEnabled:          cfgutil.BoolToPtr(true),
					SshdDropInEnabled:    cfgutil.BoolToPtr(true),
					SudoersDropInEnabled: cfgutil.BoolToPtr(true),
					ResourceDiskEnabled:  cfgutil.BoolToPtr(true),
				},
			},
		},
	}

	fetcher := newStubFetcher()
	out, err := applyExtensions(&fetcher.Fetcher, cfg)
	if err != nil {
		t.Fatalf("applyExtensions() err = %v", err)
	}

	if len(out.Passwd.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(out.Passwd.Users))
	}
	if out.Passwd.Users[0].Name != "imds-user" {
		t.Fatalf("expected username imds-user, got %s", out.Passwd.Users[0].Name)
	}

	if fileByPath(t, out.Storage.Files, azureSshdDropInPath) == nil {
		t.Fatalf("expected sshd drop-in to be created")
	}
	if fileByPath(t, out.Storage.Files, azureSudoersDropInPath) == nil {
		t.Fatalf("expected sudoers drop-in to be created")
	}
	if fileByPath(t, out.Storage.Files, "/var/lib/waagent/CustomData") == nil {
		t.Fatalf("expected provisioning CustomData file to be created")
	}

	foundUnit := false
	for _, unit := range out.Systemd.Units {
		if unit.Name == azureResourceDiskUnit {
			foundUnit = true
			break
		}
	}
	if !foundUnit {
		t.Fatalf("expected resource disk mount unit to be created")
	}
}

func TestApplyExtensionsConflict(t *testing.T) {
	t.Cleanup(func() {
		fetchInstanceMetadataFunc = fetchInstanceMetadata
		readOvfEnvironmentFunc = readOvfEnvironment
	})

	fetchInstanceMetadataFunc = func(f *resource.Fetcher) (*instanceMetadata, error) {
		return &instanceMetadata{
			Compute: instanceComputeMetadata{
				OSProfile: instanceOSProfile{AdminUsername: "imds-user"},
			},
		}, nil
	}
	readOvfEnvironmentFunc = func(f *resource.Fetcher, _ []string) ([]byte, error) {
		return nil, nil
	}

	cfg := types.Config{
		Ignition: types.Ignition{
			Version: "3.6.0",
			Extensions: types.Extensions{
				Azure: types.AzureExtensions{
					SshdDropInEnabled: cfgutil.BoolToPtr(true),
				},
			},
		},
		Storage: types.Storage{
			Files: []types.File{
				{
					Node: types.Node{Path: azureSshdDropInPath},
				},
			},
		},
	}

	fetcher := newStubFetcher()
	_, err := applyExtensions(&fetcher.Fetcher, cfg)
	if err == nil || err != configErrors.ErrInvalid {
		t.Fatalf("expected ErrInvalid for conflicting sshd drop-in, got %v", err)
	}
}
