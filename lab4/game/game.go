package game

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
	"snake_game/protobuf"
	"sync"
)

type Game struct {
	field *Field
	lock  *sync.Mutex
}

func NewGame(config *protobuf.GameConfig) *Game {
	return &Game{
		field: NewField(config),
		lock:  new(sync.Mutex),
	}
}

func (g *Game) Update() {
	g.lock.Lock()
	defer g.lock.Unlock()

	foodNeeded := g.field.AmountOfFoodNeeded(len(g.field.Snakes())) - len(g.field.Foods())
	for i := 0; i < foodNeeded; i++ {
		if g.field.HasPlace() {
			g.PlaceFood()
		} else {
			fmt.Println("[game] place for food not found")
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
		g.field.RemoveSnake(snakeToRemove.playerID)
	}

	snakesToRemove = nil
	for _, snake := range g.field.Snakes() {
		for _, otherSnake := range g.field.Snakes() {
			if otherSnake == snake {
				continue
			}
			if otherSnake.Head().GetX() == snake.Head().GetX() && otherSnake.Head().GetY() == snake.Head().GetY() {
				snakesToRemove = append(snakesToRemove, snake, otherSnake)
				continue
			}
			for _, part := range otherSnake.Body() {
				if part.GetX() == snake.Head().GetX() && part.GetY() == snake.Head().GetY() {
					otherSnake.AddScore(1)
					snakesToRemove = append(snakesToRemove, snake)
					break
				}
			}
		}
	}

	for _, snakeToRemove := range snakesToRemove {
		g.field.RemoveSnake(snakeToRemove.playerID)
	}
}

func (g *Game) UpdateSnakeDirection(playerID int, newDirection protobuf.Direction) {
	for _, snake := range g.field.Snakes() {
		if snake.PlayerID() == playerID {
			snake.SetNextDirection(newDirection)
			return
		}
	}
	log.Printf("[game] direction update: player %d not found", playerID)
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

func (g *Game) AddSnake(playerId int) error {
	err := g.field.AddNewSnake(playerId)
	if err != nil {
		return err
	}
	return nil
}

func (g *Game) RemoveSnake(playerId int) {
	g.field.RemoveSnake(playerId)
}

func (g *Game) Field() *Field {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.field
}

func (g *Game) Lock() *sync.Mutex {
	return g.lock
}
