package packs_test

import (
	"testing"

	"github.com/thgrace/training-wheels/internal/packs"
)

// TestKubectl_StructuralPatterns verifies that known-destructive kubectl
// commands are blocked by the kubernetes.kubectl pack.
func TestKubectl_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("kubernetes.kubectl")
	if p == nil {
		t.Skipf("pack kubernetes.kubectl not registered")
	}

	tests := []struct {
		name string
		cmd  string
	}{
		{"delete namespace", "kubectl delete namespace production"},
		{"delete ns", "kubectl delete ns production"},
		{"delete all", "kubectl delete --all pods"},
		{"drain node", "kubectl drain node-1"},
		{"cordon node", "kubectl cordon node-1"},
		// The kubectl pack blocks :NoExecute taints (immediate pod eviction).
		// :NoSchedule is intentionally not blocked by this pack.
		{"taint node NoExecute", "kubectl taint nodes node-1 key=value:NoExecute"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected kubernetes.kubectl to block %q, but it was allowed", tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s reason=%s", m.Name, m.Severity, m.Reason)
		})
	}
}

// TestKubectl_SafePatterns verifies that read-only and dry-run kubectl commands
// are allowed by the kubernetes.kubectl pack.
func TestKubectl_SafePatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("kubernetes.kubectl")
	if p == nil {
		t.Skipf("pack kubernetes.kubectl not registered")
	}

	safeCmds := []string{
		"kubectl get pods",
		"kubectl describe pod my-pod",
		"kubectl logs my-pod",
		"kubectl top nodes",
		"kubectl config view",
		"kubectl explain pod",
		"kubectl api-resources",
		"kubectl diff -f manifest.yaml",
		"kubectl delete pod my-pod --dry-run=client",
		"kubectl apply --server-side --force-conflicts -f deploy.yaml",
	}

	for _, cmd := range safeCmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m != nil {
				t.Fatalf("expected %q to be allowed, but it was blocked: name=%s severity=%s", cmd, m.Name, m.Severity)
			}
		})
	}
}

// TestHelm_StructuralPatterns verifies that helm uninstall and rollback are
// blocked by the kubernetes.helm pack.
func TestHelm_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("kubernetes.helm")
	if p == nil {
		t.Skipf("pack kubernetes.helm not registered")
	}

	tests := []struct {
		name string
		cmd  string
	}{
		{"uninstall", "helm uninstall my-release"},
		{"rollback", "helm rollback my-release 1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected kubernetes.helm to block %q, but it was allowed", tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s reason=%s", m.Name, m.Severity, m.Reason)
		})
	}
}

// TestDocker_StructuralPatterns verifies that destructive docker commands are
// blocked by the containers.docker pack.
func TestDocker_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("containers.docker")
	if p == nil {
		t.Skipf("pack containers.docker not registered")
	}

	tests := []struct {
		name string
		cmd  string
	}{
		{"system prune", "docker system prune -af"},
		{"system prune force", "docker system prune --all --force"},
		{"container prune", "docker container prune -f"},
		{"image prune", "docker image prune -af"},
		{"volume prune", "docker volume prune -f"},
		{"rm force", "docker rm -f container1"},
		{"rmi force", "docker rmi -f image1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected containers.docker to block %q, but it was allowed", tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s reason=%s", m.Name, m.Severity, m.Reason)
		})
	}
}

// TestDocker_SafePatterns verifies that read-only docker commands are allowed
// by the containers.docker pack.
func TestDocker_SafePatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("containers.docker")
	if p == nil {
		t.Skipf("pack containers.docker not registered")
	}

	safeCmds := []string{
		"docker ps",
		"docker images",
		"docker logs container1",
		"docker inspect container1",
		"docker stats",
		"docker version",
		"docker info",
	}

	for _, cmd := range safeCmds {
		t.Run(cmd, func(t *testing.T) {
			m := p.Check(cmd)
			if m != nil {
				t.Fatalf("expected %q to be allowed, but it was blocked: name=%s severity=%s", cmd, m.Name, m.Severity)
			}
		})
	}
}

