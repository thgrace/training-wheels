# Infrastructure: Terraform, Pulumi, CDK & Serverless, Ansible

# =============================================================================
section "Infrastructure: Terraform"
# =============================================================================
expect_deny 'terraform destroy'                        'terraform destroy'
expect_deny 'terraform destroy -auto-approve'          'terraform destroy -auto-approve'
expect_deny 'terraform apply -auto-approve'            'terraform apply -auto-approve'
expect_deny 'terraform taint aws_instance.example'     'terraform taint'
expect_deny 'terraform state rm aws_instance.example'  'terraform state rm'
expect_deny 'terraform state mv old new'               'terraform state mv'
expect_deny 'terraform force-unlock 12345'             'terraform force-unlock'
expect_deny 'terraform workspace delete staging'       'terraform workspace delete'

# Terraform safe commands
expect_allow 'terraform plan'                          'terraform plan'
expect_allow 'terraform init'                          'terraform init'
expect_allow 'terraform validate'                      'terraform validate'
expect_allow 'terraform fmt'                           'terraform fmt'
expect_allow 'terraform show'                          'terraform show'
expect_allow 'terraform output'                        'terraform output'
expect_allow 'terraform state list'                    'terraform state list'
expect_allow 'terraform state show resource'           'terraform state show'
expect_allow 'terraform version'                       'terraform version'
expect_allow 'terraform graph'                         'terraform graph'
expect_allow 'terraform providers'                     'terraform providers'

# =============================================================================
section "Infrastructure: Pulumi"
# =============================================================================
expect_deny 'pulumi destroy'                           'pulumi destroy'
expect_deny 'pulumi destroy --yes'                     'pulumi destroy --yes'
expect_deny 'pulumi up --yes'                          'pulumi up --yes'
expect_deny 'pulumi state delete urn'                  'pulumi state delete'
expect_deny 'pulumi stack rm mystack'                  'pulumi stack rm'
expect_deny 'pulumi refresh --yes'                     'pulumi refresh --yes'
expect_deny 'pulumi cancel'                            'pulumi cancel'

# =============================================================================
section "Infrastructure: CDK & Serverless"
# =============================================================================
expect_deny 'cdk destroy'                              'cdk destroy'
expect_deny 'sls remove'                               'serverless remove'

# =============================================================================
section "Infrastructure: Ansible"
# =============================================================================
expect_deny 'ansible all -m shell -a "rm -rf /"'       'ansible shell rm -rf'
expect_deny 'ansible all -m shell -a "reboot"'         'ansible shell reboot'
expect_deny 'ansible-playbook playbook.yml -e "delete=true"' \
                                                       'ansible extra-vars delete'
