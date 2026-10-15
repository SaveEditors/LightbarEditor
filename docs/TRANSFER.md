# Transfer and recovery

Lightbar Editor transfers exactly two files: `shutdown_lightbar.fbxd` and `shutdown_ramp.gnf`. Use assets obtained from your own console and retain a known-good pair outside the editor's working folders.

## Choose the destination

The default remote folder is `/system_ex/vsh_asset`. Enter the address and port reported by your existing FTP server. The editor starts with a blank address and never connects automatically. A successful check confirms both files are readable and in supported formats; it does not test write permissions.

Some console setups present ShellUI assets through an overlay or sandbox mount. Writing the base folder may leave the active view unchanged. Select the correct existing folder and follow your setup's refresh or remount procedure. The editor does not discover or update additional mounts, and does not launch payloads. Avoid changing unrelated files.

## Download and edit

**Download pair** retrieves and validates both files before writing a new working folder under `%APPDATA%\LightbarEditor\downloads`. Existing local working folders are retained. Both downloaded files open in the editor. Save your changes before preparing an upload.

## Review and upload

**Review upload** reads the currently installed pair and creates a new folder under `%APPDATA%\LightbarEditor\console-backups`. Both local backup files are read back and compared before the upload review appears. Its manifest records the remote folder, server address, timestamp, sizes and SHA-256 hashes. Credentials are never included.

The review shows the exact destination and two filenames. Nothing is written to the console until you choose **Upload this pair**. Reviews expire after 10 minutes and are single-use. Before writing, the editor verifies that the local saved files and remote pair still match the reviewed copies. Any change requires a new review.

Each uploaded file is downloaded again and compared byte for byte. A success message confirms both files at that destination. It does not establish that ShellUI uses that folder or that the console's animation matches the desktop preview.

## A transfer failed

The two-file replacement is not atomic. Keep the PC, console and network running until the result appears.

If an upload or its readback fails, the editor reconnects and attempts to restore every file it tried to write, including a partially written file. Each restoration is also downloaded and verified. The status explicitly reports either verified restoration or **automatic restoration incomplete**, and identifies the backup folder.

If restoration is incomplete:

1. Keep the displayed local backup folder. Do not replace its files with edited copies.
2. Restore FTP connectivity and use an FTP client to upload **both backup files** to the destination recorded in `manifest.json`, using binary mode and their original filenames.
3. Download the restored pair and compare their SHA-256 hashes with the manifest. PowerShell: `Get-FileHash -Algorithm SHA256 <file>`.
4. Refresh the active asset view as required by your console setup before testing again.

A local `transfer-result.json` records the result when it can be saved. If the editor or PC stops unexpectedly, use the backup manifest to restore the pair manually; automatic restoration requires the editor to remain running.

## Local editing recovery

The first save of an existing local file creates `<filename>.bak` if one does not already exist. **Restore .bak backup** validates and restores that copy. A `.bak` can contain earlier custom edits; it is not necessarily a stock asset.

**Reset to as opened** discards changes made since the current file was opened. For an original reference, reopen your independently retained known-good pair.

## Network details

FTP is unencrypted. Use a trusted local network and do not expose this workflow to the internet. The address, credentials and pending upload plans remain in memory for the current app session. Backup manifests retain the server address and remote folder so recovery remains possible. No console address is embedded in the executable.

The editor transfers only supported pairs, with a 32 MiB limit per file. Unsupported or unreadable remote assets are rejected before backup review. Recover damaged or unsupported assets using your own known-good pair and an external FTP client.
