package shellcontext

import "testing"

func TestLookupSafeCommand_Echo(t *testing.T) {
	entry := LookupSafeCommand("echo")
	if entry == nil {
		t.Fatal("expected entry for echo")
	}
	if entry.Mode != SafeArgAll {
		t.Errorf("echo mode = %d, want SafeArgAll", entry.Mode)
	}
}

func TestLookupSafeCommand_Git(t *testing.T) {
	entry := LookupSafeCommand("git")
	if entry == nil {
		t.Fatal("expected entry for git")
	}
	if entry.Mode != SafeArgFlags {
		t.Errorf("git mode = %d, want SafeArgFlags", entry.Mode)
	}
	if !entry.SafeFlags["-m"] {
		t.Error("git -m should be a safe flag")
	}
	if !entry.SafeFlags["--message"] {
		t.Error("git --message should be a safe flag")
	}
}

func TestLookupSafeCommand_Grep(t *testing.T) {
	entry := LookupSafeCommand("grep")
	if entry == nil {
		t.Fatal("expected entry for grep")
	}
	if entry.Mode != SafeArgAll {
		t.Errorf("grep mode = %d, want SafeArgAll", entry.Mode)
	}
}

func TestLookupSafeCommand_Curl(t *testing.T) {
	entry := LookupSafeCommand("curl")
	if entry == nil {
		t.Fatal("expected entry for curl")
	}
	if !entry.SafeFlags["-d"] {
		t.Error("curl -d should be a safe flag")
	}
	if !entry.SafeFlags["--data"] {
		t.Error("curl --data should be a safe flag")
	}
}

func TestLookupSafeCommand_Unknown(t *testing.T) {
	entry := LookupSafeCommand("randomcmd")
	if entry != nil {
		t.Error("expected nil for unknown command")
	}
}

func TestIsSafeFlag_GitMessage(t *testing.T) {
	if !IsSafeFlag("git", "--message") {
		t.Error("git --message should be safe")
	}
}

func TestIsSafeFlag_GitInvalid(t *testing.T) {
	if IsSafeFlag("git", "--status") {
		t.Error("git --status should not be safe")
	}
}

func TestLookupSafeCommand_Test(t *testing.T) {
	entry := LookupSafeCommand("test")
	if entry == nil || entry.Mode != SafeArgAll {
		t.Error("test should be SafeArgAll")
	}
}

func TestLookupSafeCommand_Bracket(t *testing.T) {
	entry := LookupSafeCommand("[")
	if entry == nil || entry.Mode != SafeArgAll {
		t.Error("[ should be SafeArgAll")
	}
}

func TestIsSafeFlag_Find(t *testing.T) {
	if !IsSafeFlag("find", "-name") {
		t.Error("find -name should be safe")
	}
}

func TestIsSafeFlag_Docker(t *testing.T) {
	if !IsSafeFlag("docker", "--name") {
		t.Error("docker --name should be safe")
	}
}

func TestIsSafeFlag_Kubectl(t *testing.T) {
	if !IsSafeFlag("kubectl", "-n") {
		t.Error("kubectl -n should be safe")
	}
}

func TestIsSafeFlag_NonexistentCommand(t *testing.T) {
	if IsSafeFlag("nosuchcmd", "-x") {
		t.Error("unknown command should not have safe flags")
	}
}
