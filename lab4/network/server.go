package network

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"sync"
	"time"

	"snake_game/game"
	"snake_game/protobuf"
)

const (
	maxPlayersCount = 10
)

type Server struct {
	gameName   string
	masterId   int
	deputyId   int
	serverAddr *net.UDPAddr
	serverConn *net.UDPConn
	lockServer *sync.Mutex
	players    []*protobuf.GamePlayer
	lastPing   map[int]time.Time
	lockGame   *sync.Mutex
	game       *game.Game
	msgSeq     int64
	uniqueId   int
	stateId    int
	cancel     context.CancelFunc
	gameDelay  time.Duration
	pingDelay  time.Duration
	waitDelay  time.Duration
}

func NewServer(gameName string, width int, height int, foodStatic int, delayMS int) *Server {
	gameConf := protobuf.GameConfig{
		Width:        proto.Int32(int32(width)),
		Height:       proto.Int32(int32(height)),
		FoodStatic:   proto.Int32(int32(foodStatic)),
		StateDelayMs: proto.Int32(int32(delayMS)),
	}

	g := game.NewGame(&gameConf)

	return &Server{
		gameName:   gameName,
		masterId:   0,
		deputyId:   -1,
		lockServer: new(sync.Mutex),
		game:       g,
		lockGame:   g.Lock(),
		lastPing:   make(map[int]time.Time),
		msgSeq:     0,
		uniqueId:   0,
		stateId:    0,
		gameDelay:  time.Duration(delayMS),
		pingDelay:  time.Duration(float64(delayMS) * 0.1),
		waitDelay:  time.Duration(float64(delayMS) * 0.8),
	}
}

func (s *Server) Start() error {
	serverAddr, err := s.getInterfaceAddress("eth0")
	if err != nil {
		log.Printf("[server] failed to get eth0 address: %v", err)
		return err
	}

	serverConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		log.Printf("[server] failed to start server: %v", err)
		return err
	}

	localAddr := serverConn.LocalAddr().(*net.UDPAddr)

	s.serverAddr = localAddr
	s.serverConn = serverConn
	s.players = make([]*protobuf.GamePlayer, 0)

	s.startThreads()

	log.Printf("[server] server started on %s:%d", s.serverAddr.IP.String(), s.serverAddr.Port)

	return nil
}

func (s *Server) getInterfaceAddress(interfaceName string) (*net.UDPAddr, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error with getting interfaces: %v", err)
	}

	for _, iface := range interfaces {
		log.Printf("found interface: %s", iface.Name)
		if iface.Name == interfaceName {
			addrs, err := iface.Addrs()
			if err != nil {
				return nil, fmt.Errorf("error getting addresses for interface %s: %v", interfaceName, err)
			}

			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPNet:
					if v.IP.To4() != nil {
						return &net.UDPAddr{IP: v.IP}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("interface not found: %s", interfaceName)
}

func (s *Server) startThreads() {

	ctx, canc := context.WithCancel(context.Background())
	s.cancel = canc

	s.startAnnouncementSendThread(ctx)
	s.startListenerThread(ctx)
	s.startGameLoopThread(ctx)
	s.startPingCheckerThread(ctx)
}

func (s *Server) startAnnouncementSendThread(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				announcementMsg := s.createAnnouncementMessage()

				err := s.sendGameMessage(announcementMsg, MulticastAddress, MulticastPort)
				if err != nil {
					log.Printf("[server] announcement send error: %v", err)
					break
				}

				log.Printf("[server] sent announcement message, players count: %d", len(s.players))

				time.Sleep(AnnouncementDelay * time.Millisecond)
			}
		}
	}()
}

func (s *Server) startListenerThread(ctx context.Context) {
	go func() {
		buffer := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, addr, err := s.serverConn.ReadFromUDP(buffer)
				if err != nil {
					log.Printf("[server] Error reading from UDP: %v", err)
					continue
				}

				var msg protobuf.GameMessage
				err = proto.Unmarshal(buffer[:n], &msg)
				if err != nil {
					log.Printf("[server] Failed to unmarshal message: %v", err)
					continue
				}

				s.handleIncomingMessage(&msg, addr)
			}
		}
	}()
}

func (s *Server) startGameLoopThread(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(s.gameDelay * time.Millisecond)

				s.game.Update()

				s.updateDeputyId()
				s.updatePlayersScore()
				err := s.sendStateForAll()
				if err != nil {
					log.Printf("[server] Game loop destroyed: %v", err)
					break
				}
			}
		}
	}()
}

