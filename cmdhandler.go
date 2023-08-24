package main

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"
	"golang.org/x/exp/slices"
)

type cmdHandlerType struct{}

func (c *cmdHandlerType) ED(ctx context.Context, msg *models.Message) {
	renderParams := RenderParams{
		OrigPrompt:  msg.Text,
		Seed:        rand.Uint32(),
		Width:       512,
		Height:      512,
		Steps:       35,
		NumOutputs:  4,
		CFGScale:    7,
		SamplerName: params.DefaultSampler,
		ModelName:   params.DefaultModel,
		HR: RenderParamsHR{
			DenoisingStrength: 0.4,
			Upscaler:          "R-ESRGAN 4x+",
			SecondPassSteps:   15,
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
					fmt.Println("  invalid CFG scale")
					sendReplyToMessage(ctx, msg, errorStr+": invalid CFG scale")
					return
				}
				renderParams.CFGScale = float32(valFloat)
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
				valFloat, err := strconv.ParseFloat(val, 32)
				if err != nil {
					fmt.Println("  invalid hr scale")
					sendReplyToMessage(ctx, msg, errorStr+": invalid hr scale")
					return
				}
				renderParams.HR.Scale = float32(valFloat)
			case "hr-denoisestrength", "hrd":
				valFloat, err := strconv.ParseFloat(val, 32)
				if err != nil {
					fmt.Println("  invalid hr denoise strength")
					sendReplyToMessage(ctx, msg, errorStr+": invalid hr denoise strength")
					return
				}
				renderParams.HR.DenoisingStrength = float32(valFloat)
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
			case "hr-steps", "hrt":
				valInt, err := strconv.Atoi(val)
				if err != nil {
					fmt.Println("  invalid hr second pass steps")
					sendReplyToMessage(ctx, msg, errorStr+": invalid hr second pass steps")
					return
				}
				renderParams.HR.SecondPassSteps = valInt
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

	if renderParams.HR.Scale > 0 {
		renderParams.NumOutputs = 1
	}

	dlQueue.Add(renderParams, msg)
}

func (c *cmdHandlerType) EDCancel(ctx context.Context, msg *models.Message) {
	if err := dlQueue.CancelCurrentEntry(ctx); err != nil {
		sendReplyToMessage(ctx, msg, errorStr+": "+err.Error())
	}
}

func (c *cmdHandlerType) Models(ctx context.Context, msg *models.Message) {
	models, err := sdAPI.GetModels(ctx)
	if err != nil {
		fmt.Println("  error getting models:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting models: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "🧩 Available models: "+strings.Join(models, ", ")+". Default: "+params.DefaultModel)
}

func (c *cmdHandlerType) Samplers(ctx context.Context, msg *models.Message) {
	samplers, err := sdAPI.GetSamplers(ctx)
	if err != nil {
		fmt.Println("  error getting samplers:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting samplers: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "🔭 Available samplers: "+strings.Join(samplers, ", ")+". Default: "+params.DefaultSampler)
}

func (c *cmdHandlerType) Embeddings(ctx context.Context, msg *models.Message) {
	embs, err := sdAPI.GetEmbeddings(ctx)
	if err != nil {
		fmt.Println("  error getting embeddings:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting embeddings: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "Available embeddings: "+strings.Join(embs, ", "))
}

func (c *cmdHandlerType) LoRAs(ctx context.Context, msg *models.Message) {
	loras, err := sdAPI.GetLoRAs(ctx)
	if err != nil {
		fmt.Println("  error getting loras:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting loras: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "Available loras: "+strings.Join(loras, ", "))
}

func (c *cmdHandlerType) Upscalers(ctx context.Context, msg *models.Message) {
	ups, err := sdAPI.GetUpscalers(ctx)
	if err != nil {
		fmt.Println("  error getting upscalers:", err)
		sendReplyToMessage(ctx, msg, errorStr+": error getting upscalers: "+err.Error())
		return
	}
	sendReplyToMessage(ctx, msg, "🔎 Available upscalers: "+strings.Join(ups, ", "))
}

func (c *cmdHandlerType) Help(ctx context.Context, msg *models.Message) {
	sendReplyToMessage(ctx, msg, "🤖 Stable Diffusion Telegram Bot\n\n"+
		"Available commands:\n\n"+
		"!sd [prompt] - render prompt\n"+
		"!sdcancel - cancel current render\n"+
		"!sdmodels - list available models\n"+
		"!sdsamplers - list available samplers\n"+
		"!sdembeddings - list available embeddings\n"+
		"!sdloras - list available LoRAs\n"+
		"!sdupscalers - list available upscalers\n"+
		"!sdhelp - show this help\n\n"+
		"For more information see https://github.com/nonoo/stable-diffusion-telegram-bot")
}
