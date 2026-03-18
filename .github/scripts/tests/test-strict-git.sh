# Strict Git

# =============================================================================
section "Strict Git (extra protections)"
# =============================================================================
expect_deny 'git push --force origin main'             'push force to main'
expect_deny 'git rebase main'                          'git rebase'
expect_deny 'git commit --amend'                       'git commit --amend'
expect_deny 'git cherry-pick abc123'                   'git cherry-pick'
expect_deny 'git filter-branch --tree-filter cmd HEAD'  'git filter-branch'
expect_deny 'git filter-repo --path src/'               'git filter-repo'
expect_deny 'git reflog expire --expire=now --all'      'git reflog expire'
expect_deny 'git gc --aggressive'                       'git gc --aggressive'
expect_deny 'git worktree remove /path/to/worktree'    'git worktree remove'
expect_deny 'git submodule deinit mymodule'             'git submodule deinit'
expect_deny 'git push origin master'                    'git push to master'
