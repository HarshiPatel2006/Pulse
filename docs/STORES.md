# Pulse store packaging and submission checklist

Packaging files are templates; no store submission has been made.

| Store | Account/fee | Exact submission steps |
|---|---|---|
| Microsoft Store | Partner Center developer account; registration fee may apply. | Reserve the app name, replace the MSIX identity/certificate placeholders, upload the signed MSIX, complete rating, privacy, pricing, markets, and listing metadata, then submit for certification. |
| WinGet | No separate fee; publish through microsoft/winget-pkgs. | Replace version, URL, and SHA256 in `packaging/winget`, run `wingetcreate update`, validate manifests, open the generated community PR, and resolve validation. |
| Flathub | Free; GitHub and Flathub account/agreement required. | Set the release source/checksum and permissions, validate with `flatpak-builder`, open a Flathub submission PR using `packaging/flatpak`, and complete review. |
| Snap Store | Ubuntu One account; public snaps are free. | Register the snap name, build/test with `snapcraft`, upload, and release the stable channel. |
| GitHub Releases | Repository write access; no fee. | Push a version tag, let `.github/workflows/release.yml` build artifacts, verify checksums, and publish the draft release. |

Before submission replace placeholders, sign Windows artifacts, test install/uninstall, provide privacy/support links, and attach the screenshots in `docs/screenshots`.
