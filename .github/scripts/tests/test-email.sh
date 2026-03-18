# Email: SES, SendGrid, Mailgun, Postmark

# =============================================================================
section "Email: SES"
# =============================================================================
expect_deny 'aws ses delete-identity --identity email@example.com' \
                                                       'ses delete-identity'
expect_deny 'aws ses delete-template --template-name mytemplate' \
                                                       'ses delete-template'
expect_deny 'aws ses delete-configuration-set --configuration-set-name myset' \
                                                       'ses delete-configuration-set'
expect_deny 'aws ses delete-receipt-rule-set --rule-set-name myruleset' \
                                                       'ses delete-receipt-rule-set'
expect_deny 'aws sesv2 delete-email-identity --email-identity domain.com' \
                                                       'sesv2 delete-email-identity'
expect_deny 'aws sesv2 delete-email-template --template-name mytemplate' \
                                                       'sesv2 delete-email-template'
expect_deny 'aws sesv2 delete-configuration-set --configuration-set-name myset' \
                                                       'sesv2 delete-configuration-set'
expect_deny 'aws sesv2 delete-contact-list --contact-list-name mylist' \
                                                       'sesv2 delete-contact-list'

# =============================================================================
section "Email: SendGrid"
# =============================================================================
expect_deny 'curl -X DELETE https://api.sendgrid.com/v3/templates/abc' \
                                                       'sendgrid delete template'
expect_deny 'curl -X DELETE https://api.sendgrid.com/v3/api_keys/abc' \
                                                       'sendgrid delete api key'

# =============================================================================
section "Email: Mailgun"
# =============================================================================
expect_deny 'curl -X DELETE https://api.mailgun.net/v3/domains/example.com' \
                                                       'mailgun delete domain'
expect_deny 'curl -X DELETE https://api.mailgun.net/v3/routes/abc' \
                                                       'mailgun delete route'

# =============================================================================
section "Email: Postmark"
# =============================================================================
expect_deny 'curl -X DELETE https://api.postmarkapp.com/servers/123' \
                                                       'postmark delete server'
expect_deny 'curl -X DELETE https://api.postmarkapp.com/templates/123' \
                                                       'postmark delete template'
