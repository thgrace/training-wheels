# Cloud: AWS, GCP, Azure, Other Providers

# =============================================================================
section "Cloud: AWS"
# =============================================================================
expect_deny 'aws ec2 terminate-instances --instance-ids i-12345' \
                                                       'aws ec2 terminate-instances'
expect_deny 'aws ec2 delete-snapshot --snapshot-id snap-123' \
                                                       'aws ec2 delete-snapshot'
expect_deny 'aws ec2 delete-volume --volume-id vol-123' \
                                                       'aws ec2 delete-volume'
expect_deny 'aws ec2 delete-vpc --vpc-id vpc-123'     'aws ec2 delete-vpc'
expect_deny 'aws ec2 delete-subnet --subnet-id sub-123' \
                                                       'aws ec2 delete-subnet'
expect_deny 'aws ec2 delete-security-group --group-id sg-123' \
                                                       'aws ec2 delete-security-group'
expect_deny 'aws ec2 delete-key-pair --key-name mykey' \
                                                       'aws ec2 delete-key-pair'
expect_deny 'aws ec2 delete-image --image-id ami-123' 'aws ec2 delete-image (deregister AMI)'
expect_deny 'aws s3 rm s3://my-bucket --recursive'    'aws s3 rm --recursive'
expect_deny 'aws s3 rb s3://my-bucket --force'        'aws s3 rb --force'
expect_deny 'aws s3 sync . s3://bucket --delete'      'aws s3 sync --delete'
expect_deny 'aws s3api delete-bucket --bucket mybucket' \
                                                       'aws s3api delete-bucket'
expect_deny 'aws rds delete-db-instance --db-instance-identifier prod' \
                                                       'aws rds delete-db-instance'
expect_deny 'aws cloudformation delete-stack --stack-name prod' \
                                                       'aws cloudformation delete-stack'
expect_deny 'aws lambda delete-function --function-name handler' \
                                                       'aws lambda delete-function'
expect_deny 'aws iam delete-user --user-name admin'   'aws iam delete-user'
expect_deny 'aws dynamodb delete-table --table-name users' \
                                                       'aws dynamodb delete-table'
expect_deny 'aws eks delete-cluster --name production' 'aws eks delete-cluster'
expect_deny 'aws ecr delete-repository --repository-name app' \
                                                       'aws ecr delete-repository'
expect_deny 'aws ecr batch-delete-image --repository-name app --image-ids imageTag=latest' \
                                                       'aws ecr batch-delete-image'
expect_deny 'aws ecr delete-lifecycle-policy --repository-name app' \
                                                       'aws ecr delete-lifecycle-policy'
expect_deny 'aws logs delete-log-group --log-group-name /app/prod' \
                                                       'aws logs delete-log-group'
expect_deny 'aws logs delete-log-stream --log-group-name /app --log-stream-name stream1' \
                                                       'aws logs delete-log-stream'

# AWS safe commands
expect_allow 'aws ec2 describe-instances'              'aws ec2 describe-instances'
expect_allow 'aws s3 ls'                               'aws s3 ls'
expect_allow 'aws s3 cp file.txt s3://bucket/'         'aws s3 cp'
expect_allow 'aws sts get-caller-identity'             'aws sts get-caller-identity'
expect_allow 'aws ec2 describe-instances --dry-run'    'aws describe --dry-run'
expect_allow 'aws ecr get-login-password'              'aws ecr get-login'
expect_allow 'aws cloudformation describe-stacks'      'aws cfn describe-stacks'

# =============================================================================
section "Cloud: GCP"
# =============================================================================
expect_deny 'gcloud compute instances delete myvm'     'gcloud compute instances delete'
expect_deny 'gcloud compute disks delete mydisk'       'gcloud compute disks delete'
expect_deny 'gcloud sql instances delete mydb'         'gcloud sql instances delete'
expect_deny 'gsutil rm -r gs://my-bucket'              'gsutil rm -r'
expect_deny 'gsutil rb gs://my-bucket'                 'gsutil rb'
expect_deny 'gcloud container clusters delete mycluster' \
                                                       'gcloud GKE delete'
expect_deny 'gcloud projects delete my-project'        'gcloud projects delete'
expect_deny 'gcloud functions delete my-function'      'gcloud functions delete'
expect_deny 'gcloud pubsub topics delete my-topic'     'gcloud pubsub delete'
expect_deny 'gcloud firestore databases delete'        'gcloud firestore delete'
expect_deny 'gcloud container images delete gcr.io/proj/img' \
                                                       'gcloud container images delete'
expect_deny 'gcloud artifacts docker images delete us-docker.pkg.dev/proj/repo/img' \
                                                       'gcloud artifacts docker images delete'
expect_deny 'gcloud artifacts repositories delete myrepo' \
                                                       'gcloud artifacts repositories delete'

# =============================================================================
section "Cloud: Azure"
# =============================================================================
expect_deny 'az vm delete -g mygroup -n myvm'          'az vm delete'
expect_deny 'az storage account delete -n myaccount'   'az storage account delete'
expect_deny 'az storage blob delete-batch -s container' \
                                                       'az storage blob delete-batch'
expect_deny 'az storage blob delete -c container -n blob' \
                                                       'az storage blob delete'
expect_deny 'az sql server delete -g rg -n server'     'az sql server delete'
expect_deny 'az group delete -n myresourcegroup'       'az group delete'
expect_deny 'az aks delete -g rg -n cluster'           'az aks delete'
expect_deny 'az webapp delete -g rg -n app'            'az webapp delete'
expect_deny 'az functionapp delete -g rg -n func'      'az functionapp delete'
expect_deny 'az cosmosdb delete -g rg -n cosmos'       'az cosmosdb delete'
expect_deny 'az keyvault delete -n myvault'            'az keyvault delete'
expect_deny 'az network vnet delete -g rg -n myvnet'   'az network vnet delete'
expect_deny 'az acr delete -n myregistry'              'az acr delete'
expect_deny 'az acr repository delete -n myreg --repository myrepo' \
                                                       'az acr repository delete'

# =============================================================================
section "Cloud: Other Providers"
# =============================================================================
expect_deny 'doctl compute droplet delete 12345'       'DigitalOcean droplet delete'
expect_deny 'heroku apps:destroy myapp'                'Heroku apps:destroy'
expect_deny 'vercel rm my-project'                     'Vercel rm'
expect_deny 'netlify sites:delete'                     'Netlify sites:delete'
