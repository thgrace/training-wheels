# Public-safe policy regression suite aligned with the private policy Docker tests.

# =============================================================================
section "Policy: Core Git"
# =============================================================================
expect_deny 'git reset --hard'                        'git reset --hard'
expect_deny 'git push --force'                        'git push --force'
expect_deny 'git clean -f'                            'git clean -f'
expect_allow 'git status'                             'git status'
expect_allow 'git diff HEAD'                          'git diff HEAD'
expect_allow 'git commit -m "test"'                   'git commit'

# =============================================================================
section "Policy: Core Filesystem"
# =============================================================================
expect_deny 'rm -rf /'                                'rm -rf /'
expect_deny 'rm -rf ~'                                'rm -rf ~'
expect_deny 'rm -rf $HOME'                            'rm -rf $HOME'
expect_allow 'ls -la'                                 'ls -la'
expect_allow 'cat README.md'                          'cat README.md'
expect_allow 'mkdir test_dir'                         'mkdir test_dir'

# =============================================================================
section "Policy: Wrappers and Compounds"
# =============================================================================
expect_deny 'sudo rm -rf /'                           'sudo rm -rf /'
expect_deny 'env rm -rf /'                            'env rm -rf /'
expect_deny 'nice rm -rf /'                           'nice rm -rf /'
expect_deny 'ls -la; rm -rf /'                        'compound rm -rf /'
expect_deny 'rm -rf / | grep foo'                     'pipe rm -rf /'

# =============================================================================
section "Policy: Database Pack"
# =============================================================================
expect_deny 'psql -c "DROP TABLE users"'              'psql DROP TABLE'
expect_deny "psql -qAt -c 'DROP TABLE users'"         'psql -qAt DROP TABLE'
expect_allow "psql -c 'SELECT 1'"                     'psql SELECT'

# =============================================================================
section "Policy: Kubernetes Pack"
# =============================================================================
expect_deny 'kubectl delete namespace production'     'kubectl delete namespace'
expect_deny 'kubectl delete pod --all'                'kubectl delete pod --all'
expect_allow 'kubectl get pods'                       'kubectl get pods'
expect_allow 'kubectl logs mypod'                     'kubectl logs mypod'

# =============================================================================
section "Policy: Containers Pack"
# =============================================================================
expect_deny 'docker system prune -af'                 'docker system prune -af'
expect_deny 'docker container prune -f'               'docker container prune -f'
expect_allow 'docker ps'                              'docker ps'
expect_allow 'docker logs mycontainer'                'docker logs mycontainer'

# =============================================================================
section "Policy: Infrastructure Pack"
# =============================================================================
expect_deny 'terraform destroy -auto-approve'         'terraform destroy -auto-approve'
expect_allow 'terraform init'                         'terraform init'
expect_allow 'terraform plan'                         'terraform plan'
