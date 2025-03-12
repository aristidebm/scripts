package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"time"
)

var ListenAddr = ":8000"

// not thread safe
var Id = 0

func main() {
	app := App{}
	app.Run()
}

type Msg struct {
	Author  Peer
	Content []byte
}

type Peer struct {
	ID          int
	Name        string
	conn        net.Conn
	ConnectedAt time.Time
	channel     chan Msg
}

func (p Peer) String() string {
	return fmt.Sprintf("%s (%s)", p.Name, p.Uptime())
}

func (p Peer) Write(msg []byte) (int, error) {
	return fmt.Fprint(p.conn, msg)
}

func (p Peer) Read(msg []byte) (int, error) {
	return p.conn.Read(msg)
}

func (p Peer) Disconnect() {
	defer p.conn.Close()
}

func (p Peer) Uptime() time.Duration {
	return time.Now().Sub(p.ConnectedAt)
}

func NewPeer(conn net.Conn, channel chan Msg) Peer {
	Id += 1
	return Peer{
		// fake ID
		ID:          Id,
		conn:        conn,
		Name:        conn.RemoteAddr().String(),
		ConnectedAt: time.Now().UTC(),
		channel:     channel,
	}
}

func (p Peer) Run() {
	slog.Info("starting an interactive session with", "peer", p.Name)
	// Listen to peer messages
	content := make([]byte, 258)
	for {
		_, err := p.Read(content)
		if err != nil {
			slog.Warn("unable to read from", "pear", p.Name)
		}
		fmt.Printf("%s > %s\n", p.Name, content)
		msg := Msg{
			Author:  p,
			Content: content,
		}
		p.channel <- msg
	}
}

type App struct {
	peers []Peer
	// a map to track peers to disconnect after a
	// certain amount of attempting to talk to that
	// specific peer
	peersToDisconnect map[int]int
}

func (app *App) AddPeer(peer Peer) {
	app.peers = append(app.peers, peer)
}

func (app *App) Run() {
	listener, err := net.Listen("tcp", ListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	slog.Info("The server has started and is running on", "address", ListenAddr)

	msgChan := make(chan Msg, 10)

	// FIXME: handle graceful shutdown
	go app.ListenMsg(msgChan)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Print("Cannot established connection with")
		}
		go app.HandleConn(conn, msgChan)
	}
}

func (app *App) HandleConn(conn net.Conn, msgChan chan Msg) {
	peer := NewPeer(conn, msgChan)
	welcomeMsg := fmt.Sprintf("> Welcome %s\n", peer.Name)
	fmt.Print(welcomeMsg)
	if _, err := peer.Write([]byte(welcomeMsg)); err != nil {
		slog.Warn("cannot communicate with", "peer", peer.Name)
		peer.Disconnect()
		return
	}
	app.AddPeer(peer)
	// launch peer discussion process
	peer.Run()
}

func (app *App) ListenMsg(channel chan Msg) {
	for {
		msg := <-channel
		app.NotifyPeers(msg)
	}
}

func (app *App) NotifyPeers(msg Msg) {
	for _, p := range app.peers {
		// it is useless to resend a message to it's author
		if p.ID == msg.Author.ID {
			continue
		}
		msg := fmt.Sprintf("%s > %s", msg.Author, string(msg.Content))
		fmt.Print(msg)
		if _, err := p.Write([]byte(msg)); err != nil {
			slog.Info("cannot communicate with", "peer", p.Name)
			app.MarkAsCandidateToDisconnect(p)
		}
	}
}

func (app *App) MarkAsCandidateToDisconnect(peer Peer) {
	value, ok := app.peersToDisconnect[peer.ID]
	// if the peer is not yet in peers to disconnect add him
	if !ok {
		app.peersToDisconnect[peer.ID] = 1
	} else {
		app.peersToDisconnect[peer.ID] = value + 1
	}
}
