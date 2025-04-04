package ollama

import (
	"encoding/hex"
	"errors"
	"github.com/ollama/ollama/api"
)

type PublicOllamaAPI struct {
	o *OllamaService
}

func NewPublicOllamaAPI(o *OllamaService) *PublicOllamaAPI {
	return &PublicOllamaAPI{
		o: o,
	}
}

func (a *PublicOllamaAPI) OllamaList() (interface{}, error) {
	return a.o.List()
}

func (a *PublicOllamaAPI) OllamaGenerate(prompt string) (interface{}, error) {
	pro, err := hex.DecodeString(prompt)
	if err != nil {
		return nil, err
	}
	ret, err := a.o.Generate(string(pro))
	if err != nil {
		return nil, err
	}
	rsp, ok := ret.(api.GenerateResponse)
	if !ok {
		return nil, errors.New("No response")
	}
	return rsp.Response, nil
}
