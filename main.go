//go:build windows

// Pulse - a lightweight, colorful Task Manager.
package main

import (
	"os"

	"pulse/app"
)

func main() {
	demo, restarted := false, false
	for _, a := range os.Args[1:] {
		switch a {
		case "--demo":
			demo = true
		case "--restarted":
			restarted = true
		}
	}
	app.Run(demo, restarted)
}
