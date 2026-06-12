package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ahmedyounis/noema64/internal/appsvc"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	app := appsvc.NewApplication("")
	err := wails.Run(&options.App{
		Title:  "Noema64",
		Width:  1280,
		Height: 860,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			app.SetEventSink(func(name string, payload any) {
				runtime.EventsEmit(ctx, name, payload)
			})
		},
		OnShutdown: func(ctx context.Context) {
			app.SetEventSink(nil)
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		panic(err)
	}
}
