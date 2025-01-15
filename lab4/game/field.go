package game

import (
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
	"snake_game/protobuf"
	"sync"
)

const (
	squareSize        = 5
	maxAttemptsToFind = 100000
)

type Field struct {
	config *protobuf.GameConfig
	snakes []*Snake
	foods  []*protobuf.GameState_Coord
	lock   sync.Mutex
}

func NewField(config *protobuf.GameConfig) *Field {
	return &Field{
		config: config,
		snakes: []*Snake{},
		foods:  []*protobuf.GameState_Coord{},
	}
}

func (f *Field) FindValidSnakePosition(initialPosition *[]*protobuf.GameState_Coord) protobuf.Direction {
	rand.Seed(rand.Int63())

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
			return headDirection
		}

	}

	panic("no space available for snake")
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
		if food.X == cell.X && food.Y == cell.Y {
			return true
		}
	}
	return false
}

func (f *Field) ContainsFood(cell *protobuf.GameState_Coord) bool {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, food := range f.foods {
		if food.X == cell.X && food.Y == cell.Y {
			return true
		}
	}
	return false
}

func (f *Field) UpdateSnake(snake *Snake) {
	f.lock.Lock()
	defer f.lock.Unlock()

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
	log.Printf("[Field] field count snakes: %d\n", len(f.snakes))
}

func (f *Field) RemoveSnake(snake *Snake) {
	f.lock.Lock()
	defer f.lock.Unlock()

	rand.Seed(rand.Int63())
	for _, body := range snake.Body() {
		if body.X == snake.Head().X && body.Y == snake.Head().Y {
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
