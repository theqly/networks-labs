package network

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"net"
	"snake_game/protobuf"
	"sync"
	"time"
)

type Client struct {
	serverAddress string
	serverPort    int
	playerName    string
	playerType    protobuf.PlayerType
	requestedRole protobuf.NodeRole
	conn          *net.UDPConn
	msgSeq        int64
	lock          sync.Mutex
	pingInterval  time.Duration
	stopChan      chan struct{}
}

func NewClient(serverAddress string, serverPort int, playerName string, playerType protobuf.PlayerType, requestedRole protobuf.NodeRole) (*Client, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverAddress, serverPort))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve server address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		serverAddress: serverAddress,
		serverPort:    serverPort,
		playerName:    playerName,
		playerType:    playerType,
		requestedRole: requestedRole,
		conn:          conn,
		msgSeq:        0,
		pingInterval:  5 * time.Second,
		stopChan:      make(chan struct{}),
	}, nil
}

func (c *Client) SendJoinRequest(gameName string) error {
	c.lock.Lock()
	c.msgSeq++
	seq := c.msgSeq
	c.lock.Unlock()

	joinMsg := &protobuf.GameMessage_JoinMsg{
		PlayerType:    &c.playerType,
		PlayerName:    &c.playerName,
		GameName:      &gameName,
		RequestedRole: &c.requestedRole,
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

	fmt.Println("Join request sent.")
	return nil
}

func (c *Client) ListenForMessages(ctx context.Context) {
	buffer := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stopping message listener.")
			return
		case <-c.stopChan:
			fmt.Println("Client stopped.")
			return
		default:
			n, _, err := c.conn.ReadFromUDP(buffer)
			if err != nil {
				fmt.Printf("Error receiving message: %v\n", err)
				continue
			}

			var msg protobuf.GameMessage
			if err := proto.Unmarshal(buffer[:n], &msg); err != nil {
				fmt.Printf("Failed to unmarshal message: %v\n", err)
				continue
			}

			switch t := msg.GetType().(type) {
			case *protobuf.GameMessage_State:
				c.handleGameState(t.State)
			case *protobuf.GameMessage_Error:
				c.handleError(t.Error)
			case *protobuf.GameMessage_Ping:
				c.handlePing()
			default:
				fmt.Printf("Unhandled message type: %T\n", t)
			}
		}
	}
}

func (c *Client) handleGameState(state *protobuf.GameMessage_StateMsg) {
	fmt.Println("Received game state update:", state)
}

func (c *Client) handleError(errorMsg *protobuf.GameMessage_ErrorMsg) {
	fmt.Printf("Received error message: %s\n", errorMsg.ErrorMessage)
}

func (c *Client) handlePing() {
	fmt.Println("Ping received from server.")
}

func (c *Client) SendPing() error {
	c.lock.Lock()
	c.msgSeq++
	seq := c.msgSeq
	c.lock.Unlock()

	pingMsg := &protobuf.GameMessage_PingMsg{}
	gameMessage := &protobuf.GameMessage{
		MsgSeq: &seq,
		Type:   &protobuf.GameMessage_Ping{Ping: pingMsg},
	}

	data, err := proto.Marshal(gameMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal ping message: %w", err)
	}

	_, err = c.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send ping message: %w", err)
	}

	fmt.Println("Ping sent.")
	return nil
}

func (c *Client) StartPingLoop() {
	go func() {
		for {
			select {
			case <-c.stopChan:
				return
			case <-time.After(c.pingInterval):
				if err := c.SendPing(); err != nil {
					fmt.Printf("Ping error: %v\n", err)
				}
			}
		}
	}()
}

func (c *Client) Close() error {
	close(c.stopChan)
	return c.conn.Close()
}
