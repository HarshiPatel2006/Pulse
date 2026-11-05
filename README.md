# Pulse — free open-source Task Manager for Windows and Linux

Pulse is a free open-source Task Manager for Windows and Linux: a lightweight system monitor and Task Manager alternative with live CPU, memory, disk, network, GPU and VRAM telemetry. It is native, colorful, fast, and small enough to keep open while gaming, coding, or troubleshooting.

![Pulse processes](docs/screenshots/processes.png)

## Highlights

- Process search by name, publisher, title, or PID, with grouped and ungrouped views.
- End task, kill a process tree, change priority, and inspect file locations with confirmation for risky actions.
- Per-process CPU, memory, disk, GPU, and GPU-memory columns with sortable heat maps.
- Live CPU, RAM, disks, network, NVIDIA GPU, temperature, power, and VRAM graphs.
- Startup apps, services, users, app history, system details, and a compact always-on-top widget.
- Dark, keyboard-friendly native interfaces: Win32 on Windows and Fyne on Linux.

## Screenshots

![GPU performance](docs/screenshots/performance-gpu.png)
![Memory performance](docs/screenshots/performance-memory.png)
![System information](docs/screenshots/system-info.png)
![Compact widget](docs/screenshots/widget.png)

## Install

### Windows

Download the latest `Pulse-windows-amd64.exe` from [GitHub Releases](https://github.com/HarshiPatel2006/Pulse/releases), then run it. Pulse is portable and needs no installer or runtime. Windows 10 and 11 (64-bit) are supported.

### Linux

Download `Pulse-linux-amd64` from Releases, make it executable, and run it:

```sh
chmod +x Pulse-linux-amd64
./Pulse-linux-amd64
```

For Ubuntu/Debian builds from source, install `gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev`, then run `./run-linux.sh`. Flatpak and Snap packaging is described in [docs/STORES.md](docs/STORES.md).

## Build from source

Go 1.26 or newer is required. `go test ./...` checks the project; cross-build with `GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-H windowsgui -s -w' -o Pulse.exe .` or `go build -trimpath -ldflags='-s -w' -o Pulse-linux-amd64 .`.

## FAQ

**Is Pulse a Task Manager alternative?** Yes. It combines familiar process, performance, startup, user, service, and detail views with GPU/VRAM telemetry.

**Does it work on Linux?** Yes. The native Linux desktop app supports X11 and Wayland through Fyne; some GPU and service data depends on local drivers and permissions.

**Is it free and open source?** Yes. Pulse is MIT-licensed and its source is available on GitHub.

**Does it need administrator/root access?** No for monitoring. Windows may require elevation for protected processes and Linux permissions may limit details for other users.

## License

[MIT](LICENSE)
