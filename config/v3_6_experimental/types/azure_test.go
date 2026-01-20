// Copyright 2026 Red Hat, Inc.
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

package types

import (
	"reflect"
	"testing"

	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"
	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

const (
	// AzureOsProfileAdminUser represents the admin username from Azure osProfile
	// In production, this would be read dynamically from Azure metadata
	AzureOsProfileAdminUser = "azureuser"
)

func TestAzureExtensionsSshdConflict(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		err    error
		at     path.ContextPath
	}{
		{
			name: "sshd drop-in enabled with conflicting file",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							SshdDropInEnabled: util.BoolToPtr(true),
						},
					},
				},
				Storage: Storage{
					Files: []File{
						{
							Node: Node{
								Path: "/etc/ssh/sshd_config.d/50-azure-cloud-sshd.conf",
							},
						},
					},
				},
			},
			err: errors.ErrAzureSshdDropInConflict,
			at:  path.New("json", "storage", "files", 0, "path"),
		},
		{
			name: "sshd drop-in enabled without conflicts",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							SshdDropInEnabled: util.BoolToPtr(true),
						},
					},
				},
				Storage: Storage{
					Files: []File{
						{
							Node: Node{
								Path: "/etc/ssh/sshd_config.d/99-user-override.conf",
							},
						},
					},
				},
			},
		},
		{
			name: "sshd drop-in disabled with same file",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							SshdDropInEnabled: util.BoolToPtr(false),
						},
					},
				},
				Storage: Storage{
					Files: []File{
						{
							Node: Node{
								Path: "/etc/ssh/sshd_config.d/50-azure-cloud-sshd.conf",
							},
						},
					},
				},
			},
		},
		{
			name: "sshd drop-in enabled with conflicting link",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							SshdDropInEnabled: util.BoolToPtr(true),
						},
					},
				},
				Storage: Storage{
					Links: []Link{
						{
							Node: Node{
								Path: "/etc/ssh/sshd_config.d/50-azure-cloud-sshd.conf",
							},
							LinkEmbedded1: LinkEmbedded1{
								Target: util.StrToPtr("/dev/null"),
							},
						},
					},
				},
			},
			err: errors.ErrAzureSshdDropInConflict,
			at:  path.New("json", "storage", "links", 0, "path"),
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.config.ValidateAzureConflicts(path.New("json"), "")
			expected := report.Report{}
			expected.AddOnError(tt.at, tt.err)
			if !reflect.DeepEqual(expected, r) {
				t.Errorf("#%d: bad report: want %v, got %v", i, expected, r)
			}
		})
	}
}

func TestAzureExtensionsSudoersConflict(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		err    error
		at     path.ContextPath
	}{
		{
			name: "sudoers drop-in enabled with conflicting file",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							SudoersDropInEnabled: util.BoolToPtr(true),
						},
					},
				},
				Storage: Storage{
					Files: []File{
						{
							Node: Node{
								Path: "/etc/sudoers.d/azure-cloud-sudoers.conf",
							},
						},
					},
				},
			},
			err: errors.ErrAzureSudoersDropInConflict,
			at:  path.New("json", "storage", "files", 0, "path"),
		},
		{
			name: "sudoers drop-in enabled without conflicts",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							SudoersDropInEnabled: util.BoolToPtr(true),
						},
					},
				},
				Storage: Storage{
					Files: []File{
						{
							Node: Node{
								Path: "/etc/sudoers.d/99-user-override",
							},
						},
					},
				},
			},
		},
		{
			name: "sudoers drop-in disabled with same file",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							SudoersDropInEnabled: util.BoolToPtr(false),
						},
					},
				},
				Storage: Storage{
					Files: []File{
						{
							Node: Node{
								Path: "/etc/sudoers.d/azure-cloud-sudoers.conf",
							},
						},
					},
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.config.ValidateAzureConflicts(path.New("json"), "")
			expected := report.Report{}
			expected.AddOnError(tt.at, tt.err)
			if !reflect.DeepEqual(expected, r) {
				t.Errorf("#%d: bad report: want %v, got %v", i, expected, r)
			}
		})
	}
}

