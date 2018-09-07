package engine

import (
	"time"

	bproxy "github.com/mosaicnetworks/babble/proxy/babble"
	"github.com/mosaicnetworks/evm-babble/service"
	"github.com/mosaicnetworks/evm-babble/state"
	"github.com/sirupsen/logrus"
)

type BabbleSocketEngine struct {
	service     *service.Service
	state       *state.State
	babbleProxy *bproxy.SocketBabbleProxy
	submitCh    chan []byte
	logger      *logrus.Logger
}

func NewBabbleSocketEngine(config Config, logger *logrus.Logger) (*BabbleSocketEngine, error) {
	submitCh := make(chan []byte)

	state, err := state.NewState(logger,
		config.Eth.DbFile,
		config.Eth.Cache)
	if err != nil {
		return nil, err
	}

	service := service.NewService(config.Eth.Genesis,
		config.Eth.Keystore,
		config.Eth.EthAPIAddr,
		config.Eth.PwdFile,
		state,
		submitCh,
		logger)

	babbleProxy, err := bproxy.NewSocketBabbleProxy(config.Babble.ProxyAddr,
		config.Babble.ClientAddr,
		time.Duration(config.Babble.TCPTimeout)*time.Millisecond,
		logger)
	if err != nil {
		return nil, err
	}

	return &BabbleSocketEngine{
		service:     service,
		state:       state,
		babbleProxy: babbleProxy,
		submitCh:    submitCh,
		logger:      logger,
	}, nil
}

func (p *BabbleSocketEngine) serve() {
	for {
		select {
		case tx := <-p.submitCh:
			p.logger.Debug("proxy about to submit tx")
			if err := p.babbleProxy.SubmitTx(tx); err != nil {
				p.logger.WithError(err).Error("SubmitTx")
			}
			p.logger.Debug("proxy submitted tx")
		case commit := <-p.babbleProxy.CommitCh():
			p.logger.Debug("CommitBlock")
			stateHash, err := p.state.ProcessBlock(commit.Block)
			commit.Respond(stateHash.Bytes(), err)
		}
	}
}

/*******************************************************************************
Implement Engine interface
*******************************************************************************/

func (p *BabbleSocketEngine) Run() error {

	go p.service.Run()

	p.serve()

	return nil
}
