package p2pnet

import (
	"sync"
	"time"

	"github.com/EducationEKT/EKT/log"
	"github.com/EducationEKT/EKT/p2p"
)

// Server manager peers and provide api
type Server struct {
	self   *p2p.Peer
	others []*p2p.Peer
	loogWG sync.WaitGroup

	quit chan bool
}

func NewServer(self *p2p.Peer, others []*p2p.Peer) *Server {
	return &Server{self: self, others: others}
}

func (srv *Server) Start() error {
	go srv.startAPILoop()
	go srv.dialAllPeer()

	srv.loogWG.Add(1)
	go srv.keepAlivedLoop()
	return nil
}

func (srv *Server) Restart() error {
	if err := srv.Stop(); err != nil {
		return err
	}
	if err := srv.Start(); err != nil {
		return err
	}
	return nil
}

func (srv *Server) Stop() error {
	close(srv.quit)
	srv.loogWG.Wait()
	return nil
}

func (srv *Server) dialAllPeer() {
	var wg sync.WaitGroup
	ok := 0
	for _, peer := range srv.others {
		wg.Add(1)
		go func(p *p2p.Peer) {
			defer wg.Done()
			err := p.Dial()
			if err != nil {
				log.Error("Peer %s dial error, %v\n", p.PeerId, err.Error())
			} else {
				log.Info("Peer %s dial success\n", p.PeerId)
				ok++
			}
		}(peer)
	}
	wg.Wait()
	log.Info("Total %d peers, %d connected", len(srv.others), ok)
}

func (srv *Server) startAPILoop() error {
	return srv.self.StartAPIService(new(Api))
}

func (srv *Server) keepAlivedLoop() {
	defer srv.loogWG.Done()
	tick := time.Tick(6 * time.Second)
	for {
		select {
		case <-tick:
			var wg sync.WaitGroup
			for _, peer := range srv.others {
				wg.Add(1)
				go func(p *p2p.Peer) {
					defer wg.Done()
					online, err := p.CheckAlive()
					if !online {
						log.Error("Peer %s is offline, address: %s, error: %v\n", p.PeerId, p.Address, err)
						err := p.Dial()
						if err != nil {
							log.Error("Peer %s dial error: %v\n", p.PeerId, err.Error())
						} else {
							log.Info("Peer %s dial success\n", p.PeerId)
						}
					}
				}(peer)
			}
			wg.Wait()
		case <-srv.quit:
			log.Info("Keep alived loop quit")
			return
		}
	}
}
