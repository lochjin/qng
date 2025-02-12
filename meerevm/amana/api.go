package amana

import (
	"fmt"
	rpcapi "github.com/Qitmeer/qng/rpc/api"
	"github.com/Qitmeer/qng/version"
)

type AmanaChainInfo struct {
	AmanaVer  string `json:"amanaver"`
	EvmVer    string `json:"evmver"`
	ChainID   uint64 `json:"chainid"`
	NetworkID uint64 `json:"networkid"`
	IPC       string `json:"ipc,omitempty"`
	HTTP      string `json:"http,omitempty"`
	WS        string `json:"ws,omitempty"`
}

type PublicAmanaChainAPI struct {
	mc *AmanaChain
}

func NewPublicAmanaChainAPI(mc *AmanaChain) *PublicAmanaChainAPI {
	return &PublicAmanaChainAPI{mc}
}

func (api *PublicAmanaChainAPI) GetAmanaChainInfo() (interface{}, error) {
	mi := AmanaChainInfo{
		AmanaVer:  version.String(),
		EvmVer:    api.mc.chain.Config().Node.Version,
		ChainID:   api.mc.chain.Config().Eth.Genesis.Config.ChainID.Uint64(),
		NetworkID: api.mc.chain.Config().Eth.NetworkId,
	}
	if len(api.mc.chain.Config().Node.IPCEndpoint()) > 0 {
		mi.IPC = api.mc.chain.Config().Node.IPCEndpoint()
	}
	if len(api.mc.chain.Config().Node.HTTPHost) > 0 {
		mi.HTTP = api.mc.chain.Config().Node.HTTPEndpoint()
	}
	if len(api.mc.chain.Config().Node.WSHost) > 0 {
		mi.WS = api.mc.chain.Config().Node.WSEndpoint()
	}
	return mi, nil
}

func (api *PublicAmanaChainAPI) HasAmanaState(hashOrNumber string) (interface{}, error) {
	hn, err := rpcapi.NewHashOrNumber(hashOrNumber)
	if err != nil {
		return false, err
	}
	if !api.mc.CheckState(hn) {
		return false, fmt.Errorf("No meer state at:%s", hashOrNumber)
	}
	return true, nil
}
