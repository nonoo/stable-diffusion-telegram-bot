package main

import (
	"fmt"
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
