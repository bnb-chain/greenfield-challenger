package monitor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func TestParseEvents(t *testing.T) {
	jsonBytes, _ := ioutil.ReadFile(`block_result.json`)

	blockRes := ctypes.ResultBlockResults{}
	err := json.Unmarshal(jsonBytes, &blockRes)
	fmt.Println(blockRes)
	fmt.Println(err)
}
