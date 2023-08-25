package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"math/rand"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const processStartStr = "🛎 Starting render..."
const processStr = "🔨 Processing"
const progressBarLength = 20
const uploadingStr = "☁ ️ Uploading..."
const errorStr = "❌ Error"
const canceledStr = "❌ Canceled"
const restartStr = "⚠️ Stable Diffusion is not running, starting, please wait..."
const restartFailedStr = "☠️ Stable Diffusion start failed, please restart the bot"

const processTimeout = 3 * time.Minute
const groupChatProgressUpdateInterval = 3 * time.Second
const privateChatProgressUpdateInterval = 500 * time.Millisecond

type DownloadQueueEntry struct {
	Params RenderParams

	TaskID           uint64
	RenderParamsText string

	ReplyMessage *models.Message
	Message      *models.Message
}

func (e *DownloadQueueEntry) checkWaitError(err error) time.Duration {
	var retryRegex = regexp.MustCompile(`{"retry_after":([0-9]+)}`)
	match := retryRegex.FindStringSubmatch(err.Error())
	if len(match) < 2 {
		return 0
	}

	retryAfter, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return time.Duration(retryAfter) * time.Second
}

func (e *DownloadQueueEntry) sendReply(ctx context.Context, s string) {
	if e.ReplyMessage == nil {
		e.ReplyMessage = sendReplyToMessage(ctx, e.Message, s)
	} else if e.ReplyMessage.Text != s {
		e.ReplyMessage.Text = s
		_, err := telegramBot.EditMessageText(ctx, &bot.EditMessageTextParams{
			MessageID: e.ReplyMessage.ID,
			ChatID:    e.ReplyMessage.Chat.ID,
			Text:      s,
		})
		if err != nil {
			fmt.Println("  reply edit error:", err)

			waitNeeded := e.checkWaitError(err)
			fmt.Println("  waiting", waitNeeded, "...")
			time.Sleep(waitNeeded)
		}
	}
}

func (e *DownloadQueueEntry) convertImagesFromPNGToJPG(ctx context.Context, imgs [][]byte) error {
	for i := range imgs {
		p, err := png.Decode(bytes.NewReader(imgs[i]))
		if err != nil {
			fmt.Println("  png decode error:", err)
			return fmt.Errorf("png decode error: %w", err)
		}
		buf := new(bytes.Buffer)
		err = jpeg.Encode(buf, p, &jpeg.Options{Quality: 80})
		if err != nil {
			fmt.Println("  jpg decode error:", err)
			return fmt.Errorf("jpg decode error: %w", err)
		}
		imgs[i] = buf.Bytes()
	}
	return nil
}

func (e *DownloadQueueEntry) sendImages(ctx context.Context, imgs [][]byte, retryAllowed bool) error {
	if len(imgs) == 0 {
		fmt.Println("  error: nothing to upload")
		return fmt.Errorf("nothing to upload")
	}

	var media []models.InputMedia
	for i := range imgs {
		var c string
		if i == 0 {
			c = e.RenderParamsText
			if len(c) > 1024 {
				c = c[:1021] + "..."
			}
		}
		media = append(media, &models.InputMediaPhoto{
			Media:           fmt.Sprintf("attach://ed-image-%x-%d-%d.jpg", e.Params.Seed, e.TaskID, i),
			MediaAttachment: bytes.NewReader(imgs[i]),
			Caption:         c,
		})
	}
	params := &bot.SendMediaGroupParams{
		ChatID:           e.Message.Chat.ID,
		ReplyToMessageID: e.Message.ID,
		Media:            media,
	}
	_, err := telegramBot.SendMediaGroup(ctx, params)
	if err != nil {
		fmt.Println("  send images error:", err)

		if !retryAllowed {
			return fmt.Errorf("send images error: %w", err)
		}

		retryAfter := e.checkWaitError(err)
		if retryAfter > 0 {
			fmt.Println("  retrying after", retryAfter, "...")
			time.Sleep(retryAfter)
			return e.sendImages(ctx, imgs, false)
		}
	}
	return nil
}

func (e *DownloadQueueEntry) deleteReply(ctx context.Context) {
	if e.ReplyMessage == nil {
		return
	}

	_, _ = telegramBot.DeleteMessage(ctx, &bot.DeleteMessageParams{
		MessageID: e.ReplyMessage.ID,
		ChatID:    e.ReplyMessage.Chat.ID,
	})
}

