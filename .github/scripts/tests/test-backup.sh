# Backup: Borg, Restic, rclone, Velero

# =============================================================================
section "Backup: Borg"
# =============================================================================
expect_deny 'borg delete /repo::archive'               'borg delete'
expect_deny 'borg prune /repo'                         'borg prune'
expect_deny 'borg compact /repo'                       'borg compact'
expect_deny 'borg recreate /repo::archive'             'borg recreate'
expect_deny 'borg break-lock /repo'                    'borg break-lock'

# =============================================================================
section "Backup: Restic"
# =============================================================================
expect_deny 'restic forget latest'                     'restic forget'
expect_deny 'restic prune'                             'restic prune'
expect_deny 'restic key remove 3'                      'restic key remove'
expect_deny 'restic unlock --remove-all'               'restic unlock --remove-all'
expect_deny 'restic cache --cleanup'                   'restic cache cleanup'

# =============================================================================
section "Backup: rclone"
# =============================================================================
expect_deny 'rclone sync source: dest:'                'rclone sync'
expect_deny 'rclone delete remote:bucket'              'rclone delete'
expect_deny 'rclone deletefile remote:bucket/file.txt' 'rclone deletefile'
expect_deny 'rclone purge remote:bucket'               'rclone purge'
expect_deny 'rclone cleanup remote:'                   'rclone cleanup'
expect_deny 'rclone dedupe remote:bucket'              'rclone dedupe'
expect_deny 'rclone move /local remote:bucket'         'rclone move'

# =============================================================================
section "Backup: Velero"
# =============================================================================
expect_deny 'velero backup delete mybackup'            'velero backup delete'
expect_deny 'velero schedule delete myschedule'        'velero schedule delete'
expect_deny 'velero restore delete myrestore'          'velero restore delete'
expect_deny 'velero backup-location delete mylocation' 'velero backup-location delete'
expect_deny 'velero snapshot-location delete myloc'    'velero snapshot-location delete'
expect_deny 'velero uninstall'                         'velero uninstall'
