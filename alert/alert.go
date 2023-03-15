package alert

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/bnb-chain/greenfield-challenger/logging"
)

func SendTelegramMessage(identity string, botId string, chatId string, msg string) {
	if botId == "" || chatId == "" || msg == "" {
		return
	}

	endPoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botId)
	formData := url.Values{
		"chat_id":    {chatId},
		"parse_mode": {"html"},
		"text":       {fmt.Sprintf("%s: %s", identity, msg)},
	}
	_, err := http.PostForm(endPoint, formData)
	if err != nil {
		logging.Logger.Errorf("send telegram message error, bot_id=%s, chat_id=%s, msg=%s, err=%s \n", botId, chatId, msg, err.Error())
		return
	}
}
