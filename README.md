# stable-diffusion-telegram-bot

This is a Telegram Bot frontend for rendering images with
[Stable Diffusion](https://github.com/AUTOMATIC1111/stable-diffusion-webui/).

<p align="center"><img src="demo.gif?raw=true"/></p>

The bot displays the progress and further information during processing by
responding to the message with the prompt. Requests are queued, only one gets
processed at a time.

The bot uses the
[Telegram Bot API](https://github.com/go-telegram-bot-api/telegram-bot-api).
Rendered images are not saved on disk. Tested on Linux, but should be able
to run on other operating systems.

## Compiling

You'll need Go installed on your computer. Install a recent package of `golang`.
Then:

```
go get github.com/nonoo/stable-diffusion-telegram-bot
go install github.com/nonoo/stable-diffusion-telegram-bot
```

This will typically install `stable-diffusion-telegram-bot` into `$HOME/go/bin`.

Or just enter `go build` in the cloned Git source repo directory.

## Prerequisites

Create a Telegram bot using [BotFather](https://t.me/BotFather) and get the
bot's `token`.

## Running

You can get the available command line arguments with `-h`.
Mandatory arguments are:

- `-bot-token`: set this to your Telegram bot's `token`
- `-sd-webui-path`: set this to the path of webui start script from the Stable
  Diffusion directory

Set your Telegram user ID as an admin with the `-admin-user-ids` argument.
Admins will get a message when the bot starts.

Other user/group IDs can be set with the `-allowed-user-ids` and
`-allowed-group-ids` arguments. IDs should be separated by commas.

You can get Telegram user IDs by writing a message to the bot and checking
the app's log, as it logs all incoming messages.

All command line arguments can be set through OS environment variables.
Note that using a command line argument overwrites a setting by the environment
variable. Available OS environment variables are:

- `BOT_TOKEN`
- `STABLE_DIFFUSION_WEBUI_PATH`
- `ALLOWED_USERIDS`
- `ADMIN_USERIDS`
- `ALLOWED_GROUPIDS`
- `DELAYED_SD_START`
- `DEFAULT_MODEL`
- `DEFAULT_SAMPLER`

## Supported commands

- `/sd` - Render images using supplied prompt
- `/sdcancel` - Cancel ongoing download
- `/sdmodels` - List available models
- `/sdsamplers` - list available samplers
- `/sdembeddings` - list available embeddings
- `/sdloras` - list available LoRAs
- `/sdupscalers` - list available upscalers
- `/sdhelp` - Cancel ongoing download

You can also use the `!` command character instead of `/`.

You don't need to enter the `/sd` command if you send a prompt to the bot using
a private chat.

### Setting render parameters

You can use the following `-attr val` assignments at the end of the prompt:

- `seed/s` - set seed (hexadecimal)
- `width/w` - set output image width
- `height/h` - set output image height
- `steps/t` - set the number of steps
- `outcnt/o` - set count of output images
- `scale/c` - set CFG scale
- `sampler/r` - set sampler, get valid values with `/sdsamplers`
- `model/m` - set model, get valid values with `/sdmodels`
- `hr` - enable highres mode and set upscale ratio
- `hr-denoisestrength/hrd` - set highres mode denoise strength
- `hr-upscaler/hru` - set highres mode upscaler, get valid values with `/sdupscalers`
- `hr-steps/hrt` - set the number of highres mode second pass steps

Example prompt with attributes: `laughing santa with beer -s 1 -o 1`

Enter negative prompts in the second line of your message (use Shift+Enter). Example:
```
laughing santa with beer
tree -s 1 -o 1
```

If you need to use spaces in sampler and upscaler names, then enclose them
in double quotes.

## Donations

If you find this bot useful then [buy me a beer](https://paypal.me/ha2non). :)
