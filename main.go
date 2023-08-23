package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"golang.org/x/exp/slices"
)

var telegramBot *bot.Bot
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

func handleCmdED(ctx context.Context, msg *models.Message) {
	renderParams := RenderParams{
		OrigPrompt:  msg.Text,
		Seed:        rand.Uint32(),
		Width:       512,
		Height:      512,
		Steps:       20,
		NumOutputs:  4,
		CfgScale:    7,
		SamplerName: params.DefaultSampler,
		ModelName:   params.DefaultModel,
		HR: RenderParamsHR{
			DenoisingStrength: 0.7,
			Scale:             2,
			Upscaler:          "R-ESRGAN 4x+",
		},
	}

	var prompt []string
	var promptLine string

	lines := strings.Split(msg.Text, "\n")
	if len(lines) >= 2 {
		promptLine = strings.TrimSpace(lines[0])
		renderParams.NegativePrompt = strings.TrimSpace(strings.Join(lines[1:], " "))
	} else {
		promptLine = strings.TrimSpace(msg.Text)
	}

	words := strings.Split(promptLine, " ")
	for i := range words {
		words[i] = strings.TrimSpace(words[i])

		if words[i][0] != '-' { // Only process words starting with -
			prompt = append(prompt, words[i])
			continue
		}

		splitword := strings.Split(words[i], ":")
		if len(splitword) == 2 {
			attr := strings.ToLower(splitword[0][1:])
			val := splitword[1]

			switch attr {
			case "seed", "s":
				val = strings.TrimPrefix(val, "🌱")
				valInt, err := strconv.ParseUint(val, 10, 32)
				if err != nil {
					fmt.Println("  invalid seed")
					sendReplyToMessage(ctx, msg, errorStr+": invalid seed")
					return
				}
				renderParams.Seed = uint32(valInt)
			case "width", "w":
				valInt, err := strconv.Atoi(val)
				if err != nil {
					fmt.Println("  invalid width")
					sendReplyToMessage(ctx, msg, errorStr+": invalid width")
					return
				}
				renderParams.Width = valInt
			case "height", "h":
				valInt, err := strconv.Atoi(val)
				if err != nil {
					fmt.Println("  invalid height")
					sendReplyToMessage(ctx, msg, errorStr+": invalid height")
					return
				}
				renderParams.Height = valInt
			case "steps", "t":
				valInt, err := strconv.Atoi(val)
				if err != nil {
					fmt.Println("  invalid steps")
					sendReplyToMessage(ctx, msg, errorStr+": invalid steps")
					return
				}
				renderParams.Steps = valInt
			case "outcnt", "o":
				valInt, err := strconv.Atoi(val)
				if err != nil {
					fmt.Println("  invalid output count")
					sendReplyToMessage(ctx, msg, errorStr+": invalid output count")
					return
				}
				renderParams.NumOutputs = valInt
			case "scale", "c":
				valFloat, err := strconv.ParseFloat(val, 32)
				if err != nil {
					fmt.Println("  invalid cfg scale")
					sendReplyToMessage(ctx, msg, errorStr+": invalid cfg scale")
					return
				}
				renderParams.CfgScale = float32(valFloat)
			case "sampler", "r":
				val = strings.ReplaceAll(val, ".", " ")
				samplers, err := sdAPI.GetSamplers(ctx)
				if err != nil {
					fmt.Println("  error getting samplers:", err)
					sendReplyToMessage(ctx, msg, errorStr+": error getting samplers: "+err.Error())
					return
				}
				if !slices.Contains(samplers, val) {
					fmt.Println("  invalid sampler")
					sendReplyToMessage(ctx, msg, errorStr+": invalid sampler")
					return
				}
				renderParams.SamplerName = val
			case "model", "m":
				models, err := sdAPI.GetModels(ctx)
				if err != nil {
					fmt.Println("  error getting models:", err)
					sendReplyToMessage(ctx, msg, errorStr+": error getting models: "+err.Error())
					return
				}
				if !slices.Contains(models, val) {
					fmt.Println("  invalid model")
					sendReplyToMessage(ctx, msg, errorStr+": invalid model")
					return
				}
				renderParams.ModelName = val
			case "hr":
				if val != "0" {
					renderParams.HR.Enable = true
				}
			case "hr-denoisestrength", "hrd":
				valFloat, err := strconv.ParseFloat(val, 32)
				if err != nil {
					fmt.Println("  invalid hr denoise strength")
					sendReplyToMessage(ctx, msg, errorStr+": invalid hr denoise strength")
					return
				}
				renderParams.HR.DenoisingStrength = float32(valFloat)
			case "hr-scale", "hrs":
				valFloat, err := strconv.ParseFloat(val, 32)
				if err != nil {
					fmt.Println("  invalid hr scale")
					sendReplyToMessage(ctx, msg, errorStr+": invalid hr scale")
					return
				}
				renderParams.HR.Scale = float32(valFloat)
			case "hr-upscaler", "hru":
				val = strings.ReplaceAll(val, ".", " ")
				upscalers, err := sdAPI.GetUpscalers(ctx)
				if err != nil {
					fmt.Println("  error getting upscalers:", err)
					sendReplyToMessage(ctx, msg, errorStr+": error getting upscalers: "+err.Error())
					return
				}
				if !slices.Contains(upscalers, val) {
					fmt.Println("  invalid upscaler")
					sendReplyToMessage(ctx, msg, errorStr+": invalid upscaler")
					return
				}
				renderParams.HR.Upscaler = val
			default:
				fmt.Println("  invalid attribute", attr)
				sendReplyToMessage(ctx, msg, errorStr+": invalid attribute "+attr)
				return
			}
		}
	}

	renderParams.Prompt = strings.Join(prompt, " ")

	if renderParams.Prompt == "" {
		fmt.Println("  missing prompt")
		sendReplyToMessage(ctx, msg, errorStr+": missing prompt")
		return
	}

	dlQueue.Add(renderParams, msg)
}

