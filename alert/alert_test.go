package alert

import (
	"github.com/bnb-chain/greenfield-challenger/config"
	"testing"
)

// Set botId, chatId in config
func TestAlert(t *testing.T) {
	configFilePath := "../config/config.json"
	cfg := config.ParseConfigFromFile(configFilePath)
	SendTelegramMessage(cfg.AlertConfig.Identity, cfg.AlertConfig.TelegramChatId, cfg.AlertConfig.TelegramBotId, "hi")
}
