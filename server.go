package gotana

import (
	"fmt"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	BUFFER_SIZE                      = 4096
	TCP_CONNECTION_READLINE_DEADLINE = 30
)

type TCPCommand func(message string, conn net.Conn, server *TCPServer)

type TCPMessage struct {
	payload string
	conn    net.Conn
}

type TCPServer struct {
	engine   *Engine
	address  string
	messages chan TCPMessage
	commands map[string]interface{}
}

func writeLine(conn net.Conn, message string) {
	conn.Write([]byte(strings.Repeat("-", utf8.RuneCountInString(message)) + "\n"))
	conn.Write([]byte(message + "\n"))
	conn.Write([]byte(strings.Repeat("-", utf8.RuneCountInString(message)) + "\n"))
}

func increaseDeadline(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(time.Second * TCP_CONNECTION_READLINE_DEADLINE))
}

func (server *TCPServer) handleMessages() {
	for {
		tcpMessage := <-server.messages
		msg := strings.ToUpper(strings.TrimSpace(tcpMessage.payload))
		conn := tcpMessage.conn

		Logger().Debugf("Got message %s", msg)
		if v, ok := server.commands[msg]; ok {
			handler := v.(TCPCommand)
			handler(msg, conn, server)
		} else {
			writeLine(conn, fmt.Sprintf("No such command: %s", msg))
		}
	}
}

func (server *TCPServer) handleConnection(conn net.Conn) {
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
	listener, err := net.Listen("tcp", server.address)

	if err != nil {
		Logger().Errorf("Cannot start TCP server at: %s", server.address)
		return
	}

	go server.handleMessages()

	Logger().Infof("Started TCP server at: %s", server.address)

	for {
		conn, err := listener.Accept()

		if err != nil {
			Logger().Error(err.Error())
			continue
		}

		go server.handleConnection(conn)
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
	keys := GetMapKeys(server.commands)

	writeLine(conn, fmt.Sprintf("Available commands: %s", strings.Join(keys, ", ")))
}

func CommandStats(message string, conn net.Conn, server *TCPServer) {
	info := fmt.Sprintf("Total scrapers: %d. Total requests: %d",
		len(server.engine.scrapers), server.engine.Meta.RequestsTotal)

	writeLine(conn, info)

	for _, scraper := range server.engine.scrapers {
		writeLine(conn, scraper.String())
		writeLine(conn, fmt.Sprintf("Currently fetching: %s", scraper.CurrentUrl))
	}
}

func CommandList(message string, conn net.Conn, server *TCPServer) {
	names := make([]string, len(server.engine.scrapers))

	i := 0
	for _, scraper := range server.engine.scrapers {
		names[i] = scraper.Name
		i++
	}

	writeLine(conn, fmt.Sprintf("Running scrapers: %s", strings.Join(names, ", ")))
}

func CommandPause(message string, conn net.Conn, server *TCPServer) {

}


func CommandResume(message string, conn net.Conn, server *TCPServer) {

}

func NewTCPServer(address string, engine *Engine) (server *TCPServer) {
	server = &TCPServer{
		address:  address,
		engine:   engine,
		messages: make(chan TCPMessage),
		commands: make(map[string]interface{}),
	}
	server.AddCommand("LIST", CommandList)
	server.AddCommand("STATS", CommandStats)
	server.AddCommand("HELP", CommandHelp)
	server.AddCommand("PAUSE", CommandPause)
	server.AddCommand("RESUME", CommandResume)
	server.AddCommand("STOP", CommandStop)
	return
}
