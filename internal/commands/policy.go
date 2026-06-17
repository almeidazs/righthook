package commands

import (
	"errors"
	"fmt"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	"github.com/almeidazs/righthook/internal/cli"
	"github.com/almeidazs/righthook/internal/config"
	"github.com/almeidazs/righthook/internal/git"
	"github.com/almeidazs/righthook/internal/output"
	"github.com/almeidazs/righthook/internal/version"
)

var currentVersion = version.Version

type policyIssue struct {
	message string
	fix     string
}

func PolicyCheck(raw cli.PolicyCheckOptions, rt cli.Runtime) error {
	opts, err := cli.ResolvePolicyCheckOptions(raw)
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

	renderer := output.New(rt.Stdout, false, false)
	renderer.Intro("Righthook policy")

	if isEmptyPolicy(cfg.Policy) {
		renderer.Warn("No policy configured")
		return nil
	}

	issues := make([]policyIssue, 0)
	checkVersionPolicy(cfg.Policy, renderer, &issues)
	checkInstalledHooksPolicy(repo, cfg.Policy, renderer, &issues)

	if len(issues) > 0 {
		renderer.Section("Fix")
		for _, issue := range issues {
			if issue.fix == "" {
				continue
			}
			fmt.Fprintf(rt.Stdout, "  %s\n", issue.fix)
		}
	}

	if len(issues) == 0 {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Policy.AllowSkip)) {
	case "warn", "ignore":
		return nil
	default:
		return errors.New("policy check failed")
	}
}

func isEmptyPolicy(policy config.PolicyConfig) bool {
	return strings.TrimSpace(policy.RequiredVersion) == "" &&
		!policy.RequireInstalled &&
		len(policy.RequiredHooks) == 0 &&
		strings.TrimSpace(policy.AllowSkip) == ""
}

func checkVersionPolicy(policy config.PolicyConfig, renderer output.Renderer, issues *[]policyIssue) {
	constraintText := strings.TrimSpace(policy.RequiredVersion)
	if constraintText == "" {
		return
	}

	constraint, err := semver.NewConstraint(constraintText)
	if err != nil {
		*issues = append(*issues, policyIssue{
			message: fmt.Sprintf("Version constraint %s is invalid", constraintText),
		})
		renderer.Error(fmt.Sprintf("Version constraint %s is invalid", constraintText))
		return
	}

	v, err := semver.NewVersion(currentVersion)
	if err != nil {
		*issues = append(*issues, policyIssue{
			message: fmt.Sprintf("Current version %s is not a valid semantic version", currentVersion),
		})
		renderer.Error(fmt.Sprintf("Current version %s is not a valid semantic version", currentVersion))
		return
	}

	if constraint.Check(v) {
		renderer.Success(fmt.Sprintf("Version satisfies %s", constraintText))
		return
	}

	*issues = append(*issues, policyIssue{
		message: fmt.Sprintf("Version does not satisfy %s", constraintText),
	})
	renderer.Error(fmt.Sprintf("Version does not satisfy %s", constraintText))
}

func checkInstalledHooksPolicy(repo git.Repository, policy config.PolicyConfig, renderer output.Renderer, issues *[]policyIssue) {
	if !policy.RequireInstalled || len(policy.RequiredHooks) == 0 {
		return
	}

	installed := git.ListInstalledHooks(repo, cli.SupportedHooks)
	installedByName := mapHookFiles(installed)
	for _, hook := range policy.RequiredHooks {
		file, ok := installedByName[hook]
		if ok && file.IsRighthook {
			renderer.Success(fmt.Sprintf("%s installed", hook))
			continue
		}

		renderer.Error(fmt.Sprintf("%s not installed", hook))
		*issues = append(*issues, policyIssue{
			message: fmt.Sprintf("%s not installed", hook),
			fix:     fmt.Sprintf("righthook install --hook %s", hook),
		})
	}
}
