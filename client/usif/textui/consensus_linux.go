// +build linux

// Place the bitcoin consensus lib (libbitcoinconsensus.so) where OS can find it.
// If this file does not build and you don't know what to do, just delete it

package textui

/*
#cgo LDFLAGS: -ldl

#include <stdio.h>
#include <dlfcn.h>


typedef signed long long int64_t;

unsigned int (*_bitcoinconsensus_version)();

int (*_bitcoinconsensus_verify_script_with_amount)(const unsigned char *scriptPubKey, unsigned int scriptPubKeyLen, int64_t amount,
                                    const unsigned char *txTo        , unsigned int txToLen,
                                    unsigned int nIn, unsigned int flags, void* err);

int bitcoinConsensusVerifyScriptWithAmount(const unsigned char *scriptPubKey, unsigned int scriptPubKeyLen, int64_t amount,
                                    const unsigned char *txTo        , unsigned int txToLen,
                                    unsigned int nIn, unsigned int flags) {
	return _bitcoinconsensus_verify_script_with_amount(scriptPubKey, scriptPubKeyLen, amount, txTo, txToLen, nIn, flags, NULL);
}

unsigned int bitcoinconsensus_version() {
	return _bitcoinconsensus_version();
}

int init_bitcoinconsensus_so() {
	void *so = dlopen("libbitcoinconsensus.so", RTLD_LAZY);
	if (so) {
		*(void **)(&_bitcoinconsensus_version) = dlsym(so, "bitcoinconsensus_version");
		*(void **)(&_bitcoinconsensus_verify_script_with_amount) = dlsym(so, "bitcoinConsensusVerifyScriptWithAmount");
		if (!_bitcoinconsensus_version) {
			printf("libbitcoinconsensus.so not found\n");
			return 0;
		}
		if (!_bitcoinconsensus_verify_script_with_amount) {
			printf("libbitcoinconsensus.so is too old. Use one of bitcoin-core 0.13.1\n");
			return 0;
		}
		return 1;
	}
	return 0;
}

*/
import "C"

import (
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"
	"strconv"

	"github.com/ParallelCoinTeam/duod/client/common"
	"github.com/ParallelCoinTeam/duod/lib/btc"
	"github.com/ParallelCoinTeam/duod/lib/script"
	"github.com/ParallelCoinTeam/duod/lib/L"
)

var (
	ConsensusChecks uint64
	ConsensusExpErr uint64
	ConsensusErrors uint64
	mut             sync.Mutex
)

func check_consensus(pkScr []byte, amount uint64, i int, tx *btc.Tx, verFlags uint32, result bool) {
	var tmp []byte
	if len(pkScr) != 0 {
		tmp = make([]byte, len(pkScr))
		copy(tmp, pkScr)
	}
	tx_raw := tx.Raw
	if tx_raw == nil {
		tx_raw = tx.Serialize()
	}
	go func(pkScr []byte, txTo []byte, amount uint64, i int, verFlags uint32, result bool) {
		var pkscrPtr *C.uchar // default to null
		var pkscrLen C.uint   // default to 0
		if pkScr != nil {
			pkscrPtr = (*C.uchar)(unsafe.Pointer(&pkScr[0]))
			pkscrLen = C.uint(len(pkScr))
		}
		r1 := int(C.bitcoinConsensusVerifyScriptWithAmount(pkscrPtr, pkscrLen, C.int64_t(amount),
			(*C.uchar)(unsafe.Pointer(&txTo[0])), C.uint(len(txTo)), C.uint(i), C.uint(verFlags)))
		res := r1 == 1
		atomic.AddUint64(&ConsensusChecks, 1)
		if !result {
			atomic.AddUint64(&ConsensusExpErr, 1)
		}
		if res != result {
			atomic.AddUint64(&ConsensusErrors, 1)
			common.CountSafe("TxConsensusERR")
			mut.Lock()
			println("Compare to consensus failed!")
			println("Duod:", result, "   ConsLIB:", res)
			println("pkScr", hex.EncodeToString(pkScr))
			println("txTo", hex.EncodeToString(txTo))
			println("amount:", amount, "  input_idx:", i, "  verFlags:", verFlags)
			println()
			mut.Unlock()
		}
	}(tmp, tx_raw, amount, i, verFlags, result)
}

func verify_script_with_amount(pkScr []byte, amount uint64, i int, tx *btc.Tx, verFlags uint32) (result bool) {
	txTo := tx.Raw
	if txTo == nil {
		txTo = tx.Serialize()
	}
	var pkscrPtr *C.uchar // default to null
	var pkscrLen C.uint   // default to 0
	if pkScr != nil {
		pkscrPtr = (*C.uchar)(unsafe.Pointer(&pkScr[0]))
		pkscrLen = C.uint(len(pkScr))
	}
	r1 := int(C.bitcoinConsensusVerifyScriptWithAmount(pkscrPtr, pkscrLen, C.int64_t(amount),
		(*C.uchar)(unsafe.Pointer(&txTo[0])), C.uint(len(txTo)), C.uint(i), C.uint(verFlags)))

	result = (r1 == 1)
	return
}

func consensus_stats(s string) {
	fmt.Println("Consensus Checks:", atomic.LoadUint64(&ConsensusChecks))
	fmt.Println("Consensus ExpErr:", atomic.LoadUint64(&ConsensusExpErr))
	fmt.Println("Consensus Errors:", atomic.LoadUint64(&ConsensusErrors))
}

func init() {
	if C.init_bitcoinconsensus_so() == 0 {
		L.Debug("Not using libbitcoinconsensus.so to cross-check consensus rules")
		return
	}
	L.Debug("Using libbitcoinconsensus.so version" + strconv.Itoa(int(C.bitcoinconsensus_version())) + "to cross-check consensus")
	script.VerifyConsensus = check_consensus
	newUI("cons", false, consensus_stats, "See statistics of the consensus cross-checks")
}
