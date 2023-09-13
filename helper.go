package main

import (
	"fmt"
	"path/filepath"
)

func getProgressbar(progressPercent, progressBarLen int) (progressBar string) {
	i := 0
	for ; i < progressPercent/(100/progressBarLen); i++ {
		progressBar += "▰"
	}
	for ; i < progressBarLen; i++ {
		progressBar += "▱"
	}
	progressBar += " " + fmt.Sprint(progressPercent) + "%"
	return
}

func fileNameWithoutExt(fileName string) string {
	return fileName[:len(fileName)-len(filepath.Ext(fileName))]
}
