package testutils

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/json"
	"github.com/Qitmeer/qng/core/types"
	"github.com/Qitmeer/qng/core/types/pow"
	"github.com/Qitmeer/qng/testutils/swap/factory"
	"github.com/Qitmeer/qng/testutils/swap/router"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// GenerateBlocks will generate a number of blocks by the input number
// It will return the hashes of the generated blocks or an error
func GenerateBlocks(t *testing.T, node *MockNode, num uint64) []*hash.Hash {
	result := make([]*hash.Hash, 0)
	blocks, err := node.GetPrivateMinerAPI().Generate(uint32(num), pow.MEERXKECCAKV1)
	if err != nil {
		t.Errorf("generate block failed : %v", err)
		return nil
	}

	for _, b := range blocks {
		bh := hash.MustHexToDecodedHash(b)
		result = append(result, &bh)
		t.Logf("%v: generate block [%v] ok", node.ID(), b)
	}
	return result
}

func GetSerializedBlock(node *MockNode, h *hash.Hash) (*types.SerializedBlock, error) {
	bol := false
	blockHex, err := node.GetPublicBlockAPI().GetBlock(*h, &bol, &bol, &bol)
	if err != nil {
		return nil, err
	}
	// Decode the serialized block hex to raw bytes.
	serializedBlock, err := hex.DecodeString(blockHex.(string))
	if err != nil {
		return nil, err
	}
	// Deserialize the block and return it.
	block, err := types.NewBlockFromBytes(serializedBlock)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func GenerateBlocksWaitForTxs(t *testing.T, h *MockNode, txs []string) error {
	tryMax := 20
	txsM := map[string]bool{}
	for _, tx := range txs {
		txsM[tx] = false
	}
	for i := 0; i < tryMax; i++ {
		ret := GenerateBlocks(t, h, 1)
		if len(ret) != 1 {
			t.Fatal("No block")
		}
		if len(txsM) <= 0 {
			return nil
		}
		sb, err := GetSerializedBlock(h, ret[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, tx := range sb.Transactions() {
			_, ok := txsM[tx.Hash().String()]
			if ok {
				txsM[tx.Hash().String()] = true
			}
			if types.IsCrossChainVMTx(tx.Tx) {
				txHash := "0x" + tx.Tx.TxIn[0].PreviousOut.Hash.String()
				_, ok := txsM[txHash]
				if ok {
					txsM[txHash] = true
				}
			}
		}
		all := true
		for _, v := range txsM {
			if !v {
				all = false
			}
		}
		if all {
			return nil
		}
		if i >= tryMax-1 {
			t.Fatal("No block")
		}
	}
	return fmt.Errorf("No block")
}

// AssertBlockOrderHeightTotal will verify the current block order, total block number
// and current main-chain height of the appointed test node and assert it ok or
// cause the test failed.
func AssertBlockOrderHeightTotal(t *testing.T, node *MockNode, order, total, height uint) {
	// order
	c, err := node.GetPublicBlockAPI().GetBlockCount()
	if err != nil {
		t.Errorf("test failed : %v", err)
	} else {
		expect := order
		if c.(uint) != expect {
			t.Errorf("test failed, expect %v , but got %v", expect, c)
		}
	}
	// total block
	tal, err := node.GetPublicBlockAPI().GetBlockTotal()
	if err != nil {
		t.Errorf("test failed : %v", err)
	} else {
		expect := total
		if tal != expect {
			t.Errorf("test failed, expect %v , but got %v", expect, tal)
		}
	}
	// main height
	h, err := node.GetPublicBlockAPI().GetMainChainHeight()
	if err != nil {
		t.Errorf("test failed : %v", err)
	} else {
		expect := height
		hi, err := strconv.ParseUint(h.(string), 10, 64)
		if err != nil {
			t.Errorf("test failed : %v", err)
		}
		if hi != uint64(expect) {
			t.Errorf("test failed, expect %v , but got %v", expect, h)
		}
	}
}

// spend first HD account to new address create by HD
func SpendUtxo(t *testing.T, node *MockNode, preOutpoint *types.TxOutPoint, amt types.Amount, lockTime int64) (*types.Transaction, types.Address) {
	addr, err := node.NewAddress()
	if err != nil {
		t.Fatalf("failed to generate new address for test wallet: %v", err)
	}
	t.Logf("test wallet generated new address %v ok", addr.String())
	feeRate := int64(10)

	inputs := []json.TransactionInput{json.TransactionInput{Txid: preOutpoint.Hash.String(), Vout: preOutpoint.OutIndex}}
	aa := json.AdreesAmount{}
	aa[addr.PKHAddress().String()] = json.Amout{CoinId: uint16(amt.Id), Amount: amt.Value - feeRate}
	tx, err := node.GetWalletManager().SpendUtxo(inputs, aa, &lockTime)
	if err != nil {
		t.Fatal(err)
	}
	return tx, addr.PKHAddress()
}

func CreateLegacyTx(node *MockNode, fromPkByte []byte, to *common.Address, nonce uint64, gas uint64, val *big.Int, d []byte) (string, error) {
	privateKey := crypto.ToECDSAUnsafe(fromPkByte)
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", errors.New("private key error")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	log.Println("from address", fromAddress.String())
	var err error
	if nonce <= 0 {
		nonce, err = node.GetEvmClient().PendingNonceAt(context.Background(), fromAddress)
		if err != nil {
			return "", err
		}
	}
	gasLimit := uint64(GAS_LIMIT) // in units
	if gas > 0 {
		gasLimit = gas
	}
	gasPrice, err := node.GetEvmClient().SuggestGasPrice(context.Background())
	if err != nil {
		return "", err
	}
	data := &etypes.LegacyTx{
		To:       to,
		Nonce:    nonce,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Value:    val,
		Data:     d,
	}
	tx := etypes.NewTx(data)
	signedTx, err := etypes.SignTx(tx, etypes.NewEIP155Signer(CHAIN_ID), privateKey)
	if err != nil {
		return "", err
	}
	err = node.GetEvmClient().SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", err
	}
	return signedTx.Hash().Hex(), nil
}
func CreateErc20(node *MockNode) (string, error) {
	return CreateLegacyTx(node, node.GetBuilder().Get(0), nil, 0, 0, big.NewInt(0), common.FromHex(ERC20Code))
}
func AuthTrans(privatekeybyte []byte) (*bind.TransactOpts, error) {
	privateKey := crypto.ToECDSAUnsafe(privatekeybyte)
	authCaller, err := bind.NewKeyedTransactorWithChainID(privateKey, CHAIN_ID)
	if err != nil {
		return nil, err
	}
	authCaller.GasLimit = uint64(GAS_LIMIT)
	return authCaller, nil
}
func CreateWETH(node *MockNode) (string, error) {
	return CreateLegacyTx(node, node.GetBuilder().Get(0), nil, 0, 0, big.NewInt(0), common.FromHex(WETH))
}

func CreateFactory(node *MockNode, _feeToSetter common.Address) (string, error) {
	parsed, _ := abi.JSON(strings.NewReader(factory.TokenMetaData.ABI))
	// constructor params
	initP, _ := parsed.Pack("", _feeToSetter)
	return CreateLegacyTx(node, node.GetBuilder().Get(0), nil, 0, 0, big.NewInt(0), append(common.FromHex(FACTORY), initP...))
}

func CreateRouter(node *MockNode, factory, weth common.Address) (string, error) {
	parsed, _ := abi.JSON(strings.NewReader(router.TokenMetaData.ABI))
	initP, _ := parsed.Pack("", factory, weth)
	return CreateLegacyTx(node, node.GetBuilder().Get(0), nil, 0, 0, big.NewInt(0), append(common.FromHex(ROUTER), initP...))
}
func SendSelfMockNode(t *testing.T, h *MockNode, amt types.Amount, lockTime *int64) *hash.Hash {
	acc := h.GetWalletManager().GetAccountByIdx(0)
	if acc == nil {
		t.Fatalf("failed to get addr")
		return nil
	}
	txId, err := h.GetWalletManager().SendTx(acc.PKHAddress().String(), json.AddressAmountV3{
		acc.PKHAddress().String(): json.AmountV3{
			CoinId: uint16(amt.Id),
			Amount: amt.Value,
		},
	}, 0, *lockTime)
	if err != nil {
		t.Fatalf("failed to pay the output: %v", err)
	}
	ret, err := hash.NewHashFromStr(txId)
	if err != nil {
		t.Fatalf("failed to get the txid: %v, err:%v", txId, err)
	}
	return ret
}

// Spend amount from the wallet of the test harness and return tx hash
func SendExportTxMockNode(t *testing.T, h *MockNode, txid string, idx uint32, value int64) *hash.Hash {
	acc := h.GetWalletManager().GetAccountByIdx(0)
	if acc == nil {
		t.Fatalf("failed to get addr")
		return nil
	}
	rawStr, err := h.GetPublicTxAPI().CreateExportRawTransaction(txid, idx, acc.PKAddress().String(), value)
	if err != nil {
		t.Fatalf("failed to pay the output: %v", err)
	}

	signRaw, err := h.GetPrivateTxAPI().TxSign(h.GetBuilder().GetHex(0), rawStr.(string), nil)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
		return nil
	}
	allHighFee := true
	tx, err := h.GetPublicTxAPI().SendRawTransaction(signRaw.(string), &allHighFee)
	if err != nil {
		t.Fatalf("failed to send raw tx: %v", err)
		return nil
	}
	ret, err := hash.NewHashFromStr(tx.(string))
	if err != nil {
		t.Fatalf("failed to decode txid: %v", err)
		return nil
	}
	return ret
}
