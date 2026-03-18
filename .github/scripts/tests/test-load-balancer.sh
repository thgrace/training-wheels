# Load Balancer: ELB, HAProxy, nginx, Traefik

# =============================================================================
section "Load Balancer: ELB"
# =============================================================================
expect_deny 'aws elbv2 delete-load-balancer --load-balancer-arn arn:xxx' \
                                                       'elbv2 delete-load-balancer'
expect_deny 'aws elbv2 delete-target-group --target-group-arn arn:xxx' \
                                                       'elbv2 delete-target-group'
expect_deny 'aws elbv2 deregister-targets --target-group-arn arn:xxx --targets Id=i-123' \
                                                       'elbv2 deregister-targets'
expect_deny 'aws elbv2 delete-listener --listener-arn arn:xxx' \
                                                       'elbv2 delete-listener'
expect_deny 'aws elbv2 delete-rule --rule-arn arn:xxx' 'elbv2 delete-rule'
expect_deny 'aws elb delete-load-balancer --load-balancer-name myclb' \
                                                       'elb delete-load-balancer (classic)'
expect_deny 'aws elb deregister-instances-from-load-balancer --load-balancer-name myclb --instances i-123' \
                                                       'elb deregister-instances'

# =============================================================================
section "Load Balancer: HAProxy"
# =============================================================================
expect_deny 'haproxy -sf 1234'                         'haproxy soft stop'
expect_deny 'haproxy -st 1234'                         'haproxy hard stop'
expect_deny 'systemctl stop haproxy'                   'systemctl stop haproxy'
expect_deny 'service haproxy stop'                     'service haproxy stop'

# =============================================================================
section "Load Balancer: nginx"
# =============================================================================
expect_deny 'nginx -s stop'                            'nginx stop'
expect_deny 'nginx -s quit'                            'nginx quit'
expect_deny 'systemctl stop nginx'                     'systemctl stop nginx'
expect_deny 'service nginx stop'                       'service nginx stop'

# =============================================================================
section "Load Balancer: Traefik"
# =============================================================================
expect_deny 'docker stop traefik'                      'docker stop traefik'
expect_deny 'docker rm traefik'                        'docker rm traefik'
expect_deny 'docker-compose down traefik'              'docker-compose down traefik'
expect_deny 'kubectl delete pod traefik'               'kubectl delete traefik pod'
expect_deny 'kubectl delete ingressroute myroute'      'kubectl delete ingressroute'
expect_deny 'systemctl stop traefik'                   'systemctl stop traefik'
expect_deny 'service traefik stop'                     'service traefik stop'
