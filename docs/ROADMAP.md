# Pulse roadmap and comparable-tool research

## Research snapshot

Windows Task Manager establishes the familiar process, performance, startup, users, details, and services model. System Informer/Process Hacker adds process-tree actions, priority controls, path inspection, and deep per-process diagnostics. btop emphasizes fast keyboard navigation and compact, sortable resource views. Mission Center provides a modern Linux process/performance UI. GNOME System Monitor demonstrates clear process search, end-process confirmation, and desktop integration.

## Choices shipped in Pulse

The high-value features that fit Pulse's lightweight scope are process search/filter by name, publisher, title, or PID; end task and process-tree termination with confirmation; per-process GPU, VRAM, disk, and other resource telemetry; and startup apps, dark theme, and a compact always-on-top widget. They are implemented on both platforms where supported, with graceful unavailable-telemetry reporting on Linux.

## Next

- Improve non-NVIDIA GPU telemetry on Linux as stable cross-vendor APIs become available.
- Add signed Windows releases and reproducible checksums.
- Add translation support and accessibility review for native controls.
- Consider tray mode after validating lifecycle and background-resource expectations.
