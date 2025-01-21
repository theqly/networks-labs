package network

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"snake_game/game"
	"snake_game/protobuf"
	"sync"
	"time"
)

type Client struct {
	playerName         string
	gameName           string
	playerId           int
	masterId           int
	deputyId           int
	playerType         protobuf.PlayerType
	role               protobuf.NodeRole
	gameLock           *sync.Mutex
	game               *game.Game
	addr               *net.UDPAddr
	conn               *net.UDPConn
	msgSeq             int64
	lock               *sync.Mutex
	cancel             context.CancelFunc
	stateId            int
	lastState          *protobuf.GameState
	lastMasterActivity time.Time
	server             *Server
	pingDelay          time.Duration
	waitDelay          time.Duration
}

func NewClient(serverAddr *net.UDPAddr, playerName string, requestedRole protobuf.NodeRole) (*Client, error) {
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		playerName: playerName,
		playerType: protobuf.PlayerType_HUMAN,
		role:       requestedRole,
		addr:       conn.LocalAddr().(*net.UDPAddr),
		conn:       conn,
		msgSeq:     0,
		lock:       new(sync.Mutex),
		stateId:    0,
		server:     nil,
	}, nil
}

func (c *Client) Start(gameName string, g *game.Game) error {
	err := c.sendJoinRequest(gameName)
	if err != nil {
		log.Printf("failed to send join request: %s", err.Error())
		return fmt.Errorf("failed to send join request: %w", err)
	}

	err = c.handleAcknowledge()
	if err != nil {
		log.Printf("[client] failed to handle acknowledge: %s", err.Error())
		return fmt.Errorf("failed to handle acknowledge: %w", err)
	}

	log.Printf("[client] successfully handled acknowledge")

	c.gameLock = g.Lock()
	c.game = g

	c.pingDelay = time.Duration(float64(g.Field().DelayMS()) * 0.1)
	c.waitDelay = time.Duration(float64(g.Field().DelayMS()) * 0.8)

	ctx, canc := context.WithCancel(context.Background())
	c.cancel = canc

	c.startPingThread(ctx)
	c.startListenThread(ctx)
	c.startMasterActivityCheckerThread(ctx)
	return nil
}

func (c *Client) StartServerFromState() {
	c.lock.Lock()
	defer c.lock.Unlock()
	state := c.lastState

	g := game.NewGame(c.game.Field().GameConfig())
	g.Field().EditFieldFromState(c.lastState)

	delay := time.Duration(g.Field().DelayMS())

	server := &Server{
		gameName:   c.gameName,
		masterId:   c.playerId,
		deputyId:   -1,
		serverAddr: c.addr,
		lockServer: new(sync.Mutex),
		players:    state.Players.Players,
		lockGame:   new(sync.Mutex),
		game:       g,
		msgSeq:     0,
		uniqueId:   -1,
		stateId:    int(*state.StateOrder),
		gameDelay:  delay,
		pingDelay:  time.Duration(float64(delay) * 0.1),
		waitDelay:  time.Duration(float64(delay) * 0.8),
	}

	c.conn.Close()

	serverConn, err := net.ListenUDP("udp", server.serverAddr)
	if err != nil {
		log.Printf("[server] failed to start server: %s", err.Error())
	}

	log.Printf("[server] new server addr: %s:%d", server.serverAddr.IP.String(), server.serverAddr.Port)

	server.serverConn = serverConn

	conn, err := net.DialUDP("udp", nil, server.serverAddr)
	if err != nil {
		log.Printf("failed to connect to server: %s", err.Error())
		return
	}
	c.conn = conn
	c.addr = conn.LocalAddr().(*net.UDPAddr)

	for _, player := range server.players {
		if c.playerId == int(player.GetId()) {
			player.IpAddress = proto.String(c.addr.IP.String())
			player.Port = proto.Int32(int32(c.addr.Port))
			player.Role = protobuf.NodeRole_MASTER.Enum()
		}
		if player.GetRole() == protobuf.NodeRole_NORMAL {
			server.deputyId = int(player.GetId())
		}
		if server.uniqueId < int(player.GetId()) {
			server.uniqueId = int(player.GetId() + 1)
		}

	}

	server.lastPing = make(map[int]time.Time, len(server.players))
	for _, player := range server.players {
		now := time.Now()
		server.lastPing[int(player.GetId())] = now.Add(server.waitDelay * time.Millisecond)
	}

	c.server = server
	server.startThreads()
}

