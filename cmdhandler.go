package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/go-telegram/bot/models"
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

	var paramsLine *string
	lines := strings.Split(msg.Text, "\n")
	if len(lines) >= 2 {
		renderParams.Prompt = lines[0]
		renderParams.NegativePrompt = strings.Join(lines[1:], " ")
		paramsLine = &renderParams.NegativePrompt
	} else {
		renderParams.Prompt = msg.Text
		paramsLine = &renderParams.Prompt
	}
	firstCmdCharAt, err := renderParams.Parse(ctx, *paramsLine)
	if err != nil {
		sendReplyToMessage(ctx, msg, errorStr+": can't parse render params: "+err.Error())
		return
	}
	if firstCmdCharAt >= 0 { // Commands found? Removing them from the line.
		*paramsLine = (*paramsLine)[:firstCmdCharAt]
	}

	renderParams.Prompt = strings.Trim(renderParams.Prompt, " ")
	renderParams.NegativePrompt = strings.Trim(renderParams.NegativePrompt, " ")

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
		"Available render parameters at the end of the prompt:\n\n"+
		"-seed/s - set seed\n"+
		"-width/w - set output image width\n"+
		"-height/h - set output image height\n"+
		"-steps/t - set the number of steps\n"+
		"-outcnt/o - set count of output images\n"+
		"-cfg/c - set CFG scale\n"+
		"-sampler/r - set sampler, get valid values with /sdsamplers\n"+
		"-model/m - set model, get valid values with /sdmodels\n"+
		"-hr - enable highres mode and set upscale ratio\n"+
		"-hr-denoisestrength/hrd - set highres mode denoise strength\n"+
		"-hr-upscaler/hru - set highres mode upscaler, get valid values with /sdupscalers\n"+
		"-hr-steps/hrt - set the number of highres mode second pass steps\n\n"+
		"For more information see https://github.com/nonoo/stable-diffusion-telegram-bot")
}
