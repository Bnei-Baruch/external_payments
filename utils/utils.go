package utils

import (
	"fmt"
	"time"
)

func LogMessage(message string) {
	currentTime := time.Now()
	m := fmt.Sprintf("%s %s", currentTime.Format("2006-01-02 15:04:05"), message)
	fmt.Println(m)
}