type DownloadQueueCurrentEntry struct {
	canceled  bool
	ctxCancel context.CancelFunc

	imgsChan    chan [][]byte
	errChan     chan error
	stoppedChan chan bool
}

type DownloadQueue struct {
	mutex          sync.Mutex
	ctx            context.Context
	entries        []DownloadQueueEntry
	processReqChan chan bool

	currentEntry DownloadQueueCurrentEntry
}

func (q *DownloadQueue) Add(params RenderParams, message *models.Message) {
	q.mutex.Lock()

	newEntry := DownloadQueueEntry{
		Params:  params,
		TaskID:  rand.Uint64(),
		Message: message,
	}

	if len(q.entries) > 0 {
		fmt.Println("  queueing request at position #", len(q.entries))
		newEntry.sendReply(q.ctx, q.getQueuePositionString(len(q.entries)))
	}

	q.entries = append(q.entries, newEntry)
	q.mutex.Unlock()

	select {
	case q.processReqChan <- true:
	default:
	}
}

func (q *DownloadQueue) CancelCurrentEntry(ctx context.Context) (err error) {
	q.mutex.Lock()
	if len(q.entries) > 0 {
		q.currentEntry.canceled = true
		q.currentEntry.ctxCancel()
	} else {
		fmt.Println("  no active request to cancel")
		err = fmt.Errorf("no active request to cancel")
	}
	q.mutex.Unlock()
	return
}

func (q *DownloadQueue) getQueuePositionString(pos int) string {
	return "👨‍👦‍👦 Request queued at position #" + fmt.Sprint(pos)
}

func (q *DownloadQueue) queryProgress(ctx context.Context, prevProgressPercent int) (progressPercent int, eta time.Duration, err error) {
	progressPercent = prevProgressPercent

	var newProgressPercent int
	newProgressPercent, eta, err = sdAPI.GetProgress(ctx)
	if err == nil && newProgressPercent > prevProgressPercent {
		progressPercent = newProgressPercent
		if progressPercent > 100 {
			progressPercent = 100
		} else if progressPercent < 0 {
			progressPercent = 0
		}
		fmt.Print("    progress: ", progressPercent, "% eta:", eta.Round(time.Second), "\n")
	}
	return
}

func (q *DownloadQueue) render(renderCtx context.Context, qEntry *DownloadQueueEntry, retryAllowed bool, imgsChan chan [][]byte, errChan chan error, stoppedChan chan bool) {
	imgs, err := sdAPI.Render(renderCtx, qEntry.Params)
	if err == nil {
		imgsChan <- imgs
		stoppedChan <- true
		return
	}

	if errors.Is(err, syscall.ECONNREFUSED) { // Can't connect to Stable Diffusion?
		qEntry.sendReply(renderCtx, restartStr)
		err := startStableDiffusionIfNeeded(renderCtx)
		if err != nil {
			fmt.Println("  error:", err)
			qEntry.sendReply(renderCtx, restartFailedStr+": "+err.Error())
			panic(err.Error())
		}
		if retryAllowed {
			q.render(renderCtx, qEntry, false, imgsChan, errChan, stoppedChan)
			return
		}
	}

	errChan <- err
	stoppedChan <- true
}

