package compliance_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/meshweb/meshweb-protocol/compliance"
	"github.com/meshweb/meshweb-protocol/compliance/report"
)

func TestComplianceRunnerLibrary(t *testing.T) {
	cfg := compliance.Config{
		Target: "127.0.0.1:4001",
		Level:  compliance.Level4ProductionHardened,
	}

	runner := compliance.NewRunner(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Runner failed: %v", err)
	}

	if res.Summary.Total == 0 {
		t.Fatalf("Expected registered tests, got 0")
	}

	if res.Summary.Failed > 0 {
		t.Fatalf("Expected 0 failures in standard compliance run, got %d", res.Summary.Failed)
	}

	if res.Certificate.ComplianceLevel != compliance.Level4ProductionHardened {
		t.Fatalf("Expected Level 4 certification, got %v", res.Certificate.ComplianceLevel)
	}

	// Verify JSON Certificate Export
	var certBuf bytes.Buffer
	if err := report.ExportCertificate(res, &certBuf); err != nil {
		t.Fatalf("ExportCertificate failed: %v", err)
	}
	if !strings.Contains(certBuf.String(), "certified_by_meshweb_tsc_v1") {
		t.Fatalf("Certificate JSON missing signature")
	}

	// Verify HTML Report Export
	var htmlBuf bytes.Buffer
	if err := report.ExportHTML(res, &htmlBuf); err != nil {
		t.Fatalf("ExportHTML failed: %v", err)
	}
	if !strings.Contains(htmlBuf.String(), "MeshWeb Protocol Compliance Report") {
		t.Fatalf("HTML report missing header title")
	}

	// Verify JUnit XML Export
	var junitBuf bytes.Buffer
	if err := report.ExportJUnit(res, &junitBuf); err != nil {
		t.Fatalf("ExportJUnit failed: %v", err)
	}
	if !strings.Contains(junitBuf.String(), "<testsuite name=\"meshweb-compliance\"") {
		t.Fatalf("JUnit report missing XML testsuite element")
	}
}
