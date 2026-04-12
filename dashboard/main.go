package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Instancia o app antes de subir o runtime do Wails.
	app := NewApp()

	// Sobe a aplicação com a configuração padrão do dashboard.
	err := wails.Run(&options.App{
		Title:  "dashboard",
		Width:  1024,
		Height: 768,
		Frameless: true,
		CSSDragProperty: "--wails-draggable",
		CSSDragValue: "drag",
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