func (s *Server) startPingCheckerThread(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(s.pingDelay * time.Millisecond)
				s.sendPings()
				s.lockServer.Lock()
				now := time.Now()

				for _, player := range s.players {
					playerId := int(*player.Id)
					if now.Sub(s.lastPing[playerId]) > s.waitDelay*time.Millisecond {
						log.Printf("[server] player %d doesnt active, removing (last ping %s, now %s)", playerId, s.lastPing[playerId], now)
						s.removePlayerWithoutSnake(playerId)
					}
				}

				s.lockServer.Unlock()
			}
		}
	}()
}

func (s *Server) handleIncomingMessage(msg *protobuf.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {
	case *protobuf.GameMessage_Join:
		s.handleJoinMessage(*msg.MsgSeq, t.Join, addr)
	case *protobuf.GameMessage_Ack:
	case *protobuf.GameMessage_Ping:
		s.handlePing(addr)
	case *protobuf.GameMessage_Steer:
		s.handleSteer(*msg.GetSteer().Direction, addr)
	case *protobuf.GameMessage_RoleChange:
		s.handleRoleChange(msg, addr)
	case *protobuf.GameMessage_Error:
		log.Printf("[server] error received: %v", t.Error.ErrorMessage)
	default:
		log.Printf("[server] Unknown message type received")
	}
}

func (s *Server) handleJoinMessage(msgSeq int64, join *protobuf.GameMessage_JoinMsg, addr *net.UDPAddr) {

	log.Printf("[server] join message received from %s:%d", addr.IP.String(), addr.Port)

	if len(s.players) > maxPlayersCount {
		log.Printf("[server] max players reached")
		s.sendError("max players count reached", addr)
		return
	}

	playerId := s.addNewPlayer(join.GetPlayerName(), addr.IP.String(), addr.Port, join.GetRequestedRole().Enum())

	if join.GetRequestedRole() == protobuf.NodeRole_VIEWER {
		s.sendAcknowledgeMessage(int32(playerId), msgSeq, addr)
		log.Printf("[server] viewer joined the game")
		return
	}

	err := s.game.AddSnake(playerId)
	if err != nil {
		s.removePlayer(playerId)
		log.Printf("[server] failed to add snake: %v", err)
		s.sendError("no space for snake", addr)
		return
	}

	s.sendAcknowledgeMessage(int32(playerId), msgSeq, addr)

	s.lastPing[playerId] = time.Now()

	log.Printf("[server] player %s joined the game", join.GetPlayerName())
}

func (s *Server) handlePing(addr *net.UDPAddr) {
	playerId := s.getIdByAddr(addr)

	s.lockServer.Lock()
	s.lastPing[playerId] = time.Now()
	s.lockServer.Unlock()
}

func (s *Server) handleSteer(direction protobuf.Direction, addr *net.UDPAddr) {
	playerId := s.getIdByAddr(addr)

	if playerId == -1 {
		log.Printf("[server] failed to find player with addr %s:%d", addr.IP.String(), addr.Port)
		return
	}

	s.game.UpdateSnakeDirection(playerId, direction)

	s.lastPing[playerId] = time.Now()
	log.Printf("[server] received steer from %s:%d", addr.IP.String(), addr.Port)
}

func (s *Server) handleRoleChange(msg *protobuf.GameMessage, addr *net.UDPAddr) {
	roleChangeMsg := msg.GetRoleChange()

	senderRole := roleChangeMsg.GetSenderRole()
	//receiverRole := roleChangeMsg.GetReceiverRole()

	if senderRole == protobuf.NodeRole_VIEWER {
		s.makePlayerViewer(int(msg.GetSenderId()))
	}

	s.sendAcknowledgeMessage(msg.GetSenderId(), msg.GetMsgSeq(), addr)
}

func (s *Server) sendAcknowledgeMessage(playerId int32, msgSeq int64, addr *net.UDPAddr) error {
	ackMsg := &protobuf.GameMessage{
		MsgSeq:     proto.Int64(msgSeq),
		SenderId:   proto.Int32(int32(s.masterId)),
		ReceiverId: proto.Int32(playerId),
		Type: &protobuf.GameMessage_Ack{
			Ack: &protobuf.GameMessage_AckMsg{},
		},
	}

	err := s.sendGameMessage(ackMsg, addr.IP.String(), addr.Port)
	return err
}