// TestDatabase_StructuralPatterns verifies that destructive database commands
// are blocked across the postgresql, mysql, mongodb, and redis packs.
func TestDatabase_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()

	tests := []struct {
		name   string
		cmd    string
		packID string
	}{
		{"psql drop database", `psql -c "DROP DATABASE production"`, "database.postgresql"},
		{"psql drop table", `psql -c "DROP TABLE users"`, "database.postgresql"},
		{"mysql drop database", `mysql -e "DROP DATABASE production"`, "database.mysql"},
		{"mongosh dropDatabase", "mongosh --eval 'db.dropDatabase()'", "database.mongodb"},
		{"redis flushall", "redis-cli FLUSHALL", "database.redis"},
		{"redis flushdb", "redis-cli FLUSHDB", "database.redis"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := reg.Get(tc.packID)
			if p == nil {
				t.Skipf("pack %s not registered", tc.packID)
			}
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected %s to block %q, but it was allowed", tc.packID, tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s reason=%s", m.Name, m.Severity, m.Reason)
		})
	}
}

func TestDatabase_SafePatterns(t *testing.T) {
	reg := packs.DefaultRegistry()

	tests := []struct {
		name   string
		cmd    string
		packID string
	}{
		{"psql select string literal", `psql -c "SELECT 'DROP TABLE users';"`, "database.postgresql"},
		{"mysql select string literal", `mysql -e "SELECT 'DROP TABLE users';"`, "database.mysql"},
		{"sqlite select string literal", `sqlite3 app.db "SELECT 'DROP TABLE users';"`, "database.sqlite"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := reg.Get(tc.packID)
			if p == nil {
				t.Skipf("pack %s not registered", tc.packID)
			}
			m := p.Check(tc.cmd)
			if m != nil {
				t.Fatalf("expected %s to allow %q, but it was blocked: name=%s severity=%s",
					tc.packID, tc.cmd, m.Name, m.Severity)
			}
		})
	}
}

// TestInfrastructure_StructuralPatterns verifies that destructive infrastructure
// commands are blocked by the terraform, ansible, and pulumi packs.
func TestInfrastructure_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()

	tests := []struct {
		name   string
		cmd    string
		packID string
	}{
		{"terraform destroy", "terraform destroy", "infrastructure.terraform"},
		{"terraform destroy auto", "terraform destroy -auto-approve", "infrastructure.terraform"},
		{"ansible shell rm-rf", "ansible all -m shell -a 'rm -rf /var/old'", "infrastructure.ansible"},
		{"pulumi destroy", "pulumi destroy", "infrastructure.pulumi"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := reg.Get(tc.packID)
			if p == nil {
				t.Skipf("pack %s not registered", tc.packID)
			}
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected %s to block %q, but it was allowed", tc.packID, tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s reason=%s", m.Name, m.Severity, m.Reason)
		})
	}
}

func TestInfrastructure_SafePatterns(t *testing.T) {
	reg := packs.DefaultRegistry()

	tests := []struct {
		name   string
		cmd    string
		packID string
	}{
		{"terraform plan destroy preview", "terraform plan -destroy", "infrastructure.terraform"},
		{"ansible benign playbook", "ansible-playbook -i inventory ping.yml", "infrastructure.ansible"},
		{"pulumi preview", "pulumi preview", "infrastructure.pulumi"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := reg.Get(tc.packID)
			if p == nil {
				t.Skipf("pack %s not registered", tc.packID)
			}
			m := p.Check(tc.cmd)
			if m != nil {
				t.Fatalf("expected %s to allow %q, but it was blocked: name=%s severity=%s",
					tc.packID, tc.cmd, m.Name, m.Severity)
			}
		})
	}
}

// TestSystemDisk_StructuralPatterns verifies that destructive disk operations
// are blocked by the system.disk pack.
// disk_tests.rs.
func TestSystemDisk_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("system.disk")
	if p == nil {
		t.Skipf("pack system.disk not registered")
	}

	tests := []struct {
		name string
		cmd  string
	}{
		{"dd to block device", "dd if=foo of=/dev/sda"},
		{"mdadm stop", "mdadm --stop /dev/md0"},
		{"mdadm zero-superblock", "mdadm --zero-superblock /dev/sda1"},
		{"mdadm create", "mdadm --create /dev/md0 --level=1 --raid-devices=2 /dev/sda1 /dev/sdb1"},
		{"btrfs subvolume delete", "btrfs subvolume delete /mnt/data/snapshot"},
		{"btrfs device remove", "btrfs device remove /dev/sdb /mnt/data"},
		{"btrfs check repair", "btrfs check --repair /dev/sda1"},
		{"dmsetup remove", "dmsetup remove my_device"},
		{"dmsetup remove_all", "dmsetup remove_all"},
		{"dmsetup wipe_table", "dmsetup wipe_table my_device"},
		{"nbd-client disconnect", "nbd-client -d /dev/nbd0"},
		{"pvremove", "pvremove /dev/sda1"},
		{"vgremove", "vgremove my_vg"},
		{"lvremove", "lvremove /dev/my_vg/my_lv"},
		{"lvreduce", "lvreduce -L 10G /dev/my_vg/my_lv"},
		{"pvmove", "pvmove /dev/sda1 /dev/sdb1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected system.disk to block %q, but it was allowed", tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s", m.Name, m.Severity)
		})
	}
}

