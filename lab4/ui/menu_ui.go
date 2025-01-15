package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"log"
	"strconv"

	"snake_game/network"
)

func ShowMainMenu(app fyne.App) {
	w := app.NewWindow("Snake Game")

	startNewGameButton := widget.NewButton("Начать новую игру", func() {
		ShowNewGameWindow(app)
	})
	joinGameButton := widget.NewButton("Подключиться к игре", func() {
		ShowJoinGameWindow(w)
	})
	exitButton := widget.NewButton("Выйти из игры", func() {
		app.Quit()
	})

	menu := container.NewVBox(
		startNewGameButton,
		joinGameButton,
		exitButton,
	)
	w.SetContent(menu)
	w.Resize(fyne.NewSize(1200, 900))
	w.Show()
}

func ShowNewGameWindow(app fyne.App) {
	newGameWindow := fyne.CurrentApp().NewWindow("Создать новую игру")

	playerNameEntry := widget.NewEntry()
	playerNameEntry.SetPlaceHolder("Введите имя игрока")
	playerNameEntry.SetText("theqly")

	gameNameEntry := widget.NewEntry()
	gameNameEntry.SetPlaceHolder("Введите имя игры")
	gameNameEntry.SetText("theqlys game")

	widthEntry := widget.NewEntry()
	widthEntry.SetPlaceHolder("Ширина карты")
	widthEntry.SetText("100")

	heightEntry := widget.NewEntry()
	heightEntry.SetPlaceHolder("Высота карты")
	heightEntry.SetText("100")

	foodStaticEntry := widget.NewEntry()
	foodStaticEntry.SetPlaceHolder("Сколько еды")
	foodStaticEntry.SetText("10")

	delayMSEntry := widget.NewEntry()
	delayMSEntry.SetPlaceHolder("Задержка")
	delayMSEntry.SetText("10")

	createButton := widget.NewButton("Создать", func() {
		playerName := playerNameEntry.Text
		gameName := gameNameEntry.Text
		width, _ := strconv.Atoi(widthEntry.Text)
		height, _ := strconv.Atoi(heightEntry.Text)
		foodStatic, _ := strconv.Atoi(foodStaticEntry.Text)
		delayMS, _ := strconv.Atoi(delayMSEntry.Text)

		server := network.NewServer(width, height, foodStatic, delayMS)
		err := server.Start(playerName)
		if err != nil {
			return
		}
		log.Printf("Создание игры: %s (Размер карты: %d x %d) (Сколько еды: %d) (Задержка: %d)",
			gameName, width, height, foodStatic, delayMS)

		newGameWindow.Close()
		ShowGameWindow(app, gameName)
	})

	form := container.NewVBox(
		widget.NewLabel("Настройки игры"),
		widget.NewForm(
			widget.NewFormItem("Имя игрока", playerNameEntry),
			widget.NewFormItem("Имя игры", gameNameEntry),
			widget.NewFormItem("Ширина карты", widthEntry),
			widget.NewFormItem("Высота карты", heightEntry),
			widget.NewFormItem("Сколько еды", foodStaticEntry),
			widget.NewFormItem("Задержка", delayMSEntry),
		),
		createButton,
	)

	newGameWindow.SetContent(form)
	newGameWindow.Resize(fyne.NewSize(1200, 900))
	newGameWindow.Show()
}

func ShowJoinGameWindow(parent fyne.Window) {
	joinGameWindow := fyne.CurrentApp().NewWindow("Подключиться к игре")

	// TODO
	availableGames := widget.NewList(
		func() int { return 5 },
		func() fyne.CanvasObject {
			return widget.NewLabel("Игра")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("Игра %d", i+1))
		},
	)

	connectButton := widget.NewButton("Подключиться", func() {
		// TODO
		log.Println("Подключение к игре...")
		joinGameWindow.Close()
	})

	form := container.NewVBox(
		widget.NewLabel("Список доступных игр"),
		availableGames,
		connectButton,
	)

	joinGameWindow.SetContent(form)
	joinGameWindow.Resize(fyne.NewSize(400, 300))
	joinGameWindow.Show()
}
