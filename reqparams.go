package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/shlex"
	"golang.org/x/exp/slices"
)

type ReqParamsUpscale struct {
	origPrompt string
	Scale      float32
	Upscaler   string
	OutputPNG  bool
}

func (r ReqParamsUpscale) String() string {
	res := "🔎 " + r.Upscaler + "x" + fmt.Sprint(r.Scale)
	if r.OutputPNG {
		res += "/PNG"
	}
	return res
}

func (r ReqParamsUpscale) OrigPrompt() string {
	return r.origPrompt
}

type ReqParamsRenderHR struct {
	DenoisingStrength float32
	Scale             float32
	Upscaler          string
	SecondPassSteps   int
}

type ReqParamsRender struct {
	origPrompt     string
	Prompt         string
	NegativePrompt string
	Seed           uint32
	Width          int
	Height         int
	Steps          int
	NumOutputs     int
	OutputPNG      bool
	CFGScale       float32
	SamplerName    string
	ModelName      string

	Upscale ReqParamsUpscale

	HR ReqParamsRenderHR
}

func (r ReqParamsRender) String() string {
	var numOutputs string
	if r.NumOutputs > 1 {
		numOutputs = fmt.Sprintf("x%d", r.NumOutputs)
	}

	var outFormatText string
	if r.OutputPNG {
		outFormatText = "/PNG"
	}

	res := fmt.Sprintf("🌱%d 👟%d 🕹%.1f 🖼%dx%d%s%s 🔭%s 🧩%s", r.Seed, r.Steps, r.CFGScale, r.Width, r.Height,
		numOutputs, outFormatText, r.SamplerName, r.ModelName)

	if r.HR.Scale > 0 {
		res += " 🔎 " + r.HR.Upscaler + "x" + fmt.Sprint(r.HR.Scale, "/", r.HR.DenoisingStrength)
	} else if r.Upscale.Scale > 0 {
		res += " " + r.Upscale.String()
	}

	if r.NegativePrompt != "" {
		negText := r.NegativePrompt
		if len(negText) > 10 {
			negText = negText[:10] + "..."
		}
		res = "📍" + negText + " " + res
	}
	return res
}

func (r ReqParamsRender) OrigPrompt() string {
	return r.origPrompt
}

type ReqParams interface {
	String() string
	OrigPrompt() string
}

