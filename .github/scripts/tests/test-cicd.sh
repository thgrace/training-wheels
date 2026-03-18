# CI/CD: GitHub Actions, GitLab CI, Jenkins, CircleCI

# =============================================================================
section "CI/CD: GitHub Actions"
# =============================================================================
expect_deny 'gh secret remove MY_SECRET'               'gh actions secret remove'
expect_deny 'gh variable remove MY_VAR'                'gh actions variable remove'
expect_deny 'gh workflow disable ci.yml'               'gh actions workflow disable'
expect_deny 'gh run cancel 12345'                      'gh actions run cancel'
expect_deny 'gh api -X DELETE repos/org/repo/actions/secrets/SECRET' \
                                                       'gh api DELETE actions secrets'

# =============================================================================
section "CI/CD: GitLab CI"
# =============================================================================
expect_deny 'glab variable delete MY_VAR'              'glab ci variable delete'
expect_deny 'glab ci delete 12345'                     'glab ci delete'
expect_deny 'gitlab-runner unregister --all-runners'   'gitlab-runner unregister'

# =============================================================================
section "CI/CD: Jenkins"
# =============================================================================
expect_deny 'jenkins-cli delete-job myjob'             'jenkins delete-job'
expect_deny 'jenkins-cli delete-node mynode'            'jenkins delete-node'
expect_deny 'jenkins-cli delete-credentials domain cred' \
                                                       'jenkins delete-credentials'
expect_deny 'jenkins-cli delete-builds myjob 1-10'     'jenkins delete-builds'
expect_deny 'jenkins-cli delete-view myview'            'jenkins delete-view'
expect_deny 'curl -X POST http://jenkins.local/job/myjob/doDelete' \
                                                       'jenkins curl doDelete'

# =============================================================================
section "CI/CD: CircleCI"
# =============================================================================
expect_deny 'circleci context delete myorg mycontext'  'circleci context delete'
expect_deny 'circleci context remove-secret myorg mycontext MY_SECRET' \
                                                       'circleci context remove-secret'
expect_deny 'circleci orb delete myorg/myorb'          'circleci orb delete'
expect_deny 'circleci namespace delete myorg'          'circleci namespace delete'
