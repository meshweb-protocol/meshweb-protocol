package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/meshweb/meshweb-protocol/compliance"
)

func ExportJSON(res *compliance.Result, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func ExportCertificate(res *compliance.Result, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res.Certificate)
}

func ExportHTML(res *compliance.Result, w io.Writer) error {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>MeshWeb Protocol Compliance Report</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #0f172a; color: #f8fafc; margin: 20px; }
        h1 { color: #38bdf8; }
        .card { background: #1e293b; padding: 20px; border-radius: 8px; margin-bottom: 20px; box-shadow: 0 4px 6px rgba(0,0,0,0.3); }
        .badge { background: #22c55e; color: #000; padding: 4px 8px; border-radius: 4px; font-weight: bold; }
        table { width: 100%%; border-collapse: collapse; margin-top: 10px; }
        th, td { padding: 12px; border-bottom: 1px solid #334155; text-align: left; }
        th { background: #0f172a; color: #94a3b8; }
        .pass { color: #4ade80; font-weight: bold; }
        .fail { color: #f87171; font-weight: bold; }
    </style>
</head>
<body>
    <h1>MeshWeb Protocol Compliance Report</h1>
    <div class="card">
        <h2>Certification Status: <span class="badge">%s (Level %d)</span></h2>
        <p><strong>Implementation:</strong> %s (v%s)</p>
        <p><strong>Runner Version:</strong> %s</p>
        <p><strong>Vector Set:</strong> %s</p>
        <p><strong>Certificate ID:</strong> %s</p>
        <p><strong>Passed Tests:</strong> %d / %d</p>
    </div>

    <div class="card">
        <h2>Test Execution Results</h2>
        <table>
            <thead>
                <tr>
                    <th>Test ID</th>
                    <th>Name</th>
                    <th>Level</th>
                    <th>Status</th>
                    <th>Duration</th>
                    <th>Description</th>
                </tr>
            </thead>
            <tbody>
`, res.Certificate.LevelName, res.Certificate.ComplianceLevel, res.Certificate.Implementation, res.Certificate.ImplementationVersion, res.Certificate.RunnerVersion, res.Certificate.VectorSet, res.Certificate.CertificateID, res.Summary.Passed, res.Summary.Total)

	for _, t := range res.Tests {
		statusClass := "pass"
		if t.Status == compliance.StatusFail {
			statusClass = "fail"
		}
		html += fmt.Sprintf(`                <tr>
                    <td><code>%s</code></td>
                    <td>%s</td>
                    <td>Level %d</td>
                    <td class="%s">%s</td>
                    <td>%v</td>
                    <td>%s</td>
                </tr>
`, t.ID, t.Name, t.Level, statusClass, t.Status, t.Duration, t.Description)
	}

	html += `            </tbody>
        </table>
    </div>
</body>
</html>`

	_, err := io.WriteString(w, html)
	return err
}

func ExportJUnit(res *compliance.Result, w io.Writer) error {
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="meshweb-compliance" tests="%d" failures="%d" errors="0" time="%.3f">
`, res.Summary.Total, res.Summary.Failed, res.Duration.Seconds())

	for _, t := range res.Tests {
		xml += fmt.Sprintf(`    <testcase classname="compliance" name="%s" time="%.3f">
`, t.ID, t.Duration.Seconds())
		if t.Status == compliance.StatusFail {
			xml += fmt.Sprintf(`        <failure message="%s">%s</failure>
`, t.Error, t.Description)
		}
		xml += `    </testcase>
`
	}
	xml += `</testsuite>`

	_, err := io.WriteString(w, xml)
	return err
}
