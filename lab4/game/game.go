package game

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"math/rand"
	"snake_game/protobuf"
	"sync"
)

type Game struct {
	field *Field
	lock  *sync.Mutex
}

func NewGame(config *protobuf.GameConfig, lock *sync.Mutex) *Game {
	return &Game{
		field: NewField(config),
		lock:  lock,
	}
}

func EditFieldFromState(field *Field, stateMsg *protobuf.GameMessage_StateMsg) {
	if field == nil || stateMsg == nil {
		return
	}

	field.SetFoods(nil)
	var foods []*protobuf.GameState_Coord
	for _, food := range stateMsg.GetState().GetFoods() {
		foods = append(foods, food)
	}
	field.SetFoods(foods)

	fmt.Printf("[Game] field edit, cur snakes count %d, new snakes count %d\n", len(field.Snakes()), len(stateMsg.GetState().GetSnakes()))

	for _, snake := range field.Snakes() {
		snake.ClearUpdated()
	}

	for _, snakeProto := range stateMsg.GetState().GetSnakes() {
		field.UpdateSnake(ParseSnake(snakeProto, field.Height(), field.Width()))
	}

	var remainingSnakes []*Snake
	for _, snake := range field.Snakes() {
		if snake.IsUpdated() {
			remainingSnakes = append(remainingSnakes, snake)
		}
	}
	field.SetSnakes(remainingSnakes)
}

func (g *Game) Update() {
	g.lock.Lock()
	defer g.lock.Unlock()

	foodNeeded := g.field.AmountOfFoodNeeded(len(g.field.Snakes())) - len(g.field.Foods())
	for i := 0; i < foodNeeded; i++ {
		if g.field.HasPlace() {
			g.PlaceFood()
		} else {
			fmt.Println("[Game] place for food not found")
			break
		}
	}

	var snakesToRemove []*Snake
	for _, snake := range g.field.Snakes() {
		if !snake.Move(g.field) {
			snakesToRemove = append(snakesToRemove, snake)
			continue
		}

		if g.field.ContainsFood(snake.Head()) {
			snake.AddScore(1)
			g.field.RemoveFood(snake.Head())
		} else {
			snake.Shrink()
		}
	}

	for _, snakeToRemove := range snakesToRemove {
		g.field.RemoveSnake(snakeToRemove)
	}

	snakesToRemove = nil
	for _, snake := range g.field.Snakes() {
		for _, otherSnake := range g.field.Snakes() {
			if otherSnake == snake {
				continue
			}
			if otherSnake.Head().X == snake.Head().X && otherSnake.Head().Y == snake.Head().Y {
				snakesToRemove = append(snakesToRemove, snake, otherSnake)
				continue
			}
			for _, part := range otherSnake.Body() {
				if part.X == snake.Head().X && part.Y == snake.Head().Y {
					otherSnake.AddScore(1)
					snakesToRemove = append(snakesToRemove, snake)
					break
				}
			}
		}
	}

	for _, snakeToRemove := range snakesToRemove {
		g.field.RemoveSnake(snakeToRemove)
	}
}

func (g *Game) UpdateSnakeDirection(playerID int, newDirection protobuf.Direction) {
	for _, snake := range g.field.Snakes() {
		if snake.PlayerID() == playerID {
			snake.SetNextDirection(newDirection)
			return
		}
	}
	fmt.Printf("[Game] direction update: PlayerID %d not found", playerID)
}

func (g *Game) PlaceFood() {
	maxAttempts := g.field.Width() * g.field.Height()
	for attempt := 0; attempt < maxAttempts; attempt++ {
		x := rand.Intn(g.field.Width())
		y := rand.Intn(g.field.Height())
		food := &protobuf.GameState_Coord{
			X: proto.Int32(int32(x)),
			Y: proto.Int32(int32(y)),
		}
		if !g.field.IsCellOccupied(food) {
			foods := g.field.Foods()
			foods = append(foods, food)
			g.field.SetFoods(foods)
			return
		}
	}
}

func (g *Game) AddSnake(snake *Snake) {
	g.field.AddSnake(snake)
}

func (g *Game) Field() *Field {
	return g.field
}
