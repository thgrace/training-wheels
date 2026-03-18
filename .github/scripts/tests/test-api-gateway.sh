# API Gateway: AWS, Kong, Apigee

# =============================================================================
section "API Gateway: AWS"
# =============================================================================
expect_deny 'aws apigateway delete-rest-api --rest-api-id abc123' \
                                                       'apigateway delete-rest-api'
expect_deny 'aws apigateway delete-resource --rest-api-id abc --resource-id xyz' \
                                                       'apigateway delete-resource'
expect_deny 'aws apigateway delete-method --rest-api-id abc --resource-id xyz --http-method GET' \
                                                       'apigateway delete-method'
expect_deny 'aws apigateway delete-stage --rest-api-id abc --stage-name prod' \
                                                       'apigateway delete-stage'
expect_deny 'aws apigateway delete-deployment --rest-api-id abc --deployment-id xyz' \
                                                       'apigateway delete-deployment'
expect_deny 'aws apigateway delete-api-key --api-key abc' \
                                                       'apigateway delete-api-key'
expect_deny 'aws apigateway delete-authorizer --rest-api-id abc --authorizer-id xyz' \
                                                       'apigateway delete-authorizer'
expect_deny 'aws apigateway delete-model --rest-api-id abc --model-name MyModel' \
                                                       'apigateway delete-model'
expect_deny 'aws apigateway delete-domain-name --domain-name api.example.com' \
                                                       'apigateway delete-domain-name'
expect_deny 'aws apigateway delete-usage-plan --usage-plan-id abc' \
                                                       'apigateway delete-usage-plan'
expect_deny 'aws apigatewayv2 delete-api --api-id abc123' \
                                                       'apigatewayv2 delete-api'
expect_deny 'aws apigatewayv2 delete-route --api-id abc --route-id xyz' \
                                                       'apigatewayv2 delete-route'
expect_deny 'aws apigatewayv2 delete-integration --api-id abc --integration-id xyz' \
                                                       'apigatewayv2 delete-integration'
expect_deny 'aws apigatewayv2 delete-stage --api-id abc --stage-name prod' \
                                                       'apigatewayv2 delete-stage'
expect_deny 'aws apigatewayv2 delete-authorizer --api-id abc --authorizer-id xyz' \
                                                       'apigatewayv2 delete-authorizer'
expect_deny 'aws apigatewayv2 delete-domain-name --domain-name api.example.com' \
                                                       'apigatewayv2 delete-domain-name'

# =============================================================================
section "API Gateway: Kong"
# =============================================================================
expect_deny 'deck reset'                               'deck reset'
expect_deny 'deck gateway reset'                       'deck gateway reset'
expect_deny 'curl -X DELETE http://localhost:8001/services/myservice' \
                                                       'kong admin delete services'
expect_deny 'curl -X DELETE http://localhost:8001/routes/myroute' \
                                                       'kong admin delete routes'
expect_deny 'curl -X DELETE http://localhost:8001/plugins/myplugin' \
                                                       'kong admin delete plugins'
expect_deny 'curl -X DELETE http://localhost:8001/consumers/myconsumer' \
                                                       'kong admin delete consumers'
expect_deny 'curl -X DELETE http://localhost:8001/upstreams/myupstream' \
                                                       'kong admin delete upstreams'

# =============================================================================
section "API Gateway: Apigee"
# =============================================================================
expect_deny 'gcloud apigee apis delete my-api --organization=myorg' \
                                                       'apigee apis delete'
expect_deny 'gcloud apigee environments delete staging --organization=myorg' \
                                                       'apigee environments delete'
expect_deny 'gcloud apigee developers delete dev@example.com --organization=myorg' \
                                                       'apigee developers delete'
expect_deny 'gcloud apigee products delete my-product --organization=myorg' \
                                                       'apigee products delete'
expect_deny 'gcloud apigee organizations delete myorg' 'apigee organizations delete'
expect_deny 'apigeecli apis delete --name my-api --org myorg' \
                                                       'apigeecli apis delete'
