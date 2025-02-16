package json

import "github.com/ethereum/go-ethereum/p2p"

// for pow diff
type PowDiff struct {
	CurrentDiff float64 `json:"current_diff"`
}

// InfoNodeResult models the data returned by the node server getnodeinfo command.
type InfoNodeResult struct {
	ID                  string                              `json:"ID"`
	Addresss            []string                            `json:"address"`
	QNR                 string                              `json:"QNR,omitempty"`
	Version             int32                               `json:"version"`
	BuildVersion        string                              `json:"buildversion"`
	ProtocolVersion     int32                               `json:"protocolversion"`
	TotalSubsidy        uint64                              `json:"totalsubsidy,omitempty"`
	StateRoot           string                              `json:"stateroot,omitempty"`
	GraphState          *GetGraphStateResult                `json:"graphstate,omitempty"`
	TimeOffset          int64                               `json:"timeoffset,omitempty"`
	PowDiff             *PowDiff                            `json:"pow_diff,omitempty"`
	Confirmations       int32                               `json:"confirmations,omitempty"`
	CoinbaseMaturity    int32                               `json:"coinbasematurity,omitempty"`
	Errors              string                              `json:"errors,omitempty"`
	Modules             []string                            `json:"modules,omitempty"`
	DNS                 string                              `json:"dns,omitempty"`
	ConsensusDeployment map[string]*ConsensusDeploymentDesc `json:"consensusdeployment,omitempty"`
	Network             string                              `json:"network"`
	Connections         int32                               `json:"connections"`
	ConsensusEngine     string                              `json:"consensusengine,omitempty"`
}

// GetPeerInfoResult models the data returned from the getpeerinfo command.
type GetPeerInfoResult struct {
	ID             string               `json:"id"`
	QNR            string               `json:"qnr,omitempty"`
	Address        string               `json:"address"`
	State          bool                 `json:"state,omitempty"`
	Active         bool                 `json:"active,omitempty"`
	Protocol       uint32               `json:"protocol,omitempty"`
	Genesis        string               `json:"genesis,omitempty"`
	Services       string               `json:"services,omitempty"`
	Name           string               `json:"name,omitempty"`
	Direction      string               `json:"direction,omitempty"`
	StateRoot      string               `json:"stateroot,omitempty"`
	GraphState     *GetGraphStateResult `json:"graphstate,omitempty"`
	GSUpdate       string               `json:"gsupdate,omitempty"`
	SyncNode       bool                 `json:"syncnode,omitempty"`
	TimeOffset     int64                `json:"timeoffset"`
	LastSend       string               `json:"lastsend,omitempty"`
	LastRecv       string               `json:"lastrecv,omitempty"`
	BytesSent      uint64               `json:"bytessent,omitempty"`
	BytesRecv      uint64               `json:"bytesrecv,omitempty"`
	ConnTime       string               `json:"conntime,omitempty"`
	Version        string               `json:"version,omitempty"`
	Network        string               `json:"network,omitempty"`
	Circuit        bool                 `json:"circuit,omitempty"`
	ReConnect      uint64               `json:"reconnect,omitempty"`
	Bads           []string             `json:"bads,omitempty"`
	MempoolReqTime string               `json:"mempoolreqtime,omitempty"`
	Tasks          int                  `json:"tasks,omitempty"`
	Broadcast      int                  `json:"broadcast,omitempty"`
	Snap           bool                 `json:"snap,omitempty"`
	InSnapSync     bool                 `json:"insnapsync,omitempty"`
	LongConnStat   bool                 `json:"longconnstat,omitempty"`
	MeerState      *MeerState           `json:"meerstate,omitempty"`
}

// GetGraphStateResult data
type GetGraphStateResult struct {
	Tips       []string `json:"tips"`
	MainOrder  uint32   `json:"mainorder"`
	MainHeight uint32   `json:"mainheight"`
	Layer      uint32   `json:"layer"`
}

// GetGraphStateResult data
type MeerState struct {
	Id     string        `json:"id"`
	Number uint64        `json:"number"`
	Enode  string        `json:"enode"`
	Enr    string        `json:"enr"`
	Info   *p2p.PeerInfo `json:"info,omitempty"`
}

type GetBanlistResult struct {
	PeerID string         `json:"peerid"`
	Bads   []*BadResponse `json:"bads"`
}

