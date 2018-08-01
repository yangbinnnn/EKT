package p2p

import (
	"encoding/json"

	"strings"
)

type Peer struct {
	PeerId         string `json:"peerId"`
	Address        string `json:"address"`
	Port           int32  `json:"port"`
	AddressVersion int    `json:"addressVersion"`
	AccountAddress string `json:"accountAddress"`
}

type Peers []Peer

func (peers Peers) Bytes() []byte {
	bts, _ := json.Marshal(peers)
	return bts
}

func (peer Peer) String() string {
	data, _ := json.Marshal(peer)
	return string(data)
}

func (peer Peer) Equal(_peer Peer) bool {
	if strings.EqualFold(peer.PeerId, _peer.PeerId) && strings.EqualFold(peer.Address, _peer.Address) && peer.Port == _peer.Port && peer.AddressVersion == _peer.AddressVersion {
		return true
	}
	return false
}
