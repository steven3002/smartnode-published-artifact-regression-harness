package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/redact"
)

func TestWriteDiagnosticsSanitisation(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Seed with known sensitive data
	jwt := "0x7a30cf1db902e4eb245d625ed8a1768846179375cd5539bc277c151fb789396e"
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
	password := "supersecretpassword123!"

	// Create test files
	file1 := filepath.Join(srcDir, "daemon.log")
	content1 := "Starting daemon... using jwt " + jwt + " and password " + password + "\n"
	os.WriteFile(file1, []byte(content1), 0644)

	file2 := filepath.Join(srcDir, "wallet.txt")
	content2 := "Mnemonic: " + mnemonic + "\nDon't lose it!"
	os.WriteFile(file2, []byte(content2), 0644)

	subDir := filepath.Join(srcDir, "logs")
	os.Mkdir(subDir, 0755)
	file3 := filepath.Join(subDir, "error.log")
	content3 := jwt + " is the jwt"
	os.WriteFile(file3, []byte(content3), 0644)

	r := redact.New()
	r.AddLiteral(password)

	err := WriteDiagnostics(srcDir, destDir, r)
	if err != nil {
		t.Fatalf("WriteDiagnostics failed: %v", err)
	}

	// Verify all files were copied and redacted
	err = filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		content := string(data)
		if strings.Contains(content, jwt) {
			t.Errorf("File %s leaked JWT", path)
		}
		if strings.Contains(content, mnemonic) {
			t.Errorf("File %s leaked mnemonic", path)
		}
		if strings.Contains(content, password) {
			t.Errorf("File %s leaked password", path)
		}

		if !strings.Contains(content, "[REDACTED]") {
			t.Errorf("File %s missing [REDACTED] tag. Original content may not have been matched.", path)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed verifying copied files: %v", err)
	}
}
