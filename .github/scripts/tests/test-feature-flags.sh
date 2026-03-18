# Feature Flags: LaunchDarkly, Flipt, Split, Unleash

# =============================================================================
section "Feature Flags: LaunchDarkly"
# =============================================================================
expect_deny 'ldcli flags delete --project default --key my-flag' \
                                                       'ldcli flags delete'
expect_deny 'ldcli flags archive --project default --key my-flag' \
                                                       'ldcli flags archive'
expect_deny 'ldcli projects delete --key myproject'    'ldcli projects delete'
expect_deny 'ldcli environments delete --project default --key staging' \
                                                       'ldcli environments delete'
expect_deny 'ldcli segments delete --project default --env prod --key myseg' \
                                                       'ldcli segments delete'

# =============================================================================
section "Feature Flags: Flipt"
# =============================================================================
expect_deny 'flipt flag delete myflag'                 'flipt flag delete'
expect_deny 'flipt segment delete mysegment'           'flipt segment delete'
expect_deny 'flipt namespace delete mynamespace'       'flipt namespace delete'
expect_deny 'flipt rule delete myrule'                 'flipt rule delete'

# =============================================================================
section "Feature Flags: Split"
# =============================================================================
expect_deny 'split splits delete my-split'             'split splits delete'
expect_deny 'split splits kill my-split'               'split splits kill'
expect_deny 'split environments delete staging'        'split environments delete'

# =============================================================================
section "Feature Flags: Unleash"
# =============================================================================
expect_deny 'unleash features delete my-toggle'        'unleash features delete'
expect_deny 'unleash features archive my-toggle'       'unleash features archive'
expect_deny 'unleash projects delete myproject'        'unleash projects delete'
