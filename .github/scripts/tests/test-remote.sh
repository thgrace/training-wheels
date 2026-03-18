# Remote: SSH, rsync, SCP

# =============================================================================
section "Remote: SSH"
# =============================================================================
expect_deny 'ssh user@host "rm -rf /"'                 'ssh remote rm -rf'
expect_deny 'ssh root@prod "git reset --hard"'         'ssh remote git reset --hard'
expect_deny 'ssh user@host "git clean -fd"'            'ssh remote git clean'
expect_deny 'ssh-keygen -R host'                       'ssh-keygen remove host'
expect_deny 'ssh-add -D'                               'ssh-add delete all'
expect_deny 'ssh root@host "sudo rm -rf /var"'         'ssh remote sudo rm -rf'

# =============================================================================
section "Remote: rsync"
# =============================================================================
expect_deny 'rsync -av --delete source/ dest/'         'rsync --delete'
expect_deny 'rsync -av --del source/ dest/'            'rsync --del (short)'

# =============================================================================
section "Remote: SCP"
# =============================================================================
expect_deny 'scp -r localdir/ root@host:/'             'scp recursive to root'
expect_deny 'scp -r files/ user@host:/etc/'            'scp to /etc'
expect_deny 'scp -r files/ user@host:/boot/'           'scp to /boot'
expect_deny 'scp -r files/ user@host:/usr/'            'scp to /usr'
expect_deny 'scp -r files/ user@host:/var/'            'scp to /var'
expect_deny 'scp -r files/ user@host:/bin/'            'scp to /bin'
expect_deny 'scp -r files/ user@host:/lib/'            'scp to /lib'
