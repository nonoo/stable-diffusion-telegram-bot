package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/shlex"
	"golang.org/x/exp/slices"
)

type RenderParamsHR struct {
	DenoisingStrength float32
	Scale             float32
	Upscaler          string
	SecondPassSteps   int
}

type RenderParams struct {
	Prompt         string
	OrigPrompt     string
	NegativePrompt string
	Seed           uint32
	Width          int
	Height         int
	Steps          int
	NumOutputs     int
	CFGScale       float32
	SamplerName    string
	ModelName      string

	HR RenderParamsHR
}

// Returns -1 as firstCmdCharAt if no params have been found in the given string.
func (r *RenderParams) Parse(ctx context.Context, s string) (firstCmdCharAt int, err error) {
	lexer := shlex.NewLexer(strings.NewReader(s))

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
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			val = strings.TrimPrefix(val, "🌱")
			valInt, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid seed")
			}
			r.Seed = uint32(valInt)
			validAttr = true
		case "width", "w":
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid width")
			}
			r.Width = valInt
			validAttr = true
		case "height", "h":
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid height")
			}
			r.Height = valInt
			validAttr = true
		case "steps", "t":
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid steps")
			}
			r.Steps = valInt
			validAttr = true
		case "outcnt", "o":
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid output count")
			}
			r.NumOutputs = valInt
			validAttr = true
		case "scale", "c":
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("  invalid CFG scale")
			}
			r.CFGScale = float32(valFloat)
			validAttr = true
		case "sampler", "r":
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
			r.SamplerName = val
			validAttr = true
		case "model", "m":
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
			r.ModelName = val
			validAttr = true
		case "hr":
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid hr scale")
			}
			r.HR.Scale = float32(valFloat)
			validAttr = true
		case "hr-denoisestrength", "hrd":
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valFloat, err := strconv.ParseFloat(val, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid hr denoise strength")
			}
			r.HR.DenoisingStrength = float32(valFloat)
			validAttr = true
		case "hr-upscaler", "hru":
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
			r.HR.Upscaler = val
			validAttr = true
		case "hr-steps", "hrt":
			val, lexErr := lexer.Next()
			if lexErr != nil {
				return 0, fmt.Errorf(attr + " is missing value")
			}
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("invalid hr second pass steps")
			}
			r.HR.SecondPassSteps = valInt
			validAttr = true
		}

		if validAttr && firstCmdCharAt == -1 {
			firstCmdCharAt = strings.Index(s, token)
		}
	}
	return
}
