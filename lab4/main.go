package main

import (
	"fyne.io/fyne/v2/app"
	"snake_game/ui"
)

func main() {
	myApp := app.New()
	ui.ShowMainMenu(myApp)
	myApp.Run()
}
