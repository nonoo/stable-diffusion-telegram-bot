package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"golang.org/x/exp/slices"
)

var telegramBot *bot.Bot
var cmdHandler cmdHandlerType
var sdAPI sdAPIType
var dlQueue DownloadQueue

func sendReplyToMessage(ctx context.Context, replyToMsg *models.Message, s string) (msg *models.Message) {
	var err error
	msg, err = telegramBot.SendMessage(ctx, &bot.SendMessageParams{
		ReplyToMessageID: replyToMsg.ID,
		ChatID:           replyToMsg.Chat.ID,
		Text:             s,
	})
	if err != nil {
		fmt.Println("  reply send error:", err)
	}
	return
}

func telegramBotUpdateHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Text == "" { // Only handling message updates.
		return
	}

	fmt.Print("msg from ", update.Message.From.Username, "#", update.Message.From.ID, ": ", update.Message.Text, "\n")

	if update.Message.Chat.ID >= 0 { // From user?
		if !slices.Contains(params.AllowedUserIDs, update.Message.From.ID) {
			fmt.Println("  user not allowed, ignoring")
			return
		}
	} else { // From group ?
		fmt.Print("  msg from group #", update.Message.Chat.ID)
		if !slices.Contains(params.AllowedGroupIDs, update.Message.Chat.ID) {
			fmt.Println(", group not allowed, ignoring")
			return
		}
		fmt.Println()
	}

	// Check if message is a command.
	if update.Message.Text[0] == '/' || update.Message.Text[0] == '!' {
		cmd := strings.Split(update.Message.Text, " ")[0]
		if strings.Contains(cmd, "@") {
			cmd = strings.Split(cmd, "@")[0]
		}
		update.Message.Text = strings.TrimPrefix(update.Message.Text, cmd+" ")
		cmd = cmd[1:] // Cutting the command character.
		switch cmd {
		case "sd":
			cmdHandler.ED(ctx, update.Message)
			return
		case "sdcancel":
			cmdHandler.EDCancel(ctx, update.Message)
			return
		case "sdmodels":
			cmdHandler.Models(ctx, update.Message)
			return
		case "sdsamplers":
			cmdHandler.Samplers(ctx, update.Message)
			return
		case "sdembeddings":
			cmdHandler.Embeddings(ctx, update.Message)
			return
		case "sdloras":
			cmdHandler.LoRAs(ctx, update.Message)
			return
		case "sdupscalers":
			cmdHandler.Upscalers(ctx, update.Message)
			return
		case "sdhelp":
			cmdHandler.Help(ctx, update.Message)
			return
		case "start":
			fmt.Println("  (start cmd)")
			if update.Message.Chat.ID >= 0 { // From user?
				sendReplyToMessage(ctx, update.Message, "🤖 Welcome! This is a Telegram Bot frontend "+
					"for rendering images with Stable Diffusion.\n\nMore info: https://github.com/nonoo/stable-diffusion-telegram-bot")
			}
			return
		default:
			fmt.Println("  (invalid cmd)")
			if update.Message.Chat.ID >= 0 {
				sendReplyToMessage(ctx, update.Message, errorStr+": invalid command")
			}
			return
		}
	}

	if update.Message.Chat.ID >= 0 { // From user?
		cmdHandler.ED(ctx, update.Message)
	}
}

func main() {
	fmt.Println("stable-diffusion-telegram-bot starting...")

	if err := params.Init(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	var cancel context.CancelFunc
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if !params.DelayedSDStart {
		if err := startStableDiffusionIfNeeded(ctx); err != nil {
			panic(err.Error())
		}
	}

	dlQueue.Init(ctx)

	opts := []bot.Option{
		bot.WithDefaultHandler(telegramBotUpdateHandler),
	}

	var err error
	telegramBot, err = bot.New(params.BotToken, opts...)
	if nil != err {
		panic(fmt.Sprint("can't init telegram bot: ", err))
	}

	for _, chatID := range params.AdminUserIDs {
		_, _ = telegramBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "🤖 Bot started",
		})
	}

	telegramBot.Start(ctx)
}