// TestSystemDisk_SafePatterns verifies that read-only and informational disk
// commands are allowed by the system.disk pack.
// disk_tests.rs.
func TestSystemDisk_SafePatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("system.disk")
	if p == nil {
		t.Skipf("pack system.disk not registered")
	}

	safeCmds := []struct {
		name string
		cmd  string
	}{
		{"dd to /dev/null", "dd if=zero.dat of=/dev/null bs=1M count=1"},
		{"mdadm detail", "mdadm --detail /dev/md0"},
		{"mdadm examine", "mdadm --examine /dev/sda1"},
		{"btrfs subvolume list", "btrfs subvolume list /mnt/data"},
		{"btrfs filesystem show", "btrfs filesystem show"},
		{"dmsetup ls", "dmsetup ls"},
		{"dmsetup status", "dmsetup status"},
		{"nbd-client list", "nbd-client -l server.example.com"},
		{"lvs", "lvs"},
		{"vgdisplay", "vgdisplay my_vg"},
	}

	for _, tc := range safeCmds {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m != nil {
				t.Fatalf("expected %q to be allowed, but it was blocked: name=%s severity=%s",
					tc.cmd, m.Name, m.Severity)
			}
		})
	}
}

// TestSystemPermissions_StructuralPatterns verifies that dangerous permission
// changes are blocked by the system.permissions pack.
// permissions_tests.rs.
func TestSystemPermissions_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("system.permissions")
	if p == nil {
		t.Skipf("pack system.permissions not registered")
	}

	tests := []struct {
		name string
		cmd  string
	}{
		{"chmod 777", "chmod 777 /etc/passwd"},
		{"chmod 0777", "chmod 0777 /etc"},
		{"chmod recursive root", "chmod --recursive 755 /etc"},
		{"chown recursive root", "chown -R root:root /etc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected system.permissions to block %q, but it was allowed", tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s", m.Name, m.Severity)
		})
	}
}

// TestSystemPermissions_SafePatterns verifies that safe permission-related
// commands are allowed by the system.permissions pack.
// permissions_tests.rs.
func TestSystemPermissions_SafePatterns(t *testing.T) {
	reg := packs.DefaultRegistry()
	p := reg.Get("system.permissions")
	if p == nil {
		t.Skipf("pack system.permissions not registered")
	}

	safeCmds := []struct {
		name string
		cmd  string
	}{
		{"chmod 644 on file with 777 in name", "chmod 644 file_777"},
		{"chmod -R 644 on file with 777 in name", "chmod -R 644 file_777"},
	}

	for _, tc := range safeCmds {
		t.Run(tc.name, func(t *testing.T) {
			m := p.Check(tc.cmd)
			if m != nil {
				t.Fatalf("expected %q to be allowed, but it was blocked: name=%s severity=%s",
					tc.cmd, m.Name, m.Severity)
			}
		})
	}
}

