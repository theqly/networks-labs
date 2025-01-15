package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ShowGameWindow отображает игровое окно.
func ShowGameWindow(app fyne.App, gameName string) {
	w := app.NewWindow("Snake Game: " + gameName)

	// Игровое поле (заглушка)
	gameCanvas := canvas.NewRectangle(nil)
	gameCanvas.SetMinSize(fyne.NewSize(400, 400))

	// Панель управления
	scoreLabel := widget.NewLabel("Счет: 0")
	exitButton := widget.NewButton("Выйти", func() {
		w.Close()
	})

	panel := container.NewVBox(scoreLabel, exitButton)

	// Размещение элементов
	layout := container.NewBorder(nil, panel, nil, nil, gameCanvas)
	w.SetContent(layout)
	w.Resize(fyne.NewSize(600, 500))
	w.Show()
}
