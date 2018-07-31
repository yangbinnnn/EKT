package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bytes"
	"strings"

	tp "github.com/henrylee2cn/teleport"
)

type Peer struct {
	PeerId         string `json:"peerId"`
	Address        string `json:"address"`
	Port           int32  `json:"port"`
	AddressVersion int    `json:"addressVersion"`
	AccountAddress string `json:"accountAddress"`

	online bool
	sess   tp.Session
	srv    tp.Peer
}

type Peers []*Peer

func (peers *Peers) Bytes() []byte {
	bts, _ := json.Marshal(peers)
	return bts
}

func (peer *Peer) String() string {
	data, _ := json.Marshal(peer)
	return string(data)
}

func (peer *Peer) Equal(other *Peer) bool {
	if strings.EqualFold(peer.PeerId, other.PeerId) && strings.EqualFold(peer.Address, other.Address) && peer.Port == other.Port && peer.AddressVersion == other.AddressVersion {
		return true
	}
	return false
}

func (peer *Peer) CopySess(other *Peer) {
	peer.sess = other.sess
	peer.online = other.online
}

func (peer *Peer) StartAPIService(apiStruct interface{}) error {
	srv := tp.NewPeer(tp.PeerConfig{
		CountTime:  true,
		ListenPort: uint16(peer.Port),
	})
	srv.RouteCall(apiStruct)
	peer.srv = srv
	return srv.ListenAndServe()
}

func (peer *Peer) Dial() error {
	sess, err := tp.NewPeer(tp.PeerConfig{}).Dial(fmt.Sprintf("%s:%d", peer.Address, peer.Port))
	if err != nil {
		return errors.New(err.String())
	}
	sess.SetId(peer.PeerId)
	peer.sess = sess
	peer.online = true
	return nil
}

func (peer *Peer) Online() bool {
	return peer.online
}

func (peer *Peer) CheckAlive() (bool, error) {
	if !peer.online {
		return false, errors.New("peer not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var result string
	rerr := peer.sess.Call(
		"/api/ping",
		peer.PeerId,
		&result,
		tp.WithContext(ctx),
	).Rerror()
	if rerr != nil {
		peer.online = false
		return false, errors.New(rerr.String())
	}

	ok := bytes.Equal([]byte(result), []byte(peer.PeerId))
	if ok {
		return true, nil
	}
	return false, errors.New("unexpected result")
}

func (peer *Peer) GetDBValue(key []byte) ([]byte, error) {
	if !peer.online {
		return nil, errors.New("peer not connected")
	}

	var result []byte
	rerr := peer.sess.Call(
		"/api/db/get",
		key,
		&result,
	).Rerror()
	if rerr != nil {
		return nil, errors.New(rerr.String())
	}
	return result, nil
}

func (peer *Peer) GetBlock(height int64) ([]byte, error) {
	if !peer.online {
		return nil, errors.New("peer not connected")
	}

	var result []byte
	rerr := peer.sess.Call(
		fmt.Sprintf("/api/block/get?height=%d", height),
		"",
		&result,
	).Rerror()
	if rerr != nil {
		return nil, errors.New(rerr.String())
	}
	return result, nil
}

func (peer *Peer) NewBlock(block []byte, forward bool) error {
	if !peer.online {
		return errors.New("peer not connected")
	}

	uri := "/api/block/new"
	if forward {
		uri += "?forward=true"
	}
	var result []byte
	rerr := peer.sess.Call(
		uri,
		block,
		&result,
	).Rerror()
	if rerr != nil {
		return errors.New(rerr.String())
	}
	if bytes.Equal(result, []byte("received")) {
		return nil
	}
	return errors.New("unexcpeted result")
}

func (peer *Peer) NewVote(vote []byte) error {
	if !peer.online {
		return errors.New("peer not connected")
	}

	var result []byte
	rerr := peer.sess.Call(
		"/api/vote/new",
		vote,
		&result,
	).Rerror()
	if rerr != nil {
		return errors.New(rerr.String())
	}
	if bytes.Equal(result, []byte("received")) {
		return nil
	}
	return errors.New("unexcpeted result")
}

func (peer *Peer) VoteResults(votes []byte) error {
	if !peer.online {
		return errors.New("peer not connected")
	}

	var result []byte
	rerr := peer.sess.Call(
		"/api/vote/results",
		votes,
		&result,
	).Rerror()
	if rerr != nil {
		return errors.New(rerr.String())
	}
	if bytes.Equal(result, []byte("received")) {
		return nil
	}
	return errors.New("unexcpeted result")
}

func (peer *Peer) GetVotes(hash string) ([]byte, error) {
	if !peer.online {
		return nil, errors.New("peer not connected")
	}

	var votes []byte
	rerr := peer.sess.Call(
		fmt.Sprintf("/api/vote/get?hash=%s", hash),
		"",
		&votes,
	).Rerror()
	if rerr != nil {
		return nil, errors.New(rerr.String())
	}
	return votes, nil
}