// TestSecrets_StructuralPatterns verifies that destructive secret management
// commands are blocked by the secrets.* packs.
func TestSecrets_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()

	tests := []struct {
		name   string
		cmd    string
		packID string
	}{
		// Bitwarden
		{"bw delete", "bw delete item 12345", "secrets.bitwarden"},
		{"bw edit", "bw edit item 12345", "secrets.bitwarden"},
		{"bw move", "bw move item 12345 org-id", "secrets.bitwarden"},

		// GCP Secret Manager
		{"gcloud secrets delete", "gcloud secrets delete my-secret", "secrets.gcp_secrets"},
		{"gcloud secrets versions destroy", "gcloud secrets versions destroy 1 --secret=my-secret", "secrets.gcp_secrets"},
		{"gcloud secrets versions disable", "gcloud secrets versions disable 1 --secret=my-secret", "secrets.gcp_secrets"},
		{"gcloud secrets update", "gcloud secrets update my-secret --update-labels=env=prod", "secrets.gcp_secrets"},
		{"gcloud secrets set-iam-policy", "gcloud secrets set-iam-policy my-secret policy.json", "secrets.gcp_secrets"},
		{"gcloud secrets remove-iam-policy-binding", "gcloud secrets remove-iam-policy-binding my-secret --member=user:a@b.com --role=roles/viewer", "secrets.gcp_secrets"},

		// Azure Key Vault
		{"az keyvault secret delete", "az keyvault secret delete --name mysecret --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault secret purge", "az keyvault secret purge --name mysecret --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault key delete", "az keyvault key delete --name mykey --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault key purge", "az keyvault key purge --name mykey --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault certificate delete", "az keyvault certificate delete --name mycert --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault certificate purge", "az keyvault certificate purge --name mycert --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault purge", "az keyvault purge --name myvault", "secrets.azure_keyvault"},

		// LastPass
		{"lpass rm", "lpass rm mysite", "secrets.lastpass"},
		{"lpass edit", "lpass edit mysite", "secrets.lastpass"},
		{"lpass mv", "lpass mv mysite Business", "secrets.lastpass"},

		// Infisical
		{"infisical secrets delete", "infisical secrets delete MY_SECRET", "secrets.infisical"},
		{"infisical secrets set", "infisical secrets set MY_SECRET=value", "secrets.infisical"},

		// Pass
		{"pass rm", "pass rm email/personal", "secrets.pass"},
		{"pass mv", "pass mv email/old email/new", "secrets.pass"},
		{"pass edit", "pass edit email/personal", "secrets.pass"},
		{"pass insert", "pass insert email/new", "secrets.pass"},
		{"pass generate", "pass generate email/new 20", "secrets.pass"},

		// Gopass
		{"gopass rm", "gopass rm email/personal", "secrets.gopass"},
		{"gopass delete", "gopass delete email/personal", "secrets.gopass"},
		{"gopass mv", "gopass mv email/old email/new", "secrets.gopass"},
		{"gopass move", "gopass move email/old email/new", "secrets.gopass"},
		{"gopass edit", "gopass edit email/personal", "secrets.gopass"},
		{"gopass insert", "gopass insert email/new", "secrets.gopass"},
		{"gopass generate", "gopass generate email/new 20", "secrets.gopass"},
		{"gopass mounts remove", "gopass mounts remove mystore", "secrets.gopass"},

		// Conjur
		{"conjur variable set", "conjur variable set -i prod/db/password -v secret123", "secrets.conjur"},
		{"conjur policy load", "conjur policy load --replace root policy.yml", "secrets.conjur"},
		{"conjur host rotate-api-key", "conjur host rotate-api-key -i myapp", "secrets.conjur"},
		{"conjur user rotate-api-key", "conjur user rotate-api-key -i admin", "secrets.conjur"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := reg.Get(tc.packID)
			if p == nil {
				t.Skipf("pack %s not registered", tc.packID)
			}
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected %s to block %q, but it was allowed", tc.packID, tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s reason=%s", m.Name, m.Severity, m.Reason)
		})
	}
}

