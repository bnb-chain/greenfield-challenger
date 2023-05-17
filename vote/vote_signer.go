package vote

import (
	"github.com/bnb-chain/greenfield-challenger/logging"
	"github.com/cometbft/cometbft/votepool"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	blscmn "github.com/prysmaticlabs/prysm/crypto/bls/common"
)

type VoteSigner struct {
	privKey blscmn.SecretKey
	pubKey  blscmn.PublicKey
}

func NewVoteSigner(pk []byte) *VoteSigner {
	privKey, err := blst.SecretKeyFromBytes(pk)
	if err != nil {
		logging.Logger.Errorf("vote signer failed to generate key from bytes, err=%+v", err.Error())
		panic(err)
	}
	pubKey := privKey.PublicKey()
	return &VoteSigner{
		privKey: privKey,
		pubKey:  pubKey,
	}
}

// SignVote sign a vote, data is used to sign and generate the signature
func (signer *VoteSigner) SignVote(vote *votepool.Vote, data []byte) {
	signature := signer.privKey.Sign(data[:])
	vote.EventHash = append(vote.EventHash, data[:]...)
	vote.PubKey = append(vote.PubKey, signer.pubKey.Marshal()...)
	vote.Signature = append(vote.Signature, signature.Marshal()...)
}
