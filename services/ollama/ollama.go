package ollama

import (
	"cmp"
	"encoding/json"
	"fmt"
	"github.com/Qitmeer/qng/config"
	"github.com/Qitmeer/qng/node/service"
	"github.com/Qitmeer/qng/rpc/api"
	"github.com/Qitmeer/qng/rpc/client/cmds"
	oapi "github.com/ollama/ollama/api"
	"github.com/ollama/ollama/discover"
	"github.com/ollama/ollama/envconfig"
	"github.com/ollama/ollama/server"
	"os"
	"slices"
)

type OllamaService struct {
	service.Service
	cfg   *config.Config
	sched *server.Scheduler
}

func NewOllamaService(cfg *config.Config) *OllamaService {
	return &OllamaService{
		cfg: cfg,
	}
}

func (o *OllamaService) Start() error {
	if err := o.Service.Start(); err != nil {
		return err
	}
	log.Info("Start Ollama Service", "model", o.cfg.OllamaModel)

	err := o.initOllama()
	if err != nil {
		return err
	}

	return nil
}

func (o *OllamaService) Stop() error {
	if o.sched != nil {
		o.sched.UnloadAllRunners()
	}
	if err := o.Service.Stop(); err != nil {
		return err
	}
	log.Info("Stop Ollama Service")

	return nil
}

func (o *OllamaService) initOllama() error {
	log.Info("Ollama config", "env", envconfig.Values())
	initLog()

	blobsDir, err := server.GetBlobsPath("")
	if err != nil {
		return err
	}
	if err := server.FixBlobs(blobsDir); err != nil {
		return err
	}

	if !envconfig.NoPrune() {
		if _, err := server.Manifests(false); err != nil {
			log.Warn("corrupt manifests detected, skipping prune operation.  Re-pull or delete to clear", "error", err)
		} else {
			// clean up unused layers and manifests
			if err := server.PruneLayers(); err != nil {
				return err
			}

			manifestsPath, err := server.GetManifestPath()
			if err != nil {
				return err
			}

			if err := server.PruneDirectory(manifestsPath); err != nil {
				return err
			}
		}
	}
	o.sched = server.InitScheduler(o.Context())
	o.sched.Run(o.Context())
	// At startup we retrieve GPU information so we can get log messages before loading a model
	// This will log warnings to the log in case we have problems with detected GPUs
	gpus := discover.GetGPUInfo()
	gpus.LogDetails()

	if len(o.cfg.OllamaModel) > 0 {
		_, err := server.GetModelInfo(oapi.ShowRequest{Model: o.cfg.OllamaModel})
		if err != nil {
			if os.IsNotExist(err) {
				log.Warn("Can't find model, try to pull", "model", o.cfg.OllamaModel)
				code, ret, err := server.Pull(o.Context(), &oapi.PullRequest{Model: o.cfg.OllamaModel})
				if err != nil {
					log.Warn(err.Error(), "code", code, "ret", ret)
				} else {
					log.Info("Pull end", "code", code, "ret", ret)
				}
			} else {
				log.Error(err.Error())
			}
		} else {
			log.Info("Model is exist", "model", o.cfg.OllamaModel)
		}
	}
	return nil
}

func (o *OllamaService) List() (interface{}, error) {
	ms, err := server.Manifests(true)
	if err != nil {
		return nil, err
	}

	models := []oapi.ListModelResponse{}
	for n, m := range ms {
		var cf server.ConfigV2

		if m.Config.Digest != "" {
			f, err := m.Config.Open()
			if err != nil {
				log.Warn("bad manifest filepath", "name", n, "error", err)
				continue
			}
			defer f.Close()

			if err := json.NewDecoder(f).Decode(&cf); err != nil {
				log.Warn("bad manifest config", "name", n, "error", err)
				continue
			}
		}

		// tag should never be masked
		models = append(models, oapi.ListModelResponse{
			Model:      n.DisplayShortest(),
			Name:       n.DisplayShortest(),
			Size:       m.Size(),
			Digest:     m.Digest(),
			ModifiedAt: m.FI().ModTime(),
			Details: oapi.ModelDetails{
				Format:            cf.ModelFormat,
				Family:            cf.ModelFamily,
				Families:          cf.ModelFamilies,
				ParameterSize:     cf.ModelType,
				QuantizationLevel: cf.FileType,
			},
		})
	}

	slices.SortStableFunc(models, func(i, j oapi.ListModelResponse) int {
		// most recently modified first
		return cmp.Compare(j.ModifiedAt.Unix(), i.ModifiedAt.Unix())
	})
	return &oapi.ListResponse{Models: models}, nil
}

func (o *OllamaService) IsModelExist() bool {
	if len(o.cfg.OllamaModel) > 0 {
		_, err := server.GetModelInfo(oapi.ShowRequest{Model: o.cfg.OllamaModel})
		if err != nil {
			return false
		}
		return true
	} else {
		return false
	}
}

func (o *OllamaService) Generate(prompt string) (interface{}, error) {
	if !o.IsModelExist() {
		return nil, fmt.Errorf("Model does not exist:%s", o.cfg.OllamaModel)
	}
	_, ret, err := server.Generate(o.Context(), &oapi.GenerateRequest{
		Model:  o.cfg.OllamaModel,
		Prompt: prompt,
	}, o.sched)
	return ret, err
}

func (o *OllamaService) APIs() []api.API {
	return []api.API{
		{
			NameSpace: cmds.DefaultServiceNameSpace,
			Service:   NewPublicOllamaAPI(o),
			Public:    true,
		},
	}
}
