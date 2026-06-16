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
