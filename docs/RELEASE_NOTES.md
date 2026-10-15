# INITIAL RELEASE — Lightbar Editor 1.7.0

Customize the PS5 on-screen shutdown lightbar with strip colors, particle palettes and ramp textures.

The initial release includes a dedicated **Console FTP** page. Enter your own console address and port, download the two supported assets, edit locally, then review an upload. The editor creates a verified backup of the installed pair before writing, checks for changed files and downloads each upload again to compare its bytes. If a transfer fails, it attempts to restore and verify the files it touched.

Created by **SaveEditors**, licensed under **GPL-3.0-only**. The README includes two console demonstration videos and a Matrix editor screenshot, with transfer and recovery instructions.

Every theme includes a matching particle palette. Choosing or editing particle colors preserves that customization across later theme changes. Enable **Use theme particle colors** to return to automatic matching. Saved effects include their particle palette.

## Downloads

- **LightbarEditor.exe** — portable Windows x64 application. Download and run; no separate runtime is needed.
- **LICENSE.txt**, **THIRD_PARTY_LICENSES.txt** and **FONT_LICENSE.txt** — project and dependency license notices.
- **SHA256SUMS.txt** — SHA-256 checksums for the executable and notices.

**Source code:** [SaveEditors/LightbarEditor at v1.7.0](https://github.com/SaveEditors/LightbarEditor/tree/v1.7.0). The complete corresponding source, build instructions and dependency lock data are published in the repository. No separate source ZIP is attached as a release asset.

Console assets are not included. Supply your own supported `shutdown_lightbar.fbxd` and `shutdown_ramp.gnf`. FTP requires an existing console FTP server with the necessary permissions. Use a trusted local network; FTP is unencrypted.

## Scope and known limitations

- Desktop previews are approximate. Console ramp timing and sampling remain under investigation.
- FTP checks the selected remote folder; overlays and cached asset views may require a separate refresh using your console setup.
- A two-file upload is not atomic. Preserve the displayed backup folder and follow the recovery guide if automatic restoration is incomplete.
- This editor does not modify audio, the ShellUI background or physical console lights.

## Validation

Local tests cover successful uploads, single-use reviews, stale file rejection, partial writes, readback mismatch, successful restoration and incomplete restoration reporting. Application integration checks cover real mesh/ramp saves, backup recovery, relocated executable paths and isolated user profiles. Live console FTP validation of this version remains pending.

Licensed under GNU GPL version 3 only. Maintained by SaveEditors.
