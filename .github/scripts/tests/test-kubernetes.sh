# Kubernetes: kubectl, Helm, Kustomize

# =============================================================================
section "Kubernetes: kubectl"
# =============================================================================
expect_deny 'kubectl delete namespace production'      'kubectl delete namespace'
expect_deny 'kubectl delete ns staging'                'kubectl delete ns'
expect_deny 'kubectl delete --all pods'                'kubectl delete --all pods'
expect_deny 'kubectl delete --all pods --all-namespaces' \
                                                       'kubectl delete all namespaces'
expect_deny 'kubectl drain node01'                     'kubectl drain node'
expect_deny 'kubectl cordon node01'                    'kubectl cordon node'
expect_deny 'kubectl taint nodes node01 key=value:NoExecute' \
                                                       'kubectl taint NoExecute'
expect_deny 'kubectl delete deployment nginx -n prod'  'kubectl delete deployment'
expect_deny 'kubectl delete pvc data-volume'           'kubectl delete pvc'
expect_deny 'kubectl delete pv my-pv'                  'kubectl delete pv'
expect_deny 'kubectl scale deployment app --replicas=0' \
                                                       'kubectl scale to zero'
expect_deny 'kubectl delete pod mypod --force --grace-period=0' \
                                                       'kubectl delete --force'
expect_deny 'kubectl apply --force -f deployment.yaml' 'kubectl apply --force'
expect_deny 'kubectl delete -f /path/to/manifests/'    'kubectl delete from directory'

# Kubernetes safe commands
expect_allow 'kubectl get pods'                        'kubectl get pods'
expect_allow 'kubectl describe pod mypod'              'kubectl describe'
expect_allow 'kubectl logs mypod'                      'kubectl logs'
expect_allow 'kubectl get all -n default'              'kubectl get all'
expect_allow 'kubectl top pods'                        'kubectl top pods'
expect_allow 'kubectl config current-context'          'kubectl config'
expect_allow 'kubectl version'                         'kubectl version'
expect_allow 'kubectl diff -f deployment.yaml'         'kubectl diff'
expect_allow 'kubectl explain pods'                    'kubectl explain'

# =============================================================================
section "Kubernetes: Helm"
# =============================================================================
expect_deny 'helm uninstall myrelease'                 'helm uninstall'
expect_deny 'helm rollback myrelease 1'                'helm rollback'
expect_deny 'helm upgrade myrelease chart --force'     'helm upgrade --force'
expect_deny 'helm upgrade myrelease chart --reset-values' \
                                                       'helm upgrade --reset-values'

# =============================================================================
section "Kubernetes: Kustomize"
# =============================================================================
expect_deny 'kubectl delete -k .'                      'kubectl delete -k'
expect_deny 'kustomize build . | kubectl delete -f -'  'kustomize pipe to kubectl delete'
