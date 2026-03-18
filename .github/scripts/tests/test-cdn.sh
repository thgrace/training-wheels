# CDN: Cloudflare Workers, Fastly, CloudFront

# =============================================================================
section "CDN: Cloudflare Workers"
# =============================================================================
expect_deny 'wrangler delete'                          'wrangler delete'
expect_deny 'wrangler deployments rollback'            'wrangler deployments rollback'
expect_deny 'wrangler kv:key delete --namespace-id abc mykey' \
                                                       'wrangler kv key delete'
expect_deny 'wrangler kv:namespace delete --namespace-id abc' \
                                                       'wrangler kv namespace delete'
expect_deny 'wrangler kv:bulk delete --namespace-id abc keys.json' \
                                                       'wrangler kv bulk delete'
expect_deny 'wrangler r2 object delete bucket/key'     'wrangler r2 object delete'
expect_deny 'wrangler r2 bucket delete mybucket'       'wrangler r2 bucket delete'
expect_deny 'wrangler d1 delete mydb'                  'wrangler d1 delete'

# =============================================================================
section "CDN: Fastly"
# =============================================================================
expect_deny 'fastly service delete'                    'fastly service delete'
expect_deny 'fastly domain delete'                     'fastly domain delete'
expect_deny 'fastly backend delete'                    'fastly backend delete'
expect_deny 'fastly vcl delete'                        'fastly vcl delete'
expect_deny 'fastly dictionary delete'                 'fastly dictionary delete'
expect_deny 'fastly dictionary-item delete'            'fastly dictionary-item delete'
expect_deny 'fastly acl delete'                        'fastly acl delete'
expect_deny 'fastly acl-entry delete'                  'fastly acl-entry delete'
expect_deny 'fastly logging s3 delete'                 'fastly logging delete'
expect_deny 'fastly service version activate'          'fastly version activate'
expect_deny 'fastly compute delete'                    'fastly compute delete'

# =============================================================================
section "CDN: CloudFront"
# =============================================================================
expect_deny 'aws cloudfront delete-distribution --id E123' \
                                                       'cloudfront delete-distribution'
expect_deny 'aws cloudfront delete-cache-policy --id abc' \
                                                       'cloudfront delete-cache-policy'
expect_deny 'aws cloudfront delete-origin-request-policy --id abc' \
                                                       'cloudfront delete-origin-request-policy'
expect_deny 'aws cloudfront delete-function --name myfunc --if-match etag' \
                                                       'cloudfront delete-function'
expect_deny 'aws cloudfront delete-response-headers-policy --id abc' \
                                                       'cloudfront delete-response-headers-policy'
expect_deny 'aws cloudfront delete-key-group --id abc' 'cloudfront delete-key-group'
expect_deny 'aws cloudfront create-invalidation --distribution-id E123 --paths "/*"' \
                                                       'cloudfront create-invalidation'
