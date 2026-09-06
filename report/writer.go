package report

import (
	"fmt"
	"os"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
)

// WriteAll generates and writes all four required outputs to the specified output directory.
// If outDir is empty, writes to the current directory.
func WriteAll(r *result.Report, dataDir, outDir string, redactor Redactor) error {
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("create outdir: %w", err)
		}
	}

	// 1. report.md
	md, err := ToMarkdown(r)
	if err != nil {
		return fmt.Errorf("generate markdown: %w", err)
	}
	if err := os.WriteFile(path(outDir, "report.md"), md, 0644); err != nil {
		return err
	}

	// 2. report.json
	js, err := ToJSON(r)
	if err != nil {
		return fmt.Errorf("generate json: %w", err)
	}
	if err := os.WriteFile(path(outDir, "report.json"), js, 0644); err != nil {
		return err
	}

	// 3. results.xml
	xmlOut, err := ToJUnit(r)
	if err != nil {
		return fmt.Errorf("generate junit: %w", err)
	}
	if err := os.WriteFile(path(outDir, "results.xml"), xmlOut, 0644); err != nil {
		return err
	}

	// 4. diagnostics/
	if dataDir != "" {
		diagDir := path(outDir, "diagnostics")
		if err := WriteDiagnostics(dataDir, diagDir, redactor); err != nil {
			return fmt.Errorf("write diagnostics: %w", err)
		}
	}

	return nil
}

func path(dir, file string) string {
	if dir == "" {
		return file
	}
	return dir + "/" + file
}
