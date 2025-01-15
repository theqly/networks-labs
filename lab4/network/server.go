package network

import (
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
	MulticastAddress = "239.192.0.4"
	MulticastPort    = 9192
)

type Server struct {
	sendConfig       SendConfig
	serverConn       *net.UDPConn
	players          []*protobuf.GamePlayer
	lockGame         sync.Mutex
	game             *game.Game
	incomingMessages chan *protobuf.GameMessage
	sentMessages     map[int64]*MessageInfo
	msgSeq           int64
}

type SendConfig struct {
	announcementDelay time.Duration
	pingDelay         time.Duration
	waitDelay         time.Duration
}

type MessageInfo struct {
	Timestamp    time.Time
	AttemptCount int
	Message      *protobuf.GameMessage
	Address      *net.UDPAddr
}

func NewServer(width int, height int, foodStatic int, delayMS int) *Server {
	sendConf := SendConfig{
		announcementDelay: 100,
		pingDelay:         100,
		waitDelay:         1000,
	}

	gameConf := protobuf.GameConfig{
		Width:        proto.Int32(int32(width)),
		Height:       proto.Int32(int32(height)),
		FoodStatic:   proto.Int32(int32(foodStatic)),
		StateDelayMs: proto.Int32(int32(delayMS)),
	}

	lock := new(sync.Mutex)

	g := game.NewGame(&gameConf, lock)

	return &Server{
		sendConfig: sendConf,
		game:       g,
		lockGame:   *lock,
	}
}

func (s *Server) Start(playerName string) error {
	serverAdrr, err := s.getInterfaceAddress("eth0")
	if err != nil {
		log.Printf("failed to get eth0 address: %v", err)
		return err
	}

	serverAdrr.Port = 0
	serverConn, err := net.ListenUDP("udp", serverAdrr)
	if err != nil {
		log.Printf("failed to start server: %v", err)
		return err
	}

	s.serverConn = serverConn
	s.incomingMessages = make(chan *protobuf.GameMessage, 100)
	s.players = make([]*protobuf.GamePlayer, 0)

	localAddr := serverConn.LocalAddr().(*net.UDPAddr)
	serverPort := localAddr.Port
	s.addNewPlayer(playerName, serverAdrr.IP.String(), serverPort, protobuf.NodeRole_MASTER.Enum())

	var initialPosition []*protobuf.GameState_Coord
	headDirection := s.game.Field().FindValidSnakePosition(&initialPosition)

	playerId := *s.players[0].Id
	newSnake := game.NewSnake(initialPosition, int(playerId))
	newSnake.SetHeadDirection(headDirection)
	newSnake.SetNextDirection(headDirection)

	s.game.AddSnake(newSnake)

	s.startThreads()

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
	go s.announcementSendThread()
	go s.gameLoop()
	go s.listenForMessages()
}

func (s *Server) listenForMessages() {
	buffer := make([]byte, 4096)
	for {
		n, addr, err := s.serverConn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("[Server] Error reading from UDP: %v", err)
			continue
		}

		var msg protobuf.GameMessage
		err = proto.Unmarshal(buffer[:n], &msg)
		if err != nil {
			log.Printf("[Server] Failed to unmarshal message: %v", err)
			continue
		}

		s.handleIncomingMessage(&msg, addr)
	}
}

func (s *Server) handleIncomingMessage(msg *protobuf.GameMessage, addr *net.UDPAddr) {
	switch t := msg.Type.(type) {
	case *protobuf.GameMessage_Join:
		s.handleJoinMessage(t.Join, addr)
	case *protobuf.GameMessage_Ping:
		//TODO: Handle Ping
	case *protobuf.GameMessage_Steer:
		//TODO: Handle Steer
	case *protobuf.GameMessage_Error:
		log.Printf("[Server] Error received: %v", t.Error.ErrorMessage)
	default:
		log.Printf("[Server] Unknown message type received")
	}
}

func (s *Server) handleJoinMessage(join *protobuf.GameMessage_JoinMsg, addr *net.UDPAddr) {
	s.addNewPlayer(join.GetPlayerName(), addr.IP.String(), addr.Port, join.GetRequestedRole().Enum())
	log.Printf("[Server] Player %s joined the game", join.GetPlayerName())
}

