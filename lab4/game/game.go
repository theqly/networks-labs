package game

type Game struct {
	field Field
}

func CreateGame() Game {
	return Game{CreateField(100, 100)}
}
