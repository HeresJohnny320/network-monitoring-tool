package utils

import (
	"fmt"
	"time"
)

var Colors = map[string]string{
	"black":   "\033[30m",
	"red":     "\033[31m",
	"green":   "\033[32m",
	"yellow":  "\033[33m",
	"blue":    "\033[34m",
	"magenta": "\033[35m",
	"cyan":    "\033[36m",
	"white":   "\033[37m",

	"bright_black":   "\033[90m",
	"bright_red":     "\033[91m",
	"bright_green":   "\033[92m",
	"bright_yellow":  "\033[93m",
	"bright_blue":    "\033[94m",
	"bright_magenta": "\033[95m",
	"bright_cyan":    "\033[96m",
	"bright_white":   "\033[97m",
}

var Styles = map[string]string{
	"reset":         "\033[0m",
	"bold":          "\033[1m",
	"dim":           "\033[2m",
	"italic":        "\033[3m",
	"underline":     "\033[4m",
	"blink":         "\033[5m",
	"inverse":       "\033[7m",
	"hidden":        "\033[8m",
	"strikethrough": "\033[9m",
}

func PrintColor(color string, text string, styles ...string) {
	timestamp := time.Now().UTC().Format("2006-01-02 15:04:05")
	code, ok := Colors[color]
	if !ok {
		code = Styles["reset"]
	}

	fmt.Println(code + "[" + timestamp + "] " + text + Styles["reset"])
}