// TestSecrets_SafePatterns verifies that read-only secret management commands
// are allowed by the secrets.* packs.
func TestSecrets_SafePatterns(t *testing.T) {
	reg := packs.DefaultRegistry()

	tests := []struct {
		name   string
		cmd    string
		packID string
	}{
		// Bitwarden
		{"bw status", "bw status", "secrets.bitwarden"},
		{"bw list", "bw list items", "secrets.bitwarden"},
		{"bw get", "bw get item 12345", "secrets.bitwarden"},
		{"bw export", "bw export", "secrets.bitwarden"},
		{"bw sync", "bw sync", "secrets.bitwarden"},

		// GCP Secret Manager
		{"gcloud secrets list", "gcloud secrets list", "secrets.gcp_secrets"},
		{"gcloud secrets describe", "gcloud secrets describe my-secret", "secrets.gcp_secrets"},
		{"gcloud secrets versions list", "gcloud secrets versions list my-secret", "secrets.gcp_secrets"},
		{"gcloud secrets versions access", "gcloud secrets versions access latest --secret=my-secret", "secrets.gcp_secrets"},
		{"gcloud secrets get-iam-policy", "gcloud secrets get-iam-policy my-secret", "secrets.gcp_secrets"},

		// Azure Key Vault
		{"az keyvault secret list", "az keyvault secret list --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault secret show", "az keyvault secret show --name mysecret --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault key list", "az keyvault key list --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault key show", "az keyvault key show --name mykey --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault certificate list", "az keyvault certificate list --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault certificate show", "az keyvault certificate show --name mycert --vault-name myvault", "secrets.azure_keyvault"},
		{"az keyvault show", "az keyvault show --name myvault", "secrets.azure_keyvault"},

		// LastPass
		{"lpass status", "lpass status", "secrets.lastpass"},
		{"lpass ls", "lpass ls", "secrets.lastpass"},
		{"lpass show", "lpass show mysite", "secrets.lastpass"},
		{"lpass export", "lpass export", "secrets.lastpass"},

		// Infisical
		{"infisical run", "infisical run -- npm start", "secrets.infisical"},
		{"infisical login", "infisical login", "secrets.infisical"},
		{"infisical export", "infisical export", "secrets.infisical"},
		{"infisical secrets get", "infisical secrets get MY_SECRET", "secrets.infisical"},
		{"infisical scan", "infisical scan", "secrets.infisical"},

		// Pass
		{"pass ls", "pass ls", "secrets.pass"},
		{"pass show", "pass show email/personal", "secrets.pass"},
		{"pass find", "pass find email", "secrets.pass"},
		{"pass grep", "pass grep pattern", "secrets.pass"},
		{"pass version", "pass version", "secrets.pass"},

		// Gopass
		{"gopass ls", "gopass ls", "secrets.gopass"},
		{"gopass show", "gopass show email/personal", "secrets.gopass"},
		{"gopass find", "gopass find email", "secrets.gopass"},
		{"gopass audit", "gopass audit", "secrets.gopass"},
		{"gopass cat", "gopass cat email/personal", "secrets.gopass"},
		{"gopass version", "gopass version", "secrets.gopass"},
		{"gopass list", "gopass list", "secrets.gopass"},
		{"gopass mounts", "gopass mounts", "secrets.gopass"},

		// Conjur
		{"conjur list", "conjur list", "secrets.conjur"},
		{"conjur show", "conjur show myresource", "secrets.conjur"},
		{"conjur variable get", "conjur variable get -i prod/db/password", "secrets.conjur"},
		{"conjur variable value", "conjur variable value -i prod/db/password", "secrets.conjur"},
		{"conjur whoami", "conjur whoami", "secrets.conjur"},
		{"conjur check", "conjur check -i myresource -p read", "secrets.conjur"},
		{"conjur resource show", "conjur resource show myresource", "secrets.conjur"},
		{"conjur role memberships", "conjur role memberships myrole", "secrets.conjur"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := reg.Get(tc.packID)
			if p == nil {
				t.Skipf("pack %s not registered", tc.packID)
			}
			m := p.Check(tc.cmd)
			if m != nil {
				t.Fatalf("expected %s to allow %q, but it was blocked: name=%s severity=%s",
					tc.packID, tc.cmd, m.Name, m.Severity)
			}
		})
	}
}

// TestCloud_StructuralPatterns verifies that destructive cloud CLI commands are
// blocked by the cloud.aws and cloud.gcp packs.
func TestCloud_StructuralPatterns(t *testing.T) {
	reg := packs.DefaultRegistry()

	tests := []struct {
		name   string
		cmd    string
		packID string
	}{
		{"aws ec2 terminate", "aws ec2 terminate-instances --instance-ids i-123", "cloud.aws"},
		{"aws s3 rm recursive", "aws s3 rm s3://bucket --recursive", "cloud.aws"},
		{"gcloud delete instance", "gcloud compute instances delete my-instance", "cloud.gcp"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := reg.Get(tc.packID)
			if p == nil {
				t.Skipf("pack %s not registered", tc.packID)
			}
			m := p.Check(tc.cmd)
			if m == nil {
				t.Fatalf("expected %s to block %q, but it was allowed", tc.packID, tc.cmd)
			}
			t.Logf("blocked: name=%s severity=%s reason=%s", m.Name, m.Severity, m.Reason)
		})
	}
}
