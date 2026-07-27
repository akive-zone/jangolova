package main

import (
	"fmt"
	"os"

	"github.com/mxschmitt/playwright-go"
)

func main() {
	if err := playwright.Install(&playwright.RunOptions{
		SkipInstallBrowsers: true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "install Playwright driver: %v\n", err)
		os.Exit(1)
	}
}
