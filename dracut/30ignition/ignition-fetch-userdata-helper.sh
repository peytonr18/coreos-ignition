#!/bin/bash
# -*- mode: shell-script; indent-tabs-mode: nil; sh-basic-offset: 4; -*-
# ex: ts=8 sw=4 sts=4 et filetype=sh
#
# ignition-fetch-userdata-helper.sh
# Reads Azure provisioning media (ovf-env.xml) and generates Ignition config

set -euo pipefail

CDROM_DEV="/dev/sr0"
OVF_FILE="ovf-env.xml"
OUTPUT_CONFIG="/run/ignition.json"

log() {
    echo "ignition-fetch-userdata-helper: $*" >&2
}

# Function to extract value from XML using basic text processing
# Usage: extract_xml_value "ElementName"
extract_xml_value() {
    local element="$1"
    local xml_content="$2"
    echo "$xml_content" | grep -oP "(?<=<${element}>)[^<]+" | head -1 || echo ""
}

# Function to extract value with namespace prefix
extract_ns_xml_value() {
    local element="$1"
    local xml_content="$2"
    echo "$xml_content" | grep -oP "(?<=<[^:]*:${element}>)[^<]+" | head -1 || echo ""
}

main() {
    log "Starting Azure provisioning media fetch..."

    if [[ ! -b "$CDROM_DEV" ]]; then
        log "CD-ROM device $CDROM_DEV not found, skipping..."
        exit 0
    fi

    # Find where /dev/sr0 is mounted (assume it's already mounted)
    mount_dir=$(findmnt -n -o TARGET --source "$CDROM_DEV" 2>/dev/null | head -1)
    
    if [[ -z "$mount_dir" ]]; then
        log "CD-ROM device $CDROM_DEV is not mounted, skipping..."
        exit 0
    fi

    log "CD-ROM mounted at $mount_dir"

    # Check if ovf-env.xml exists
    if [[ ! -f "$mount_dir/$OVF_FILE" ]]; then
        log "ovf-env.xml not found at $mount_dir/$OVF_FILE, skipping..."
        exit 0
    fi

    log "Found $OVF_FILE, parsing..."

    xml_content=$(cat "$mount_dir/$OVF_FILE")

    # Extract user data from LinuxProvisioningConfigurationSet
    username=$(extract_ns_xml_value "UserName" "$xml_content")
    userpassword=$(extract_ns_xml_value "UserPassword" "$xml_content")
    ssh_keys=$(echo "$xml_content" | grep -oP '(?<=<Value>)[^<]+' | grep '^ssh-' || echo "")

    log "Extracted username: ${username:-<none>}"
    log "Extracted password: ${userpassword:+<present>}"
    log "Extracted SSH keys: ${ssh_keys:+<present>}"

    if [[ -z "$username" ]]; then
        log "No username found in provisioning data, skipping config generation..."
        exit 0
    fi

    # Generate password hash if we have a plain password
    password_hash="$userpassword"

    # Build SSH authorized keys JSON array
    ssh_keys_json="[]"
    if [[ -n "$ssh_keys" ]]; then
        ssh_keys_json="["
        first=true
        while IFS= read -r key; do
            [[ -z "$key" ]] && continue
            if [[ "$first" == "true" ]]; then
                first=false
            else
                ssh_keys_json+=","
            fi
            escaped_key=$(echo "$key" | sed 's/"/\\"/g')
            ssh_keys_json+="\"$escaped_key\""
        done <<< "$ssh_keys"
        ssh_keys_json+="]"
    fi

    log "Generating Ignition config..."

    cat > "$OUTPUT_CONFIG" <<EOF
{
  "ignition": {
    "version": "3.3.0"
  },
  "passwd": {
    "users": [
      {
        "name": "$username",
        "groups": ["wheel"],
        "homeDir": "/home/$username",
        "shell": "/bin/bash"$(
if [[ -n "$password_hash" ]]; then
    echo ","
    echo "        \"passwordHash\": \"$password_hash\""
fi
)$(
if [[ "$ssh_keys_json" != "[]" ]]; then
    echo ","
    echo "        \"sshAuthorizedKeys\": $ssh_keys_json"
fi
)
      }
    ]
  },
  "storage": {
    "files": [
      {
        "path": "/etc/sudoers.d/99_wheel_nopasswd",
        "contents": {
          "compression": "",
          "source": "data:,%25wheel%20ALL%3D(ALL)%20NOPASSWD%3AALL%0A"
        },
        "mode": 288
      },
      {
        "path": "/etc/ssh/sshd_config.d/10-custom.conf",
        "contents": {
          "compression": "",
          "source": "data:,%23%20Custom%20SSHD%20settings%0APasswordAuthentication%20yes%0APermitRootLogin%20no%0A"
        },
        "mode": 420
      }
    ]
  }
}
EOF

    log "Ignition config written to $OUTPUT_CONFIG"
    log "Config will be processed by subsequent Ignition stages"
    
    exit 0
}

# Run main function
main "$@"

