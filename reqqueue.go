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

const imageReqStr = "🩻 Please post the image file to process."
const processStartStr = "🛎 Starting render..."
const processStr = "🔨 Processing"
const progressBarLength = 20
const downloadingStr = "⬇ Downloading..."
const uploadingStr = "☁ ️ Uploading..."
const doneStr = "✅ Done"
const errorStr = "❌ Error"
const canceledStr = "❌ Canceled"
const restartStr = "⚠️ Stable Diffusion is not running, starting, please wait..."
const restartFailedStr = "☠️ Stable Diffusion start failed, please restart the bot"

const processTimeout = 10 * time.Minute
const groupChatProgressUpdateInterval = 3 * time.Second
const privateChatProgressUpdateInterval = 500 * time.Millisecond

type ReqType int

const (
	ReqTypeRender ReqType = iota
	ReqTypeUpscale
)

type ReqQueueEntry struct {
	Type ReqType

	Params ReqParams

	TaskID uint64

	ReplyMessage *models.Message
	Message      *models.Message
}

func (e *ReqQueueEntry) checkWaitError(err error) time.Duration {
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

func (e *ReqQueueEntry) sendReply(ctx context.Context, s string) {
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

func (e *ReqQueueEntry) convertImagesFromPNGToJPG(ctx context.Context, imgs [][]byte) error {
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

// If filename is empty then a filename will be automatically generated.
func (e *ReqQueueEntry) uploadImages(ctx context.Context, firstImageID uint32, description string, imgs [][]byte, filename string, retryAllowed bool) error {
	if len(imgs) == 0 {
		fmt.Println("  error: nothing to upload")
		return fmt.Errorf("nothing to upload")
	}

	generateFilename := (filename == "")

	var media []models.InputMedia
	for i := range imgs {
		var c string
		if i == 0 {
			c = description
			if len(c) > 1024 {
				c = c[:1021] + "..."
			}
		}
		if generateFilename {
			filename = fmt.Sprintf("sd-image-%d-%d-%d.jpg", firstImageID, e.TaskID, i)
		}
		media = append(media, &models.InputMediaPhoto{
			Media:           "attach://" + filename,
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
			return e.uploadImages(ctx, firstImageID, description, imgs, filename, false)
		}
	}
	return nil
}

func (e *ReqQueueEntry) deleteReply(ctx context.Context) {
	if e.ReplyMessage == nil {
		return
	}

	_, _ = telegramBot.DeleteMessage(ctx, &bot.DeleteMessageParams{
		MessageID: e.ReplyMessage.ID,
		ChatID:    e.ReplyMessage.Chat.ID,
	})
}

type ReqQueueCurrentEntry struct {
	entry     *ReqQueueEntry
	canceled  bool
	ctxCancel context.CancelFunc

	imgsChan    chan [][]byte
	errChan     chan error
	stoppedChan chan bool

	gotImageChan chan ImageFileData
}

type ReqQueue struct {
	mutex          sync.Mutex
	ctx            context.Context
	entries        []ReqQueueEntry
	processReqChan chan bool

	currentEntry ReqQueueCurrentEntry
}

type ReqQueueReq struct {
	Type    ReqType
	Message *models.Message
	Params  ReqParams
}

func (q *ReqQueue) Add(req ReqQueueReq) {
	q.mutex.Lock()

	newEntry := ReqQueueEntry{
		Type:    req.Type,
		Message: req.Message,
		Params:  req.Params,
		TaskID:  rand.Uint64(),
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

func (q *ReqQueue) CancelCurrentEntry(ctx context.Context) (err error) {
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

func (q *ReqQueue) getQueuePositionString(pos int) string {
	return "👨‍👦‍👦 Request queued at position #" + fmt.Sprint(pos)
}

func (q *ReqQueue) queryProgress(ctx context.Context, prevProgressPercent int) (progressPercent int, eta time.Duration, err error) {
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
		fmt.Print("    progress: ", progressPercent, "% eta: ", eta.Round(time.Second), "\n")
	}
	return
}

type ReqQueueEntryProcessFn func(context.Context, ReqParams, ImageFileData) (imgs [][]byte, err error)

func (q *ReqQueue) runProcessThread(processCtx context.Context, processFn ReqQueueEntryProcessFn, reqParams ReqParams, imageData ImageFileData, retryAllowed bool,
	imgsChan chan [][]byte, errChan chan error, stoppedChan chan bool) {

	imgs, err := processFn(processCtx, reqParams, imageData)
	if err == nil {
		imgsChan <- imgs
		stoppedChan <- true
		return
	}

	if errors.Is(err, syscall.ECONNREFUSED) { // Can't connect to Stable Diffusion?
		if params.SDStart {
			q.currentEntry.entry.sendReply(processCtx, restartStr)
			err = startStableDiffusionIfNeeded(processCtx)
			if err != nil {
				fmt.Println("  error:", err)
				q.currentEntry.entry.sendReply(processCtx, restartFailedStr+": "+err.Error())
				panic(err.Error())
			}
			if retryAllowed {
				q.runProcessThread(processCtx, processFn, reqParams, imageData, false, imgsChan, errChan, stoppedChan)
				return
			}
		} else {
			err = fmt.Errorf("Stable Diffusion is not running and start is disabled.")
			fmt.Println("  error:", err)
		}
	}

	errChan <- err
	stoppedChan <- true
}

func (q *ReqQueue) runProcess(processCtx context.Context, processFn ReqQueueEntryProcessFn, reqParams ReqParams, imageData ImageFileData, reqParamsText string) (imgs [][]byte, err error) {
	q.currentEntry.entry.sendReply(q.ctx, processStartStr+"\n"+reqParamsText)

	q.currentEntry.imgsChan = make(chan [][]byte)
	q.currentEntry.errChan = make(chan error, 1)
	q.currentEntry.stoppedChan = make(chan bool, 1)

	go q.runProcessThread(processCtx, processFn, reqParams, imageData, true, q.currentEntry.imgsChan, q.currentEntry.errChan, q.currentEntry.stoppedChan)
	fmt.Println("  render started")

	progressUpdateInterval := groupChatProgressUpdateInterval
	if q.currentEntry.entry.Message.Chat.ID >= 0 {
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
	for {
		select {
		case <-processCtx.Done():
			return nil, fmt.Errorf("timeout")
		case <-progressPercentUpdateTicker.C:
			q.currentEntry.entry.sendReply(q.ctx, processStr+" "+getProgressbar(progressPercent, progressBarLength)+" ETA: "+fmt.Sprint(eta.Round(time.Second))+"\n"+reqParamsText)
		case <-progressCheckTicker.C:
			progressPercent, eta, _ = q.queryProgress(processCtx, progressPercent)
		case err = <-q.currentEntry.errChan:
			return nil, err
		case imgs = <-q.currentEntry.imgsChan:
			return imgs, nil
		}
	}
}

func (q *ReqQueue) upscale(processCtx context.Context, reqParams ReqParamsUpscale, imageData ImageFileData) error {
	reqParamsText := reqParams.String()

	imgs, err := q.runProcess(processCtx, sdAPI.Upscale, reqParams, imageData, reqParamsText)
	if err != nil {
		return err
	}

	fn := fileNameWithoutExt(imageData.filename) + "-upscaled"
	if !reqParams.OutputPNG {
		err = q.currentEntry.entry.convertImagesFromPNGToJPG(q.ctx, imgs)
		if err != nil {
			return err
		}
		fn += ".jpg"
	} else {
		fn += ".png"
	}

	fmt.Println("  uploading...")
	q.currentEntry.entry.sendReply(q.ctx, uploadingStr+"\n"+reqParamsText)

	err = q.currentEntry.entry.uploadImages(q.ctx, 0, "", imgs, fn, true)
	if err == nil {
		q.currentEntry.entry.deleteReply(q.ctx)
	}
	return err
}

func (q *ReqQueue) render(processCtx context.Context, reqParams ReqParamsRender) error {
	reqParamsText := reqParams.String()

	imgs, err := q.runProcess(processCtx, sdAPI.Render, reqParams, ImageFileData{}, reqParamsText)
	if err != nil {
		return err
	}

	// Now we have the output images.
	if reqParams.Upscale.Scale > 0 {
		reqParamsUpscale := ReqParamsUpscale{
			origPrompt: reqParams.OrigPrompt(),
			Scale:      reqParams.Upscale.Scale,
			Upscaler:   reqParams.Upscale.Upscaler,
			OutputPNG:  reqParams.OutputPNG,
		}
		imgs, err = q.runProcess(processCtx, sdAPI.Upscale, reqParamsUpscale, ImageFileData{data: imgs[0], filename: ""}, reqParamsUpscale.String())
		if err != nil {
			return err
		}
	}

	if !reqParams.OutputPNG {
		err = q.currentEntry.entry.convertImagesFromPNGToJPG(q.ctx, imgs)
		if err != nil {
			return err
		}
	}

	fmt.Println("  uploading...")
	q.currentEntry.entry.sendReply(q.ctx, uploadingStr+"\n"+reqParamsText)

	err = q.currentEntry.entry.uploadImages(q.ctx, reqParams.Seed, reqParams.OrigPrompt()+"\n"+reqParamsText, imgs, "", true)
	if err == nil {
		q.currentEntry.entry.deleteReply(q.ctx)
	}
	return err
}

func (q *ReqQueue) processQueueEntry(processCtx context.Context, imageData ImageFileData) error {
	fmt.Print("processing request from ", q.currentEntry.entry.Message.From.Username, "#",
		q.currentEntry.entry.Message.From.ID, ": ", q.currentEntry.entry.Params.OrigPrompt(), "\n")

	switch q.currentEntry.entry.Type {
	case ReqTypeRender:
		return q.render(processCtx, q.currentEntry.entry.Params.(ReqParamsRender))
	case ReqTypeUpscale:
		return q.upscale(processCtx, q.currentEntry.entry.Params.(ReqParamsUpscale), imageData)
	default:
		return fmt.Errorf("unknown request")
	}
}

func (q *ReqQueue) processor() {
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

		q.currentEntry = ReqQueueCurrentEntry{
			entry: &q.entries[0],
		}
		var processCtx context.Context
		processCtx, q.currentEntry.ctxCancel = context.WithTimeout(q.ctx, processTimeout)
		q.mutex.Unlock()

		var err error
		var imageData ImageFileData
		imageNeededFirst := false
		switch q.currentEntry.entry.Type {
		case ReqTypeUpscale:
			imageNeededFirst = true
		}
		if imageNeededFirst {
			fmt.Println("  waiting for image file...")
			q.currentEntry.entry.sendReply(q.ctx, imageReqStr)
			q.currentEntry.gotImageChan = make(chan ImageFileData)
			select {
			case imageData = <-q.currentEntry.gotImageChan:
			case <-processCtx.Done():
				q.currentEntry.canceled = true
			case <-time.NewTimer(3 * time.Minute).C:
				fmt.Println("  waiting for image file timeout")
				err = fmt.Errorf("waiting for image data timeout")
			}
			close(q.currentEntry.gotImageChan)
			q.currentEntry.gotImageChan = nil

			if err == nil && len(imageData.data) == 0 {
				err = fmt.Errorf("got no image data")
			}
		}

		if err == nil {
			err = q.processQueueEntry(processCtx, imageData)
		}

		q.mutex.Lock()
		if q.currentEntry.canceled {
			fmt.Print("  canceled\n")
			err = sdAPI.Interrupt(q.ctx)
			if err != nil {
				fmt.Println("  can't interrupt:", err)
			}
			q.currentEntry.entry.sendReply(q.ctx, canceledStr)
		} else if err != nil {
			fmt.Println("  error:", err)
			q.currentEntry.entry.sendReply(q.ctx, errorStr+": "+err.Error())
		}

		q.currentEntry.ctxCancel()

		if q.currentEntry.stoppedChan != nil {
			<-q.currentEntry.stoppedChan
			close(q.currentEntry.imgsChan)
			close(q.currentEntry.errChan)
			close(q.currentEntry.stoppedChan)
			q.currentEntry.stoppedChan = nil
		}

		q.entries = q.entries[1:]
		if len(q.entries) == 0 {
			fmt.Print("finished queue processing\n")
		}
		q.mutex.Unlock()
	}
}

func (q *ReqQueue) Init(ctx context.Context) {
	q.ctx = ctx
	q.processReqChan = make(chan bool)
	go q.processor()
}
