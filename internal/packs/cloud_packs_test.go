package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

func TestKubernetes_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "kubernetes.kubectl")

	tests := []struct {
		name    string
		cmdLine string
		wantID  string
		wantSev packs.Severity
	}{
		{"delete ns", "kubectl delete namespace prod", "delete-namespace", packs.SeverityCritical},
		{"delete all", "kubectl delete pods --all", "delete-all", packs.SeverityHigh},
		{"delete all massive", "kubectl delete all --all", "delete-all-massive", packs.SeverityCritical},
		{"drain node", "kubectl drain node-1", "drain-node", packs.SeverityHigh},
		{"scale to zero", "kubectl scale deployment/myapp --replicas=0", "scale-to-zero", packs.SeverityHigh},
		{"force delete pod", "kubectl delete pod mypod --force --grace-period=0", "delete-force", packs.SeverityCritical},
		{"context switch", "kubectl config use-context prod-cluster", "context-switch", packs.SeverityMedium},
		{"kubectx switch", "kubectx prod-cluster", "kubectx-switch", packs.SeverityMedium},
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
			if m.Severity != tc.wantSev {
				t.Errorf("Check(%q) severity %s, want %s", tc.cmdLine, m.Severity, tc.wantSev)
			}
		})
	}
}

func TestContainers_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "containers.docker")

	tests := []struct {
		name    string
		cmdLine string
		wantID  string
		wantSev packs.Severity
	}{
		{"system prune", "docker system prune", "system-prune", packs.SeverityHigh},
		{"volume prune", "docker volume prune", "volume-prune", packs.SeverityHigh},
		{"network prune", "docker network prune", "network-prune", packs.SeverityHigh},
		{"force rm", "docker rm -f my-container", "rm-force", packs.SeverityHigh},
		{"force rmi", "docker rmi -f my-image", "rmi-force", packs.SeverityHigh},
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

func TestInfrastructure_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "infrastructure.terraform")

	tests := []struct {
		name    string
		cmdLine string
		wantID  string
		wantSev packs.Severity
	}{
		{"terraform destroy", "terraform destroy", "destroy", packs.SeverityCritical},
		{"terraform apply auto-approve", "terraform apply -auto-approve", "apply-auto-approve", packs.SeverityHigh},
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

func TestCloud_PatternRuleIDs(t *testing.T) {
	p := loadPack(t, "cloud.aws")

	tests := []struct {
		name    string
		cmdLine string
		wantID  string
		wantSev packs.Severity
	}{
		{"ec2 terminate", "aws ec2 terminate-instances --instance-ids i-12345", "ec2-terminate", packs.SeverityCritical},
		{"s3 rb", "aws s3 rb s3://my-bucket", "s3-rb", packs.SeverityCritical},
		{"s3 rm recursive", "aws s3 rm s3://my-bucket/ --recursive", "s3-rm-recursive", packs.SeverityCritical},
		{"s3 sync delete", "aws s3 sync . s3://my-bucket/ --delete", "s3-sync-delete", packs.SeverityHigh},
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
