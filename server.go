package gotana

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
	"unicode/utf8"
)

const (
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
	listener net.Listener
}

func writeLine(conn net.Conn, message string) {
	writer := bufio.NewWriter(conn)
	defer writer.Flush()

	writer.WriteString(strings.Repeat("-", utf8.RuneCountInString(message)) + "\n")
	writer.WriteString(message + "\n")
	writer.WriteString(strings.Repeat("-", utf8.RuneCountInString(message)) + "\n")
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
	reader := bufio.NewReader(conn)
	scanner := bufio.NewScanner(reader)

	for {
		scanned := scanner.Scan()
		if !scanned {
			if err := scanner.Err(); err != nil {
				Logger().Errorf("%v(%v)", err, conn.RemoteAddr())
			}
			break
		}
		server.messages <- TCPMessage{scanner.Text(), conn}
		increaseDeadline(conn)
	}

	Logger().Debugf("Connection from %v closed", conn.RemoteAddr())
}

func (server *TCPServer) Start() {
	listener, err := net.Listen("tcp", server.address)
	server.listener = listener

	defer listener.Close()
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

func (server *TCPServer) Stop() {
	Logger().Infof("Shutting down TCP server")
	server.listener.Close()
}

func (server *TCPServer) AddCommand(name string, handler TCPCommand) {
	server.commands[name] = handler
}

func CommandStop(message string, conn net.Conn, server *TCPServer) {
	writeLine(conn, "Stopping the engine...")
	server.engine.Stop()
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

func CommandExtensions(message string, conn net.Conn, server *TCPServer) {
	for _, extension := range server.engine.extensions {
		writeLine(conn, DescribeStruct(extension))
	}
}

func CommandMiddleware(message string, conn net.Conn, server *TCPServer) {
	for _, middleware := range server.engine.requestMiddleware {
		writeLine(conn, DescribeFunc(middleware))
	}
}

func CommandItems(message string, conn net.Conn, server *TCPServer) {

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
	server.AddCommand("STOP", CommandStop)
	server.AddCommand("EXTENSIONS", CommandExtensions)
	server.AddCommand("MIDDLEWARE", CommandMiddleware)
	server.AddCommand("ITEMS", CommandItems)

	return
}
