package compliance

import (
	"time"
)

type Level int

const (
	Level1WireCompatible       Level = 1
	Level2StorageCompatible    Level = 2
	Level3FaultTolerant        Level = 3
	Level4ProductionHardened   Level = 4
	Level5InteroperableStandard Level = 5
)

func (l Level) String() string {
	switch l {
	case Level1WireCompatible:
		return "Wire Compatible"
	case Level2StorageCompatible:
		return "Storage Compatible"
	case Level3FaultTolerant:
		return "Fault Tolerant"
	case Level4ProductionHardened:
		return "Production Hardened"
	case Level5InteroperableStandard:
		return "Interoperable Standard"
	default:
		return "Unknown Level"
	}
}

type TestStatus string

const (
	StatusPass TestStatus = "PASS"
	StatusFail TestStatus = "FAIL"
	StatusSkip TestStatus = "SKIP"
)

type TestResult struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Level       Level         `json:"level"`
	Status      TestStatus    `json:"status"`
	Duration    time.Duration `json:"duration_ns"`
	Description string        `json:"description"`
	Error       string        `json:"error,omitempty"`
}

type Certificate struct {
	CertificateVersion    int    `json:"certificate_version"`
	CertificateID         string `json:"certificate_id"`
	ProtocolVersion       string `json:"protocol_version"`
	ComplianceProfile     string `json:"compliance_profile"`
	ProfileVersion        int    `json:"profile_version"`
	SpecRevision          string `json:"spec_revision"`
	Implementation        string `json:"implementation"`
	ImplementationVersion string `json:"implementation_version"`
	RunnerVersion         string `json:"runner_version"`
	VectorSet             string `json:"vector_set"`
	ComplianceLevel       Level  `json:"compliance_level"`
	LevelName             string `json:"level_name"`
	ReferenceIndependent  bool   `json:"reference_independent"`
	VerificationMethod    string `json:"verification_method"`
	TotalTests            int    `json:"total_tests"`
	PassedTests           int    `json:"passed_tests"`
	FailedTests           int    `json:"failed_tests"`
	IssuedAt              int64  `json:"issued_at"`
	Signature             string `json:"signature"`
}

type Summary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Skipped  int `json:"skipped"`
	MaxLevel Level `json:"max_level_certified"`
}

type Result struct {
	Certificate Certificate   `json:"certificate"`
	Summary     Summary       `json:"summary"`
	Tests       []TestResult  `json:"tests"`
	Duration    time.Duration `json:"duration_ns"`
}
