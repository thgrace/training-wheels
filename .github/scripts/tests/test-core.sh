# Core: Git and Core: Filesystem

# =============================================================================
section "Core: Git"
# =============================================================================
expect_deny 'git reset --hard'                        'git reset --hard'
expect_deny 'git reset --hard HEAD'                   'git reset --hard HEAD'
expect_deny 'git reset --hard HEAD~3'                 'git reset --hard HEAD~3'
expect_deny 'git reset --hard origin/main'            'git reset --hard origin/main'
expect_deny 'git reset --merge'                       'git reset --merge'
expect_deny 'git push --force'                        'git push --force'
expect_deny 'git push -f'                             'git push -f'
expect_deny 'git push origin main --force'            'git push origin main --force'
expect_deny 'git push -f origin feature'              'git push -f origin feature'
expect_deny 'git push --mirror'                       'git push --mirror'
expect_deny 'git clean -f'                            'git clean -f'
expect_deny 'git clean -fd'                           'git clean -fd'
expect_deny 'git clean -fdx'                          'git clean -fdx'
expect_deny 'git checkout -- .'                       'git checkout -- .'
expect_deny 'git checkout -- file.txt'                'git checkout -- file.txt'
expect_deny 'git checkout HEAD -- src/'               'git checkout HEAD -- src/'
expect_allow 'git restore file.txt'                    'git restore file.txt (specific file is safe)'
expect_deny 'git restore --worktree file.txt'         'git restore --worktree file.txt'
expect_deny 'git branch -D feature-branch'            'git branch -D feature-branch'
expect_deny 'git stash drop'                          'git stash drop'
expect_deny 'git stash drop stash@{0}'                'git stash drop stash@{0}'
expect_deny 'git stash clear'                         'git stash clear'

# Git safe commands (must allow)
expect_allow 'git status'                             'git status'
expect_allow 'git log --oneline'                      'git log --oneline'
expect_allow 'git diff'                               'git diff'
expect_allow 'git add .'                              'git add .'
expect_allow 'git commit -m "fix: typo"'              'git commit -m "fix: typo"'
expect_allow 'git push origin main'                   'git push origin main'
expect_allow 'git pull origin main'                   'git pull origin main'
expect_allow 'git fetch --all'                        'git fetch --all'
expect_allow 'git checkout -b new-branch'             'git checkout -b new-branch'
expect_allow 'git branch -d merged-branch'            'git branch -d merged-branch'
expect_allow 'git stash'                              'git stash'
expect_allow 'git stash list'                         'git stash list'
expect_allow 'git stash pop'                          'git stash pop'
expect_allow 'git tag v1.0.0'                         'git tag v1.0.0'
expect_allow 'git clean -n'                           'git clean -n (dry run)'
expect_allow 'git clean --dry-run'                    'git clean --dry-run'

# =============================================================================
section "Core: Filesystem"
# =============================================================================
expect_deny 'rm -rf /'                                'rm -rf /'
expect_deny 'rm -rf /home'                            'rm -rf /home'
expect_deny 'rm -rf /etc'                             'rm -rf /etc'
expect_deny 'rm -rf /var'                             'rm -rf /var'
expect_deny 'rm -rf /usr'                             'rm -rf /usr'
expect_deny 'rm -rf ./src'                            'rm -rf ./src'
expect_deny 'rm -rf ./build'                          'rm -rf ./build'
expect_deny 'rm -r -f /var/log'                       'rm -r -f /var/log'
expect_deny 'rm --recursive --force /opt'             'rm --recursive --force /opt'
expect_deny 'rm -r /important-data'                   'rm -r /important-data'
expect_deny 'rm -fr /'                                'rm -fr / (flag reorder)'

# Filesystem safe commands
expect_allow 'rm -rf /tmp/build'                      'rm -rf /tmp/build (safe temp path)'
expect_allow 'rm -rf /tmp/mydir'                      'rm -rf /tmp/mydir (safe temp path)'
expect_allow 'ls -la'                                 'ls -la'
expect_allow 'cat file.txt'                           'cat file.txt'
expect_allow 'mkdir -p ./build'                       'mkdir -p ./build'
expect_allow 'cp -r src/ dst/'                        'cp -r src/ dst/'
expect_allow 'mv old.txt new.txt'                     'mv old.txt new.txt'