func (c *Client) sendJoinRequest(gameName string) error {
	c.lock.Lock()
	c.msgSeq++
	seq := c.msgSeq
	c.lock.Unlock()

	joinMsg := &protobuf.GameMessage_JoinMsg{
		PlayerType:    &c.playerType,
		PlayerName:    &c.playerName,
		GameName:      &gameName,
		RequestedRole: &c.role,
	}

	gameMessage := &protobuf.GameMessage{
		MsgSeq: &seq,
		Type:   &protobuf.GameMessage_Join{Join: joinMsg},
	}

	data, err := proto.Marshal(gameMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal join message: %w", err)
	}

	n, err := c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send join message: %w", err)
	}

	if n != len(data) {
		return errors.New("incomplete message sent")
	}

	log.Println("[client] join request sent.")
	return nil
}

func (c *Client) handleAcknowledge() error {
	buffer := make([]byte, 1024)
	n, _, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		log.Printf("[cleint] error receiving ack message: %v\n", err)
		return err
	}

	var msg protobuf.GameMessage
	if err := proto.Unmarshal(buffer[:n], &msg); err != nil {
		log.Printf("Failed to unmarshal message: %v\n", err)
		return err
	}

	switch msg.Type.(type) {
	case *protobuf.GameMessage_Ack:
		break
	case *protobuf.GameMessage_Error:
		{
			log.Printf("[client] got error message: %s\n", *msg.GetError().ErrorMessage)
			return fmt.Errorf("got error message: %s", *msg.GetError().ErrorMessage)
		}
	}

	c.playerId = int(*msg.ReceiverId)
	c.masterId = int(*msg.SenderId)

	return nil
}

func (c *Client) startListenThread(ctx context.Context) {
	go func() {
		buffer := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				//c.lock.Lock()
				n, _, err := c.conn.ReadFromUDP(buffer)
				if err != nil {
					log.Printf("[client] error receiving message: %v\n", err)
					time.Sleep(c.pingDelay * time.Millisecond)
					continue
				}
				//c.lock.Unlock()

				var msg protobuf.GameMessage
				if err := proto.Unmarshal(buffer[:n], &msg); err != nil {
					log.Printf("Failed to unmarshal message: %v\n", err)
					continue
				}

				switch t := msg.GetType().(type) {
				case *protobuf.GameMessage_State:
					c.handleGameState(&msg)
				case *protobuf.GameMessage_Error:
					c.handleError(&msg)
				case *protobuf.GameMessage_Ping:
					c.lastMasterActivity = time.Now()
				case *protobuf.GameMessage_Ack:
				case *protobuf.GameMessage_RoleChange:
					c.handleRoleChange(&msg)
				default:
					log.Printf("[client] unhandled message type: %T\n", t)
				}
			}
		}
	}()
}

func (c *Client) startMasterActivityCheckerThread(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(c.waitDelay * time.Millisecond)
				now := time.Now()
				if now.Sub(c.lastMasterActivity) > c.waitDelay*time.Millisecond {

					c.updateMaster()
				}
			}
		}
	}()
}

func (c *Client) handleGameState(msg *protobuf.GameMessage) {
	stateMsg := msg.GetState()
	log.Printf("[client] received state: %d", *stateMsg.State.StateOrder)
	c.game.Field().EditFieldFromState(stateMsg.State)
	c.updateDeputy(stateMsg.State)
	c.sendAcknowledgeMessage(int32(c.masterId), *msg.MsgSeq)
	c.lastState = msg.GetState().State
	c.lastMasterActivity = time.Now()
}

func (c *Client) handleError(msg *protobuf.GameMessage) {
	errorMsg := msg.GetError()
	log.Printf("[client] received error message: %s\n", *errorMsg.ErrorMessage)
	c.sendAcknowledgeMessage(int32(c.masterId), *msg.MsgSeq)
}

func (c *Client) handleRoleChange(msg *protobuf.GameMessage) {
	roleChangeMsg := msg.GetRoleChange()

	//senderRole := roleChangeMsg.GetSenderRole()
	receiverRole := roleChangeMsg.GetReceiverRole()

	if receiverRole == protobuf.NodeRole_MASTER {
		log.Printf("[cleint] received role change to master")
		c.role = protobuf.NodeRole_MASTER
		c.masterId = c.playerId
		c.StartServerFromState()
	}

	if receiverRole == protobuf.NodeRole_DEPUTY {
		log.Printf("[cleint] received role change to deputy")
		c.role = protobuf.NodeRole_DEPUTY
		c.deputyId = c.PlayerId()
	}

	if receiverRole == protobuf.NodeRole_VIEWER {
		log.Printf("[cleint] received role change to viewer")
		c.role = protobuf.NodeRole_VIEWER
	}

}

func (c *Client) SendSteer(direction protobuf.Direction) {

	if c.role == protobuf.NodeRole_VIEWER {
		return
	}

	seq := c.incrementMsgSeq()

	steerMsg := &protobuf.GameMessage{
		MsgSeq: &seq,
		Type: &protobuf.GameMessage_Steer{
			Steer: &protobuf.GameMessage_SteerMsg{
				Direction: direction.Enum(),
			},
		},
	}

	c.sendGameMessage(steerMsg)

	log.Println("[client] steer sent")
}

