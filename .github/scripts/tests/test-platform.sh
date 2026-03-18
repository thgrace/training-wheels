# Platform: GitHub, GitLab

# =============================================================================
section "Platform: GitHub"
# =============================================================================
expect_deny 'gh repo delete myorg/myrepo'              'gh repo delete'
expect_deny 'gh repo archive myorg/myrepo'             'gh repo archive'
expect_deny 'gh gist delete abc123'                    'gh gist delete'
expect_deny 'gh release delete v1.0.0'                 'gh release delete'
expect_deny 'gh issue delete 42'                       'gh issue delete'
expect_deny 'gh ssh-key delete 12345'                  'gh ssh-key delete'
expect_deny 'gh secret delete MY_SECRET'               'gh secret delete'
expect_deny 'gh variable delete MY_VAR'                'gh variable delete'
expect_deny 'gh repo deploy-key delete 12345'          'gh repo deploy-key delete'
expect_deny 'gh run cancel 12345'                      'gh run cancel'
expect_deny 'gh api -X DELETE repos/org/repo/actions/secrets/SECRET' \
                                                       'gh api DELETE actions secret'
expect_deny 'gh api -X DELETE repos/org/repo/actions/variables/VAR' \
                                                       'gh api DELETE actions variable'
expect_deny 'gh api -X DELETE repos/org/repo/hooks/1'  'gh api DELETE hook'
expect_deny 'gh api -X DELETE repos/org/repo/keys/1'   'gh api DELETE deploy key'
expect_deny 'gh api -X DELETE repos/org/repo/releases/1' \
                                                       'gh api DELETE release'
expect_deny 'gh api -X DELETE repos/org/repo'          'gh api DELETE repo'

# =============================================================================
section "Platform: GitLab"
# =============================================================================
expect_deny 'glab repo delete myproject'               'glab repo delete'
expect_deny 'glab repo archive myproject'              'glab repo archive'
expect_deny 'glab release delete v1.0.0'               'glab release delete'
expect_deny 'glab variable delete MY_VAR'              'glab variable delete'
expect_deny 'glab api -X DELETE /projects/1'           'glab api DELETE project'
expect_deny 'glab api -X DELETE /projects/1/releases/v1' \
                                                       'glab api DELETE release'
expect_deny 'glab api -X DELETE /projects/1/variables/VAR' \
                                                       'glab api DELETE variable'
expect_deny 'glab api -X DELETE /projects/1/protected_branches/main' \
                                                       'glab api DELETE protected branch'
expect_deny 'glab api -X DELETE /projects/1/hooks/1'   'glab api DELETE hook'
expect_deny 'gitlab-rails runner "Project.find(1).destroy"' \
                                                       'gitlab-rails runner destructive'
expect_deny 'gitlab-rake gitlab:cleanup:repos'         'gitlab-rake destructive'
