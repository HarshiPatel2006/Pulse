# Pulse roadmap and comparable-tool research

## Research snapshot

Windows Task Manager establishes the familiar process, performance, startup, users, details, and services model. System Informer/Process Hacker adds process-tree actions, priority controls, path inspection, and deep per-process diagnostics. btop emphasizes fast keyboard navigation and compact, sortable resource views. Mission Center provides a modern Linux process/performance UI. GNOME System Monitor demonstrates clear process search, end-process confirmation, and desktop integration.

## Features that already existed on main

Pulse already had process search/filter, confirmed end-task and process-tree termination, per-process GPU/VRAM/disk telemetry, startup apps, a dark theme, and a compact always-on-top widget on main. These remain supported on both platforms where the operating system exposes the data.

## New in this branch

- **Export CSV:** the Processes view now exports the current process snapshot to `Pulse/processes.csv` on Windows and `$XDG_CONFIG_HOME/Pulse/processes.csv` on Linux. Columns are platform-appropriate and spreadsheet-friendly.
- **Shared export behavior:** both native front ends use small, deterministic CSV writers with headers and explicit error reporting, without adding a background service or privileged dependency.
- **Cleanup:** removed an unused Windows path helper and an obsolete Linux CSV import.

## Next

- Improve non-NVIDIA GPU telemetry on Linux as stable cross-vendor APIs become available.
- Add signed Windows releases and reproducible checksums.
- Add translation support and accessibility review for native controls.
- Consider tray mode after validating lifecycle and background-resource expectations.
