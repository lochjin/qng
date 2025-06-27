package ollama

import (
	"github.com/ollama/ollama/envconfig"
	"log/slog"
	"os"
	"path/filepath"
)

func initLog() {
	level := envconfig.LogLevel()
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.SourceKey {
				source := attr.Value.Any().(*slog.Source)
				source.File = filepath.Base(source.File)
			}

			return attr
		},
	})
	slog.SetDefault(slog.New(handler))
}
