# Monitoring: Splunk, Datadog, PagerDuty, New Relic, Prometheus/Grafana

# =============================================================================
section "Monitoring: Splunk"
# =============================================================================
expect_deny 'splunk remove index myindex'              'splunk remove index'
expect_deny 'splunk clean eventdata -index myindex'    'splunk clean eventdata'

# =============================================================================
section "Monitoring: Datadog"
# =============================================================================
expect_deny 'datadog-ci monitors delete 12345'         'datadog-ci monitors delete'
expect_deny 'datadog-ci dashboards delete abc-def'     'datadog-ci dashboards delete'
expect_deny 'curl -X DELETE https://api.datadoghq.com/api/v1/monitor/12345' \
                                                       'datadog api delete'

# =============================================================================
section "Monitoring: PagerDuty"
# =============================================================================
expect_deny 'pd service delete PXXXXXX'                'pd service delete'
expect_deny 'pd schedule delete PXXXXXX'               'pd schedule delete'
expect_deny 'pd escalation-policy delete PXXXXXX'      'pd escalation-policy delete'
expect_deny 'pd user delete PXXXXXX'                   'pd user delete'
expect_deny 'pd team delete PXXXXXX'                   'pd team delete'

# =============================================================================
section "Monitoring: New Relic"
# =============================================================================
expect_deny 'newrelic entity delete --guid abc'        'newrelic entity delete'
expect_deny 'newrelic apm application delete --applicationId 123' \
                                                       'newrelic apm app delete'
expect_deny 'newrelic workload delete --guid abc'      'newrelic workload delete'
expect_deny 'newrelic synthetics delete --monitorId abc' \
                                                       'newrelic synthetics delete'

# =============================================================================
section "Monitoring: Prometheus/Grafana"
# =============================================================================
expect_deny 'kubectl delete prometheusrule myrule'     'kubectl delete prometheusrule'
expect_deny 'grafana-cli plugins uninstall myplugin'   'grafana-cli plugins uninstall'
expect_deny 'curl -X DELETE http://grafana:3000/api/dashboards/uid/abc' \
                                                       'grafana api delete dashboard'
expect_deny 'curl -X DELETE http://grafana:3000/api/datasources/1' \
                                                       'grafana api delete datasource'
