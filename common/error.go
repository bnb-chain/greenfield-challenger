package common

import "fmt"

var ErrEventExpired = fmt.Errorf("event expired")
var ErrDuplicatedSlashing = fmt.Errorf("duplicated slashing")
