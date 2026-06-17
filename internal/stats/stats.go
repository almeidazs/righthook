package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/almeidazs/righthook/internal/config"
)

const maxSummaryRuns = 30

type JobSample struct {
	Name         string
	Duration     time.Duration
	Status       string
	CacheEnabled bool
}

type RunSample struct {
	Hook     string
	Duration time.Duration
	Jobs     []JobSample
}

type fileData struct {
	Runs []runRecord `json:"runs"`
}

type runRecord struct {
	Timestamp  time.Time   `json:"timestamp"`
	Hook       string      `json:"hook"`
	DurationMS int64       `json:"duration_ms"`
	Jobs       []jobRecord `json:"jobs"`
}

type jobRecord struct {
	Name         string `json:"name"`
	DurationMS   int64  `json:"duration_ms"`
	Status       string `json:"status"`
	CacheEnabled bool   `json:"cache_enabled"`
}

type Summary struct {
	RunCount     int
	HookAverages []HookAverage
	CacheHitRate float64
	SlowestJobs  []JobAverage
}

type HookAverage struct {
	Hook     string
	Duration time.Duration
}

type JobAverage struct {
	Name     string
	Duration time.Duration
}

func Path(repoRoot string) string {
	return filepath.Join(repoRoot, ".righthook", "stats.json")
}

func Record(repoRoot string, cfg config.StatsConfig, sample RunSample, now time.Time) error {
	if !cfg.Enabled {
		return nil
	}

	data, err := load(repoRoot)
	if err != nil {
		return err
	}

	retention, err := retentionDuration(cfg)
	if err != nil {
		return err
	}

	data.Runs = prune(data.Runs, now.Add(-retention))
	record := runRecord{
		Timestamp:  now.UTC(),
		Hook:       sample.Hook,
		DurationMS: sample.Duration.Milliseconds(),
		Jobs:       make([]jobRecord, 0, len(sample.Jobs)),
	}
	for _, job := range sample.Jobs {
		record.Jobs = append(record.Jobs, jobRecord{
			Name:         job.Name,
			DurationMS:   job.Duration.Milliseconds(),
			Status:       job.Status,
			CacheEnabled: job.CacheEnabled,
		})
	}
	data.Runs = append(data.Runs, record)
	return save(repoRoot, data)
}

func Summarize(repoRoot string, cfg config.StatsConfig, now time.Time) (Summary, error) {
	data, err := load(repoRoot)
	if err != nil {
		return Summary{}, err
	}

	retention, err := retentionDuration(cfg)
	if err != nil {
		return Summary{}, err
	}

	runs := prune(data.Runs, now.Add(-retention))
	if len(runs) > maxSummaryRuns {
		runs = runs[len(runs)-maxSummaryRuns:]
	}

	summary := Summary{RunCount: len(runs)}
	if len(runs) == 0 {
		return summary, nil
	}

	hookTotals := map[string]time.Duration{}
	hookCounts := map[string]int{}
	jobTotals := map[string]time.Duration{}
	jobCounts := map[string]int{}
	cacheEnabledJobs := 0
	cacheHits := 0

	for _, run := range runs {
		hookTotals[run.Hook] += time.Duration(run.DurationMS) * time.Millisecond
		hookCounts[run.Hook]++
		for _, job := range run.Jobs {
			jobTotals[job.Name] += time.Duration(job.DurationMS) * time.Millisecond
			jobCounts[job.Name]++
			if job.CacheEnabled && (job.Status == "ran" || job.Status == "failed" || job.Status == "cached") {
				cacheEnabledJobs++
				if job.Status == "cached" {
					cacheHits++
				}
			}
		}
	}

	for hook, total := range hookTotals {
		summary.HookAverages = append(summary.HookAverages, HookAverage{
			Hook:     hook,
			Duration: total / time.Duration(hookCounts[hook]),
		})
	}
	sort.Slice(summary.HookAverages, func(i, j int) bool {
		return summary.HookAverages[i].Hook < summary.HookAverages[j].Hook
	})

	for name, total := range jobTotals {
		summary.SlowestJobs = append(summary.SlowestJobs, JobAverage{
			Name:     name,
			Duration: total / time.Duration(jobCounts[name]),
		})
	}
	sort.Slice(summary.SlowestJobs, func(i, j int) bool {
		if summary.SlowestJobs[i].Duration == summary.SlowestJobs[j].Duration {
			return summary.SlowestJobs[i].Name < summary.SlowestJobs[j].Name
		}
		return summary.SlowestJobs[i].Duration > summary.SlowestJobs[j].Duration
	})
	if len(summary.SlowestJobs) > 3 {
		summary.SlowestJobs = summary.SlowestJobs[:3]
	}

	if cacheEnabledJobs > 0 {
		summary.CacheHitRate = float64(cacheHits) / float64(cacheEnabledJobs)
	}

	return summary, nil
}

func retentionDuration(cfg config.StatsConfig) (time.Duration, error) {
	retention := cfg.Retention
	if retention == "" {
		retention = "30d"
	}
	return config.ParseHumanDuration(retention)
}

func prune(runs []runRecord, cutoff time.Time) []runRecord {
	if len(runs) == 0 {
		return nil
	}
	pruned := make([]runRecord, 0, len(runs))
	for _, run := range runs {
		if run.Timestamp.Before(cutoff) {
			continue
		}
		pruned = append(pruned, run)
	}
	return pruned
}

func load(repoRoot string) (fileData, error) {
	path := Path(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileData{}, nil
		}
		return fileData{}, err
	}

	var decoded fileData
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fileData{}, fmt.Errorf("decode stats: %w", err)
	}
	return decoded, nil
}

func save(repoRoot string, data fileData) error {
	path := Path(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
