package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-telegram/bot"
)

type GetFile struct {
}

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer interface
// and we can pass this into io.TeeReader() which will report progress on each write cycle.
type WriteCounter struct {
	Ctx                   context.Context
	GotBytes              int64
	TotalBytes            int64
	ProgressPrintInterval time.Duration
	LastProgressPrintAt   time.Time
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.GotBytes += int64(n)

	if time.Since(wc.LastProgressPrintAt) > wc.ProgressPrintInterval {
		progressPercent := int(float64(wc.GotBytes) / float64(wc.TotalBytes) * 100)
		fmt.Print("    progress: ", progressPercent, "%\n")
		reqQueue.currentEntry.entry.sendReply(wc.Ctx, downloadingStr+" "+getProgressbar(progressPercent, progressBarLength))
		wc.LastProgressPrintAt = time.Now()
	}
	return n, nil
}

func (g *GetFile) GetFile(ctx context.Context, fileID string) (d []byte, err error) {
	fmt.Println("  downloading...")

	f, err := telegramBot.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return nil, err
	}
	resp, err := http.Get("https://api.telegram.org/file/bot" + params.BotToken + "/" + f.FilePath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	counter := &WriteCounter{
		Ctx:                   ctx,
		TotalBytes:            int64(f.FileSize),
		ProgressPrintInterval: groupChatProgressUpdateInterval,
	}

	if reqQueue.currentEntry.entry.Message.Chat.ID >= 0 {
		counter.ProgressPrintInterval = privateChatProgressUpdateInterval
	}

	d, err = io.ReadAll(io.TeeReader(resp.Body, counter))
	if err != nil {
		return nil, err
	}

	fmt.Println("  downloading done")
	return d, nil
}