func handleCmdEDCancel(ctx context.Context, msg *models.Message) {
	if err := dlQueue.CancelCurrentEntry(ctx); err != nil {
		sendReplyToMessage(ctx, msg, errorStr+": "+err.Error())
	}
}

func handleCmdModels(ctx context.Context, msg *models.Message) {
	models, err := sdAPI.GetModels(ctx)
	if err != nil {
		fmt.Println("  error getting models:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting models: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "🧩 Available models: "+strings.Join(models, ", ")+". Default: "+params.DefaultModel)
}

func handleCmdSamplers(ctx context.Context, msg *models.Message) {
	samplers, err := sdAPI.GetSamplers(ctx)
	if err != nil {
		fmt.Println("  error getting samplers:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting samplers: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "🔭 Available samplers: "+strings.Join(samplers, ", ")+". Default: "+params.DefaultSampler)
}

func handleCmdEmbeddings(ctx context.Context, msg *models.Message) {
	embs, err := sdAPI.GetEmbeddings(ctx)
	if err != nil {
		fmt.Println("  error getting embeddings:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting embeddings: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "Available embeddings: "+strings.Join(embs, ", "))
}

func handleCmdLORAs(ctx context.Context, msg *models.Message) {
	loras, err := sdAPI.GetLORAs(ctx)
	if err != nil {
		fmt.Println("  error getting loras:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting loras: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "Available loras: "+strings.Join(loras, ", "))
}

func handleCmdUpscalers(ctx context.Context, msg *models.Message) {
	ups, err := sdAPI.GetUpscalers(ctx)
	if err != nil {
		fmt.Println("  error getting upscalers:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting upscalers: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "🔎 Available upscalers: "+strings.Join(ups, ", "))
}

func handleCmdHelp(ctx context.Context, msg *models.Message) {
	sendReplyToMessage(ctx, msg, "🤖 Stable Diffusion Telegram Bot\n\n"+
		"Available commands:\n\n"+
		"!sd [prompt] - render prompt\n"+
		"!sdcancel - cancel current render\n"+
		"!sdmodels - list available models\n"+
		"!sdsamplers - list available samplers\n"+
		"!sdembeddings - list available embeddings\n"+
		"!sdloras - list available loras\n"+
		"!sdupscalers - list available upscalers\n"+
		"!sdhelp - show this help\n\n"+
		"For more information see https://github.com/nonoo/stable-diffusion-telegram-bot")
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
			handleCmdED(ctx, update.Message)
			return
		case "sdcancel":
			handleCmdEDCancel(ctx, update.Message)
			return
		case "sdmodels":
			handleCmdModels(ctx, update.Message)
			return
		case "sdsamplers":
			handleCmdSamplers(ctx, update.Message)
			return
		case "sdembeddings":
			handleCmdEmbeddings(ctx, update.Message)
			return
		case "sdloras":
			handleCmdLORAs(ctx, update.Message)
			return
		case "sdupscalers":
			handleCmdUpscalers(ctx, update.Message)
			return
		case "sdhelp":
			handleCmdHelp(ctx, update.Message)
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
		handleCmdED(ctx, update.Message)
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
