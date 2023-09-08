package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
)

type paramsType struct {
	BotToken                 string
	StableDiffusionPath      string
	StableDiffusionWebUIPath string

	AllowedUserIDs  []int64
	AdminUserIDs    []int64
	AllowedGroupIDs []int64

	SDStart           bool
	DelayedSDStart    bool
	DefaultModel      string
	DefaultSampler    string
	DefaultWidth      int
	DefaultHeight     int
	DefaultWidthSDXL  int
	DefaultHeightSDXL int
}

var params paramsType

func (p *paramsType) Init() error {
	flag.StringVar(&p.BotToken, "bot-token", "", "telegram bot token")
	flag.StringVar(&p.StableDiffusionPath, "sd-path", "", "path of the stable diffusion directory")
	flag.StringVar(&p.StableDiffusionWebUIPath, "sd-webui-path", "", "path of the stable diffusion webui start script")
	var allowedUserIDs string
	flag.StringVar(&allowedUserIDs, "allowed-user-ids", "", "allowed telegram user ids")
	var adminUserIDs string
	flag.StringVar(&adminUserIDs, "admin-user-ids", "", "admin telegram user ids")
	var allowedGroupIDs string
	flag.StringVar(&allowedGroupIDs, "allowed-group-ids", "", "allowed telegram group ids")
	flag.BoolVar(&p.SDStart, "sd-start", true, "start stable diffusion if needed")
	flag.BoolVar(&p.DelayedSDStart, "delayed-sd-start", false, "start stable diffusion only when the first prompt arrives")
	flag.StringVar(&p.DefaultModel, "default-model", "", "default model name")
	flag.StringVar(&p.DefaultSampler, "default-sampler", "", "default sampler name")
	flag.IntVar(&p.DefaultWidth, "default-width", 512, "default image width")
	flag.IntVar(&p.DefaultHeight, "default-height", 512, "default image height")
	flag.IntVar(&p.DefaultWidthSDXL, "default-width-sdxl", 1024, "default image width for SDXL models")
	flag.IntVar(&p.DefaultHeightSDXL, "default-height-sdxl", 1024, "default image height for SDXL models")
	flag.Parse()

	if p.BotToken == "" {
		p.BotToken = os.Getenv("BOT_TOKEN")
	}
	if p.BotToken == "" {
		return fmt.Errorf("bot token not set")
	}

	if p.StableDiffusionPath == "" {
		p.StableDiffusionPath = os.Getenv("STABLE_DIFFUSION_PATH")
	}
	if p.StableDiffusionPath == "" {
		return fmt.Errorf("stable diffusion path not set")
	}
	if p.StableDiffusionWebUIPath == "" {
		p.StableDiffusionWebUIPath = os.Getenv("STABLE_DIFFUSION_WEBUI_PATH")
	}
	if p.StableDiffusionWebUIPath == "" {
		return fmt.Errorf("stable diffusion webui path not set")
	}

	if allowedUserIDs == "" {
		allowedUserIDs = os.Getenv("ALLOWED_USERIDS")
	}
	sa := strings.Split(allowedUserIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("allowed user ids contains invalid user ID: " + idStr)
		}
		p.AllowedUserIDs = append(p.AllowedUserIDs, id)
	}

	if adminUserIDs == "" {
		adminUserIDs = os.Getenv("ADMIN_USERIDS")
	}
	sa = strings.Split(adminUserIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("admin ids contains invalid user ID: " + idStr)
		}
		p.AdminUserIDs = append(p.AdminUserIDs, id)
		if !slices.Contains(p.AllowedUserIDs, id) {
			p.AllowedUserIDs = append(p.AllowedUserIDs, id)
		}
	}

	if allowedGroupIDs == "" {
		allowedGroupIDs = os.Getenv("ALLOWED_GROUPIDS")
	}
	sa = strings.Split(allowedGroupIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("allowed group ids contains invalid group ID: " + idStr)
		}
		p.AllowedGroupIDs = append(p.AllowedGroupIDs, id)
	}

	s := os.Getenv("SD_START")
	if s != "" {
		if s == "0" {
			p.SDStart = false
		} else {
			p.SDStart = true
		}
	}

	s = os.Getenv("DELAYED_SD_START")
	if s != "" {
		if s == "0" {
			p.DelayedSDStart = false
		} else {
			p.DelayedSDStart = true
		}
	}

	if p.DefaultModel == "" {
		p.DefaultModel = os.Getenv("DEFAULT_MODEL")
	}

	if p.DefaultSampler == "" {
		p.DefaultSampler = os.Getenv("DEFAULT_SAMPLER")
	}

	s = os.Getenv("DEFAULT_WIDTH")
	if s != "" {
		val, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid default width")
		}
		p.DefaultWidth = val
	}
	s = os.Getenv("DEFAULT_HEIGHT")
	if s != "" {
		val, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid default height")
		}
		p.DefaultHeight = val
	}
	s = os.Getenv("DEFAULT_WIDTH_SDXL")
	if s != "" {
		val, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid default width for SDXL")
		}
		p.DefaultWidthSDXL = val
	}
	s = os.Getenv("DEFAULT_HEIGHT_SDXL")
	if s != "" {
		val, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid default height for SDXL")
		}
		p.DefaultHeightSDXL = val
	}

	return nil
}
