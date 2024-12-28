package game

type Field struct {
	width  int
	height int
}

func CreateField(width, height int) Field {
	return Field{width, height}
}
