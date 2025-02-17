package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Qitmeer/qng/consensus/engine"
	"github.com/Qitmeer/qng/core/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"io"
)

const (
	headSize = 4
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
	Vanity  []byte
	Seal    []byte
	Signers []common.Address
}

func (p *PoA) Type() engine.EngineType {
	return engine.PoAEngineType
}

func (p *PoA) Name() string {
	return "MeerDAG Proof of Authority"
}

func (p *PoA) Bytes() []byte {
	var buff bytes.Buffer
	err := buff.WriteByte(byte(p.Type()))
	if err != nil {
		panic(err)
	}
	dataSize := VanitySize + SealSize
	if len(p.Signers) > 0 {
		dataSize += len(p.Signers) * common.AddressLength
	}
	bs := make([]byte, headSize)
	binary.LittleEndian.PutUint32(bs, uint32(dataSize))
	size, err := buff.Write(bs)
	if err != nil {
		panic(err)
	}
	if size != headSize {
		panic(fmt.Errorf("Write size error:%d", size))
	}
	size, err = buff.Write(p.Vanity)
	if err != nil {
		panic(err)
	}
	if size != VanitySize {
		panic(fmt.Errorf("Write size error:%d", size))
	}
	size, err = buff.Write(p.Seal)
	if err != nil {
		panic(err)
	}
	if size != SealSize {
		panic(fmt.Errorf("Write size error:%d", size))
	}
	if len(p.Signers) > 0 {
		for _, s := range p.Signers {
			size, err = buff.Write(s.Bytes())
			if err != nil {
				panic(err)
			}
		}
	}
	return buff.Bytes()
}

func (p *PoA) Digest() []byte {
	var buff bytes.Buffer
	err := buff.WriteByte(byte(p.Type()))
	if err != nil {
		panic(err)
	}
	dataSize := VanitySize
	if len(p.Signers) > 0 {
		dataSize += len(p.Signers) * common.AddressLength
	}
	bs := make([]byte, headSize)
	binary.LittleEndian.PutUint32(bs, uint32(dataSize))
	size, err := buff.Write(bs)
	if err != nil {
		panic(err)
	}
	if size != headSize {
		panic(fmt.Errorf("Write size error:%d", size))
	}
	size, err = buff.Write(p.Vanity)
	if err != nil {
		panic(err)
	}
	if size != VanitySize {
		panic(fmt.Errorf("Write size error:%d", size))
	}
	if len(p.Signers) > 0 {
		for _, s := range p.Signers {
			size, err = buff.Write(s.Bytes())
			if err != nil {
				panic(err)
			}
		}
	}
	return buff.Bytes()
}

func (p *PoA) Info() *json.PoAInfo {
	pi := &json.PoAInfo{
		Vanity: hexutil.Encode(p.Vanity),
		Seal:   hexutil.Encode(p.Seal),
	}
	if len(p.Signers) > 0 {
		pi.Signers = []string{}
		for _, s := range p.Signers {
			pi.Signers = append(pi.Signers, s.String())
		}
	}
	return pi
}

func New(r io.Reader) (*PoA, error) {
	lb := make([]byte, headSize)
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
	if size != dataLength {
		return nil, fmt.Errorf("Read size error:%d != %d", size, dataLength)
	}
	poa := &PoA{
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

func Default() *PoA {
	return &PoA{
		Vanity: make([]byte, VanitySize),
		Seal:   make([]byte, SealSize),
		Signers: []common.Address{
			common.HexToAddress("0x71bc4403af41634cda7c32600a8024d54e7f6499"),
		},
	}
}

func Empty() *PoA {
	return &PoA{
		Vanity:  make([]byte, VanitySize),
		Seal:    make([]byte, SealSize),
		Signers: []common.Address{},
	}
}
