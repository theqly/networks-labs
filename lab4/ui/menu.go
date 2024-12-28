package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func RunApp() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Snakes Game")
	myWindow.Resize(fyne.NewSize(1200, 900))

	menu := container.NewVBox(
		widget.NewLabel("Main Menu"),
		widget.NewButton("Create Game", func() {
			showCreateGameDialog(myWindow)
		}),
		widget.NewButton("Join Game", func() {
			showJoinGameDialog(myWindow)
		}),
		widget.NewButton("Exit", func() {
			myApp.Quit()
		}),
	)

	myWindow.SetContent(container.NewCenter(menu))
	myWindow.ShowAndRun()
}

func showCreateGameDialog(parent fyne.Window) {
	widthEntry := widget.NewEntry()
	widthEntry.SetPlaceHolder("Enter width (e.g., 40)")

	heightEntry := widget.NewEntry()
	heightEntry.SetPlaceHolder("Enter height (e.g., 30)")

	foodStaticEntry := widget.NewEntry()
	foodStaticEntry.SetPlaceHolder("Enter static food count (e.g., 1)")

	stateDelayEntry := widget.NewEntry()
	stateDelayEntry.SetPlaceHolder("Enter state delay in ms (e.g., 1000)")

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Width", Widget: widthEntry},
			{Text: "Height", Widget: heightEntry},
			{Text: "Food Static", Widget: foodStaticEntry},
			{Text: "State Delay (ms)", Widget: stateDelayEntry},
		},
		OnSubmit: func() {
			width := widthEntry.Text
			height := heightEntry.Text
			foodStatic := foodStaticEntry.Text
			stateDelay := stateDelayEntry.Text

			dialog.ShowInformation("Game Parameters",
				"Width: "+width+"\nHeight: "+height+"\nFood Static: "+foodStatic+"\nState Delay: "+stateDelay,
				parent)
		},
	}

	dialog.ShowCustom("Create Game", "OK", container.NewVBox(form), parent)
}

func showJoinGameDialog(parent fyne.Window) {
	games := []string{"Game 1 - Host A", "Game 2 - Host B", "Game 3 - Host C"}

	list := widget.NewList(
		func() int {
			return len(games)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Game")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(games[id])
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		dialog.ShowInformation("Joining Game", "Joining: "+games[id], parent)
	}

	dialog.ShowCustom("Join Game", "OK", container.NewVBox(list), parent)
}
