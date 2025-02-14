package difficultymanager

import (
	"github.com/Qitmeer/qng/consensus/model"
	"github.com/Qitmeer/qng/consensus/pow"
	"github.com/Qitmeer/qng/params"
)

func NewDiffManager(con model.Consensus, cfg *params.Params) model.DifficultyManager {
	switch cfg.PowConfig.DifficultyMode {
	case pow.DIFFICULTY_MODE_GHOSTDAG:
		return &ghostdagDiff{
			con:                            con,
			b:                              con.BlockChain(),
			powMax:                         cfg.PowConfig.MeerXKeccakV1PowLimit,
			difficultyAdjustmentWindowSize: int(cfg.WorkDiffWindowSize),
			disableDifficultyAdjustment:    false,
			targetTimePerBlock:             cfg.TargetTimePerBlock,
			genesisBits:                    cfg.PowConfig.MeerXKeccakV1PowLimitBits,
			cfg:                            cfg,
		}
	case pow.DIFFICULTY_MODE_DEVELOP:
		return &developDiff{
			b:   con.BlockChain(),
			cfg: cfg,
		}
	}
	return &meerDiff{
		con: con,
		b:   con.BlockChain(),
		cfg: cfg,
	}
}
