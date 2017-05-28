package gotana

import (
	"net"
	"strings"
	"time"
	"fmt"
)

const (
	BUFFER_SIZE = 4096
	TCP_CONNECTION_READLINE_DEADLINE = 30
)

type TCPCommand func(message string, conn net.Conn, server *TCPServer)

type TCPMessage struct {
	payload string
	conn net.Conn
}

type TCPServer struct {
	engine *Engine
	messages chan TCPMessage
	commands map[string]TCPCommand
}


func writeLine(conn net.Conn, message string) {
	conn.Write([]byte(message + "\n"))
}

func increaseDeadline(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(time.Second * TCP_CONNECTION_READLINE_DEADLINE))
}

func (server *TCPServer) handleTCPMessages() {
	for {
		tcpMessage := <-server.messages
		msg := strings.ToUpper(strings.TrimSpace(tcpMessage.payload))
		conn := tcpMessage.conn

		Logger().Debugf("Got message %s", msg)
		if commandHandler, ok := server.commands[msg]; ok {
			commandHandler(msg, conn, server)
		} else {
			writeLine(conn, fmt.Sprintf("No such command: %s", msg))
		}
	}
}

func (server *TCPServer) handleTCPConnection(conn net.Conn) {
	defer conn.Close()
	Logger().Debugf("Got new TCP connection: %v", conn.RemoteAddr())
	writeLine(conn, "Connection established. Enter a command")

	increaseDeadline(conn)
	buf := make([]byte, BUFFER_SIZE)

	for {
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			break
		}
		server.messages <- TCPMessage{string(buf[0:n]), conn}
		increaseDeadline(conn)
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

func (server *TCPServer) AddCommand(name string, handler TCPCommand) {
	server.commands[name] = handler
}

func CommandStop(message string, conn net.Conn, server *TCPServer) {
	writeLine(conn, "Stopping scrapers...")
	server.engine.StopScrapers()
}

func CommandHelp(message string, conn net.Conn, server *TCPServer) {

}


func CommandStats(message string, conn net.Conn, server *TCPServer) {
	Logger().Debug(message)
}


func NewTCPServer(engine *Engine) (server *TCPServer){
	server = &TCPServer{
		engine: engine,
		messages: make(chan TCPMessage),
		commands: make(map[string]TCPCommand),
	}
	server.AddCommand("STATS", CommandStats)
	server.AddCommand("HELP", CommandHelp)
	server.AddCommand("STOP", CommandStop)
	return
}