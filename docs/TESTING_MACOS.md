# macOS port validation

## Completed on Apple Silicon

- Built Apple Silicon (arm64) host and Go backend targeting macOS 13+.
- Go tests and vet pass with FTP v0.2.4, matching the upstream third-party notice.
- Particle regression test passes: 21 theme defaults, custom palette preservation, ramp adjustments, and undo/redo.
- Windows x64 backend cross-build succeeds after the platform build-tag split.
- Actual macOS window renders both editor pages and console FTP controls.
- Native Open sheet loads a supported FBXD mesh and BC7 GNF ramp downloaded from an owned firmware-13.60 console.
- Applied a preset, saved separate edited FBXD and GNF copies through native Save sheets, and verified both file structures with the backend CLI.
- Original downloaded reference files retain their original SHA-256 hashes.
- Captured actual macOS editor, ramp and native Save screenshots using a temporary demo directory without private paths.
- Apple Silicon bundle signature verified; DMG checksum and packaged bundle validated separately.

## Still pending

- macOS 13 runtime/UI testing on that exact OS version.
- Apple notarization of the signed preview distribution.
- A console FTP upload and on-console shutdown preview using edited assets. No system assets were changed during this port validation.

## Repeat locally

```sh
cd source
go test ./...
go vet ./...
node --test particle_themes_test.mjs
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/LightbarEditor.exe .
cd ..
./script/build_and_run.sh --verify
./script/package_dmg.sh
codesign --verify --deep --strict 'dist/Lightbar Editor.app'
hdiutil verify dist/LightbarEditor-1.7.0-macOS-arm64.dmg
```

For UI testing, use your own supported assets. Never commit or distribute console system files, FTP credentials, presets containing private paths, or downloaded backup folders.