func TestAzureExtensionsResourceDiskConflict(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		err    error
		at     path.ContextPath
	}{
		{
			name: "resource disk enabled with conflicting unit",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							ResourceDiskEnabled: util.BoolToPtr(true),
						},
					},
				},
				Systemd: Systemd{
					Units: []Unit{
						{
							Name:     "mnt-resource.mount",
							Contents: util.StrToPtr("[Mount]\nWhat=/dev/sdb1\nWhere=/mnt/resource\n"),
						},
					},
				},
			},
			err: errors.ErrAzureResourceDiskConflict,
			at:  path.New("json", "systemd", "units", 0, "name"),
		},
		{
			name: "resource disk enabled with conflicting file",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							ResourceDiskEnabled: util.BoolToPtr(true),
						},
					},
				},
				Storage: Storage{
					Files: []File{
						{
							Node: Node{
								Path: "/etc/systemd/system/mnt-resource.mount",
							},
						},
					},
				},
			},
			err: errors.ErrAzureResourceDiskConflict,
			at:  path.New("json", "storage", "files", 0, "path"),
		},
		{
			name: "resource disk enabled with conflicting filesystem",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							ResourceDiskEnabled: util.BoolToPtr(true),
						},
					},
				},
				Storage: Storage{
					Filesystems: []Filesystem{
						{
							Device: "/dev/sdb1",
							Format: util.StrToPtr("ext4"),
							Path:   util.StrToPtr("/mnt/resource"),
						},
					},
				},
			},
			err: errors.ErrAzureResourceDiskConflict,
			at:  path.New("json", "storage", "filesystems", 0, "path"),
		},
		{
			name: "resource disk enabled without conflicts",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							ResourceDiskEnabled: util.BoolToPtr(true),
						},
					},
				},
				Storage: Storage{
					Filesystems: []Filesystem{
						{
							Device: "/dev/sdc1",
							Format: util.StrToPtr("ext4"),
							Path:   util.StrToPtr("/mnt/data"),
						},
					},
				},
			},
		},
		{
			name: "resource disk disabled with same unit",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							ResourceDiskEnabled: util.BoolToPtr(false),
						},
					},
				},
				Systemd: Systemd{
					Units: []Unit{
						{
							Name:     "mnt-resource.mount",
							Contents: util.StrToPtr("[Mount]\nWhat=/dev/sdb1\nWhere=/mnt/resource\n"),
						},
					},
				},
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.config.ValidateAzureConflicts(path.New("json"), "")
			expected := report.Report{}
			expected.AddOnError(tt.at, tt.err)
			if !reflect.DeepEqual(expected, r) {
				t.Errorf("#%d: bad report: want %v, got %v", i, expected, r)
			}
		})
	}
}

func TestAzureExtensionsUserConflict(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		err    error
		at     path.ContextPath
	}{
		{
			name: "user enabled with matching adminUsername",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							UserEnabled: util.BoolToPtr(true),
						},
					},
				},
				Passwd: Passwd{
					Users: []PasswdUser{
						{
							Name: AzureOsProfileAdminUser,
						},
					},
				},
			},
			err: errors.ErrAzureUserConflict,
			at:  path.New("json", "passwd", "users", 0, "name"),
		},
		{
			name: "user enabled with different username - no conflict",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							UserEnabled: util.BoolToPtr(true),
						},
					},
				},
				Passwd: Passwd{
					Users: []PasswdUser{
						{
							Name: "myuser",
						},
					},
				},
			},
		},
		{
			name: "user disabled with matching adminUsername - no conflict",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							UserEnabled: util.BoolToPtr(false),
						},
					},
				},
				Passwd: Passwd{
					Users: []PasswdUser{
						{
							Name: AzureOsProfileAdminUser,
						},
					},
				},
			},
		},
		{
			name: "multiple users with one matching adminUsername",
			config: Config{
				Ignition: Ignition{
					Version: "3.6.0",
					Extensions: Extensions{
						Azure: AzureExtensions{
							UserEnabled: util.BoolToPtr(true),
						},
					},
				},
				Passwd: Passwd{
					Users: []PasswdUser{
						{
							Name: "user1",
						},
						{
							Name: AzureOsProfileAdminUser,
						},
						{
							Name: "user3",
						},
					},
				},
			},
			err: errors.ErrAzureUserConflict,
			at:  path.New("json", "passwd", "users", 1, "name"),
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.config.ValidateAzureConflicts(path.New("json"), AzureOsProfileAdminUser)
			expected := report.Report{}
			expected.AddOnError(tt.at, tt.err)
			if !reflect.DeepEqual(expected, r) {
				t.Errorf("#%d: bad report: want %v, got %v", i, expected, r)
			}
		})
	}
}

func TestAzureExtensionsMultipleConflicts(t *testing.T) {
	config := Config{
		Ignition: Ignition{
			Version: "3.6.0",
			Extensions: Extensions{
				Azure: AzureExtensions{
					SshdDropInEnabled:    util.BoolToPtr(true),
					SudoersDropInEnabled: util.BoolToPtr(true),
					ResourceDiskEnabled:  util.BoolToPtr(true),
				},
			},
		},
		Storage: Storage{
			Files: []File{
				{
					Node: Node{
						Path: "/etc/ssh/sshd_config.d/50-azure-cloud-sshd.conf",
					},
				},
				{
					Node: Node{
						Path: "/etc/sudoers.d/azure-cloud-sudoers.conf",
					},
				},
			},
			Filesystems: []Filesystem{
				{
					Device: "/dev/sdb1",
					Format: util.StrToPtr("ext4"),
					Path:   util.StrToPtr("/mnt/resource"),
				},
			},
		},
	}

	r := config.ValidateAzureConflicts(path.New("json"), "")

	// Should have 3 errors (sshd, sudoers, resource disk)
	if len(r.Entries) != 3 {
		t.Errorf("expected 3 errors, got %d", len(r.Entries))
	}
}
