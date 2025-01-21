package ui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"log"
	"snake_game/game"
	"snake_game/network"
	"snake_game/protobuf"
	"strconv"
	"sync"
	"time"
)

func ShowMainMenu(app fyne.App) {
	w := app.NewWindow("Snake Game")

	startNewGameButton := widget.NewButton("Начать новую игру", func() {
		ShowNewGameWindow(w)
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

func ShowNewGameWindow(parent fyne.Window) {
	newGameWindow := fyne.CurrentApp().NewWindow("Создать новую игру")

	playerNameEntry := widget.NewEntry()
	playerNameEntry.SetPlaceHolder("Введите имя игрока")
	playerNameEntry.SetText("theqly")

	gameNameEntry := widget.NewEntry()
	gameNameEntry.SetPlaceHolder("Введите имя игры")
	gameNameEntry.SetText("theqlys game")

	widthEntry := widget.NewEntry()
	widthEntry.SetPlaceHolder("Ширина карты")
	widthEntry.SetText("120")

	heightEntry := widget.NewEntry()
	heightEntry.SetPlaceHolder("Высота карты")
	heightEntry.SetText("90")

	foodStaticEntry := widget.NewEntry()
	foodStaticEntry.SetPlaceHolder("Сколько еды")
	foodStaticEntry.SetText("30")

	delayMSEntry := widget.NewEntry()
	delayMSEntry.SetPlaceHolder("Задержка")
	delayMSEntry.SetText("500")

	createButton := widget.NewButton("Создать", func() {
		playerName := playerNameEntry.Text
		gameName := gameNameEntry.Text
		width, _ := strconv.Atoi(widthEntry.Text)
		height, _ := strconv.Atoi(heightEntry.Text)
		foodStatic, _ := strconv.Atoi(foodStaticEntry.Text)
		delayMS, _ := strconv.Atoi(delayMSEntry.Text)

		server := network.NewServer(gameName, width, height, foodStatic, delayMS)
		err := server.Start()
		if err != nil {
			return
		}
		log.Printf("Создание игры: %s (Размер карты: %d x %d) (Сколько еды: %d) (Задержка: %d)",
			gameName, width, height, foodStatic, delayMS)

		client, err := network.NewClient(server.ServerAddr(), playerName, protobuf.NodeRole_MASTER)
		if err != nil {
			log.Printf("[server] cannot add server player: %s", err.Error())
		}

		client.Start(gameName, server.Game())

		client.SetServer(server)

		startGame(gameName, client, playerName)

		newGameWindow.Close()
		parent.Close()
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

	announcements := make([]*network.Announcement, 0)
	lock := &sync.Mutex{}

	gamesList := widget.NewList(
		func() int {
			lock.Lock()
			defer lock.Unlock()
			return len(announcements)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Игра")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			log.Println("UPDATE")
			lock.Lock()
			defer lock.Unlock()
			if i < len(announcements) {
				a := announcements[i]
				o.(*widget.Label).SetText(fmt.Sprintf("%s (%d игроков)", a.Announce().GetGameName(), len(a.Announce().GetPlayers().GetPlayers())))
			}
		},
	)

	var selectedAnnouncement *network.Announcement

	gamesList.OnSelected = func(id widget.ListItemID) {
		lock.Lock()
		defer lock.Unlock()
		if id >= 0 && id < len(announcements) {
			selectedAnnouncement = announcements[id]
		}
	}

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Имя")
	nameEntry.SetText("player")

	roleSelection := widget.NewRadioGroup([]string{"Игрок", "Зритель"}, func(selected string) {})
	roleSelection.SetSelected("Игрок")

	connectButton := widget.NewButton("Подключиться", func() {
		if selectedAnnouncement == nil {
			dialog.ShowInformation("Ошибка!", "Выберите игру", joinGameWindow)
			return
		}
		serverAddress := selectedAnnouncement.ServerAddr()

		var role protobuf.NodeRole
		if roleSelection.Selected == "Зритель" {
			role = protobuf.NodeRole_VIEWER
		} else {
			role = protobuf.NodeRole_NORMAL
		}

		name := nameEntry.Text

		client, err := network.NewClient(&serverAddress, name, role)
		if err != nil {
			log.Printf("[client] cannot connect to server: %s", err.Error())
		}

		g := game.NewGame(selectedAnnouncement.Announce().Config)

		err = client.Start(*selectedAnnouncement.Announce().GameName, g)
		if err != nil {
			dialog.ShowInformation("Не удалось подключиться к игре", err.Error(), joinGameWindow)
			return
		}

		log.Printf("[client]: connected to %s:%d with game %s",
			serverAddress.IP.String(), serverAddress.Port, selectedAnnouncement.Announce().GetGameName())

		startGame(*selectedAnnouncement.Announce().GameName, client, name)

		joinGameWindow.Close()
		parent.Close()
	})

	form := container.NewVBox(
		widget.NewLabel("Список доступных игр"),
		gamesList,
		widget.NewForm(widget.NewFormItem("Имя", nameEntry)),
		widget.NewLabel("Выберите роль:"),
		roleSelection,
		connectButton,
	)

	joinGameWindow.SetContent(form)
	joinGameWindow.Resize(fyne.NewSize(1200, 900))
	joinGameWindow.Show()

	ctx, cancel := context.WithCancel(context.Background())

	go network.ListenForAnnouncements(ctx, &announcements, lock)

	go func() {
		for {
			select {
			case <-ctx.Done():
				break
			default:
				time.Sleep(5 * time.Second)
				//TODO: refresh game list
			}
		}
	}()

	joinGameWindow.SetOnClosed(func() {
		cancel()
	})
}
