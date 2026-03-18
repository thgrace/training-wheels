# Database: PostgreSQL, MySQL, MongoDB, Redis, SQLite, Supabase, Cassandra, ClickHouse, Snowflake

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
