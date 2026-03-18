# Secrets: Vault, AWS, 1Password, Doppler, Bitwarden, GCP, Azure, LastPass, Infisical, Pass, Gopass, Conjur

# =============================================================================
section "Secrets: HashiCorp Vault"
# =============================================================================
expect_deny 'vault secrets disable secret/'            'vault secrets disable'
expect_deny 'vault kv destroy secret/myapp'            'vault kv destroy'
expect_deny 'vault kv metadata delete secret/myapp'    'vault kv metadata delete'
expect_deny 'vault kv delete secret/myapp'             'vault kv delete'
expect_deny 'vault delete secret/myapp'                'vault delete'
expect_deny 'vault policy delete mypolicy'             'vault policy delete'
expect_deny 'vault auth disable userpass/'             'vault auth disable'
expect_deny 'vault token revoke s.xxxxxxxx'            'vault token revoke'
expect_deny 'vault lease revoke lease-id'              'vault lease revoke'

# Vault safe commands
expect_allow 'vault status'                            'vault status'
expect_allow 'vault version'                           'vault version'
expect_allow 'vault kv get secret/myapp'               'vault kv get'
expect_allow 'vault kv list secret/'                   'vault kv list'
expect_allow 'vault secrets list'                      'vault secrets list'
expect_allow 'vault auth list'                         'vault auth list'
expect_allow 'vault policy list'                       'vault policy list'
expect_allow 'vault token lookup'                      'vault token lookup'
expect_allow 'vault audit list'                        'vault audit list'
expect_allow 'vault lease lookup lease-id'             'vault lease lookup'

# =============================================================================
section "Secrets: AWS Secrets Manager & SSM"
# =============================================================================
expect_deny 'aws secretsmanager delete-secret --secret-id prod/db' \
                                                       'aws secretsmanager delete-secret'
expect_deny 'aws secretsmanager delete-secret --secret-id key --force-delete-without-recovery' \
                                                       'aws secretsmanager force delete'
expect_deny 'aws secretsmanager delete-resource-policy --secret-id key' \
                                                       'aws secretsmanager delete-resource-policy'
expect_deny 'aws secretsmanager update-secret --secret-id key --secret-string newval' \
                                                       'aws secretsmanager update-secret'
expect_deny 'aws secretsmanager put-secret-value --secret-id key --secret-string newval' \
                                                       'aws secretsmanager put-secret-value'
expect_deny 'aws ssm delete-parameter --name /app/key' 'aws ssm delete-parameter'
expect_deny 'aws ssm delete-parameters --names /a /b'  'aws ssm delete-parameters'

# =============================================================================
section "Secrets: 1Password"
# =============================================================================
expect_deny 'op item delete myitem'                    'op item delete'
expect_deny 'op document delete "Prod Cert"'           'op document delete'
expect_deny 'op vault delete myvault'                  'op vault delete'
expect_deny 'op user delete user@example.com'          'op user delete'
expect_deny 'op group delete Contractors'              'op group delete'
expect_deny 'op connect token delete tokenid'          'op connect token delete'

# =============================================================================
section "Secrets: Doppler"
# =============================================================================
expect_deny 'doppler secrets delete MY_SECRET'         'doppler secrets delete'
expect_deny 'doppler projects delete myproject'        'doppler projects delete'
expect_deny 'doppler environments delete staging'      'doppler environments delete'
expect_deny 'doppler configs delete dev'               'doppler configs delete'

# =============================================================================
section "Secrets: Bitwarden"
# =============================================================================
expect_deny 'bw delete item 12345'                     'bw delete'
expect_deny 'bw edit item 12345'                       'bw edit'
expect_deny 'bw move item 12345 org-id'                'bw move'

expect_allow 'bw status'                               'bw status'
expect_allow 'bw list items'                           'bw list'
expect_allow 'bw get item 12345'                       'bw get'
expect_allow 'bw export'                               'bw export'
expect_allow 'bw sync'                                 'bw sync'

# =============================================================================
section "Secrets: GCP Secret Manager"
# =============================================================================
expect_deny 'gcloud secrets delete my-secret'          'gcloud secrets delete'
expect_deny 'gcloud secrets versions destroy 1 --secret=my-secret' \
                                                       'gcloud secrets versions destroy'
expect_deny 'gcloud secrets versions disable 1 --secret=my-secret' \
                                                       'gcloud secrets versions disable'
expect_deny 'gcloud secrets update my-secret --update-labels=env=prod' \
                                                       'gcloud secrets update'
expect_deny 'gcloud secrets set-iam-policy my-secret policy.json' \
                                                       'gcloud secrets set-iam-policy'
expect_deny 'gcloud secrets remove-iam-policy-binding my-secret --member=user:a@b.com --role=roles/viewer' \
                                                       'gcloud secrets remove-iam-policy-binding'

expect_allow 'gcloud secrets list'                     'gcloud secrets list'
expect_allow 'gcloud secrets describe my-secret'       'gcloud secrets describe'
expect_allow 'gcloud secrets versions list my-secret'  'gcloud secrets versions list'
expect_allow 'gcloud secrets versions access latest --secret=my-secret' \
                                                       'gcloud secrets versions access'
expect_allow 'gcloud secrets get-iam-policy my-secret' 'gcloud secrets get-iam-policy'

