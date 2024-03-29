package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"golang.org/x/exp/slices"
)

var telegramBot *bot.Bot
var cmdHandler cmdHandlerType
var sdAPI sdAPIType
var reqQueue ReqQueue

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

func sendTextToAdmins(ctx context.Context, s string) {
	for _, chatID := range params.AdminUserIDs {
		_, _ = telegramBot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   s,
		})
	}
}

type ImageFileData struct {
	data     []byte
	filename string
}

func handleImage(ctx context.Context, update *models.Update, fileID, filename string) {
	// Are we expecting image data from this user?
	if reqQueue.currentEntry.gotImageChan == nil || update.Message.From.ID != reqQueue.currentEntry.entry.Message.From.ID {
		return
	}

	var g GetFile
	d, err := g.GetFile(ctx, fileID)
	if err != nil {
		reqQueue.currentEntry.entry.sendReply(ctx, errorStr+": can't get file: "+err.Error())
		return
	}
	reqQueue.currentEntry.entry.sendReply(ctx, doneStr+" downloading\n"+reqQueue.currentEntry.entry.Params.String())
	// Updating the message to reply to this document.
	reqQueue.currentEntry.entry.Message = update.Message
	reqQueue.currentEntry.entry.ReplyMessage = nil
	// Notifying the request queue that we now got the image data.
	reqQueue.currentEntry.gotImageChan <- ImageFileData{
		data:     d,
		filename: filename,
	}
}

func handleMessage(ctx context.Context, update *models.Update) {
	if update.Message.Text == "" {
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
		cmdChar := string(cmd[0])
		cmd = cmd[1:] // Cutting the command character.
		switch cmd {
		case "sd":
			fmt.Println("  interpreting as cmd sd")
			cmdHandler.SD(ctx, update.Message)
			return
		case "sdupscale":
			fmt.Println("  interpreting as cmd sdupscale")
			cmdHandler.SDUpscale(ctx, update.Message)
			return
		case "sdcancel":
			fmt.Println("  interpreting as cmd sdcancel")
			cmdHandler.SDCancel(ctx, update.Message)
			return
		case "sdmodels":
			fmt.Println("  interpreting as cmd sdmodels")
			cmdHandler.Models(ctx, update.Message)
			return
		case "sdsamplers":
			fmt.Println("  interpreting as cmd sdsamplers")
			cmdHandler.Samplers(ctx, update.Message)
			return
		case "sdembeddings":
			fmt.Println("  interpreting as cmd sdembeddings")
			cmdHandler.Embeddings(ctx, update.Message)
			return
		case "sdloras":
			fmt.Println("  interpreting as cmd sdloras")
			cmdHandler.LoRAs(ctx, update.Message)
			return
		case "sdupscalers":
			fmt.Println("  interpreting as cmd sdupscalers")
			cmdHandler.Upscalers(ctx, update.Message)
			return
		case "sdvaes":
			fmt.Println("  interpreting as cmd sdvaes")
			cmdHandler.VAEs(ctx, update.Message)
			return
		case "sdsmi":
			fmt.Println("  interpreting as cmd sdsmi")
			cmdHandler.SMI(ctx, update.Message)
			return
		case "sdhelp":
			fmt.Println("  interpreting as cmd sdhelp")
			cmdHandler.Help(ctx, update.Message, cmdChar)
			return
		case "start":
			fmt.Println("  interpreting as cmd start")
			if update.Message.Chat.ID >= 0 { // From user?
				sendReplyToMessage(ctx, update.Message, "🤖 Welcome! This is a Telegram Bot frontend "+
					"for rendering images with Stable Diffusion.\n\nMore info: https://github.com/nonoo/stable-diffusion-telegram-bot")
			}
			return
		default:
			fmt.Println("  invalid cmd")
			if update.Message.Chat.ID >= 0 {
				sendReplyToMessage(ctx, update.Message, errorStr+": invalid command")
			}
			return
		}
	}

	if update.Message.Chat.ID >= 0 { // From user?
		cmdHandler.SD(ctx, update.Message)
	}
}

func telegramBotUpdateHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	if update.Message.Document != nil {
		handleImage(ctx, update, update.Message.Document.FileID, update.Message.Document.FileName)
	} else if update.Message.Photo != nil && len(update.Message.Photo) > 0 {
		handleImage(ctx, update, update.Message.Photo[len(update.Message.Photo)-1].FileID, "image.jpg")
	} else if update.Message.Text != "" {
		handleMessage(ctx, update)
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

	if params.SDStart && !params.DelayedSDStart {
		if err := startStableDiffusionIfNeeded(ctx); err != nil {
			panic(err.Error())
		}
	}

	reqQueue.Init(ctx)

	opts := []bot.Option{
		bot.WithDefaultHandler(telegramBotUpdateHandler),
	}

	var err error
	telegramBot, err = bot.New(params.BotToken, opts...)
	if nil != err {
		panic(fmt.Sprint("can't init telegram bot: ", err))
	}

	verStr, _ := versionCheckGetStr(ctx)
	sendTextToAdmins(ctx, "🤖 Bot started, "+verStr)

	go func() {
		for {
			time.Sleep(24 * time.Hour)
			if s, updateNeededOrError := versionCheckGetStr(ctx); updateNeededOrError {
				sendTextToAdmins(ctx, s)
			}
		}
	}()

	telegramBot.Start(ctx)
}
