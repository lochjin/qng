package graph

type Option func(*Config)

type Config struct {
	NodeStartHandler NodeStartHandler
	NodeEndHandler   NodeEndHandler
	EdgeEntryHandler EdgeEntryHandler
	EdgeExitHandler  EdgeExitHandler
}

func WithNodeStartHandler(callback NodeStartHandler) Option {
	return func(opts *Config) {
		opts.NodeStartHandler = callback
	}
}

func WithNodeEndHandler(callback NodeEndHandler) Option {
	return func(opts *Config) {
		opts.NodeEndHandler = callback
	}
}

func WithEdgeEntryHandler(callback EdgeEntryHandler) Option {
	return func(opts *Config) {
		opts.EdgeEntryHandler = callback
	}
}

func WithEdgeExitHandler(callback EdgeExitHandler) Option {
	return func(opts *Config) {
		opts.EdgeExitHandler = callback
	}
}