func (q *DownloadQueue) processQueueEntry(renderCtx context.Context, qEntry *DownloadQueueEntry) error {
	fmt.Print("processing request from ", qEntry.Message.From.Username, "#", qEntry.Message.From.ID, ": ", qEntry.Params.Prompt, "\n")

	var numOutputs string
	if qEntry.Params.NumOutputs > 1 {
		numOutputs = fmt.Sprintf("x%d", qEntry.Params.NumOutputs)
	}

	var outFormatText string
	if qEntry.Params.OutputPNG {
		outFormatText = "/PNG"
	}

	qEntry.RenderParamsText = fmt.Sprintf("🌱%d 👟%d 🕹%.1f 🖼%dx%d%s%s 🔭%s 🧩%s", qEntry.Params.Seed, qEntry.Params.Steps,
		qEntry.Params.CFGScale, qEntry.Params.Width, qEntry.Params.Height, numOutputs, outFormatText, qEntry.Params.SamplerName,
		qEntry.Params.ModelName)

	if qEntry.Params.HR.Scale > 0 {
		qEntry.RenderParamsText += " 🔎" + qEntry.Params.HR.Upscaler + "x" + fmt.Sprint(qEntry.Params.HR.Scale, "/", qEntry.Params.HR.DenoisingStrength)
	}

	if qEntry.Params.NegativePrompt != "" {
		negText := qEntry.Params.NegativePrompt
		if len(negText) > 10 {
			negText = negText[:10] + "..."
		}
		qEntry.RenderParamsText = "📍" + negText + " " + qEntry.RenderParamsText
	}

	qEntry.sendReply(q.ctx, processStartStr+"\n"+qEntry.RenderParamsText)

	go q.render(renderCtx, qEntry, true, q.currentEntry.imgsChan, q.currentEntry.errChan, q.currentEntry.stoppedChan)
	fmt.Println("  render started")

	progressUpdateInterval := groupChatProgressUpdateInterval
	if qEntry.Message.Chat.ID >= 0 {
		progressUpdateInterval = privateChatProgressUpdateInterval
	}
	progressPercentUpdateTicker := time.NewTicker(progressUpdateInterval)
	defer func() {
		progressPercentUpdateTicker.Stop()
		select {
		case <-progressPercentUpdateTicker.C:
		default:
		}
	}()
	progressCheckTicker := time.NewTicker(100 * time.Millisecond)
	defer func() {
		progressCheckTicker.Stop()
		select {
		case <-progressCheckTicker.C:
		default:
		}
	}()

	var progressPercent int
	var eta time.Duration
	var imgs [][]byte
	var err error
checkLoop:
	for {
		select {
		case <-renderCtx.Done():
			return fmt.Errorf("timeout")
		case <-progressPercentUpdateTicker.C:
			qEntry.sendReply(q.ctx, processStr+" "+getProgressbar(progressPercent, progressBarLength)+" ETA: "+fmt.Sprint(eta.Round(time.Second))+"\n"+qEntry.RenderParamsText)
		case <-progressCheckTicker.C:
			progressPercent, eta, err = q.queryProgress(renderCtx, progressPercent)
			if err != nil {
				return err
			}
		case err = <-q.currentEntry.errChan:
			return err
		case imgs = <-q.currentEntry.imgsChan:
			break checkLoop
		}
	}

	if !qEntry.Params.OutputPNG {
		err = qEntry.convertImagesFromPNGToJPG(q.ctx, imgs)
		if err != nil {
			return err
		}
	}

	fmt.Println("  uploading...")
	qEntry.sendReply(q.ctx, uploadingStr+"\n"+qEntry.RenderParamsText)

	err = qEntry.sendImages(q.ctx, imgs, true)
	if err == nil {
		qEntry.deleteReply(q.ctx)
	}
	return err
}

func (q *DownloadQueue) processor() {
	for {
		q.mutex.Lock()
		if (len(q.entries)) == 0 {
			q.mutex.Unlock()
			<-q.processReqChan
			continue
		}

		// Updating queue positions for all waiting entries.
		for i := 1; i < len(q.entries); i++ {
			sendReplyToMessage(q.ctx, q.entries[i].Message, q.getQueuePositionString(i))
		}

		qEntry := &q.entries[0]

		q.currentEntry = DownloadQueueCurrentEntry{}
		var renderCtx context.Context
		renderCtx, q.currentEntry.ctxCancel = context.WithTimeout(q.ctx, processTimeout)
		q.currentEntry.imgsChan = make(chan [][]byte)
		q.currentEntry.errChan = make(chan error, 1)
		q.currentEntry.stoppedChan = make(chan bool, 1)
		q.mutex.Unlock()

		err := q.processQueueEntry(renderCtx, qEntry)

		q.mutex.Lock()
		if q.currentEntry.canceled {
			fmt.Print("  canceled\n")
			err := sdAPI.Interrupt(q.ctx)
			if err != nil {
				fmt.Println("  can't interrupt:", err)
			}
			qEntry.sendReply(q.ctx, canceledStr)
		} else if err != nil {
			fmt.Println("  error:", err)
			qEntry.sendReply(q.ctx, errorStr+": "+err.Error())
		}

		q.currentEntry.ctxCancel()

		<-q.currentEntry.stoppedChan
		close(q.currentEntry.imgsChan)
		close(q.currentEntry.errChan)
		close(q.currentEntry.stoppedChan)

		q.entries = q.entries[1:]
		if len(q.entries) == 0 {
			fmt.Print("finished queue processing\n")
		}
		q.mutex.Unlock()
	}
}

func (q *DownloadQueue) Init(ctx context.Context) {
	q.ctx = ctx
	q.processReqChan = make(chan bool)
	go q.processor()
}
