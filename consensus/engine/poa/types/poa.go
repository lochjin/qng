package types

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Qitmeer/qng/consensus/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"io"
)

var (
	VanitySize = 32                     // Fixed number of data prefix bytes reserved for signer vanity
	SealSize   = crypto.SignatureLength // Fixed number of data suffix bytes reserved for signer seal

	// ErrMissingVanity is returned if a block's data section is shorter than
	// 32 bytes, which is required to store the signer vanity.
	ErrMissingVanity = errors.New("data 32 byte vanity prefix missing")

	// ErrMissingSignature is returned if a block's data section doesn't seem
	// to contain a 65 byte secp256k1 signature.
	ErrMissingSignature = errors.New("data 65 byte signature suffix missing")

	// errInvalidCheckpointSigners is returned if a checkpoint block contains an
	// invalid list of signers (i.e. non divisible by 20 bytes).
	ErrInvalidCheckpointSigners = errors.New("invalid signer list on checkpoint block")
)

type PoA struct {
	Ty      engine.EngineType
	Vanity  []byte
	Seal    []byte
	Signers []common.Address
}

func (p *PoA) Type() engine.EngineType {
	return p.Ty
}

func (p *PoA) Name() string {
	return "MeerDAG POA"
}

func New(r io.Reader) (*PoA, error) {
	lb := make([]byte, 4)
	size, err := io.ReadFull(r, lb)
	if err != nil {
		return nil, err
	}
	if size != len(lb) {
		return nil, fmt.Errorf("Read size error:%d != %d", size, len(lb))
	}
	dataLength := int(binary.LittleEndian.Uint32(lb))
	// Check that the extra-data contains both the vanity and signature
	if dataLength < VanitySize {
		return nil, ErrMissingVanity
	}
	if dataLength < VanitySize+SealSize {
		return nil, ErrMissingSignature
	}
	signersSize := dataLength - VanitySize - SealSize

	if signersSize%common.AddressLength != 0 {
		return nil, ErrInvalidCheckpointSigners
	}

	data := make([]byte, dataLength)
	size, err = io.ReadFull(r, data)
	if err != nil {
		return nil, err
	}
	if size != len(lb) {
		return nil, fmt.Errorf("Read size error:%d != %d", size, len(lb))
	}
	poa := &PoA{
		Ty:      engine.PoAEngineType,
		Vanity:  data[:VanitySize],
		Seal:    data[VanitySize : VanitySize+SealSize],
		Signers: []common.Address{},
	}
	if signersSize > 0 {
		size = signersSize / common.AddressLength
		signersBytes := data[VanitySize+SealSize:]
		for i := 0; i < size; i++ {
			index := i * common.AddressLength
			addr := signersBytes[index : index+common.AddressLength]
			poa.Signers = append(poa.Signers, common.BytesToAddress(addr))
		}
	}
	return poa, nil
}
