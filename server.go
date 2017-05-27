package gotana

import (
	"net"
	"strings"
	"time"
)

const (
	TCP_CONNECTION_READLINE_DEADLINE = 5
)


type TCPServer struct {
	engine *Engine
	messages chan string
}


func increaseDeadline(conn *net.Conn) {
	(*conn).SetReadDeadline(time.Now().Add(time.Second * TCP_CONNECTION_READLINE_DEADLINE))
}

func (server *TCPServer) handleTCPMessages() {
	for {
		msg := strings.TrimSpace(<-server.messages)
		Logger().Debug(msg)
	}
}

func (server *TCPServer) handleTCPConnection(conn net.Conn) {
	defer conn.Close()
	Logger().Debugf("Got new TCP connection: %v", conn.RemoteAddr())

	increaseDeadline(&conn)
	buf := make([]byte, 4096)

	for {
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			break
		}
		server.messages <- string(buf[0:n])
		increaseDeadline(&conn)
	}

	Logger().Debugf("Connection from %v closed", conn.RemoteAddr())
}

func (server *TCPServer) Start() {
	address := "localhost:7654"
	listener, err := net.Listen("tcp", address)

	if err != nil {
		Logger().Errorf("Cannot start TCP server at: %s", address)
		return
	}

	go server.handleTCPMessages()

	Logger().Infof("Started TCP server at: %s", address)

	for {
		conn, err := listener.Accept();

		if  err != nil {
			Logger().Error(err.Error())
			continue
		}

		go server.handleTCPConnection(conn)
	}
}


func NewTCPServer(engine *Engine) *TCPServer{
	return &TCPServer{
		engine: engine,
		messages: make(chan string),
	}
}