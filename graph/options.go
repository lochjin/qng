package graph

type GraphOptions func(*Options)

type Options struct {
	CallbackHandler GraphCallback
}

func WithCallback(callback GraphCallback) GraphOptions {
	return func(opts *Options) {
		opts.CallbackHandler = callback
	}
}