func (c *Client) sendAcknowledgeMessage(playerId int32, msgSeq int64) {
	ackMsg := &protobuf.GameMessage{
		MsgSeq:     proto.Int64(msgSeq),
		SenderId:   proto.Int32(int32(c.masterId)),
		ReceiverId: proto.Int32(playerId),
		Type: &protobuf.GameMessage_Ack{
			Ack: &protobuf.GameMessage_AckMsg{},
		},
	}

	c.sendGameMessage(ackMsg)
}

func (c *Client) sendRoleChange(receiverRole protobuf.NodeRole, senderRole protobuf.NodeRole, senderId int, receiverId int) {
	roleChangeMsg := &protobuf.GameMessage{
		MsgSeq:     proto.Int64(c.incrementMsgSeq()),
		SenderId:   proto.Int32(int32(senderId)),
		ReceiverId: proto.Int32(int32(receiverId)),
		Type: &protobuf.GameMessage_RoleChange{
			RoleChange: &protobuf.GameMessage_RoleChangeMsg{
				SenderRole:   senderRole.Enum(),
				ReceiverRole: receiverRole.Enum(),
			},
		},
	}
	c.sendGameMessage(roleChangeMsg)
}

func (c *Client) sendPing() error {
	seq := c.incrementMsgSeq()

	pingMsg := &protobuf.GameMessage{
		MsgSeq: &seq,
		Type:   &protobuf.GameMessage_Ping{Ping: &protobuf.GameMessage_PingMsg{}},
	}

	c.sendGameMessage(pingMsg)
	return nil
}

func (c *Client) startPingThread(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.pingDelay * time.Millisecond):
				if err := c.sendPing(); err != nil {
					fmt.Printf("Ping error: %v\n", err)
				}
			}
		}
	}()
}

func (c *Client) sendGameMessage(msg *protobuf.GameMessage) {
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal message: %s", err.Error())
	}

	c.lock.Lock()
	_, err = c.conn.Write(data)
	c.lock.Unlock()
	if err != nil {
		log.Printf("[client] failed to send message: %s", err.Error())
	}
}

func (c *Client) updateDeputy(state *protobuf.GameState) {
	for _, player := range state.Players.GetPlayers() {
		if player.GetRole() == protobuf.NodeRole_DEPUTY {
			if c.deputyId != int(player.GetId()) {
				log.Printf("[client] old deputy id: %d, new deputy id: %d", c.deputyId, player.GetId())
			}
			c.deputyId = int(player.GetId())
		}
	}
}

func (c *Client) updateMaster() {
	log.Printf("[client] start updating master")

	if c.role == protobuf.NodeRole_DEPUTY {
		c.role = protobuf.NodeRole_MASTER
		c.masterId = c.playerId
		c.StartServerFromState()
		return
	}

	var newMasterAddr *net.UDPAddr
	for _, player := range c.lastState.Players.GetPlayers() {
		if int(player.GetId()) == c.deputyId {
			newMasterAddr = &net.UDPAddr{IP: net.ParseIP(player.GetIpAddress()), Port: int(player.GetPort())}
		}
	}

	if newMasterAddr == nil {
		log.Printf("[client] no deputy found")
		c.Stop()
		return
	}

	c.lock.Lock()
	c.conn.Close()

	log.Printf("[client] trying connect to new server: %s:%d", newMasterAddr.IP.String(), newMasterAddr.Port)
	conn, err := net.DialUDP("udp", c.addr, newMasterAddr)
	if err != nil {
		log.Printf("failed to connect to server: %s", err.Error())
	}
	c.lock.Unlock()

	log.Printf("[client] connected to new server: %s:%d", newMasterAddr.IP.String(), newMasterAddr.Port)
	c.sendPing()
	c.masterId = c.deputyId
	c.deputyId = -1
	c.conn = conn
}

func (c *Client) Stop() error {
	log.Println("[client] stopping")
	c.sendRoleChange(protobuf.NodeRole_MASTER, protobuf.NodeRole_VIEWER, c.playerId, c.masterId)
	c.cancel()
	if c.server != nil {
		c.server.Stop()
	}
	return c.conn.Close()
}

func (c *Client) incrementMsgSeq() int64 {
	c.msgSeq++
	return c.msgSeq
}

func (c *Client) SetServer(s *Server) {
	c.server = s
}

func (c *Client) Game() *game.Game {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.game
}

func (c *Client) Role() protobuf.NodeRole {
	return c.role
}

func (c *Client) PlayerId() int {
	return c.playerId
}

func (c *Client) PlayerScore() int {
	for _, snake := range c.game.Field().Snakes() {
		if snake.PlayerID() == c.playerId {
			return snake.Score()
		}
	}
	return 0
}
