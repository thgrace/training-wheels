# DNS: Cloudflare, Route53, Generic

# =============================================================================
section "DNS: Cloudflare"
# =============================================================================
expect_deny 'wrangler dns delete zone-id record-id'    'wrangler dns delete'

# =============================================================================
section "DNS: Route53"
# =============================================================================
expect_deny 'aws route53 delete-hosted-zone --id Z123' 'route53 delete-hosted-zone'
expect_deny 'aws route53 change-resource-record-sets --hosted-zone-id Z123 --change-batch DELETE' \
                                                       'route53 change-resource-record-sets DELETE'
expect_deny 'aws route53 delete-health-check --health-check-id abc' \
                                                       'route53 delete-health-check'
expect_deny 'aws route53 delete-query-logging-config --id abc' \
                                                       'route53 delete-query-logging-config'
expect_deny 'aws route53 delete-traffic-policy --id abc --version 1' \
                                                       'route53 delete-traffic-policy'

# =============================================================================
section "DNS: Generic"
# =============================================================================
expect_deny 'nsupdate -k keyfile <<< "update delete example.com A"' \
                                                       'nsupdate delete'
expect_deny 'nsupdate -l'                              'nsupdate local'
expect_deny 'dig axfr example.com @ns1.example.com'    'dig zone transfer'