# =============================================================================
section "Secrets: Azure Key Vault"
# =============================================================================
expect_deny 'az keyvault secret delete --name mysecret --vault-name myvault' \
                                                       'az keyvault secret delete'
expect_deny 'az keyvault secret purge --name mysecret --vault-name myvault' \
                                                       'az keyvault secret purge'
expect_deny 'az keyvault key delete --name mykey --vault-name myvault' \
                                                       'az keyvault key delete'
expect_deny 'az keyvault key purge --name mykey --vault-name myvault' \
                                                       'az keyvault key purge'
expect_deny 'az keyvault certificate delete --name mycert --vault-name myvault' \
                                                       'az keyvault certificate delete'
expect_deny 'az keyvault certificate purge --name mycert --vault-name myvault' \
                                                       'az keyvault certificate purge'
expect_deny 'az keyvault purge --name myvault'         'az keyvault purge'

expect_allow 'az keyvault secret list --vault-name myvault' \
                                                       'az keyvault secret list'
expect_allow 'az keyvault secret show --name mysecret --vault-name myvault' \
                                                       'az keyvault secret show'
expect_allow 'az keyvault key list --vault-name myvault' \
                                                       'az keyvault key list'
expect_allow 'az keyvault key show --name mykey --vault-name myvault' \
                                                       'az keyvault key show'
expect_allow 'az keyvault certificate list --vault-name myvault' \
                                                       'az keyvault certificate list'
expect_allow 'az keyvault show --name myvault'         'az keyvault show'

# =============================================================================
section "Secrets: LastPass"
# =============================================================================
expect_deny 'lpass rm mysite'                          'lpass rm'
expect_deny 'lpass edit mysite'                        'lpass edit'
expect_deny 'lpass mv mysite Business'                 'lpass mv'

expect_allow 'lpass status'                            'lpass status'
expect_allow 'lpass ls'                                'lpass ls'
expect_allow 'lpass show mysite'                       'lpass show'
expect_allow 'lpass export'                            'lpass export'

# =============================================================================
section "Secrets: Infisical"
# =============================================================================
expect_deny 'infisical secrets delete MY_SECRET'       'infisical secrets delete'
expect_deny 'infisical secrets set MY_SECRET=value'    'infisical secrets set'

expect_allow 'infisical run -- npm start'              'infisical run'
expect_allow 'infisical login'                         'infisical login'
expect_allow 'infisical export'                        'infisical export'
expect_allow 'infisical secrets get MY_SECRET'         'infisical secrets get'
expect_allow 'infisical scan'                          'infisical scan'

# =============================================================================
section "Secrets: Pass"
# =============================================================================
expect_deny 'pass rm email/personal'                   'pass rm'
expect_deny 'pass mv email/old email/new'              'pass mv'
expect_deny 'pass edit email/personal'                 'pass edit'
expect_deny 'pass insert email/new'                    'pass insert'
expect_deny 'pass generate email/new 20'               'pass generate'

expect_allow 'pass ls'                                 'pass ls'
expect_allow 'pass show email/personal'                'pass show'
expect_allow 'pass find email'                         'pass find'
expect_allow 'pass grep pattern'                       'pass grep'
expect_allow 'pass version'                            'pass version'

# =============================================================================
section "Secrets: Gopass"
# =============================================================================
expect_deny 'gopass rm email/personal'                 'gopass rm'
expect_deny 'gopass delete email/personal'             'gopass delete'
expect_deny 'gopass mv email/old email/new'            'gopass mv'
expect_deny 'gopass move email/old email/new'          'gopass move'
expect_deny 'gopass edit email/personal'               'gopass edit'
expect_deny 'gopass insert email/new'                  'gopass insert'
expect_deny 'gopass generate email/new 20'             'gopass generate'
expect_deny 'gopass mounts remove mystore'             'gopass mounts remove'

expect_allow 'gopass ls'                               'gopass ls'
expect_allow 'gopass show email/personal'              'gopass show'
expect_allow 'gopass find email'                       'gopass find'
expect_allow 'gopass audit'                            'gopass audit'
expect_allow 'gopass cat email/personal'               'gopass cat'
expect_allow 'gopass version'                          'gopass version'
expect_allow 'gopass list'                             'gopass list'
expect_allow 'gopass mounts'                           'gopass mounts'

# =============================================================================
section "Secrets: Conjur"
# =============================================================================
expect_deny 'conjur variable set -i prod/db/password -v secret123' \
                                                       'conjur variable set'
expect_deny 'conjur policy load --replace root policy.yml' \
                                                       'conjur policy load'
expect_deny 'conjur host rotate-api-key -i myapp'     'conjur host rotate-api-key'
expect_deny 'conjur user rotate-api-key -i admin'     'conjur user rotate-api-key'

expect_allow 'conjur list'                             'conjur list'
expect_allow 'conjur show myresource'                  'conjur show'
expect_allow 'conjur variable get -i prod/db/password' 'conjur variable get'
expect_allow 'conjur variable value -i prod/db/password' \
                                                       'conjur variable value'
expect_allow 'conjur whoami'                           'conjur whoami'
expect_allow 'conjur check -i myresource -p read'     'conjur check'
expect_allow 'conjur resource show myresource'         'conjur resource show'
expect_allow 'conjur role memberships myrole'          'conjur role memberships'