func (s *Server) sendPings() error {
	s.lockServer.Lock()
	defer s.lockServer.Unlock()
	seq := s.incrementMsgSeq()

	pingMsg := &protobuf.GameMessage{
		MsgSeq: &seq,
		Type:   &protobuf.GameMessage_Ping{Ping: &protobuf.GameMessage_PingMsg{}},
	}

	for _, player := range s.players {
		s.sendGameMessage(pingMsg, player.GetIpAddress(), int(player.GetPort()))
	}

	return nil
}

func (s *Server) sendError(errorMessage string, addr *net.UDPAddr) error {
	errMsg := &protobuf.GameMessage{
		MsgSeq: proto.Int64(s.incrementMsgSeq()),
		Type: &protobuf.GameMessage_Error{
			Error: &protobuf.GameMessage_ErrorMsg{
				ErrorMessage: proto.String(errorMessage),
			},
		},
	}
	err := s.sendGameMessage(errMsg, addr.IP.String(), addr.Port)
	return err
}

func (s *Server) sendRoleChange(receiverRole protobuf.NodeRole, receiverId int) error {
	roleChangeMsg := &protobuf.GameMessage{
		MsgSeq:     proto.Int64(s.incrementMsgSeq()),
		SenderId:   proto.Int32(int32(s.masterId)),
		ReceiverId: proto.Int32(int32(receiverId)),
		Type: &protobuf.GameMessage_RoleChange{
			RoleChange: &protobuf.GameMessage_RoleChangeMsg{
				SenderRole:   protobuf.NodeRole_MASTER.Enum(),
				ReceiverRole: receiverRole.Enum(),
			},
		},
	}
	receiver := s.getAddrById(receiverId)
	log.Printf("[server] role change receiver: id: %d, addr: %s:%d", receiverId, receiver.IP.String(), receiver.Port)
	err := s.sendGameMessage(roleChangeMsg, receiver.IP.String(), receiver.Port)
	return err
}

func (s *Server) createAnnouncementMessage() *protobuf.GameMessage {
	serverName := s.gameName

	announcement := &protobuf.GameMessage{
		MsgSeq: proto.Int64(s.incrementMsgSeq()),
		Type: &protobuf.GameMessage_Announcement{
			Announcement: &protobuf.GameMessage_AnnouncementMsg{
				Games: []*protobuf.GameAnnouncement{
					{
						Players: &protobuf.GamePlayers{
							Players: s.getPlayerList(),
						},
						Config:   s.game.Field().GameConfig(),
						CanJoin:  proto.Bool(protobuf.Default_GameAnnouncement_CanJoin),
						GameName: proto.String(serverName),
					},
				},
			},
		},
	}

	return announcement
}

func (s *Server) sendStateForAll() error {
	gameState := s.createGameState()

	for _, player := range s.players {
		err := s.sendGameMessage(gameState, *player.IpAddress, int(*player.Port))
		if err != nil {
			log.Printf("[server] failed to send game message to player %s:%d: %s", *player.IpAddress, int(*player.Port), err.Error())
			continue
		}
		log.Printf("[server] send state for player %d", *player.Id)
	}

	return nil
}

func (s *Server) createGameState() *protobuf.GameMessage {
	stateId := s.incrementStateId()
	field := s.game.Field()

	state := &protobuf.GameState{
		StateOrder: proto.Int32(int32(stateId)),
	}

	var snakes []*protobuf.GameState_Snake
	for _, snake := range field.Snakes() {
		snakes = append(snakes, game.GenerateSnakeProto(snake, field.Width(), field.Height()))
	}

	state.Snakes = snakes

	state.Foods = append(state.Foods, s.game.Field().Foods()...)

	players := &protobuf.GamePlayers{
		Players: s.getPlayerList(),
	}

	state.Players = players

	stateMsg := &protobuf.GameMessage{
		MsgSeq: proto.Int64(s.incrementMsgSeq()),
		Type: &protobuf.GameMessage_State{
			State: &protobuf.GameMessage_StateMsg{
				State: state,
			},
		},
	}

	return stateMsg
}

func (s *Server) sendGameMessage(message *protobuf.GameMessage, address string, port int) error {
	data, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal game message: %v", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %v", err)
	}

	_, err = s.serverConn.WriteToUDP(data, udpAddr)
	if err != nil {
		return fmt.Errorf("failed to send game message: %v", err)
	}

	return nil
}

