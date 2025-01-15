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
	lock          sync.Mutex
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
	}
}

func ParseSnake(protoSnake *protobuf.GameState_Snake, height, width int) *Snake {
	var bodySnake []*protobuf.GameState_Coord
	head := protoSnake.GetPoints()[0]
	bodySnake = append(bodySnake, head)

	x, y := int(head.GetX()), int(head.GetY())

	for _, offset := range protoSnake.GetPoints()[1:] {
		for j := 0; j < abs(int(offset.GetX())); j++ {
			x += sign(int(offset.GetX()))
			if x < 0 {
				x += width
			} else if x >= width {
				x -= width
			}
			bodySnake = append(bodySnake, &protobuf.GameState_Coord{X: proto.Int32(int32(x)), Y: proto.Int32(int32(y))})
		}

		for j := 0; j < abs(int(offset.GetY())); j++ {
			y += sign(int(offset.GetY()))
			if y < 0 {
				y += height
			} else if y >= height {
				y -= height
			}
			bodySnake = append(bodySnake, &protobuf.GameState_Coord{X: proto.Int32(int32(x)), Y: proto.Int32(int32(y))})
		}
	}

	return NewSnake(bodySnake, int(protoSnake.GetPlayerId()))
}

func GenerateSnakeProto(snake *Snake, height, width int) *protobuf.GameState_Snake {
	snake.lock.Lock()
	defer snake.lock.Unlock()

	snakeBuilder := &protobuf.GameState_Snake{
		PlayerId:      proto.Int32(int32(snake.playerID)),
		HeadDirection: &snake.headDirection,
		State:         &snake.state,
	}

	if len(snake.body) > 0 {
		head := snake.body[0]
		snakeBuilder.Points = append(snakeBuilder.Points, head)

		prevX, prevY := int(head.GetX()), int(head.GetY())
		cumulativeX, cumulativeY := 0, 0

		for _, coord := range snake.body[1:] {
			currentX, currentY := int(coord.GetX()), int(coord.GetY())
			deltaX, deltaY := calculateDelta(prevX, currentX, width), calculateDelta(prevY, currentY, height)

			if (deltaX != 0 && cumulativeY != 0) || (deltaY != 0 && cumulativeX != 0) {
				snakeBuilder.Points = append(snakeBuilder.Points, &protobuf.GameState_Coord{
					X: proto.Int32(int32(cumulativeX)),
					Y: proto.Int32(int32(cumulativeY)),
				})
				cumulativeX, cumulativeY = 0, 0
			}

			cumulativeX += deltaX
			cumulativeY += deltaY
			prevX, prevY = currentX, currentY
		}

		if cumulativeX != 0 || cumulativeY != 0 {
			snakeBuilder.Points = append(snakeBuilder.Points, &protobuf.GameState_Coord{
				X: proto.Int32(int32(cumulativeX)),
				Y: proto.Int32(int32(cumulativeY)),
			})
		}
	}

	return snakeBuilder
}

func (s *Snake) Move(field *Field) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	head := s.body[0]
	dx, dy := 0, 0

	for len(s.nextDirection) > 0 {
		dir := s.nextDirection[0]
		s.nextDirection = s.nextDirection[1:]
		if dir == s.headDirection {
			continue
		}
		if isValidDirectionChange(s.headDirection, dir) {
			s.headDirection = dir
		}
	}

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
		X: proto.Int32((head.GetX() + int32(dx) + int32(field.Width())) % int32(field.Width())),
		Y: proto.Int32((head.GetY() + int32(dy) + int32(field.Height())) % int32(field.Height())),
	}

	for _, coord := range s.body {
		if coord.GetX() == newHead.GetX() && coord.GetY() == newHead.GetY() {
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

func calculateDelta(prev, current, size int) int {
	delta := current - prev
	if abs(delta) > size/2 {
		if delta > 0 {
			delta -= size
		} else {
			delta += size
		}
	}
	return delta
}

func isValidDirectionChange(current, next protobuf.Direction) bool {
	return !(current == protobuf.Direction_LEFT && next == protobuf.Direction_RIGHT) &&
		!(current == protobuf.Direction_RIGHT && next == protobuf.Direction_LEFT) &&
		!(current == protobuf.Direction_UP && next == protobuf.Direction_DOWN) &&
		!(current == protobuf.Direction_DOWN && next == protobuf.Direction_UP)
}

func (s *Snake) Shrink() {
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

func (s *Snake) Head() *protobuf.GameState_Coord {
	return s.body[0]
}

func (s *Snake) Body() []*protobuf.GameState_Coord {
	return s.body
}

func (s *Snake) BodyContains(cell *protobuf.GameState_Coord) bool {
	for _, part := range s.body {
		if part.X == cell.X && part.Y == cell.Y {
			return true
		}
	}
	return false
}

func (s *Snake) HeadDirection() protobuf.Direction {
	return s.headDirection
}

func (s *Snake) PlayerID() int {
	return s.playerID
}

func (s *Snake) IsUpdated() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.updated
}
