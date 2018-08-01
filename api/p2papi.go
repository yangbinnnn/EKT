package api

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/EducationEKT/EKT/blockchain"
	"github.com/EducationEKT/EKT/blockchain_manager"
	"github.com/EducationEKT/EKT/conf"
	"github.com/EducationEKT/EKT/crypto"
	"github.com/EducationEKT/EKT/ctxlog"
	"github.com/EducationEKT/EKT/db"
	"github.com/EducationEKT/EKT/log"
	"github.com/EducationEKT/EKT/p2p"
	tp "github.com/henrylee2cn/teleport"
)

/**
api name: Xx_yy -> /api/xx/yy
**/

type Api struct {
	tp.CallCtx
}

// /api/ping
func (api *Api) Ping(arg *string) (*string, *tp.Rerror) {
	return arg, nil
}

// /api/block/get
func (api *Api) Block_get(arg *string) ([]byte, *tp.Rerror) {
	bc := blockchain_manager.MainBlockChain
	_height, _ := strconv.Atoi(api.Query().Get("height"))
	height := int64(_height)
	if bc.GetLastHeight() < height {
		log.Info("Heigth %d is heigher than current height, current height is %d \n", height, bc.GetLastHeight())
		return nil, tp.NewRerror(-1, "", fmt.Sprintf("Heigth %d is heigher than current height, current height is %d \n ", height, bc.GetLastHeight()))
	}
	block, err := bc.GetBlockByHeight(height)
	if err != nil {
		return nil, tp.NewRerror(-1, "", err.Error())
	}
	bytes, err := json.Marshal(block)
	if err != nil {
		return nil, tp.NewRerror(-1, "", err.Error())
	}
	return bytes, nil
}

func (api *Api) Block_new(body *[]byte) ([]byte, *tp.Rerror) {
	ctxlog := ctxlog.NewContextLog("Block from peer")
	defer ctxlog.Finish()
	var block blockchain.Block
	json.Unmarshal(*body, &block)
	ctxlog.Log("block", block)
	log.Info("Recieved new block : block=%v, blockHash=%s \n", string(block.Bytes()), hex.EncodeToString(block.Hash()))
	lastHeight := blockchain_manager.GetMainChain().GetLastHeight()
	if lastHeight+1 != block.Height {
		ctxlog.Log("Invalid height", true)
		log.Info("Block height is not right, want %d, get %d, give up voting. \n", lastHeight+1, block.Height)
		return nil, tp.NewRerror(-1, "", "error invalid height")
	}
	IP := strings.Split(api.RealIp(), ":")[0]
	if !strings.EqualFold(IP, conf.EKTConfig.Node.Address) &&
		strings.EqualFold(block.GetRound().Peers[block.GetRound().CurrentIndex].Address, IP) && block.GetRound().MyIndex() != -1 &&
		(block.GetRound().MyIndex()-block.GetRound().CurrentIndex+len(block.GetRound().Peers))%len(block.GetRound().Peers) < len(block.GetRound().Peers)/2 {
		//当前节点是打包节点广播，而且当前节点满足(currentIndex - miningIndex + len(DPoSNodes)) % len(DPoSNodes) < len(DPoSNodes) / 2
		if forward := api.Query().Get("forward"); forward == "true" {
			for i := 0; i < len(block.GetRound().Peers); i++ {
				if i == block.GetRound().CurrentIndex || i == block.GetRound().MyIndex() {
					continue
				}
				p := block.GetRound().Peers[i]
				p2p.GetSess(p).NewBlock(*body, true)
			}
			log.Info("Forward block to other succeed.")
		}
	}
	blockchain_manager.MainBlockChainConsensus.BlockFromPeer(ctxlog, block)
	return []byte("received"), nil
}

func (api *Api) Vote_new(body *[]byte) ([]byte, *tp.Rerror) {
	var vote blockchain.BlockVote
	err := json.Unmarshal(*body, &vote)
	if err != nil {
		log.Info("Invalid vote, unmarshal fail, abort.")
		return nil, tp.NewRerror(-1, "Invalid vote, unmarshal fail", err.Error())
	}

	log.Info("Received a vote: %s.\n", string(vote.Bytes()))
	if !vote.Validate() {
		log.Info("Invalid vote, validate fail, abort.")
		return nil, tp.NewRerror(-1, "Invalid vote, validate fail", "")
	}
	go blockchain_manager.GetMainChainConsensus().VoteFromPeer(vote)
	return []byte("received"), nil
}

func (api *Api) Vote_results(body *[]byte) ([]byte, *tp.Rerror) {
	var votes blockchain.Votes
	err := json.Unmarshal(*body, &votes)
	if err != nil {
		log.Info("Invalid votes, unmarshal fail, abort.")
		return nil, tp.NewRerror(-1, "Invalid votes, validate fail", err.Error())
	}
	go blockchain_manager.GetMainChainConsensus().RecieveVoteResult(votes)
	return []byte("received"), nil
}

func (api *Api) Vote_get(empty *[]byte) ([]byte, *tp.Rerror) {
	blockHash := api.Query().Get("hash")
	votes := blockchain_manager.GetMainChainConsensus().GetVotes(blockHash)
	if votes == nil {
		return nil, tp.NewRerror(-1, "", "votes not found")
	}
	return votes.Bytes(), nil
}

func (api *Api) DB_get(key *[]byte) ([]byte, *tp.Rerror) {
	if len(*key) != 32 {
		log.Info("Remote peer want a db value that len(key) is not 32 byte, return fail.")
		return nil, tp.NewRerror(-1, "", InvalidKey.Error())
	}
	data, err := db.GetDBInst().Get(*key)
	if err != nil {
		return nil, tp.NewRerror(-1, "", err.Error())
	}

	if !bytes.Equal(crypto.Sha3_256(data), *key) {
		log.Info("This key is not the hash of the db value, return fail.")
		return nil, tp.NewRerror(-1, "", "Invalid key")
	}

	return data, nil
}
