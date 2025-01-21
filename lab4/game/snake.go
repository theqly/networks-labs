package game

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"math/rand"
	"snake_game/protobuf"
	"sync"
)

type Snake struct {
	body          []*protobuf.GameState_Coord
	headDirection protobuf.Direction
	nextDirection []protobuf.Direction
	state         protobuf.GameState_Snake_SnakeState
	updated       bool
	color         string
	playerID      int
	score         int
	lock          *sync.Mutex
}

func NewSnake(initialPosition []*protobuf.GameState_Coord, playerID int) *Snake {
	randomColor := fmt.Sprintf("#%02x%02x%02x", rand.Intn(256), rand.Intn(256), rand.Intn(256))
	return &Snake{
		body:          initialPosition,
		headDirection: protobuf.Direction_RIGHT,
		nextDirection: []protobuf.Direction{},
		state:         protobuf.GameState_Snake_ALIVE,
		color:         randomColor,
		playerID:      playerID,
		score:         0,
		lock:          new(sync.Mutex),
	}
}

func ParseSnake(snake *protobuf.GameState_Snake, width, height int) *Snake {
	bodySnake := make([]*protobuf.GameState_Coord, 0)
	head := snake.GetPoints()[0]
	bodySnake = append(bodySnake, head)

	x, y := int(head.GetX()), int(head.GetY())

	for i := 1; i < len(snake.GetPoints()); i++ {
		offset := snake.GetPoints()[i]
		offsetX, offsetY := int(offset.GetX()), int(offset.GetY())

		for j := 0; j < abs(offsetX); j++ {
			x += sign(offsetX)
			x = (x + width) % width
			bodySnake = append(bodySnake, &protobuf.GameState_Coord{X: proto.Int32(int32(x)), Y: proto.Int32(int32(y))})
		}

		for j := 0; j < abs(offsetY); j++ {
			y += sign(offsetY)
			y = (y + height) % height
			bodySnake = append(bodySnake, &protobuf.GameState_Coord{X: proto.Int32(int32(x)), Y: proto.Int32(int32(y))})
		}
	}

	newSnake := NewSnake(bodySnake, int(*snake.PlayerId))
	newSnake.SetHeadDirection(snake.GetHeadDirection())
	newSnake.SetNextDirection(snake.GetHeadDirection())
	return newSnake
}

func GenerateSnakeProto(snake *Snake, width, height int) *protobuf.GameState_Snake {
	if len(snake.Body()) == 0 {
		return nil
	}

	head := snake.Body()[0]
	protoSnake := &protobuf.GameState_Snake{
		PlayerId:      proto.Int32(int32(snake.PlayerID())),
		Points:        []*protobuf.GameState_Coord{{X: proto.Int32(head.GetX()), Y: proto.Int32(head.GetY())}},
		State:         protobuf.GameState_Snake_ALIVE.Enum(),
		HeadDirection: snake.HeadDirection().Enum(),
	}

	for i := 1; i < len(snake.Body()); i++ {
		current := snake.Body()[i]
		previous := snake.Body()[i-1]

		dx := current.GetX() - previous.GetX()
		dy := current.GetY() - previous.GetY()

		if int(dx) == -(width - 1) {
			dx = 1
		} else if int(dx) == (width - 1) {
			dx = -1
		}

		if int(dy) == -(height - 1) {
			dy = 1
		} else if int(dy) == (height - 1) {
			dy = -1
		}

		protoSnake.Points = append(protoSnake.Points, &protobuf.GameState_Coord{X: proto.Int32(dx), Y: proto.Int32(dy)})
	}

	return protoSnake
}

func (s *Snake) Move(gameField *Field) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if len(s.body) == 0 {
		return false
	}

	head := s.body[0]
	var dx, dy int

	lastDirection := s.headDirection
	for len(s.nextDirection) > 0 {
		dir := s.nextDirection[0]
		s.nextDirection = s.nextDirection[1:]

		if dir == lastDirection {
			continue
		}

		switch dir {
		case protobuf.Direction_LEFT:
			if lastDirection != protobuf.Direction_RIGHT {
				lastDirection = protobuf.Direction_LEFT
			}
		case protobuf.Direction_RIGHT:
			if lastDirection != protobuf.Direction_LEFT {
				lastDirection = protobuf.Direction_RIGHT
			}
		case protobuf.Direction_UP:
			if lastDirection != protobuf.Direction_DOWN {
				lastDirection = protobuf.Direction_UP
			}
		case protobuf.Direction_DOWN:
			if lastDirection != protobuf.Direction_UP {
				lastDirection = protobuf.Direction_DOWN
			}
		}
	}

	s.headDirection = lastDirection

	switch s.headDirection {
	case protobuf.Direction_UP:
		dy = -1
	case protobuf.Direction_DOWN:
		dy = 1
	case protobuf.Direction_LEFT:
		dx = -1
	case protobuf.Direction_RIGHT:
		dx = 1
	}

	newHead := &protobuf.GameState_Coord{
		X: proto.Int32((int32(gameField.Width()) + head.GetX() + int32(dx)) % int32(gameField.Width())),
		Y: proto.Int32((int32(gameField.Height()) + head.GetY() + int32(dy)) % int32(gameField.Height())),
	}

	for _, segment := range s.body {
		if segment.GetX() == newHead.GetX() && segment.GetY() == newHead.GetY() {
			return false
		}
	}

	s.body = append([]*protobuf.GameState_Coord{newHead}, s.body...)

	return true
}

func (s *Snake) AddScore(val int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.score += val
}

func sign(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (s *Snake) Shrink() {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.body) > 1 {
		s.body = s.body[:len(s.body)-1]
	}
}

func (s *Snake) SetBody(body []*protobuf.GameState_Coord) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.body = body
}

func (s *Snake) SetHeadDirection(newDirection protobuf.Direction) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.headDirection = newDirection
}

func (s *Snake) SetNextDirection(newDirection protobuf.Direction) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.nextDirection = append(s.nextDirection, newDirection)
}

func (s *Snake) SetUpdated() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.updated = true
}

func (s *Snake) ClearUpdated() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.updated = false
}

func (s *Snake) SetScore(score int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.score = score
}

func (s *Snake) BodyContains(cell *protobuf.GameState_Coord) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	for _, part := range s.body {
		if part.GetX() == cell.GetX() && part.GetY() == cell.GetY() {
			return true
		}
	}
	return false
}

func (s *Snake) IsUpdated() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.updated
}

func (s *Snake) Head() *protobuf.GameState_Coord {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.body[0]
}

func (s *Snake) Body() []*protobuf.GameState_Coord {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.body
}

func (s *Snake) HeadDirection() protobuf.Direction {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.headDirection
}

func (s *Snake) PlayerID() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.playerID
}

func (s *Snake) Score() int {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.score
}

func (s *Snake) Color() string { return s.color }

func (s *Snake) State() protobuf.GameState_Snake_SnakeState {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.state
}

func (s *Snake) Lock() *sync.Mutex {
	return s.lock
}
