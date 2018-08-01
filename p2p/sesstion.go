package p2p

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/EducationEKT/EKT/log"

	tp "github.com/henrylee2cn/teleport"
)

type PeerSess struct {
	peer   Peer
	online bool
	sess   tp.Session
	srv    tp.Peer
}

var peerSesss map[string]*PeerSess
var sesslock sync.RWMutex

func InitP2Pnet(peers []Peer, apiport uint16, api interface{}, debug bool) {
	peerSesss = make(map[string]*PeerSess)
	apiSrv := tp.NewPeer(tp.PeerConfig{
		CountTime:  true,
		ListenPort: apiport,
	})
	apiSrv.RouteCall(api)
	if !debug {
		tp.SetLoggerLevel("OFF")
	}
	go apiSrv.ListenAndServe()

	sm := sessmgr{peers: peers}
	sm.Start()
}

func GetSess(peer Peer) *PeerSess {
	if sess, ok := peerSesss[peer.PeerId]; ok {
		return sess
	}
	sesslock.Lock()
	defer sesslock.Unlock()
	peerSess := &PeerSess{peer: peer}
	err := peerSess.Dial()
	if err != nil {
		log.Error("get sess error, peerid %s, address %s, error %s", peerSess.peer.PeerId, peerSess.peer.Address, err.Error())
	}
	peerSesss[peer.PeerId] = peerSess
	return peerSess
}

func (ps *PeerSess) StartAPIService(self *Peer, apiStruct interface{}) error {
	srv := tp.NewPeer(tp.PeerConfig{
		CountTime:  true,
		ListenPort: uint16(self.Port),
	})
	srv.RouteCall(apiStruct)
	ps.srv = srv
	return srv.ListenAndServe()
}

func (ps *PeerSess) Dial() error {
	sess, err := tp.NewPeer(tp.PeerConfig{}).Dial(fmt.Sprintf("%s:%d", ps.peer.Address, ps.peer.Port))
	if err != nil {
		return errors.New(err.String())
	}
	sess.SetId(ps.peer.PeerId)
	ps.sess = sess
	ps.online = true
	return nil
}

func (ps *PeerSess) Online() bool {
	return ps.online
}

func (ps *PeerSess) CheckAlive() (bool, error) {
	if !ps.online {
		return false, errors.New("peer not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var result string
	rerr := ps.sess.Call(
		"/api/ping",
		ps.peer.PeerId,
		&result,
		tp.WithContext(ctx),
	).Rerror()
	if rerr != nil {
		ps.online = false
		return false, errors.New(rerr.String())
	}

	ok := bytes.Equal([]byte(result), []byte(ps.peer.PeerId))
	if ok {
		return true, nil
	}
	return false, errors.New("unexpected result")
}

func (ps *PeerSess) GetDBValue(key []byte) ([]byte, error) {
	if !ps.online {
		return nil, errors.New("peer not connected")
	}

	var result []byte
	rerr := ps.sess.Call(
		"/api/db/get",
		key,
		&result,
	).Rerror()
	if rerr != nil {
		return nil, errors.New(rerr.String())
	}
	return result, nil
}

func (ps *PeerSess) GetBlock(height int64) ([]byte, error) {
	if !ps.online {
		return nil, errors.New("peer not connected")
	}

	var result []byte
	rerr := ps.sess.Call(
		fmt.Sprintf("/api/block/get?height=%d", height),
		"",
		&result,
	).Rerror()
	if rerr != nil {
		return nil, errors.New(rerr.String())
	}
	return result, nil
}

func (ps *PeerSess) NewBlock(block []byte, forward bool) error {
	if !ps.online {
		return errors.New("peer not connected")
	}

	uri := "/api/block/new"
	if forward {
		uri += "?forward=true"
	}
	var result []byte
	rerr := ps.sess.Call(
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

func (ps *PeerSess) NewVote(vote []byte) error {
	if !ps.online {
		return errors.New("peer not connected")
	}

	var result []byte
	rerr := ps.sess.Call(
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

func (ps *PeerSess) VoteResults(votes []byte) error {
	if !ps.online {
		return errors.New("peer not connected")
	}

	var result []byte
	rerr := ps.sess.Call(
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

func (ps *PeerSess) GetVotes(hash string) ([]byte, error) {
	if !ps.online {
		return nil, errors.New("peer not connected")
	}

	var votes []byte
	rerr := ps.sess.Call(
		fmt.Sprintf("/api/vote/get?hash=%s", hash),
		"",
		&votes,
	).Rerror()
	if rerr != nil {
		return nil, errors.New(rerr.String())
	}
	return votes, nil
}

type sessmgr struct {
	peers  []Peer
	loogWG sync.WaitGroup
	quit   chan bool
}

func (sm *sessmgr) Start() error {
	go sm.dialAllPeer()

	sm.loogWG.Add(1)
	go sm.keepAlivedLoop()
	return nil
}

func (sm *sessmgr) dialAllPeer() {
	var wg sync.WaitGroup
	ok := 0
	for _, p := range sm.peers {
		wg.Add(1)
		go func(p Peer) {
			defer wg.Done()
			if GetSess(p).Online() {
				ok++
			}
		}(p)
	}
	wg.Wait()
	log.Info("Total %d peers, %d connected", len(sm.peers), ok)
}

func (sm *sessmgr) keepAlivedLoop() {
	defer sm.loogWG.Done()
	tick := time.Tick(6 * time.Second)
	for {
		select {
		case <-tick:
			var wg sync.WaitGroup
			for _, sess := range peerSesss {
				wg.Add(1)
				go func(s *PeerSess) {
					defer wg.Done()
					online, err := s.CheckAlive()
					if !online {
						s.Dial()
						log.Error("Peer %s is offline, address %s, error: %v\n", s.peer.PeerId, s.peer.Address, err)
						if err != nil {
							log.Error("Peer %s dial error: %v\n", s.peer.PeerId, err.Error())
						} else {
							log.Info("Peer %s dial success\n", s.peer.PeerId)
						}
					}
				}(sess)
			}
			wg.Wait()
		case <-sm.quit:
			log.Info("Keep alived loop quit")
			return
		}
	}
}
