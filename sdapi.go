package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const sdAPIURL = "http://localhost:7860/sdapi/v1/"

type sdAPIType struct{}

func (a *sdAPIType) req(ctx context.Context, path, service string, postData []byte) (string, error) {
	path, err := url.JoinPath(sdAPIURL, path)
	if err != nil {
		return "", err
	}

	path += service

	var request *http.Request
	if postData != nil {
		request, err = http.NewRequestWithContext(ctx, "POST", path, bytes.NewBuffer(postData))
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	} else {
		request, err = http.NewRequestWithContext(ctx, "GET", path, nil)
		if err != nil {
			return "", err
		}
	}

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("api status code: %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

type RenderReq struct {
	EnableHR          bool                   `json:"enable_hr"`
	DenoisingStrength float32                `json:"denoising_strength"`
	HRScale           float32                `json:"hr_scale"`
	HRUpscaler        string                 `json:"hr_upscaler"`
	HRSecondPassSteps int                    `json:"hr_second_pass_steps"`
	HRSamplerName     string                 `json:"hr_sampler_name"`
	HRPrompt          string                 `json:"hr_prompt"`
	HRNegativePrompt  string                 `json:"hr_negative_prompt"`
	Prompt            string                 `json:"prompt"`
	Seed              uint32                 `json:"seed"`
	SamplerName       string                 `json:"sampler_name"`
	BatchSize         int                    `json:"batch_size"`
	NIter             int                    `json:"n_iter"`
	Steps             int                    `json:"steps"`
	CFGScale          float32                `json:"cfg_scale"`
	Width             int                    `json:"width"`
	Height            int                    `json:"height"`
	NegativePrompt    string                 `json:"negative_prompt"`
	OverrideSettings  map[string]interface{} `json:"override_settings"`
	SendImages        bool                   `json:"send_images"`
}

func (a *sdAPIType) Render(ctx context.Context, params RenderParams) (imgs [][]byte, err error) {
	postData, err := json.Marshal(RenderReq{
		EnableHR:          params.HR.Scale > 0,
		DenoisingStrength: params.HR.DenoisingStrength,
		HRScale:           params.HR.Scale,
		HRUpscaler:        params.HR.Upscaler,
		HRSecondPassSteps: params.HR.SecondPassSteps,
		HRSamplerName:     params.SamplerName,
		HRPrompt:          params.Prompt,
		HRNegativePrompt:  params.NegativePrompt,
		Prompt:            params.Prompt,
		Seed:              params.Seed,
		SamplerName:       params.SamplerName,
		BatchSize:         params.NumOutputs,
		NIter:             1,
		Steps:             params.Steps,
		CFGScale:          params.CFGScale,
		Width:             params.Width,
		Height:            params.Height,
		NegativePrompt:    params.NegativePrompt,
		OverrideSettings: map[string]interface{}{
			"sd_model_checkpoint": params.ModelName,
		},
		SendImages: true,
	})
	if err != nil {
		return nil, err
	}

	res, err := a.req(ctx, "/txt2img", "", postData)
	if err != nil {
		return nil, err
	}

	var renderResp struct {
		Images []string `json:"images"`
	}
	err = json.Unmarshal([]byte(res), &renderResp)
	if err != nil {
		return nil, err
	}
	if len(renderResp.Images) == 0 {
		return nil, fmt.Errorf("unknown error")
	}

	for _, img := range renderResp.Images {
		var unbased []byte
		if unbased, err = base64.StdEncoding.DecodeString(img); err != nil {
			return nil, fmt.Errorf("image base64 decode error")
		}
		imgs = append(imgs, unbased)
	}

	return imgs, nil
}

func (a *sdAPIType) Interrupt(ctx context.Context) error {
	_, err := a.req(ctx, "/interrupt", "", []byte{})
	if err != nil {
		return err
	}
	return nil
}

func (a *sdAPIType) GetProgress(ctx context.Context) (progressPercent int, eta time.Duration, err error) {
	res, err := a.req(ctx, "/progress", "?skip_current_image=false", nil)
	if err != nil {
		return 0, 0, err
	}

	var progressRes struct {
		Progress float32 `json:"progress"`
		ETA      float32 `json:"eta_relative"`
		Detail   string  `json:"detail"`
	}
	err = json.Unmarshal([]byte(res), &progressRes)
	if err != nil {
		return 0, 0, err
	}

	if progressRes.Detail != "" {
		return 0, 0, fmt.Errorf(progressRes.Detail)
	}

	return int(progressRes.Progress * 100), time.Duration(progressRes.ETA * float32(time.Second)), nil
}

func (a *sdAPIType) GetModels(ctx context.Context) (models []string, err error) {
	res, err := a.req(ctx, "/sd-models", "", nil)
	if err != nil {
		return nil, err
	}

	var modelsRes []struct {
		Name string `json:"model_name"`
	}
	err = json.Unmarshal([]byte(res), &modelsRes)
	if err != nil {
		return nil, err
	}

	for _, m := range modelsRes {
		models = append(models, m.Name)
	}
	return
}

func (a *sdAPIType) GetSamplers(ctx context.Context) (samplers []string, err error) {
	res, err := a.req(ctx, "/samplers", "", nil)
	if err != nil {
		return nil, err
	}

	var samplersRes []struct {
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(res), &samplersRes)
	if err != nil {
		return nil, err
	}

	for _, sampler := range samplersRes {
		samplers = append(samplers, sampler.Name)
	}
	return
}

func (a *sdAPIType) GetEmbeddings(ctx context.Context) (embs []string, err error) {
	res, err := a.req(ctx, "/embeddings", "", nil)
	if err != nil {
		return nil, err
	}

	var embList struct {
		Loaded map[string]struct{} `json:"loaded"`
	}
	err = json.Unmarshal([]byte(res), &embList)
	if err != nil {
		return nil, err
	}

	for i := range embList.Loaded {
		embs = append(embs, i)
	}
	return
}

func (a *sdAPIType) GetLoRAs(ctx context.Context) (loras []string, err error) {
	res, err := a.req(ctx, "/loras", "", nil)
	if err != nil {
		return nil, err
	}

	var lorasRes []struct {
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(res), &lorasRes)
	if err != nil {
		return nil, err
	}

	for _, lora := range lorasRes {
		loras = append(loras, lora.Name)
	}
	return
}

func (a *sdAPIType) GetUpscalers(ctx context.Context) (upscalers []string, err error) {
	res, err := a.req(ctx, "/upscalers", "", nil)
	if err != nil {
		return nil, err
	}

	var upscalersRes []struct {
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(res), &upscalersRes)
	if err != nil {
		return nil, err
	}

	for _, u := range upscalersRes {
		upscalers = append(upscalers, u.Name)
	}
	return
}

func (a *sdAPIType) GetVAEs(ctx context.Context) (vaes []string, err error) {
	res, err := a.req(ctx, "/sd-vae", "", nil)
	if err != nil {
		return nil, err
	}

	var vaesRes []struct {
		Name string `json:"model_name"`
	}
	err = json.Unmarshal([]byte(res), &vaesRes)
	if err != nil {
		return nil, err
	}

	for _, u := range vaesRes {
		vaes = append(vaes, u.Name)
	}
	return
}