type BadResponse struct {
	ID    uint64 `json:"id"`
	Time  string `json:"time"`
	Error string `json:"error"`
}

type ConsensusDeploymentDesc struct {
	Status      string `json:"status"`
	StartHeight int64  `json:"startHeight,omitempty"`
}

type NetworkStat struct {
	TotalPeers     int            `json:"totalpeers"`
	MaxConnected   uint           `json:"maxconnected"`
	MaxInbound     int            `json:"maxinbound"`
	TotalConnected int            `json:"totalconnected"`
	TotalRelays    int            `json:"totalrelays"`
	Infos          []*NetworkInfo `json:"infos"`
	Services       string         `json:"services"`
	Snap           bool           `json:"snap"`
	SnapSync       *SnapSyncInfo  `json:"snapsync,omitempty"`
	MaxOrder       uint           `json:"maxorder,omitempty"`
	MinOrder       uint           `json:"minorder,omitempty"`
	MyOrder        uint           `json:"myorder,omitempty"`
}

type NetworkInfo struct {
	Name       string `json:"name"`
	Peers      int    `json:"peers"`
	Relays     int    `json:"relays"`
	Connecteds int    `json:"connecteds"`
	AverageGS  string `json:"averagegs,omitempty"`
	MaxGS      string `json:"maxgs,omitempty"`
	MinGS      string `json:"mings,omitempty"`
}

type SnapSyncInfo struct {
	PeerID       string `json:"peerid"`
	TargetBlock  string `json:"targetblock"`
	StateRoot    string `json:"stateroot"`
	Evm          string `json:"evm"`
	Pivot        uint64 `json:"pivot,omitempty"`
	Point        string `json:"point,omitempty"`
	PointOrder   uint64 `json:"pointorder,omitempty"`
	EVMCompleted bool   `json:"evmcompleted,omitempty"`
	Completed    bool   `json:"completed,omitempty"`
}

type SubsidyInfo struct {
	MainHeight                   int64  `json:"current_mainheight"`
	Mode                         string `json:"mode"`
	TotalSubsidy                 uint64 `json:"total_subsidy"`
	TargetTotalSubsidy           int64  `json:"target_total_subsidy,omitempty"`
	CurTotalSubsidy              int64  `json:"current_total_subsidy,omitempty"`
	LeftTotalSubsidy             int64  `json:"left_total_subsidy,omitempty"`
	TotalTime                    string `json:"target_total_time,omitempty"`
	LeftTotalTime                string `json:"left_time,omitempty"`
	BaseSubsidy                  int64  `json:"init_block_subsidy"`
	NextSubsidy                  int64  `json:"current_block_subsidy"`
	EstimateDailyMainheightRange int64  `json:"estimate_daily_mainheight_range,omitempty"`
	EstimateDailyBlocksMined     int64  `json:"estimate_daily_blocks_mined,omitempty"`
	EstimateDailySubsidy         string `json:"estimate_daily_subsidy,omitempty"`
}

type AcctInfo struct {
	Mode      bool     `json:"mode"`
	Version   uint32   `json:"version"`
	StatPoint string   `json:"statpoint"`
	StatOrder uint32   `json:"statorder"`
	Total     uint32   `json:"total"`
	Addrs     []string `json:"addrs,omitempty"`
}

type MeerDAGInfoResult struct {
	Name                 string `json:"name"`
	Total                uint   `json:"total"`
	BlockCacheSize       uint64 `json:"bcachesize"`
	BlockCacheHeightSize uint64 `json:"bcacheheightsize"`
	BlockCacheRate       string `json:"bcacherate"`
	BlockDataCacheSize   string `json:"bdcachesize"`
	AnticoneSize         int    `json:"anticonesize"`
}

type ChainInfoResult struct {
	Count            uint64  `json:"count"`
	Start            string  `json:"start"`
	End              string  `json:"end"`
	BlocksPerSecond  float64 `json:"blockspersecond"`
	TxsPerSecond     float64 `json:"txspersecond"`
	SecondPerHeight  string  `json:"secondperheight"`
	Concurrency      float64 `json:"concurrency"`
	EmptyBlockRate   string  `json:"emptyblockrate"`
	ProcessQueueSize int32   `json:"processqueuesize"`
}