// Returns -1 as firstCmdCharAt if no params have been found in the given string.
func ReqParamsParse(ctx context.Context, s string, reqParams ReqParams) (firstCmdCharAt int, err error) {
	lexer := shlex.NewLexer(strings.NewReader(s))

	var reqParamsRender *ReqParamsRender
	var reqParamsUpscale *ReqParamsUpscale
	switch v := reqParams.(type) {
	case *ReqParamsRender:
		reqParamsRender = v
	case *ReqParamsUpscale:
		reqParamsUpscale = v
	default:
		return 0, fmt.Errorf("invalid reqParams type")
	}

	gotWidth := false
	gotHeight := false

	firstCmdCharAt = -1
	for {
		token, lexErr := lexer.Next()
		if lexErr != nil { // No more tokens?
			break
		}

		if token[0] != '-' {
			if firstCmdCharAt > -1 {
				return 0, fmt.Errorf("params need to be after the prompt")
			}
			continue // Ignore tokens not starting with -
		}

		attr := strings.ToLower(token[1:])
		validAttr := false

		switch attr {
		case "seed", "s":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			val = strings.TrimPrefix(val, "🌱")
			valInt, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid seed")
			}
			reqParamsRender.Seed = uint32(valInt)
			validAttr = true
		case "width", "w":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid width")
			}
			reqParamsRender.Width = valInt
			validAttr = true
			gotWidth = true
		case "height", "h":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid height")
			}
			reqParamsRender.Height = valInt
			validAttr = true
			gotHeight = true
		case "steps", "t":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid steps")
			}
			reqParamsRender.Steps = valInt
			validAttr = true
		case "outcnt", "o":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid output count")
			}
			reqParamsRender.NumOutputs = valInt
			validAttr = true
		case "png", "p":
			if reqParamsRender != nil {
				reqParamsRender.OutputPNG = true
			} else if reqParamsUpscale != nil {
				reqParamsUpscale.OutputPNG = true
			}
		case "cfg", "c":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("  invalid CFG scale")
			}
			reqParamsRender.CFGScale = float32(valFloat)
			validAttr = true
		case "sampler", "r":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			samplers, err := sdAPI.GetSamplers(ctx)
			if err != nil {
				return 0, fmt.Errorf("error getting samplers: %w", err)
			}
			if !slices.Contains(samplers, val) {
				return 0, fmt.Errorf("invalid sampler")
			}
			reqParamsRender.SamplerName = val
			validAttr = true
		case "model", "m":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			models, err := sdAPI.GetModels(ctx)
			if err != nil {
				return 0, fmt.Errorf("error getting models: %w", err)
			}
			if !slices.Contains(models, val) {
				return 0, fmt.Errorf(" invalid model")
			}
			reqParamsRender.ModelName = val
			validAttr = true
		case "upscale", "u":
			if reqParamsRender == nil && reqParamsUpscale == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid hr scale")
			}
			if reqParamsRender != nil {
				reqParamsRender.Upscale.Scale = float32(valFloat)
			} else if reqParamsUpscale != nil {
				reqParamsUpscale.Scale = float32(valFloat)
			}
			validAttr = true
		case "upscaler":
			if reqParamsRender == nil && reqParamsUpscale == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			upscalers, err := sdAPI.GetUpscalers(ctx)
			if err != nil {
				return 0, fmt.Errorf("error getting upscalers: %w", err)
			}
			if !slices.Contains(upscalers, val) {
				return 0, fmt.Errorf("invalid upscaler")
			}
			if reqParamsRender != nil {
				reqParamsRender.Upscale.Upscaler = val
			} else if reqParamsUpscale != nil {
				reqParamsUpscale.Upscaler = val
			}
			validAttr = true
		case "hr":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid hr scale")
			}
			reqParamsRender.HR.Scale = float32(valFloat)
			validAttr = true
		case "hr-denoisestrength", "hrd":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid hr denoise strength")
			}
			reqParamsRender.HR.DenoisingStrength = float32(valFloat)
			validAttr = true
		case "hr-upscaler", "hru":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			upscalers, err := sdAPI.GetUpscalers(ctx)
			if err != nil {
				return 0, fmt.Errorf("error getting upscalers: %w", err)
			}
			if !slices.Contains(upscalers, val) {
				return 0, fmt.Errorf("invalid upscaler")
			}
			reqParamsRender.HR.Upscaler = val
			validAttr = true
		case "hr-steps", "hrt":
			if reqParamsRender == nil {
				break
			}
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid hr second pass steps")
			}
			reqParamsRender.HR.SecondPassSteps = valInt
			validAttr = true
		}

		if validAttr && firstCmdCharAt == -1 {
			firstCmdCharAt = strings.Index(s, token)
		}
	}

	if reqParamsRender != nil {
		if strings.HasSuffix(strings.ToLower(reqParamsRender.ModelName), "sdxl") {
			if !gotWidth {
				reqParamsRender.Width = params.DefaultWidthSDXL
			}
			if !gotHeight {
				reqParamsRender.Height = params.DefaultHeightSDXL
			}
		} else {
			if !gotWidth {
				reqParamsRender.Width = params.DefaultWidth
			}
			if !gotHeight {
				reqParamsRender.Height = params.DefaultHeight
			}
		}

		// Don't allow upscaler while HR is enabled.
		if reqParamsRender.HR.Scale > 0 {
			reqParamsRender.Upscale.Scale = 0
		}
	}

	return
}
