package handler_test

import (
	"io"
	"net"
	"sync"
	"testing"

	idl "github.com/Team-OurPlayground/idl/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"

	"github.com/Team-OurPlayground/our-playground-game-server/internal/handler"
	"github.com/Team-OurPlayground/our-playground-game-server/internal/util/threadsafe"
)

type tcpHandlerSuite struct {
	suite.Suite
	listenerChan chan struct{}
	dialChan     chan struct{}
	tcpHandler   handler.TCPHandler
	tcpChannels  *threadsafe.TCPChannels
	clientMap    *sync.Map
}

func (suite *tcpHandlerSuite) SetupSuite() {
	suite.listenerChan = make(chan struct{})
	suite.dialChan = make(chan struct{})

	suite.tcpChannels = &threadsafe.TCPChannels{
		FromClient: make(chan []byte, handler.MaxUser),
		ToClient:   make(chan []byte, handler.MaxUser),
		ErrChan:    make(chan error, 1),
	}
	suite.clientMap = new(sync.Map)
	suite.setConnections()
	suite.tcpHandler = handler.NewTCPHandler(suite.tcpChannels, suite.clientMap)
}

func (suite *tcpHandlerSuite) TestHandlePacket() {
	echoMessage := &idl.SearchRequest{
		Query: handler.ECHO,
		PosX:  1,
		PosY:  1,
	}

	echoMessageByte, err := proto.Marshal(echoMessage)
	if err != nil {
		suite.NoError(err, "proto Marshal Error at TestHandlePacket")
	}

	<-suite.dialChan // 데이퍼 받을 준비 완료 확인 후, 전송

	suite.tcpChannels.FromClient <- echoMessageByte

	go suite.tcpHandler.HandlePacket()

	// 테스트 끝날 때까지 대기
	<-suite.listenerChan
	<-suite.dialChan
}

func (suite *tcpHandlerSuite) setConnections() {
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		suite.NoError(err, "net.Listen Error at addClients")
	}

	go func() { // Listener
		defer func() {
			suite.listenerChan <- struct{}{}
			suite.T().Log("listening connections stored")
		}()

		suite.T().Log("conn listener.Accept() start")
		conn, err := listener.Accept()
		if err != nil {
			suite.NoError(err, "listener.Accept Error at addClients")
		}
		id := uuid.New().String()
		suite.clientMap.Store(id, conn)
		suite.T().Logf("clientMap saved %s", id)
	}()

	go func() { // Dial
		defer func() {
			suite.dialChan <- struct{}{}
			suite.T().Log("dial finished")
		}()
		suite.T().Log("dial start")
		conn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			suite.NoError(err, "net.Dial Error at addClients")
		}

		suite.dialChan <- struct{}{} // 데이터 받을 준비 완료

		buf := make([]byte, 1024)
		suite.T().Log("starting to Read")
		n, err := conn.Read(buf)
		suite.T().Log("data read")
		if err != nil {
			if err != io.EOF {
				suite.NoError(err, "conn.Read Error at addClients")
			}
		}

		searchRequest := &idl.SearchRequest{}
		if err := proto.Unmarshal(buf[:n], searchRequest); err != nil {
			suite.NoError(err, "proto.Unmarshal error")
		}
		suite.T().Log("dial received")
		suite.T().Logf("searchRequest.Query: %s, searchRequest.PosX: %d, searchRequest.PosY: %d", searchRequest.Query, searchRequest.PosX, searchRequest.PosY)
		suite.Equal(handler.ECHO, searchRequest.Query)
	}()
}

func TestAttachment(t *testing.T) {
	t.Run("tcpHandler", func(t *testing.T) {
		suite.Run(t, new(tcpHandlerSuite))
	})
}