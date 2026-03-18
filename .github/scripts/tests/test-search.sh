# Search: Elasticsearch, OpenSearch, Algolia, Meilisearch

# =============================================================================
section "Search: Elasticsearch"
# =============================================================================
expect_deny 'curl -X DELETE http://localhost:9200/myindex' \
                                                       'ES curl DELETE index'
expect_deny 'curl -X DELETE http://localhost:9200/myindex/_doc/1' \
                                                       'ES curl DELETE doc'
expect_deny 'curl -X POST http://localhost:9200/myindex/_close' \
                                                       'ES curl close index'
expect_deny 'curl -X POST http://localhost:9200/myindex/_delete_by_query' \
                                                       'ES curl delete by query'
expect_deny 'curl -X PUT http://localhost:9200/_cluster/settings -d "{\"persistent\":{\"cluster.routing.allocation.enable\":\"none\"}}"' \
                                                       'ES curl cluster settings'
expect_deny 'http DELETE http://localhost:9200/myindex' 'ES httpie DELETE index'
expect_deny 'http DELETE http://localhost:9200/myindex/_doc/1' \
                                                       'ES httpie DELETE doc'
expect_deny 'http POST http://localhost:9200/myindex/_close' \
                                                       'ES httpie close index'

# =============================================================================
section "Search: OpenSearch"
# =============================================================================
expect_deny 'aws opensearch delete-domain --domain-name mydomain' \
                                                       'aws opensearch delete-domain'
expect_deny 'curl -X DELETE https://search-domain.us-east-1.es.amazonaws.com/myindex' \
                                                       'OpenSearch curl DELETE index'

# =============================================================================
section "Search: Algolia"
# =============================================================================
expect_deny 'algolia indices delete myindex'           'algolia indices delete'
expect_deny 'algolia indices clear myindex'            'algolia indices clear'
expect_deny 'algolia rules delete myindex'             'algolia rules delete'
expect_deny 'algolia apikeys delete mykey'             'algolia apikeys delete'

# =============================================================================
section "Search: Meilisearch"
# =============================================================================
expect_deny 'curl -X DELETE http://localhost:7700/indexes/movies/documents/1' \
                                                       'meili curl delete document'
expect_deny 'curl -X DELETE http://localhost:7700/indexes/movies' \
                                                       'meili curl delete index'
expect_deny 'curl -X DELETE http://localhost:7700/keys/mykey' \
                                                       'meili curl delete key'
