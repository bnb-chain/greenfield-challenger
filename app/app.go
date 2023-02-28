package app

import (
	"github.com/bnb-chain/gnfd-challenger/config"
)

type App struct {
}

func NewApp(cfg *config.Config) *App {
	return &App{}
}

func (a *App) Start() {
	// do nothing now
}
