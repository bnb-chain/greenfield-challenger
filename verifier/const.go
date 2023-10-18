package verifier

import "time"

var VerifyHashLoopInterval = 2 * time.Second

var InternalSPEndpoints = []string{
	"https://greenfield-sp.bnbchain.org",
	"https://greenfield-sp.nodereal.io",
	"https://greenfield-sp.ninicoin.io",
	"https://greenfield-sp.defibit.io",
	"https://greenfield-sp.nariox.org",
	"https://greenfield-sp.lumibot.org",
	"https://greenfield-sp.voltbot.io",
}
