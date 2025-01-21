package game

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
	"snake_game/protobuf"
	"sync"
)

const (
	squareSize        = 5
	maxAttemptsToFind = 100
)

type Field struct {
	config *protobuf.GameConfig
	snakes []*Snake
	foods  []*protobuf.GameState_Coord
	lock   *sync.Mutex
}

func NewField(config *protobuf.GameConfig) *Field {
	return &Field{
		config: config,
		snakes: []*Snake{},
		foods:  []*protobuf.GameState_Coord{},
		lock:   new(sync.Mutex),
	}
}

func (f *Field) EditFieldFromState(state *protobuf.GameState) {
	if state == nil {
		return
	}

	f.SetFoods(nil)
	var foods []*protobuf.GameState_Coord
	for _, food := range state.GetFoods() {
		foods = append(foods, food)
	}
	f.SetFoods(foods)

	for _, snake := range f.Snakes() {
		snake.ClearUpdated()
	}

	for _, snakeProto := range state.GetSnakes() {
		f.UpdateSnake(ParseSnake(snakeProto, f.Width(), f.Height()))
	}

	var remainingSnakes []*Snake
	for _, snake := range f.Snakes() {
		if snake.IsUpdated() {
			remainingSnakes = append(remainingSnakes, snake)
		}
		for _, player := range state.Players.GetPlayers() {
			if snake.PlayerID() == int(player.GetId()) {
				snake.SetScore(int(player.GetScore()))
			}
		}
	}

	f.SetSnakes(remainingSnakes)
}

func (f *Field) findValidSnakePosition(initialPosition *[]*protobuf.GameState_Coord) (protobuf.Direction, error) {

	for attempt := 0; attempt < maxAttemptsToFind; attempt++ {
		centerX := rand.Intn(f.Width())
		centerY := rand.Intn(f.Height())
		squareIsFree := true

		for dx := -squareSize / 2; dx <= squareSize/2; dx++ {
			for dy := -squareSize / 2; dy <= squareSize/2; dy++ {
				x := (centerX + dx + f.Width()) % f.Width()
				y := (centerY + dy + f.Height()) % f.Height()

				cell := &protobuf.GameState_Coord{X: proto.Int32(int32(x)), Y: proto.Int32(int32(y))}

				if f.IsCellOccupied(cell) {
					squareIsFree = false
					break
				}
			}
			if !squareIsFree {
				break
			}
		}

		head := &protobuf.GameState_Coord{X: proto.Int32(int32(centerX)), Y: proto.Int32(int32(centerY))}
		direction := rand.Intn(4)

		tailX, tailY := centerX, centerY
		var headDirection protobuf.Direction

		switch direction {
		case 0:
			tailY = (centerY - 1 + f.Width()) % f.Height()
			headDirection = protobuf.Direction_DOWN
		case 1:
			tailX = (centerX + 1) % f.Width()
			headDirection = protobuf.Direction_LEFT
		case 2:
			tailY = (centerY + 1) % f.Height()
			headDirection = protobuf.Direction_UP
		case 3:
			tailX = (centerX - 1 + f.Width()) % f.Width()
			headDirection = protobuf.Direction_RIGHT
		}

		tail := &protobuf.GameState_Coord{X: proto.Int32(int32(tailX)), Y: proto.Int32(int32(tailY))}

		if !f.IsCellOccupied(head) && !f.IsCellOccupied(tail) {
			*initialPosition = append(*initialPosition, head, tail)
			return headDirection, nil
		}

	}

	return 0, fmt.Errorf("no space for snake")
}

func (f *Field) IsCellOccupied(cell *protobuf.GameState_Coord) bool {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, snake := range f.snakes {
		if snake.BodyContains(cell) {
			return true
		}
	}
	for _, food := range f.foods {
		if food.GetX() == cell.GetX() && food.GetY() == cell.GetY() {
			return true
		}
	}
	return false
}

