package meer

import (
	"testing"

	"github.com/Qitmeer/qng/params"
	"github.com/stretchr/testify/assert"
)

func TestGenesisHash(t *testing.T) {
	assert.Equal(t, MainNetGenesisHash, Genesis(&params.MainNetParams, nil).ToBlock().Hash().String(),
		params.MainNetParams.Name+" genesis hash not equal latest")
	assert.Equal(t, MixNetGenesisHash, Genesis(&params.MixNetParams, nil).ToBlock().Hash().String(),
		params.MixNetParams.Name+" genesis hash not equal latest")
	assert.Equal(t, TestNetGenesisHash, Genesis(&params.TestNetParams, nil).ToBlock().Hash().String(),
		params.TestNetParams.Name+" genesis hash not equal latest")
	assert.Equal(t, PrivNetGenesisHash, Genesis(&params.PrivNetParams, nil).ToBlock().Hash().String(),
		params.PrivNetParam.Name+" genesis hash not equal latest")
}
