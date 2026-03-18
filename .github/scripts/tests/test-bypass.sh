# Bypass Vectors: Wrappers AND Bypass Vectors: Chaining & Subshells

# =============================================================================
section "Bypass Vectors: Wrappers"
# =============================================================================
# sudo wrapper
expect_deny 'sudo rm -rf /'                           'sudo rm -rf /'
expect_deny 'sudo git reset --hard'                    'sudo git reset --hard'
expect_deny 'sudo dd if=/dev/zero of=/dev/sda'         'sudo dd to device'
expect_deny 'sudo terraform destroy'                   'sudo terraform destroy'

# env wrapper
expect_deny 'env rm -rf /'                             'env rm -rf /'
expect_deny 'env git reset --hard'                     'env git reset --hard'

# absolute path
expect_deny '/usr/bin/git reset --hard'                '/usr/bin/git reset --hard'
expect_deny '/bin/rm -rf /'                            '/bin/rm -rf /'

# .exe suffix
expect_deny 'git.exe reset --hard'                     'git.exe reset --hard'

# quoted binary
expect_deny '"git" reset --hard'                       '"git" reset --hard'

# line continuation
expect_deny "$(printf 'git re\\\nset --hard')"         'line continuation bypass'

# path traversal in safe paths
expect_deny 'rm -rf /tmp/../etc'                       'path traversal /tmp/../etc'

# =============================================================================
section "Bypass Vectors: Chaining & Subshells"
# =============================================================================
expect_deny 'echo hello && rm -rf /'                   'chained && destructive'
expect_deny 'ls; rm -rf /'                             'chained ; destructive'
expect_deny 'true || rm -rf /'                         'chained || destructive'
expect_deny 'bash -c "rm -rf /"'                       'bash -c destructive'
expect_deny 'sh -c "rm -rf /"'                         'sh -c destructive'
