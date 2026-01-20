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
	"github.com/coreos/ignition/v2/config/shared/errors"
	"github.com/coreos/ignition/v2/config/util"

	"github.com/coreos/vcontext/path"
	"github.com/coreos/vcontext/report"
)

const (
	// Azure-managed artifact paths
	azureSshdDropInPath    = "/etc/ssh/sshd_config.d/50-azure-cloud-sshd.conf"
	azureSudoersDropInPath = "/etc/sudoers.d/azure-cloud-sudoers.conf"
	azureResourceDiskUnit  = "mnt-resource.mount"
)

// Validate Azure extensions and check for conflicts with user configuration
func (ext AzureExtensions) Validate(c path.ContextPath) (r report.Report) {
	// No validation needed on the extensions themselves
	// All conflict checking is done at the Config level
	return
}

// ValidateAzureConflicts checks for conflicts between Azure-managed artifacts
// and user-provided configuration. The azureOsProfileAdminUser parameter should
// be populated from Azure metadata (osProfile) at runtime.
func (cfg Config) ValidateAzureConflicts(c path.ContextPath, azureOsProfileAdminUser string) (r report.Report) {
	azure := cfg.Ignition.Extensions.Azure

	// Check sshd drop-in conflicts
	if util.IsTrue(azure.SshdDropInEnabled) {
		r.Merge(cfg.checkSshdDropInConflict(c))
	}

	// Check sudoers drop-in conflicts
	if util.IsTrue(azure.SudoersDropInEnabled) {
		r.Merge(cfg.checkSudoersDropInConflict(c))
	}

	// Check resource disk conflicts
	if util.IsTrue(azure.ResourceDiskEnabled) {
		r.Merge(cfg.checkResourceDiskConflict(c))
	}

	// Check user conflicts
	if util.IsTrue(azure.UserEnabled) {
		r.Merge(cfg.checkUserConflict(c, azureOsProfileAdminUser))
	}

	return
}

// checkSshdDropInConflict checks if user has defined the Azure sshd drop-in path
func (cfg Config) checkSshdDropInConflict(c path.ContextPath) (r report.Report) {
	// Check in storage files
	for i, f := range cfg.Storage.Files {
		if f.Path == azureSshdDropInPath {
			r.AddOnError(c.Append("storage", "files", i, "path"),
				errors.ErrAzureSshdDropInConflict)
		}
	}

	// Check in storage links
	for i, l := range cfg.Storage.Links {
		if l.Path == azureSshdDropInPath {
			r.AddOnError(c.Append("storage", "links", i, "path"),
				errors.ErrAzureSshdDropInConflict)
		}
	}

	return
}

// checkSudoersDropInConflict checks if user has defined the Azure sudoers drop-in path
func (cfg Config) checkSudoersDropInConflict(c path.ContextPath) (r report.Report) {
	// Check in storage files
	for i, f := range cfg.Storage.Files {
		if f.Path == azureSudoersDropInPath {
			r.AddOnError(c.Append("storage", "files", i, "path"),
				errors.ErrAzureSudoersDropInConflict)
		}
	}

	// Check in storage links
	for i, l := range cfg.Storage.Links {
		if l.Path == azureSudoersDropInPath {
			r.AddOnError(c.Append("storage", "links", i, "path"),
				errors.ErrAzureSudoersDropInConflict)
		}
	}

	return
}

// checkResourceDiskConflict checks if user has defined the Azure resource disk mount
func (cfg Config) checkResourceDiskConflict(c path.ContextPath) (r report.Report) {
	systemdPath := "/etc/systemd/system/"
	azureUnitPath := systemdPath + azureResourceDiskUnit

	// Check in systemd units
	for i, unit := range cfg.Systemd.Units {
		if unit.Name == azureResourceDiskUnit && !util.NilOrEmpty(unit.Contents) {
			r.AddOnError(c.Append("systemd", "units", i, "name"),
				errors.ErrAzureResourceDiskConflict)
		}
	}

	// Check in storage files
	for i, f := range cfg.Storage.Files {
		if f.Path == azureUnitPath {
			r.AddOnError(c.Append("storage", "files", i, "path"),
				errors.ErrAzureResourceDiskConflict)
		}
	}

	// Check in storage links
	for i, l := range cfg.Storage.Links {
		if l.Path == azureUnitPath {
			r.AddOnError(c.Append("storage", "links", i, "path"),
				errors.ErrAzureResourceDiskConflict)
		}
	}

	// Check for /mnt/resource filesystem or mount point
	for i, fs := range cfg.Storage.Filesystems {
		if !util.NilOrEmpty(fs.Path) && *fs.Path == "/mnt/resource" {
			r.AddOnError(c.Append("storage", "filesystems", i, "path"),
				errors.ErrAzureResourceDiskConflict)
		}
	}

	return
}

// checkUserConflict checks if user has defined the Azure-managed admin user
func (cfg Config) checkUserConflict(c path.ContextPath, azureOsProfileAdminUser string) (r report.Report) {
	// When Azure userEnabled is true, Azure manages the admin user creation
	// Check if the user config conflicts with the Azure osProfile admin username
	if azureOsProfileAdminUser == "" {
		// Can't validate without knowing the username
		return
	}

	for i, user := range cfg.Passwd.Users {
		if user.Name == azureOsProfileAdminUser {
			r.AddOnError(c.Append("passwd", "users", i, "name"),
				errors.ErrAzureUserConflict)
		}
	}

	return
}
