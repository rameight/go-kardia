package vm

import (
	"math/big"

	"github.com/kardiachain/go-kardia/common"
	"github.com/kardiachain/go-kardia/common/math"
	"github.com/kardiachain/go-kardia/params"
)

// Gas costs
const (
	GasQuickStep   uint64 = 2
	GasFastestStep uint64 = 3
	GasFastStep    uint64 = 5
	GasMidStep     uint64 = 8
	GasSlowStep    uint64 = 10
	GasExtStep     uint64 = 20
)

// calcGas returns the actual gas cost of the call.
//
func callGas(gasTable params.GasTable, availableGas, base uint64, callCost *big.Int) (uint64, error) {
	if gasTable.CreateBySuicide > 0 {
		availableGas = availableGas - base
		gas := availableGas - availableGas/64
		// If the bit length exceeds 64 bit we know that the newly calculated "gas" for EIP150
		// is smaller than the requested amount. Therefor we return the new gas instead
		// of returning an error.
		if callCost.BitLen() > 64 || gas < callCost.Uint64() {
			return gas, nil
		}
	}
	if callCost.BitLen() > 64 {
		return 0, errGasUintOverflow
	}

	return callCost.Uint64(), nil
}

// memoryGasCosts calculates the quadratic gas for memory expansion. It does so
// only for the memory region that is expanded, not the total memory.
func memoryGasCost(mem *Memory, newMemSize uint64) (uint64, error) {

	if newMemSize == 0 {
		return 0, nil
	}
	// The maximum that will fit in a uint64 is max_word_count - 1
	// anything above that will result in an overflow.
	// Additionally, a newMemSize which results in a
	// newMemSizeWords larger than 0x7ffffffff will cause the square operation
	// to overflow.
	// The constant 0xffffffffe0 is the highest number that can be used without
	// overflowing the gas calculation
	if newMemSize > 0xffffffffe0 {
		return 0, errGasUintOverflow
	}

	newMemSizeWords := toWordSize(newMemSize)
	newMemSize = newMemSizeWords * 32

	if newMemSize > uint64(mem.Len()) {
		square := newMemSizeWords * newMemSizeWords
		linCoef := newMemSizeWords * params.MemoryGas
		quadCoef := square / params.QuadCoeffDiv
		newTotalFee := linCoef + quadCoef

		fee := newTotalFee - mem.lastGasCost
		mem.lastGasCost = newTotalFee

		return fee, nil
	}
	return 0, nil
}

func constGasFunc(gas uint64) gasFunc {
	return func(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		return gas, nil
	}
}

func makeGasLog(n uint64) gasFunc {
	return func(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		requestedSize, overflow := bigUint64(stack.Back(1))
		if overflow {
			return 0, errGasUintOverflow
		}

		gas, err := memoryGasCost(mem, memorySize)
		if err != nil {
			return 0, err
		}

		if gas, overflow = math.SafeAdd(gas, params.LogGas); overflow {
			return 0, errGasUintOverflow
		}
		if gas, overflow = math.SafeAdd(gas, n*params.LogTopicGas); overflow {
			return 0, errGasUintOverflow
		}

		var memorySizeGas uint64
		if memorySizeGas, overflow = math.SafeMul(requestedSize, params.LogDataGas); overflow {
			return 0, errGasUintOverflow
		}
		if gas, overflow = math.SafeAdd(gas, memorySizeGas); overflow {
			return 0, errGasUintOverflow
		}
		return gas, nil
	}
}

