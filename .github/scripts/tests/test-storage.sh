# Storage: S3, GCS, MinIO, Azure Blob

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
