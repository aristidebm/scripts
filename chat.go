package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"
)

var ListenAddr = ":8000"

// not thread safe
var Id = 0

func main() {
	hostname, _ := os.Hostname()
	app := App{
		peersToDisconnect:          sync.Map{},
		hostname:                   hostname,
		maxAttemptBeforeDisconnect: 3,
	}
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
	return fmt.Fprintln(p.conn, string(msg))
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
			// slog.Warn("unable to read from", "pear", p.Name)
			continue
		}
		// fmt.Printf("%s > %s\n", p.Name, content)
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
	peersToDisconnect          sync.Map
	hostname                   string
	maxAttemptBeforeDisconnect int
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

	var wg sync.WaitGroup

	wg.Add(1)
	go app.ListenMsg(msgChan)

	wg.Add(2)
	go app.Heartbeat()

	wg.Add(3)
	go app.GarbageConnectionCollector()

	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Warn("Cannot established connection with", "peer", conn.RemoteAddr().String())
		}
		go app.HandleConn(conn, msgChan)
	}

	// wait for all spawned goroutines to finish before ending the function
	wg.Wait()
}

func (app *App) HandleConn(conn net.Conn, msgChan chan Msg) {
	peer := NewPeer(conn, msgChan)
	welcomeMsg := fmt.Sprintf("> Welcome %s", peer.Name)
	// fmt.Print(welcomeMsg)
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
		// fmt.Print(msg)
		if _, err := p.Write([]byte(msg)); err != nil {
			slog.Info("cannot communicate with", "peer", p.Name)
			app.MarkAsCandidateToDisconnect(p)
		}
	}
}

func (app *App) MarkAsCandidateToDisconnect(peer Peer) {
	data, ok := app.peersToDisconnect.Load(peer.ID)

	value := -1
	if ok {
		value = data.(int)
	}

	// if the peer is not yet in peers to disconnect add him
	if !ok {
		app.peersToDisconnect.Store(peer.ID, 1)
	} else {
		app.peersToDisconnect.Store(peer.ID, value+1)
	}
}

func (app *App) Heartbeat() {
	for {
		for _, peer := range app.peers {
			app.sendHeartbeat(peer)
		}
		time.Sleep(10 * time.Second)
	}
}

func (app *App) sendHeartbeat(peer Peer) error {
	msg := fmt.Sprintf("%s > Heartbeat", app.hostname)
	_, err := peer.Write([]byte(msg))
	if err != nil {
		app.MarkAsCandidateToDisconnect(peer)
	}
	return err
}

func (app *App) GarbageConnectionCollector() {
	for {
		for _, peer := range app.peers {

			data, ok := app.peersToDisconnect.Load(peer.ID)
			// dont worry, we are sure, will not crash, since
			// we know that we always only store integers in this map
			value := -1
			if ok {
				value = data.(int)
			}
			if !ok || value <= app.maxAttemptBeforeDisconnect {
				continue
			}

			// cannot reach the client, disconnect the client
			if err := app.sendHeartbeat(peer); err != nil {
				slog.Info("Disconnecting...", "peer", peer.Name)
				peer.Disconnect()
				slog.Info("Disconnected", "peer", peer.Name)
				// remove the peer to peers to delete and from
				// connected peers
				app.peersToDisconnect.Delete(peer.ID)
				peers := []Peer{}
				for _, p := range app.peers {
					if p.ID != peer.ID {
						peers = append(peers, peer)
					}
				}
				app.peers = peers
				continue
			}

			// the client come back online, remove him from peers
			// to disconnect
			app.peersToDisconnect.Delete(peer.ID)
		}
	}
}