func gasExp(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	expByteLen := uint64((stack.data[stack.len()-2].BitLen() + 7) / 8)

	var (
		gas      = expByteLen * gt.ExpByte // no overflow check required. Max is 256 * ExpByte gas
		overflow bool
	)
	if gas, overflow = math.SafeAdd(gas, GasSlowStep); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasSha3(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	if gas, overflow = math.SafeAdd(gas, params.Sha3Gas); overflow {
		return 0, errGasUintOverflow
	}

	wordGas, overflow := bigUint64(stack.Back(1))
	if overflow {
		return 0, errGasUintOverflow
	}
	if wordGas, overflow = math.SafeMul(toWordSize(wordGas), params.Sha3WordGas); overflow {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, wordGas); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasCreate(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, params.CreateGas); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasBalance(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return gt.Balance, nil
}

func gasCallDataCopy(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	var overflow bool
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}

	words, overflow := bigUint64(stack.Back(2))
	if overflow {
		return 0, errGasUintOverflow
	}

	if words, overflow = math.SafeMul(toWordSize(words), params.CopyGas); overflow {
		return 0, errGasUintOverflow
	}

	if gas, overflow = math.SafeAdd(gas, words); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasReturnDataCopy(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	var overflow bool
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}

	words, overflow := bigUint64(stack.Back(2))
	if overflow {
		return 0, errGasUintOverflow
	}

	if words, overflow = math.SafeMul(toWordSize(words), params.CopyGas); overflow {
		return 0, errGasUintOverflow
	}

	if gas, overflow = math.SafeAdd(gas, words); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasCodeCopy(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	var overflow bool
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}

	wordGas, overflow := bigUint64(stack.Back(2))
	if overflow {
		return 0, errGasUintOverflow
	}
	if wordGas, overflow = math.SafeMul(toWordSize(wordGas), params.CopyGas); overflow {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, wordGas); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasExtCodeCopy(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}

	var overflow bool
	if gas, overflow = math.SafeAdd(gas, gt.ExtcodeCopy); overflow {
		return 0, errGasUintOverflow
	}

	wordGas, overflow := bigUint64(stack.Back(3))
	if overflow {
		return 0, errGasUintOverflow
	}

	if wordGas, overflow = math.SafeMul(toWordSize(wordGas), params.CopyGas); overflow {
		return 0, errGasUintOverflow
	}

	if gas, overflow = math.SafeAdd(gas, wordGas); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasExtCodeSize(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return gt.ExtcodeSize, nil
}

func gasMLoad(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasMStore8(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasMStore(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var overflow bool
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, errGasUintOverflow
	}
	if gas, overflow = math.SafeAdd(gas, GasFastestStep); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasSLoad(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return gt.SLoad, nil
}

func gasSStore(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var (
		y, x = stack.Back(1), stack.Back(0)
		val  = kvm.StateDB.GetState(contract.Address(), common.BigToHash(x))
	)
	// This checks for 3 scenario's and calculates gas accordingly
	// 1. From a zero-value address to a non-zero value         (NEW VALUE)
	// 2. From a non-zero value address to a zero-value address (DELETE)
	// 3. From a non-zero to a non-zero                         (CHANGE)
	if val == (common.Hash{}) && y.Sign() != 0 {
		// 0 => non 0
		return params.SstoreSetGas, nil
	} else if val != (common.Hash{}) && y.Sign() == 0 {
		// non 0 => 0
		kvm.StateDB.AddRefund(params.SstoreRefundGas)
		return params.SstoreClearGas, nil
	} else {
		// non 0 => non 0 (or 0 => 0)
		return params.SstoreResetGas, nil
	}
}

func gasCall(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var (
		gas            = gt.Calls
		transfersValue = stack.Back(2).Sign() != 0
		address        = common.BigToAddress(stack.Back(1))
	)
	if transfersValue && kvm.StateDB.Empty(address) {
		gas += params.CallNewAccountGas
	}
	if transfersValue {
		gas += params.CallValueTransferGas
	}
	memoryGas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if gas, overflow = math.SafeAdd(gas, memoryGas); overflow {
		return 0, errGasUintOverflow
	}

	kvm.callGasTemp, err = callGas(gt, contract.Gas, gas, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, kvm.callGasTemp); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasCallCode(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas := gt.Calls
	if stack.Back(2).Sign() != 0 {
		gas += params.CallValueTransferGas
	}
	memoryGas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if gas, overflow = math.SafeAdd(gas, memoryGas); overflow {
		return 0, errGasUintOverflow
	}

	kvm.callGasTemp, err = callGas(gt, contract.Gas, gas, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, kvm.callGasTemp); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasReturn(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return memoryGasCost(mem, memorySize)
}

func gasRevert(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return memoryGasCost(mem, memorySize)
}

func gasSuicide(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas := gt.Suicide
	address := common.BigToAddress(stack.Back(0))

	// if empty and transfers value
	if kvm.StateDB.Empty(address) && kvm.StateDB.GetBalance(contract.Address()).Sign() != 0 {
		gas += gt.CreateBySuicide
	}

	if !kvm.StateDB.HasSuicided(contract.Address()) {
		kvm.StateDB.AddRefund(params.SuicideRefundGas)
	}
	return gas, nil
}

func gasDelegateCall(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if gas, overflow = math.SafeAdd(gas, gt.Calls); overflow {
		return 0, errGasUintOverflow
	}

	kvm.callGasTemp, err = callGas(gt, contract.Gas, gas, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, kvm.callGasTemp); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasStaticCall(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	gas, err := memoryGasCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if gas, overflow = math.SafeAdd(gas, gt.Calls); overflow {
		return 0, errGasUintOverflow
	}

	kvm.callGasTemp, err = callGas(gt, contract.Gas, gas, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if gas, overflow = math.SafeAdd(gas, kvm.callGasTemp); overflow {
		return 0, errGasUintOverflow
	}
	return gas, nil
}

func gasPush(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return GasFastestStep, nil
}

func gasSwap(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return GasFastestStep, nil
}

func gasDup(gt params.GasTable, kvm *KVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return GasFastestStep, nil
}
