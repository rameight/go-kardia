/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package tests

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/kai/kaidb/memorydb"
	"github.com/kardiachain/go-kardiamain/kai/state"
	"github.com/kardiachain/go-kardiamain/kai/storage/kvstore"
	kvm "github.com/kardiachain/go-kardiamain/kvm"
	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lib/log"
	"github.com/kardiachain/go-kardiamain/mainchain/blockchain"
	g "github.com/kardiachain/go-kardiamain/mainchain/genesis"

	"github.com/kardiachain/go-kardiamain/mainchain/staking"
	"github.com/kardiachain/go-kardiamain/types"
)

func GetBlockchainStaking() (*blockchain.BlockChain, error, *state.StateDB) {
	logger := log.New()
	logger.AddTag("test state")
	// Start setting up blockchain
	initValue, _ := big.NewInt(0).SetString("10000000000000000", 10)
	var genesisAccounts = map[string]*big.Int{
		"0xc1fe56E3F58D3244F606306611a5d10c8333f1f6": initValue,
		"0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5": initValue,
	}

	configs.AddDefaultContract()

	for address, _ := range genesisAccounts {
		genesisAccounts[address] = initValue
	}

	genesisContracts := make(map[string]string)
	for key, contract := range configs.GetContracts() {
		configs.LoadGenesisContract(key, contract.Address, contract.ByteCode, contract.ABI)
		if key != configs.StakingContractKey {
			genesisContracts[contract.Address] = contract.ByteCode
		}
	}

	blockDB := memorydb.New()
	kaiDb := kvstore.NewStoreDB(blockDB)
	genesis := g.DefaulTestnetFullGenesisBlock(genesisAccounts, genesisContracts)
	chainConfig, _, genesisErr := setupGenesis(genesis, kaiDb)
	if genesisErr != nil {
		log.Error("Error setting genesis block", "err", genesisErr)
		return nil, genesisErr, nil
	}

	bc, err := blockchain.NewBlockChain(log.New(), kaiDb, chainConfig, false)
	if err != nil {
		log.Error("Error creating new blockchain", "err", err)
		return nil, err, nil
	}
	stateDB, err := bc.State()
	if err != nil {
		return nil, nil, nil
	}
	return bc, nil, stateDB
}

func GetSmcStakingUtil() (*staking.StakingSmcUtil, error, *state.StateDB) {
	_, err, stateDB := GetBlockchainStaking()
	if err != nil {
		return nil, err, nil
	}
	util, err := staking.NewSmcStakingnUtil()
	if err != nil {
		return nil, err, nil
	}
	return util, nil, stateDB
}

func setup() (*state.StateDB, *staking.StakingSmcUtil, *types.Block, error) {
	util, err, stateDB := GetSmcStakingUtil()
	if err != nil {
		return nil, nil, nil, err
	}
	head := &types.Header{
		Height:   0,
		GasLimit: uint64(100000000000),
		AppHash:  common.Hash{},
		LastBlockID: types.BlockID{
			Hash: common.Hash{},
			PartsHeader: types.PartSetHeader{
				Hash:  common.Hash{},
				Total: uint32(0),
			},
		},
	}
	block := types.NewBlock(head, nil, &types.Commit{}, nil)
	return stateDB, util, block, nil
}

func finalizeTest(stateDB *state.StateDB, util *staking.StakingSmcUtil, block *types.Block) error {
	//test finalizeCommit finalize commit
	err := util.FinalizeCommit(stateDB, block.Header(), nil, kvm.Config{}, staking.LastCommitInfo{})
	if err != nil {
		return err
	}

	//test double sign
	err = util.DoubleSign(stateDB, block.Header(), nil, kvm.Config{}, []staking.Evidence{})
	if err != nil {
		return err
	}

	//test set address root
	err = util.SetRoot(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		return err
	}

	return nil
}

func TestCreateValidator(t *testing.T) {
	stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	_, err = util.ApplyAndReturnValidatorSets(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetValidators(t *testing.T) {
	stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	var votingPower int64 = 1000000
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, votingPower)
	if err != nil {
		t.Fatal(err)
	}
	newValidator := types.NewValidator(address, votingPower)

	validators, err := util.GetValidators(stateDB, block.Header(), nil, kvm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValuesf(t, validators[0], newValidator, "Validators fetched from staking SMC must be the same with created one")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetValidator(t *testing.T) {
	stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	var votingPower int64 = 1000000
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, votingPower)
	if err != nil {
		t.Fatal(err)
	}
	newValidator := types.NewValidator(address, votingPower)

	validator, err := util.GetValidator(stateDB, block.Header(), nil, kvm.Config{}, address)
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValuesf(t, validator, newValidator, "Validator fetched from staking SMC must be the same with created one")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetValidatorPower(t *testing.T) {
	stateDB, util, block, err := setup()
	if err != nil {
		t.Fatal(err)
	}

	address := common.HexToAddress("0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5")
	var votingPower int64 = 1000000
	err = util.CreateGenesisValidator(stateDB, block.Header(), nil, kvm.Config{}, address, votingPower)
	if err != nil {
		t.Fatal(err)
	}
	validatorPower, err := util.GetValidatorPower(stateDB, block.Header(), nil, kvm.Config{}, address)
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValuesf(t, validatorPower, votingPower, "Validator power fetched from staking SMC must be the same with created one")

	err = finalizeTest(stateDB, util, block)
	if err != nil {
		t.Fatal(err)
	}
}
