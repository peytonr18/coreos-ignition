```mermaid
    %% ===== IGNITION BOOT FLOW =====

    %% --- Early Boot ---
    boot["Boot"] --> setup_pre["ignition-setup-pre.service"]
    setup_pre --> setup["ignition-setup.service"]
    setup --> fetch_offline["ignition-fetch-offline.service"]

    %% --- Fetch Offline Details ---
    subgraph FETCH_OFFLINE ["Ignition Fetch Offline"]
        direction TB
        offline_detect_platform["Detect platform"]
        offline_check_configs["Check configs at:"]
        offline_base_dir["/usr/lib/ignition/base.d"]
        offline_platform_dir["/usr/lib/ignition/base.platform.d/{platform}"]
        offline_detect_platform --> offline_check_configs
        offline_check_configs --> offline_base_dir
        offline_check_configs --> offline_platform_dir
        offline_check_user_ign{"/usr/lib/ignition/user.ign exists?"}
        offline_base_dir --> offline_check_user_ign
        offline_platform_dir --> offline_check_user_ign
        offline_check_user_ign -->|Yes| offline_copy_user_ign["Write to /run/ignition.json"]
        offline_check_user_ign -->|No| offline_done["Done"]
        offline_copy_user_ign --> offline_done
    end
    fetch_offline --> FETCH_OFFLINE
    
    FETCH_OFFLINE --> fetch_check{"/run/ignition.json exists?"}
    fetch_check -->|Yes, skip ignition-fetch.service| kargs_service["ignition-kargs.service"]
    fetch_check -->|No| fetch_service["ignition-fetch.service"]
    
    %% --- Fetch Service Details ---
    subgraph FETCH_ONLINE ["Ignition Fetch"]
        direction TB
        online_detect_platform["Detect platform"]
        
        %% Option 1B Injection
        o1b_check_flag{"1B: --generate-cloud-config=true?"}
        online_detect_platform --> online_check_configs & o1b_check_flag
        o1b_pull_imds["Pull data from IMDS"]
        o1b_pull_ovf["Pull data from OVF"]
        o1b_write_fragment["Write config fragment to /usr/lib/ignition/base.platform.d/azure/*"]
        
        o1b_check_flag -- No --> online_check_configs
        o1b_check_flag -- Yes --> o1b_pull_imds
        o1b_pull_imds --> o1b_pull_ovf
        o1b_pull_ovf --> o1b_write_fragment
        o1b_write_fragment --> online_check_configs
        o1b_write_fragment -.-> o1b_extensions["ignition.azure.extensions"]
        o1b_extensions -.-> o1b_write_fragment

        online_check_configs["Check configs at:"]
        online_base_dir["/usr/lib/ignition/base.d"]
        online_platform_dir["/usr/lib/ignition/base.platform.d/{platform}"]
        online_check_configs --> online_base_dir
        online_check_configs --> online_platform_dir
        online_request_cloud_configs["Request cloud specific configs"]
        online_cloud_configs_present{"Cloud configs present?"}
        online_open_config_device["Open and read config device"]
        online_check_user_ign{"/usr/lib/ignition/user.ign exists?"}
        online_base_dir --> online_check_user_ign
        online_platform_dir --> online_check_user_ign
        online_check_user_ign -->|Yes| online_copy_user_ign["Write config to /run/ignition.json"]
        online_check_user_ign -->|No| online_request_cloud_configs
        online_copy_user_ign --> online_done["Done"]
        online_request_cloud_configs --> online_cloud_configs_present
        online_cloud_configs_present -->|Yes| online_write_cloud["Write config to /run/ignition.json"]
        online_write_cloud --> online_done
        online_cloud_configs_present -->|No| online_open_config_device
        online_config_device_present{"Config present?"}
        online_open_config_device --> online_config_device_present
        online_config_device_present -->|Yes| online_write_device["Write config to /run/ignition.json"]
        online_write_device --> online_done
        online_config_device_present -->|No| online_done
    end
    fetch_service --> FETCH_ONLINE
    
    %% --- Network Stack ---
    subgraph NETWORK ["Network Stack"]
        direction TB
        networkd_service["systemd-networkd.service"]
        network_config["systemd-networkd.service - Network Configuration"]
        network_target["network.target reached"]
        
        networkd_service --> network_config --> network_target
    end
    setup --> NETWORK
    NETWORK --> FETCH_ONLINE
    NETWORK --> get_dhcp_address["Get DHCP address"]
    get_dhcp_address --> online_request_cloud_configs
    
    %% --- PPS SUPPORT ---
    subgraph PPS_SUPPORT ["PPS Support"]
        direction TB
        pps_service["azure-init-pps.service"]
        pps_output["output pps artifacts / state"]
        pps_service --> pps_output
    end

    %% --- PROPOSAL ---
    subgraph AZURE_OPTIONS["User Integration Options"]
    direction TB
        option_1a["Option 1A: New service writes config fragment to /usr/lib/ignition/base.platform.d/azure/"]
        option_1b["Option 1B: Incorporate config fragment logic in fetch.service with azure-specific knobs"]
        option_2["Option 2: Utilize azure-init / walinuxagent for user creation capabilities"]
        option_3["Option 3: Leverage afterburn for user-creation capabilities"]
        option_1a --> option_1b --> option_2 --> option_3
    end

    %% --- AZURE USER OPTIONS (Nested Subgraphs) ---
    subgraph AZURE_USER_OPTIONS ["Azure User Integration Options"]
        direction TB
        
        subgraph OPTION_1A ["1A: azure-user.service"]
            direction TB
            o1a_imds["Pull data from IMDS"]
            o1a_ovf["Pull data from OVF"]
            o1a_write_fragment["Write config to /usr/lib/ignition/base.platform.d/azure/*"]
            o1a_await["Merge config during ignition.files"]
            o1a_imds --> o1a_ovf --> o1a_write_fragment --> o1a_await
        end

        subgraph OPTION_2 ["2: azure-init / walinuxagent"]
            direction TB
            o2_azure_init["Wait for agent"]
            o2_data["Wait for OVF/IMDS to populate"]
            o2_generate_config["Generate ignition config"]
            o2_write_config["Write config to /usr/lib/ignition/base.platform.d/azure/*"]
            o2_await["Ignition resumes"]
            o2_azure_init --> o2_data --> o2_generate_config --> o2_write_config --> o2_await
        end

        subgraph OPTION_3 ["3: afterburn"]
            direction TB
            o3_afterburn["Wait for afterburn"]
            o3_data["Wait for OVF/IMDS to populate"]
            o3_generate_config["Generate ignition config"]
            o3_write_config["Write config to /usr/lib/ignition/base.platform.d/azure/*"]
            o3_await["Ignition resumes"]
            o3_afterburn --> o3_data --> o3_generate_config --> o3_write_config --> o3_await
        end
    end

    %% Cleaned up routing
    FETCH_ONLINE --> AZURE_USER_OPTIONS
    AZURE_USER_OPTIONS --> kargs_service
    
    %% --- Disk & Mount Services ---
    kargs_service -->|kargs changed| reboot_kargs["Reboot & restart from top"]
    kargs_service -->|no changes| disks_service["ignition-disks.service"]
    disks_service --> diskful_target["ignition-diskful.target reached"]
    diskful_target --> mount_service["ignition-mount.service"]
    
    %% --- Files ---
    mount_service --> files_service["ignition-files.service"]
    initrd_root_fs_target["initrd-root-fs.target"] --> afterburn_hostname_service["afterburn-hostname.service"]
    afterburn_hostname_service -.-> files_service
    
    %% --- Files Service Details ---
    subgraph FILES ["Ignition Files"]
        direction TB
        files_detect_platform["Detect platform"]
        files_check_configs["Check configs at:"]
        files_base_dir["/usr/lib/ignition/base.d"]
        files_platform_dir["/usr/lib/ignition/base.platform.d/{platform}"]
        files_detect_platform --> files_check_configs
        files_check_configs --> files_base_dir
        files_check_configs --> files_platform_dir
        files_base_dir --> files_check_run_json
        files_platform_dir --> files_check_run_json
        files_check_run_json{"/run/ignition.json exists?"}
        files_check_run_json -->|Yes| files_merge_all["Merge all detected configs and apply"]
        files_merge_all --> files_done["Done"]
        files_check_run_json -->|No| files_error["Error"]
    end
    files_service --> FILES
    
    FILES --> quench_service["ignition-quench.service"]
    quench_service --> initrd_setup_root["initrd-setup-root-after-ignition.service"]
    quench_service --> complete_target["ignition-complete.target"]
    
    %% ===== STYLING =====
    classDef service fill:#42a5f5,stroke:#1565c0,stroke-width:2px,color:#000
    classDef target fill:#ffa726,stroke:#e65100,stroke-width:2px,color:#000
    classDef azure fill:#a5d6a7,stroke:#2e7d32,stroke-width:2px,color:#000
    classDef dashed stroke-dasharray: 5 5

    %% Service and Target assignments
    class setup_pre,setup,fetch_offline,fetch_service,kargs_service,disks_service,mount_service,files_service,quench_service,initrd_setup_root,network_config,networkd_service,afterburn_hostname_service service
    class diskful_target,complete_target,network_target,initrd_root_fs_target target

    %% Azure assignments from user
    class o1b_check_flag,o1b_pull_imds,o1b_pull_ovf,o1b_write_fragment,o1b_extensions azure
    class pps_service,pps_output azure
    class option_1a,option_1b,option_2,option_3 azure
    class o1a_imds,o1a_ovf,o1a_write_fragment,o1a_await azure
    class o2_azure_init,o2_data,o2_generate_config,o2_write_config,o2_await azure
    class o3_afterburn,o3_data,o3_generate_config,o3_write_config,o3_await azure
    class o1b_extensions dashed

    %% Style subgraphs
    style AZURE_USER_OPTIONS fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,stroke-dasharray: 5 5
    style OPTION_1A fill:#c8e6c9,stroke:#2e7d32,stroke-width:1px,color:#000
    style OPTION_2 fill:#c8e6c9,stroke:#2e7d32,stroke-width:1px,color:#000
    style OPTION_3 fill:#c8e6c9,stroke:#2e7d32,stroke-width:1px,color:#000
```