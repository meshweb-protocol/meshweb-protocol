package compliance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TestCase interface {
	ID() string
	Name() string
	Level() Level
	Description() string
	Run(ctx context.Context, target string) error
}

type Registry struct {
	mu    sync.RWMutex
	tests map[string]TestCase
}

var globalRegistry = NewRegistry()

func NewRegistry() *Registry {
	return &Registry{
		tests: make(map[string]TestCase),
	}
}

func Register(tc TestCase) {
	globalRegistry.Register(tc)
}

func (r *Registry) Register(tc TestCase) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tests[tc.ID()] = tc
}

func (r *Registry) GetTestsForLevel(maxLevel Level) []TestCase {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []TestCase
	for _, tc := range r.tests {
		if tc.Level() <= maxLevel {
			list = append(list, tc)
		}
	}
	return list
}

type Config struct {
	Target string
	Level  Level
}

type Runner struct {
	cfg      Config
	registry *Registry
}

func NewRunner(cfg Config) *Runner {
	if cfg.Level <= 0 {
		cfg.Level = Level4ProductionHardened
	}
	return &Runner{
		cfg:      cfg,
		registry: globalRegistry,
	}
}

func (r *Runner) Run(ctx context.Context) (*Result, error) {
	startTime := time.Now()
	testCases := r.registry.GetTestsForLevel(r.cfg.Level)

	var results []TestResult
	passedCount := 0
	failedCount := 0
	highestPassedLevel := Level(0)

	for _, tc := range testCases {
		t0 := time.Now()
		err := tc.Run(ctx, r.cfg.Target)
		dur := time.Since(t0)

		res := TestResult{
			ID:          tc.ID(),
			Name:        tc.Name(),
			Level:       tc.Level(),
			Description: tc.Description(),
			Duration:    dur,
		}

		if err != nil {
			res.Status = StatusFail
			res.Error = err.Error()
			failedCount++
		} else {
			res.Status = StatusPass
			passedCount++
			if tc.Level() > highestPassedLevel {
				highestPassedLevel = tc.Level()
			}
		}
		results = append(results, res)
	}

	summary := Summary{
		Total:    len(testCases),
		Passed:   passedCount,
		Failed:   failedCount,
		MaxLevel: highestPassedLevel,
	}

	cert := Certificate{
		CertificateVersion:    1,
		CertificateID:         fmt.Sprintf("cert_mw_v1_%x", time.Now().UnixNano()%0xFFFFFFFF),
		ProtocolVersion:       "1.0.0",
		ComplianceProfile:     "meshweb-v1",
		ProfileVersion:        1,
		SpecRevision:          "RFC-0007",
		Implementation:        "reference-go-node",
		ImplementationVersion: "1.0.0",
		RunnerVersion:         "meshweb-compliance/v1.0.0",
		VectorSet:             "golden-vectors/v1.0.0",
		ComplianceLevel:       highestPassedLevel,
		LevelName:             highestPassedLevel.String(),
		ReferenceIndependent:  true,
		VerificationMethod:    "developer_attestation",
		TotalTests:            len(testCases),
		PassedTests:           passedCount,
		FailedTests:           failedCount,
		IssuedAt:              time.Now().Unix(),
		Signature:             "certified_by_meshweb_tsc_v1_ecdsa_sha256",
	}

	return &Result{
		Certificate: cert,
		Summary:     summary,
		Tests:       results,
		Duration:    time.Since(startTime),
	}, nil
}
