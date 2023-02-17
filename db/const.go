package db

type TxStatus int

const (
	Saved     TxStatus = 0
	SelfVoted TxStatus = 1
	AllVoted  TxStatus = 2
	Filled    TxStatus = 3
)
