package ui

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"image/color"
	"snake_game/game"
	"snake_game/network"
	"snake_game/protobuf"
	"time"
)

const (
	screenWidth  = 1400
	screenHeight = 910
	gameWidth    = 1200
	gameHeight   = 900
)

func startGame(gameName string, client *network.Client, playerName string) {
	gameWindow := fyne.CurrentApp().NewWindow(gameName)

	canvasContainer := container.NewWithoutLayout()
	gameWindow.SetContent(canvasContainer)
	gameWindow.Resize(fyne.NewSize(screenWidth, screenHeight))
	gameWindow.Show()

	go func() {
		sleepTime := client.Game().Field().DelayMS()

		for {
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
			score := client.PlayerScore()
			updateGameCanvas(canvasContainer, client.Game(), score, playerName)
		}
	}()

	gameWindow.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		switch key.Name {
		case fyne.KeyEscape:
			gameWindow.Close()
		case fyne.KeyUp:
			client.SendSteer(protobuf.Direction_UP)
		case fyne.KeyDown:
			client.SendSteer(protobuf.Direction_DOWN)
		case fyne.KeyLeft:
			client.SendSteer(protobuf.Direction_LEFT)
		case fyne.KeyRight:
			client.SendSteer(protobuf.Direction_RIGHT)
		}
	})

	gameWindow.SetOnClosed(func() {
		client.Stop()
	})
}

func updateGameCanvas(canvasContainer *fyne.Container, gameInstance *game.Game, score int, playerName string) {
	canvasContainer.Objects = nil

	field := gameInstance.Field()

	fieldWidth := field.Width()
	fieldHeight := field.Height()

	cellWidth := gameWidth / fieldWidth
	cellHeight := gameHeight / fieldHeight

	for x := 0; x < fieldWidth; x++ {
		for y := 0; y < fieldHeight; y++ {
			rect := &canvas.Rectangle{
				FillColor:   color.RGBA{R: 200, G: 200, B: 200, A: 255},
				StrokeColor: color.Black,
				StrokeWidth: 1,
			}
			rect.Move(fyne.NewPos(float32(x*cellWidth), float32(y*cellHeight)))
			rect.Resize(fyne.NewSize(float32(cellWidth), float32(cellHeight)))
			canvasContainer.Add(rect)
		}
	}

	snakes := field.Snakes()

	for _, snake := range snakes {
		body := snake.Body()
		for i, segment := range body {
			rect := &canvas.Rectangle{
				FillColor:   getColorById(i, snake.PlayerID()),
				StrokeColor: color.Black,
				StrokeWidth: 1,
			}
			rect.Move(fyne.NewPos(float32(segment.GetX()*int32(cellWidth)), float32(segment.GetY()*int32(cellHeight))))
			rect.Resize(fyne.NewSize(float32(cellWidth), float32(cellHeight)))
			canvasContainer.Add(rect)
		}
	}

	foods := field.Foods()

	for _, food := range foods {
		rect := &canvas.Rectangle{
			FillColor:   color.RGBA{R: 255, A: 255},
			StrokeColor: color.Black,
			StrokeWidth: 1,
		}
		rect.Move(fyne.NewPos(float32(food.GetX()*int32(cellWidth)), float32(food.GetY()*int32(cellHeight))))
		rect.Resize(fyne.NewSize(float32(cellWidth), float32(cellHeight)))
		canvasContainer.Add(rect)
	}
	playerNameText := canvas.NewText(fmt.Sprintf("Игрок: %s", playerName), color.White)
	playerNameText.Alignment = fyne.TextAlignCenter
	playerNameText.TextStyle = fyne.TextStyle{Bold: true}
	playerNameText.Resize(fyne.NewSize(100, 30))
	playerNameText.Move(fyne.NewPos(screenWidth-150, 10))
	canvasContainer.Add(playerNameText)

	scoreText := canvas.NewText(fmt.Sprintf("Счет: %d", score), color.White)
	scoreText.Alignment = fyne.TextAlignCenter
	scoreText.TextStyle = fyne.TextStyle{Bold: true}
	scoreText.Resize(fyne.NewSize(100, 30))
	scoreText.Move(fyne.NewPos(screenWidth-150, 40))
	canvasContainer.Add(scoreText)

	canvasContainer.Refresh()
}

func getColorById(segmentIndex int, id int) color.RGBA {
	if segmentIndex == 0 {
		return color.RGBA{R: uint8((90*id + 15) % 255), G: uint8((135*id + 15) % 255), B: uint8((55*id + 15) % 255), A: 255}
	}
	return color.RGBA{R: uint8((90 * id) % 255), G: uint8((135 * id) % 255), B: uint8((55 * id) % 255), A: 255}
}
