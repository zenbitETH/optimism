package genesis_test

import (
	"context"
	"encoding/json"
	"flag"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

var writeFile bool

func init() {
	flag.BoolVar(&writeFile, "write-file", false, "write the genesis file to disk")
}

var testKey, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")

// Tests the BuildL2MainnetGenesis factory with the provided config.
func testBuildL2Genesis(t *testing.T, config *genesis.DeployConfig) *core.Genesis {
	backend := backends.NewSimulatedBackend(
		core.GenesisAlloc{
			crypto.PubkeyToAddress(testKey.PublicKey): {Balance: big.NewInt(10000000000000000)},
		},
		15000000,
	)
	block, err := backend.BlockByNumber(context.Background(), common.Big0)
	require.NoError(t, err)

	gen, err := genesis.BuildL2Genesis(config, block)
	require.Nil(t, err)
	require.NotNil(t, gen)

	proxyBytecode, err := bindings.GetDeployedBytecode("Proxy")
	require.NoError(t, err)

	// Apply the genesis block to the backend
	backend = backends.NewSimulatedBackendWithOpts(backends.WithAlloc(gen.Alloc))

	for name, predeploy := range predeploys.Predeploys {
		addr := predeploy.Address

		if addr == predeploys.L1BlockAddr {
			testL1Block(t, backend, config, block)
		}

		account, ok := gen.Alloc[addr]
		require.Equal(t, true, ok, name)
		require.Greater(t, len(account.Code), 0)

		adminSlot, ok := account.Storage[genesis.AdminSlot]
		isProxy := !predeploy.ProxyDisabled ||
			(!config.EnableGovernance && addr == predeploys.GovernanceTokenAddr)
		if isProxy {
			require.Equal(t, true, ok, name)
			require.Equal(t, eth.AddressAsLeftPaddedHash(predeploys.ProxyAdminAddr), adminSlot)
			require.Equal(t, proxyBytecode, account.Code)
		} else {
			require.Equal(t, false, ok, name)
			require.NotEqual(t, proxyBytecode, account.Code, name)
		}
	}

	// All of the precompile addresses should be funded with a single wei
	for i := 0; i < genesis.PrecompileCount; i++ {
		addr := common.BytesToAddress([]byte{byte(i)})
		require.Equal(t, common.Big1, gen.Alloc[addr].Balance)
	}

	create2Deployer := gen.Alloc[predeploys.Create2DeployerAddr]
	codeHash := crypto.Keccak256Hash(create2Deployer.Code)
	require.Equal(t, codeHash, bindings.Create2DeployerCodeHash)

	if writeFile {
		file, _ := json.MarshalIndent(gen, "", " ")
		_ = os.WriteFile("genesis.json", file, 0644)
	}
	return gen
}

// testL1Block tests that the state is set correctly in the L1Block predeploy
func testL1Block(t *testing.T, caller bind.ContractCaller, config *genesis.DeployConfig, block *types.Block) {
	contract, err := bindings.NewL1BlockCaller(predeploys.L1BlockAddr, caller)
	require.NoError(t, err)

	number, err := contract.Number(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, block.Number().Uint64(), number)

	timestamp, err := contract.Timestamp(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, block.Time(), timestamp)

	basefee, err := contract.Basefee(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, block.BaseFee(), basefee)

	hash, err := contract.Hash(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, block.Hash(), common.Hash(hash))

	sequenceNumber, err := contract.SequenceNumber(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, uint64(0), sequenceNumber)

	blobBaseFeeScalar, err := contract.BlobBaseFeeScalar(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, config.GasPriceOracleBlobBaseFeeScalar, blobBaseFeeScalar)

	baseFeeScalar, err := contract.BaseFeeScalar(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, config.GasPriceOracleBaseFeeScalar, baseFeeScalar)

	batcherHeader, err := contract.BatcherHash(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, eth.AddressAsLeftPaddedHash(config.BatchSenderAddress), common.Hash(batcherHeader))

	l1FeeOverhead, err := contract.L1FeeOverhead(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, config.GasPriceOracleOverhead, l1FeeOverhead.Uint64())

	l1FeeScalar, err := contract.L1FeeScalar(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, config.GasPriceOracleScalar, l1FeeScalar.Uint64())

	blobBaseFee, err := contract.BlobBaseFee(&bind.CallOpts{})
	require.NoError(t, err)
	require.Equal(t, big.NewInt(1), blobBaseFee)
}

func TestBuildL2MainnetGenesis(t *testing.T) {
	config, err := genesis.NewDeployConfig("./testdata/test-deploy-config-devnet-l1.json")
	require.Nil(t, err)
	config.EnableGovernance = true
	config.FundDevAccounts = false
	gen := testBuildL2Genesis(t, config)
	require.Equal(t, 2333, len(gen.Alloc))
}

func TestBuildL2MainnetNoGovernanceGenesis(t *testing.T) {
	config, err := genesis.NewDeployConfig("./testdata/test-deploy-config-devnet-l1.json")
	require.Nil(t, err)
	config.EnableGovernance = false
	config.FundDevAccounts = false
	gen := testBuildL2Genesis(t, config)
	require.Equal(t, 2333, len(gen.Alloc))
}
