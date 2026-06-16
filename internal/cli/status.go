package cli

type StatusOptions struct {
	Path       string
	ConfigPath string
}

type ResolvedStatusOptions struct {
	Path       string
	ConfigPath string
}

func ResolveStatusOptions(opts StatusOptions) (ResolvedStatusOptions, error) {
	path, err := resolveAbsolutePath(opts.Path)
	if err != nil {
		return ResolvedStatusOptions{}, err
	}
	return ResolvedStatusOptions{
		Path:       path,
		ConfigPath: opts.ConfigPath,
	}, nil
}

type ListOptions struct {
	Path       string
	ConfigPath string
	JSON       bool
	OnlyJobs   bool
}

type ResolvedListOptions struct {
	Path       string
	ConfigPath string
	JSON       bool
	OnlyJobs   bool
}

func ResolveListOptions(opts ListOptions) (ResolvedListOptions, error) {
	path, err := resolveAbsolutePath(opts.Path)
	if err != nil {
		return ResolvedListOptions{}, err
	}
	return ResolvedListOptions{
		Path:       path,
		ConfigPath: opts.ConfigPath,
		JSON:       opts.JSON,
		OnlyJobs:   opts.OnlyJobs,
	}, nil
}