func (s *Server) sendError(errorMessage string, addr *net.UDPAddr) {
	errMsg := &protobuf.GameMessage{
		MsgSeq: proto.Int64(s.incrementMsgSeq()),
		Type: &protobuf.GameMessage_Error{
			Error: &protobuf.GameMessage_ErrorMsg{
				ErrorMessage: proto.String(errorMessage),
			},
		},
	}
	s.sendGameMessage(errMsg, addr.IP.String(), addr.Port)
}

func (s *Server) announcementSendThread() {
	for {
		announcementMsg := s.createAnnouncementMessage()

		err := s.sendGameMessage(announcementMsg, MulticastAddress, MulticastPort)
		if err != nil {
			log.Printf("[Server] Announcement send error: %v", err)
			break
		}

		log.Printf("sent announcement message")

		time.Sleep(s.sendConfig.announcementDelay * time.Millisecond)
	}
}

func (s *Server) gameLoop() {
	for {
		time.Sleep(time.Duration(s.game.Field().DelayMS()) * time.Millisecond)

		s.game.Update()

		s.updatePlayersScore()
		err := s.sendStateForAll()
		if err != nil {
			log.Printf("[Server] Game loop destroyed: %v", err)
			break
		}
	}
}

func (s *Server) createAnnouncementMessage() *protobuf.GameMessage {
	var field *game.Field
	field = s.game.Field()

	serverName := "Snake Game Server"

	announcement := &protobuf.GameMessage{
		MsgSeq: proto.Int64(s.incrementMsgSeq()),
		Type: &protobuf.GameMessage_Announcement{
			Announcement: &protobuf.GameMessage_AnnouncementMsg{
				Games: []*protobuf.GameAnnouncement{
					{
						Players: &protobuf.GamePlayers{
							Players: s.getPlayerList(),
						},
						Config: &protobuf.GameConfig{
							Width:        proto.Int32(int32(field.Width())),
							Height:       proto.Int32(int32(field.Height())),
							FoodStatic:   proto.Int32(int32(field.FoodStatic())),
							StateDelayMs: proto.Int32(int32(field.DelayMS())),
						},
						GameName: proto.String(serverName),
					},
				},
			},
		},
	}

	return announcement
}

//TODO
/*func (s *Server) startResenderThread() {
	go func() {
		ticker := time.NewTicker(time.Millisecond * s.sendConfig.pingDelay)
		defer ticker.Stop()

		for range ticker.C {
			currentTime := time.Now()
			for _, player := range s.players {
				for seq, info := range s.sentMessages {
					if time.Since(info.Timestamp) > time.Millisecond*800 {
						info.AttemptCount++
						if info.AttemptCount > 5 { // 5 попыток
							log.Printf("[Resender] Player %d does not respond, disconnecting...", info.Message.GetMsgSeq())
							s.handleDisconnection(player)
							continue
						}
						log.Printf("[Resender] Resending message seq %d to %s", seq, player.String())
						s.sendGameMessage(info.Message, *player.IpAddress, int(*player.Port))
					}
				}
			}
		}
	}()
}*/

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

func (s *Server) addNewPlayer(playerName string, address string, port int, role *protobuf.NodeRole) {
	player := &protobuf.GamePlayer{
		Name:      proto.String(playerName),
		Id:        proto.Int32(int32(len(s.players) + 1)),
		IpAddress: proto.String(address),
		Port:      proto.Int32(int32(port)),
		Role:      role,
		Type:      protobuf.PlayerType_HUMAN.Enum(),
		Score:     proto.Int32(0),
	}

	s.players = append(s.players, player)
}

func (s *Server) getPlayerList() []*protobuf.GamePlayer {
	return s.players
}

func (s *Server) incrementMsgSeq() int64 {
	s.msgSeq++
	return s.msgSeq
}

func (s *Server) updatePlayersScore() {}

func (s *Server) sendStateForAll() error {
	return nil
}
