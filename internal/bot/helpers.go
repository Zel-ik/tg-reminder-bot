// internal/bot/helpers.go
package bot

import (
	"strings"

	"gopkg.in/telebot.v3"
)

func isMessageToBot(b *telebot.Bot, msg *telebot.Message) bool {
	if msg.Chat.Type == telebot.ChatPrivate {
		return true

	}
	for _, e := range msg.Entities {
		if e.Type == telebot.EntityMention && e.Offset == 0 {
			mention := msg.Text[e.Offset : e.Offset+e.Length]
			if strings.HasPrefix(mention, "@") {
				username := mention[1:]
				if strings.Contains(username, b.Me.Username) {
					return true
				}
			}
		}
	}
	return false
}

func extractPlainTextAfterBotMention(b *telebot.Bot, msg *telebot.Message) string {
	text := msg.Text
	if msg.Chat.Type == telebot.ChatPrivate {
		return text
	}
	for _, e := range msg.Entities {
		if e.Type == telebot.EntityMention {
			mention := text[e.Offset : e.Offset+e.Length]
			if strings.EqualFold(mention[1:], b.Me.Username) {
				return strings.TrimSpace(text[e.Offset+e.Length:])
			}
		}
	}
	return text
}
