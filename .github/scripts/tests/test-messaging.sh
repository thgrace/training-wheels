# Messaging: Kafka, RabbitMQ, NATS, AWS SQS/SNS

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
