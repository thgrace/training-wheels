# Context-Aware False Positive Reduction

# =============================================================================
section "Context-Aware False Positive Reduction"
# =============================================================================
# Destructive strings in data contexts should be ALLOWED
expect_allow 'echo "DROP TABLE users"'                 'echo data context'
expect_allow 'echo "rm -rf /"'                         'echo rm -rf in quotes'
expect_allow 'git commit -m "rm -rf /"'                'commit message context'
expect_allow 'git commit -m "DROP DATABASE test"'      'commit message with SQL'
expect_allow 'grep "git reset --hard" file.txt'        'grep search pattern'
expect_allow 'grep "rm -rf" changelog.md'              'grep rm -rf in file'
expect_allow 'ls # rm -rf /'                           'comment after command'
expect_allow 'cat README.md # DROP TABLE'              'comment with SQL'
