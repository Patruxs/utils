package scripts_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMacOSKeychainSmokeScriptWritesOnlySanitizedEvidence(t *testing.T) {
	script := readScript(t, filepath.Join("..", "..", "scripts", "smoke-macos-keychain.sh"))
	body := keychainEvidenceBody(t, script)

	for _, want := range []string{
		"utils-macos-keychain-smoke",
		"platform",
		"status",
		"timestamp_utc",
		"command",
		"message",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("macOS Keychain smoke evidence writer missing safe field %q:\n%s", want, body)
		}
	}

	for _, forbidden := range []string{
		"keychain_path",
		"keychain_password",
		"old_keychains_file",
		"HOME",
		"UTILS_KEYCHAIN_SMOKE",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("macOS Keychain smoke evidence writer includes forbidden runtime marker %q:\n%s", forbidden, body)
		}
	}
}

func TestMacOSKeychainSmokeWorkflowUploadsSanitizedEvidence(t *testing.T) {
	workflow := readScript(t, filepath.Join("..", "..", ".github", "workflows", "macos-keychain-smoke.yml"))

	for _, want := range []string{
		"workflow_dispatch:",
		"runs-on: macos-latest",
		"actions/setup-go@v5",
		"UTILS_KEYCHAIN_SMOKE_EVIDENCE_PATH: ./macos-keychain-smoke.json",
		"bash scripts/smoke-macos-keychain.sh",
		"actions/upload-artifact@v4",
		"name: macos-keychain-smoke",
		"path: macos-keychain-smoke.json",
		"if-no-files-found: ignore",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("macOS Keychain smoke workflow missing %q:\n%s", want, workflow)
		}
	}
}

func TestMacOSKeychainSmokeScriptUsesHostedMacOSCompatibleKeychainSetup(t *testing.T) {
	script := readScript(t, filepath.Join("..", "..", "scripts", "smoke-macos-keychain.sh"))

	for _, want := range []string{
		`old_default_keychain_file="$tmp_dir/default-keychain.txt"`,
		`security default-keychain -d user | sed 's/^ *"//; s/"$//' >"$old_default_keychain_file"`,
		`security default-keychain -d user -s "$keychain_path"`,
		`security default-keychain -d user -s "$old_default_keychain"`,
		`export UTILS_KEYCHAIN_PATH="$keychain_path"`,
		`while IFS= read -r old_keychain; do`,
		`old_keychains+=("$old_keychain")`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("macOS Keychain smoke script missing hosted macOS setup %q:\n%s", want, script)
		}
	}

	if strings.Contains(script, "mapfile ") {
		t.Fatal("macOS Keychain smoke script must not use mapfile because hosted macOS uses Bash 3")
	}
}

func keychainEvidenceBody(t *testing.T, script string) string {
	t.Helper()
	start := strings.Index(script, "write_keychain_evidence()")
	if start < 0 {
		t.Fatal("macOS Keychain smoke script missing write_keychain_evidence")
	}
	end := strings.Index(script[start:], "if [[ \"$(uname -s)\" != \"Darwin\" ]]")
	if end < 0 {
		t.Fatal("macOS Keychain smoke evidence body end marker missing")
	}
	return script[start : start+end]
}
