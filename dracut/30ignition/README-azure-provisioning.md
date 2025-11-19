    `# Azure Provisioning Integration - Proof of Concept

## Overview

This proof-of-concept demonstrates how to integrate Azure's provisioning metadata directly into Ignition's fetch stage. Instead of relying on external agents like WAAgent, this implementation reads user provisioning data from Azure's virtual CD-ROM during early boot and generates an Ignition configuration on-the-fly.

## How It Works

### Boot Flow

1. **Early Boot** - System boots into initrd/initramfs environment
2. **Media Mount** - Azure's virtual CD-ROM (`/dev/sr0`) is auto-mounted by the system
3. **Fetch Stage** - `ignition-fetch.service` runs
4. **Helper Script** - `ignition-fetch-userdata-helper` (ExecStartPre) executes:
   - Locates the mounted CD-ROM using `findmnt`
   - Reads and parses `ovf-env.xml`
   - Extracts user provisioning data (username, password, SSH keys)
   - Generates an Ignition 3.3.0 configuration
   - Writes it to `/run/ignition.json`
5. **Skip Normal Fetch** - The condition `ConditionPathExists=!/run/ignition.json` in the service file prevents the normal Ignition fetch from running since our helper already created the config
6. **Subsequent Stages** - Later stages (disks, mount, files) process the generated config to create users and configure the system

### Stage Order

```
fetch-offline -> fetch (with our helper) -> kargs -> disks -> mount -> files
```

## Files Modified/Created

### New Files

- **`ignition-fetch-userdata-helper.sh`** - Shell script that:
  - Finds where `/dev/sr0` is mounted
  - Parses Azure's `ovf-env.xml` 
  - Generates Ignition JSON configuration
  - Writes to `/run/ignition.json`

### Modified Files

- **`ignition-fetch.service`** - Added `ExecStartPre` to run helper script before normal fetch
- **`module-setup.sh`** - Added helper script installation and `findmnt` utility

## Azure Provisioning Media Format

On Azure, the provisioning media is:
- **Device**: Virtual CD-ROM attached as `/dev/sr0` (also visible as `ata-Virtual_CD`)
- **Filesystem**: UDF format
- **Content**: Single XML file called `ovf-env.xml`

### Expected ovf-env.xml Structure

```xml
<?xml version="1.0" encoding="utf-8"?>
<Environment xmlns="http://schemas.dmtf.org/ovf/environment/1"
             xmlns:oe="http://schemas.dmtf.org/ovf/environment/1"
             xmlns:wa="http://schemas.microsoft.com/windowsazure"
             xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <wa:ProvisioningSection>
    <wa:Version>1.0</wa:Version>
    <LinuxProvisioningConfigurationSet>
      <ConfigurationSetType>LinuxProvisioningConfiguration</ConfigurationSetType>
      <HostName>myhost</HostName>
      <UserName>azureuser</UserName>
      <UserPassword>$6$rounds=4096$abcdefgh$hashedpassword...</UserPassword>
      <DisableSshPasswordAuthentication>false</DisableSshPasswordAuthentication>
      <SSH>
        <PublicKeys>
          <PublicKey>
            <Fingerprint>...</Fingerprint>
            <Path>/home/azureuser/.ssh/authorized_keys</Path>
            <Value>ssh-rsa AAAAB3NzaC1yc2EAAAA... user@host</Value>
          </PublicKey>
        </PublicKeys>
      </SSH>
    </LinuxProvisioningConfigurationSet>
  </wa:ProvisioningSection>
</Environment>
```

### Extracted Fields

The helper script extracts only what it uses:
- **UserName** - Linux username to create
- **UserPassword** - Password hash (Azure typically provides pre-hashed)
- **SSH Keys** - Public SSH keys from `<Value>` elements

## Generated Ignition Configuration

The helper generates an Ignition 3.3.0 config that:

1. **Creates a user** with:
   - Username from `<UserName>`
   - Password hash from `<UserPassword>`
   - SSH authorized keys from `<SSH><PublicKeys>`
   - Home directory at `/home/<username>`
   - Shell set to `/bin/bash`
   - Membership in `wheel` group

2. **Configures sudo** via `/etc/sudoers.d/99_wheel_nopasswd`:
   - Allows wheel group passwordless sudo

3. **Configures SSH** via `/etc/ssh/sshd_config.d/10-custom.conf`:
   - Enables password authentication
   - Disables root login
   - Sets custom SSHD settings

### Sample Output

```json
{
  "ignition": {
    "version": "3.3.0"
  },
  "passwd": {
    "users": [
      {
        "name": "azureuser",
        "groups": ["wheel"],
        "homeDir": "/home/azureuser",
        "shell": "/bin/bash",
        "passwordHash": "$6$rounds=4096$...",
        "sshAuthorizedKeys": ["ssh-rsa AAAAB3..."]
      }
    ]
  },
  "storage": {
    "files": [
      {
        "path": "/etc/sudoers.d/99_wheel_nopasswd",
        "contents": {
          "source": "data:,%25wheel%20ALL%3D(ALL)%20NOPASSWD%3AALL%0A"
        },
        "mode": 288
      },
      {
        "path": "/etc/ssh/sshd_config.d/10-custom.conf",
        "contents": {
          "source": "data:,..."
        },
        "mode": 420
      }
    ]
  }
}
```


## References

- [Ignition Documentation](https://coreos.github.io/ignition/)
- [Ignition Configuration v3.3.0 Spec](https://coreos.github.io/ignition/configuration-v3_3/)
- [Azure Linux Provisioning](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/)
- [OVF Environment Specification](http://schemas.dmtf.org/ovf/)