func (f *Field) ContainsFood(cell *protobuf.GameState_Coord) bool {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, food := range f.foods {
		if food.GetX() == cell.GetX() && food.GetY() == cell.GetY() {
			return true
		}
	}
	return false
}

func (f *Field) UpdateSnake(snake *Snake) {
	for _, snk := range f.snakes {
		if snk.PlayerID() == snake.PlayerID() {
			snk.SetBody(snake.Body())
			snk.SetHeadDirection(snake.HeadDirection())
			snk.SetUpdated()
			return
		}
	}
	f.AddSnake(snake)
	snake.SetUpdated()
}

func (f *Field) HasPlace() bool {
	f.lock.Lock()
	defer f.lock.Unlock()

	foodSize := len(f.foods)
	snakeSize := 0
	for _, snake := range f.snakes {
		snakeSize += len(snake.Body())
	}
	return (foodSize + snakeSize) < (f.Width()*f.Height() - 1)
}

func (f *Field) AddSnake(snake *Snake) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.snakes = append(f.snakes, snake)
}

func (f *Field) AddNewSnake(playerId int) error {

	initialPosition := make([]*protobuf.GameState_Coord, 0)
	headDirection, err := f.findValidSnakePosition(&initialPosition)
	if err != nil {
		log.Printf("[field] cannot add snake: %s", err.Error())
		return err
	}

	snake := NewSnake(initialPosition, playerId)
	snake.SetHeadDirection(headDirection)
	snake.SetNextDirection(headDirection)

	f.lock.Lock()
	f.snakes = append(f.snakes, snake)
	f.lock.Unlock()

	return nil
}

func (f *Field) RemoveSnake(playerId int) {
	f.lock.Lock()
	defer f.lock.Unlock()

	var snake *Snake
	for _, s := range f.snakes {
		if s.PlayerID() == playerId {
			snake = s
		}
	}

	if snake == nil {
		log.Printf("[field] snake with id %d doesnt exists", playerId)
		return
	}

	for _, body := range snake.Body() {
		if body.GetX() == snake.Head().GetX() && body.GetY() == snake.Head().GetY() {
			continue
		}
		if rand.Intn(100) < 50 {
			f.foods = append(f.foods, body)
		}
	}

	for i, snk := range f.snakes {
		if snk == snake {
			f.snakes = append(f.snakes[:i], f.snakes[i+1:]...)
			break
		}
	}
}

func (f *Field) RemoveFood(coord *protobuf.GameState_Coord) {
	var newFoods []*protobuf.GameState_Coord
	for _, food := range f.Foods() {
		if !(food.GetX() == coord.GetX() && food.GetY() == coord.GetY()) {
			newFoods = append(newFoods, food)
		}
	}
	f.SetFoods(newFoods)
}

func (f *Field) SetFoods(foods []*protobuf.GameState_Coord) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.foods = foods
}

func (f *Field) SetSnakes(snakes []*Snake) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.snakes = snakes
}

func (f *Field) AmountOfFoodNeeded(playerCount int) int {
	return playerCount + f.FoodStatic()
}

func (f *Field) GameConfig() *protobuf.GameConfig {
	return f.config
}

func (f *Field) Width() int {
	return int(*f.config.Width)
}

func (f *Field) Height() int {
	return int(*f.config.Height)
}

func (f *Field) FoodStatic() int {
	return int(*f.config.FoodStatic)
}

func (f *Field) DelayMS() int {
	return int(*f.config.StateDelayMs)
}

func (f *Field) Foods() []*protobuf.GameState_Coord {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.foods
}

func (f *Field) Snakes() []*Snake {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.snakes
}

func (f *Field) SnakeById(playerId int) *Snake {
	f.lock.Lock()
	defer f.lock.Unlock()
	for _, snake := range f.snakes {
		if playerId == snake.playerID {
			return snake
		}
	}
	return nil
}

func (f *Field) Lock() *sync.Mutex {
	return f.lock
}
