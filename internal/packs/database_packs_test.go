package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestPostgreSQL_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "database.postgresql")

	tests := []struct {
		name    string
		cmdLine string
		wantID  string
		wantSev packs.Severity
	}{
		{"drop database", "psql -c 'DROP DATABASE prod'", "drop-database", packs.SeverityCritical},
		{"drop table", "psql -c 'DROP TABLE users'", "drop-table", packs.SeverityHigh},
		{"truncate table", "psql -c 'TRUNCATE TABLE logs'", "truncate-table", packs.SeverityHigh},
		{"delete without where", "psql -c 'DELETE FROM users'", "delete-without-where", packs.SeverityHigh},
		{"dropdb cli", "dropdb myapp_prod", "dropdb-cli", packs.SeverityCritical},
		{"pg_dump clean", "pg_dump --clean dbname", "pg-dump-clean", packs.SeverityHigh},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmdLine)
			if m == nil {
				t.Fatalf("Check(%q) should be denied, but was allowed", tc.cmdLine)
			}
			if m.Name != tc.wantID {
				t.Errorf("Check(%q) matched rule %q, want %q", tc.cmdLine, m.Name, tc.wantID)
			}
		})
	}
}

func TestMySQL_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "database.mysql")

	tests := []struct {
		name    string
		cmdLine string
		wantID  string
		wantSev packs.Severity
	}{
		{"drop database", "mysql -e 'DROP DATABASE prod'", "drop-database", packs.SeverityCritical},
		{"drop table", "mysql -e 'DROP TABLE users'", "drop-table", packs.SeverityHigh},
		{"truncate table", "mysql -e 'TRUNCATE TABLE logs'", "truncate-table", packs.SeverityHigh},
		{"delete without where", "mysql -e 'DELETE FROM users'", "delete-without-where", packs.SeverityHigh},
		{"mysqladmin drop", "mysqladmin drop prod", "mysqladmin-drop", packs.SeverityCritical},
		{"grant all", "mysql -e 'GRANT ALL PRIVILEGES ON *.* TO user'", "grant-all", packs.SeverityHigh},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmdLine)
			if m == nil {
				t.Fatalf("Check(%q) should be denied, but was allowed", tc.cmdLine)
			}
			if m.Name != tc.wantID {
				t.Errorf("Check(%q) matched rule %q, want %q", tc.cmdLine, m.Name, tc.wantID)
			}
		})
	}
}

func TestRedis_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "database.redis")

	tests := []struct {
		name    string
		cmdLine string
		wantID  string
		wantSev packs.Severity
	}{
		{"flushall", "redis-cli FLUSHALL", "flushall", packs.SeverityCritical},
		{"flushdb", "redis-cli FLUSHDB", "flushdb", packs.SeverityHigh},
		{"shutdown", "redis-cli SHUTDOWN", "shutdown", packs.SeverityHigh},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmdLine)
			if m == nil {
				t.Fatalf("Check(%q) should be denied, but was allowed", tc.cmdLine)
			}
			if m.Name != tc.wantID {
				t.Errorf("Check(%q) matched rule %q, want %q", tc.cmdLine, m.Name, tc.wantID)
			}
		})
	}
}

func TestMongoDB_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "database.mongodb")

	tests := []struct {
		name    string
		cmdLine string
		wantID  string
		wantSev packs.Severity
	}{
		{"dropDatabase", "mongosh --eval 'db.dropDatabase()'", "drop-database", packs.SeverityCritical},
		{"dropCollection", "mongosh --eval 'db.collection.drop()'", "drop-collection", packs.SeverityHigh},
		{"deleteMany all", "mongosh --eval 'db.collection.deleteMany({})'", "delete-all", packs.SeverityHigh},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmdLine)
			if m == nil {
				t.Fatalf("Check(%q) should be denied, but was allowed", tc.cmdLine)
			}
			if m.Name != tc.wantID {
				t.Errorf("Check(%q) matched rule %q, want %q", tc.cmdLine, m.Name, tc.wantID)
			}
		})
	}
}