func (s *Server) updateDeputyId() {
	deputyId := 666
	for _, player := range s.players {
		if deputyId > int(player.GetId()) && player.GetRole() != protobuf.NodeRole_VIEWER && player.GetRole() != protobuf.NodeRole_MASTER {
			deputyId = int(player.GetId())
		}
	}

	if s.deputyId != deputyId && deputyId != 666 {
		s.sendRoleChange(protobuf.NodeRole_DEPUTY, deputyId)
		s.deputyId = deputyId
		s.changePlayerRole(s.deputyId, protobuf.NodeRole_DEPUTY)
		log.Printf("[server] new deputy id: %d", deputyId)
	}

}

func (s *Server) addNewPlayer(playerName string, address string, port int, role *protobuf.NodeRole) int {
	s.lockServer.Lock()
	defer s.lockServer.Unlock()

	playerId := s.uniqueId
	s.uniqueId++

	player := &protobuf.GamePlayer{
		Name:      proto.String(playerName),
		Id:        proto.Int32(int32(playerId)),
		IpAddress: proto.String(address),
		Port:      proto.Int32(int32(port)),
		Role:      role,
		Type:      protobuf.Default_GamePlayer_Type.Enum(),
		Score:     proto.Int32(0),
	}

	s.players = append(s.players, player)
	s.lastPing[playerId] = time.Now()
	return playerId
}

func (s *Server) removePlayer(playerId int) {
	newPlayers := make([]*protobuf.GamePlayer, 0, len(s.players))

	for _, player := range s.players {
		if player.GetId() != int32(playerId) {
			newPlayers = append(newPlayers, player)
		}
	}

	s.players = newPlayers

	s.game.RemoveSnake(playerId)
}

func (s *Server) removePlayerWithoutSnake(playerId int) {
	newPlayers := make([]*protobuf.GamePlayer, 0, len(s.players))

	for _, player := range s.players {
		if player.GetId() != int32(playerId) {
			newPlayers = append(newPlayers, player)
		}
	}

	if s.deputyId == playerId {
		s.deputyId = -1
	}

	s.players = newPlayers
}

func (s *Server) removeViewer(playerId int) {
	newPlayers := make([]*protobuf.GamePlayer, 0, len(s.players))

	for _, player := range s.players {
		if player.GetId() != int32(playerId) {
			newPlayers = append(newPlayers, player)
		}
	}

	s.players = newPlayers
}

func (s *Server) makePlayerViewer(playerId int) {

	for _, player := range s.players {
		if player.GetId() != int32(playerId) {
			player.Role = protobuf.NodeRole_VIEWER.Enum()
		}
	}

}

func (s *Server) updatePlayersScore() {
	for _, player := range s.players {
		if snake := s.game.Field().SnakeById(int(*player.Id)); snake != nil {
			s.lockServer.Lock()
			player.Score = proto.Int32(int32(snake.Score()))
			s.lockServer.Unlock()
		}
	}
}

func (s *Server) changePlayerRole(playerId int, role protobuf.NodeRole) {
	for _, player := range s.players {
		if player.GetId() == int32(playerId) {
			player.Role = role.Enum()
		}
	}
}

func (s *Server) Stop() error {
	log.Println("[server] stopping")
	log.Printf("[server] master id: %d, deputy id: %d", s.masterId, s.deputyId)
	if len(s.players) != 1 {
		err := s.sendRoleChange(protobuf.NodeRole_MASTER, s.deputyId)
		if err != nil {
			log.Printf("[server] failed to send role change to master: %v", err)
		}
	}
	s.cancel()
	return s.serverConn.Close()
}

func (s *Server) GameName() string {
	return s.gameName
}

func (s *Server) ServerAddr() *net.UDPAddr {
	return s.serverAddr
}

func (s *Server) GameConfig() *protobuf.GameConfig {
	return s.game.Field().GameConfig()
}

func (s *Server) Game() *game.Game {
	return s.game
}

func (s *Server) getPlayerList() []*protobuf.GamePlayer {
	return s.players
}

func (s *Server) getIdByAddr(addr *net.UDPAddr) int {
	var playerId int
	for _, player := range s.players {
		if *player.IpAddress == addr.IP.String() && *player.Port == int32(addr.Port) {
			playerId = int(*player.Id)
			return playerId
		}
	}
	return -1
}

func (s *Server) getAddrById(playerId int) *net.UDPAddr {
	for _, player := range s.players {
		if int(player.GetId()) == playerId {
			return &net.UDPAddr{IP: net.ParseIP(player.GetIpAddress()), Port: int(player.GetPort())}
		}
	}
	return nil
}

func (s *Server) incrementMsgSeq() int64 {
	s.msgSeq++
	return s.msgSeq
}

func (s *Server) incrementStateId() int {
	s.stateId++
	return s.stateId
}
