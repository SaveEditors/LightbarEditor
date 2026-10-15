LIGHTBAR EDITOR 1.7.0
Maintained by SaveEditors | GPL-3.0-only

Download and run LightbarEditor.exe. The editor opens in your web
browser. Keep the program window open while editing; use Quit to close it.
No separate runtime is required on Windows x64.

Open your own shutdown_lightbar.fbxd and shutdown_ramp.gnf, kept together in a
writable working folder. Console assets are not included. Choose a preset,
adjust strips/particles/ramp, and use Save effect. First saves of existing
local files retain a .bak when one does not already exist.

CONSOLE FTP
Start your existing console FTP server. Enter its address, port (default 1337)
and credentials in Console FTP. The usual remote folder is
/system_ex/vsh_asset. Check connection or Download pair to begin.

After saving your edits, Review upload creates and verifies a local backup
of both installed files. Check the destination, then select Upload this pair.
Each upload is downloaded again and compared. If a transfer fails, the editor
attempts to restore and verify files it touched. See docs/TRANSFER.md for full
instructions, including manual FTP transfer and failure recovery.

FTP is unencrypted; use a trusted local network. Credentials stay in memory.
Settings, presets, downloads and console backups use the current user's
configuration folder: %APPDATA%\LightbarEditor on Windows.

Desktop previews are approximate. Console animation timing remains under
investigation. Overlays/cached asset views can require a separate refresh.
This version edits the on-screen lightbar; it does not modify audio, ShellUI
backgrounds or physical case lights.

PROJECT
https://github.com/SaveEditors/LightbarEditor
README.md contains the full guide and demonstration media links.

SOURCE AND LICENSE
The complete corresponding source is published at
https://github.com/SaveEditors/LightbarEditor/tree/v1.7.0
Follow source/BUILD.txt to build it. No separate source ZIP is shipped.
Licensed under GNU GPL version 3 only. See LICENSE and THIRD_PARTY_LICENSES.txt.
Distributed without warranty. Independent project, not affiliated with Sony.
