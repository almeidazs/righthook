package commands

import (
	"fmt"
	"time"

	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
	statspkg "github.com/almeidazs/righthook/internal/stats"
)

func Stats(raw cli.StatsOptions, rt cli.Runtime) error {
	opts, err := cli.ResolveStatsOptions(raw)
	if err != nil {
		return err
	}

	repo, err := git.ResolveRepository(opts.Path)
	if err != nil {
		return err
	}

	configPath := resolveRepoConfigPath(repo.Root, opts.ConfigPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(rt.Stdout, "\n◇ Righthook stats\n")
	if !cfg.Stats.Enabled {
		fmt.Fprintln(rt.Stdout, "\n▲ Stats are disabled")
		return nil
	}

	summary, err := statspkg.Summarize(repo.Root, cfg.Stats, time.Now())
	if err != nil {
		return err
	}

	fmt.Fprintln(rt.Stdout, "\nLast 30 runs:")
	if summary.RunCount == 0 {
		fmt.Fprintln(rt.Stdout, "  No recorded runs yet")
		return nil
	}

	for _, avg := range summary.HookAverages {
		fmt.Fprintf(rt.Stdout, "  Average %-10s %s\n", avg.Hook+":", formatDuration(avg.Duration))
	}
	fmt.Fprintf(rt.Stdout, "  Cache hit rate:     %.0f%%\n", summary.CacheHitRate*100)

	fmt.Fprintln(rt.Stdout, "\nSlowest jobs:")
	for _, job := range summary.SlowestJobs {
		fmt.Fprintf(rt.Stdout, "  %-12s %s avg\n", job.Name, formatDuration(job.Duration))
	}
	return nil
}

func formatDuration(d time.Duration) string {
	if d < 100*time.Millisecond {
		return d.Round(time.Millisecond).String()
	}
	seconds := d.Seconds()
	return fmt.Sprintf("%.1fs", seconds)
}
