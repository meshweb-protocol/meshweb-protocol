package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/meshweb/meshweb-protocol/compliance"
	"github.com/meshweb/meshweb-protocol/compliance/report"
)

// Exit Codes Contract:
// 0 = Success (100% PASS)
// 1 = Compliance Failed
// 2 = Config Error
// 3 = Network Error
// 4 = Internal Harness Error (MUST NOT be interpreted as compliance failure)
const (
	ExitSuccess          = 0
	ExitComplianceFailed = 1
	ExitConfigError      = 2
	ExitNetworkError     = 3
	ExitInternalError    = 4
)

func main() {
	var target string
	var levelInt int
	var outDir string
	var timeoutSec int

	flag.StringVar(&target, "target", "127.0.0.1:4001", "Target libp2p multiaddr or host")
	flag.IntVar(&levelInt, "level", 4, "Target compliance level (1 to 5)")
	flag.StringVar(&outDir, "out", "./compliance-output", "Directory for certificate.json, HTML, and JUnit reports")
	flag.IntVar(&timeoutSec, "timeout", 60, "Execution timeout in seconds")
	flag.Parse()

	if levelInt < 1 || levelInt > 5 {
		fmt.Fprintf(os.Stderr, "[CONFIG ERROR] Invalid compliance level %d (Must be 1-5)\n", levelInt)
		os.Exit(ExitConfigError)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[CONFIG ERROR] Failed to create output directory: %v\n", err)
		os.Exit(ExitConfigError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cfg := compliance.Config{
		Target: target,
		Level:  compliance.Level(levelInt),
	}

	runner := compliance.NewRunner(cfg)

	fmt.Printf("====================================================\n")
	fmt.Printf(" MeshWeb Protocol Compliance Harness v1.0.0\n")
	fmt.Printf(" Target: %s | Target Level: %s (%d)\n", target, compliance.Level(levelInt).String(), levelInt)
	fmt.Printf("====================================================\n\n")

	res, err := runner.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[INTERNAL HARNESS ERROR] Unexpected runner crash: %v\n", err)
		fmt.Fprintf(os.Stderr, "NOTE: Internal harness errors MUST NOT be interpreted as compliance failures.\n")
		os.Exit(ExitInternalError)
	}

	// 1. Export JSON Certificate
	certPath := filepath.Join(outDir, "certificate.json")
	cf, err := os.Create(certPath)
	if err == nil {
		_ = report.ExportCertificate(res, cf)
		cf.Close()
		fmt.Printf("[REPORT] Certificate saved to: %s\n", certPath)
	}

	// 2. Export Full Result JSON
	jsonPath := filepath.Join(outDir, "results.json")
	jf, err := os.Create(jsonPath)
	if err == nil {
		_ = report.ExportJSON(res, jf)
		jf.Close()
		fmt.Printf("[REPORT] Full JSON results saved to: %s\n", jsonPath)
	}

	// 3. Export HTML Report
	htmlPath := filepath.Join(outDir, "compliance.html")
	hf, err := os.Create(htmlPath)
	if err == nil {
		_ = report.ExportHTML(res, hf)
		hf.Close()
		fmt.Printf("[REPORT] HTML report saved to: %s\n", htmlPath)
	}

	// 4. Export JUnit XML Report
	junitPath := filepath.Join(outDir, "junit.xml")
	uf, err := os.Create(junitPath)
	if err == nil {
		_ = report.ExportJUnit(res, uf)
		uf.Close()
		fmt.Printf("[REPORT] JUnit XML saved to: %s\n", junitPath)
	}

	fmt.Printf("\n----------------------------------------------------\n")
	fmt.Printf(" Execution Summary: %d Passed, %d Failed out of %d Tests\n", res.Summary.Passed, res.Summary.Failed, res.Summary.Total)
	fmt.Printf(" Max Compliance Level Certified: Level %d (%s)\n", res.Summary.MaxLevel, res.Summary.MaxLevel.String())
	fmt.Printf("----------------------------------------------------\n")

	if res.Summary.Failed > 0 {
		fmt.Printf("\n[COMPLIANCE FAIL] Target failed %d compliance test(s).\n", res.Summary.Failed)
		os.Exit(ExitComplianceFailed)
	}

	fmt.Printf("\n[COMPLIANCE PASS] Target certified at Level %d (%s)!\n", res.Summary.MaxLevel, res.Summary.MaxLevel.String())
	os.Exit(ExitSuccess)
}
