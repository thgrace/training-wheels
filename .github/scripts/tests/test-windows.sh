# Windows (cross-platform detection)

# =============================================================================
section "Windows (cross-platform detection)"
# =============================================================================
expect_deny 'del /s /q C:\Windows\System32'            'del /s /q system32'
expect_deny 'del /s C:\Users'                          'del /s recursive'
expect_deny 'rmdir /s /q C:\Windows\System32'          'rmdir /s /q system'
expect_deny 'rmdir /s C:\Users'                        'rmdir /s recursive'
expect_deny 'Remove-Item -Recurse -Force C:\Windows'   'Remove-Item system'
expect_deny 'Remove-Item -Recurse -Force C:\Windows\System32' \
                                                       'Remove-Item system32'
expect_deny 'sc stop W32Time'                          'sc stop critical service'
expect_deny 'sc delete MyService'                      'sc delete service'
expect_deny 'Stop-Service W32Time'                     'Stop-Service critical'
expect_deny 'taskkill /F /IM svchost.exe'              'taskkill force system'
expect_deny 'diskpart'                                 'diskpart'
expect_deny 'format C:'                                'format drive'
expect_deny 'Clear-Disk -Number 0'                     'Clear-Disk'
expect_deny 'Initialize-Disk -Number 0'                'Initialize-Disk'
expect_deny 'Remove-Partition -DiskNumber 0'           'Remove-Partition'
expect_deny 'Set-ExecutionPolicy Bypass'               'Set-ExecutionPolicy Bypass'
expect_deny 'reg delete HKLM\SYSTEM /f'                'reg delete system hive'
expect_deny 'reg delete HKLM\SOFTWARE /f'              'reg delete force'
expect_deny 'reg import malicious.reg'                 'reg import'
expect_deny 'netsh advfirewall set allprofiles state off' \
                                                       'netsh firewall disable'
expect_deny 'netsh advfirewall reset'                  'netsh firewall reset'
expect_deny 'Set-NetFirewallProfile -Enabled False'    'Set-NetFirewall disable'
expect_deny 'Disable-NetAdapter -Name Ethernet'        'Disable-NetAdapter'
expect_deny 'icacls C:\Windows /grant Everyone:F /T'   'icacls grant everyone system'
expect_deny 'icacls C:\Users /grant Everyone:F'        'icacls grant everyone'
expect_deny 'icacls C:\Windows /reset /T'              'icacls reset recursive'
expect_deny 'takeown /F C:\Windows /R'                 'takeown recursive'
