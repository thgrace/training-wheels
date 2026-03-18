# macOS System

# =============================================================================
section "macOS System"
# =============================================================================
expect_deny 'tmutil delete /backup/2024-01-01'         'tmutil delete'
expect_deny 'diskutil eraseDisk JHFS+ Clean disk0'     'diskutil erase'
