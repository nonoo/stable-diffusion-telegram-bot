#!/bin/bash

. config.inc.sh

bin=./stable-diffusion-telegram-bot
if [ ! -x "$bin" ]; then
	bin="go run *.go"
fi

BOT_TOKEN=$BOT_TOKEN \
STABLE_DIFFUSION_PATH=$STABLE_DIFFUSION_PATH \
STABLE_DIFFUSION_WEBUI_PATH=$STABLE_DIFFUSION_WEBUI_PATH \
ALLOWED_USERIDS=$ALLOWED_USERIDS \
ADMIN_USERIDS=$ADMIN_USERIDS \
ALLOWED_GROUPIDS=$ALLOWED_GROUPIDS \
SD_START=$SD_START \
DELAYED_SD_START=$DELAYED_SD_START \
DEFAULT_MODEL=$DEFAULT_MODEL \
DEFAULT_SAMPLER=$DEFAULT_SAMPLER \
DEFAULT_WIDTH=$DEFAULT_WIDTH \
DEFAULT_HEIGHT=$DEFAULT_HEIGHT \
DEFAULT_WIDTH_SDXL=$DEFAULT_WIDTH_SDXL \
DEFAULT_HEIGHT_SDXL=$DEFAULT_HEIGHT_SDXL \
$bin $*
