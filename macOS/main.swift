// SPDX-License-Identifier: GPL-3.0-only
import AppKit
import WebKit

let args = CommandLine.arguments
if args.contains("--open-dialog") || args.contains("--save-dialog") {
    let app = NSApplication.shared
    app.setActivationPolicy(.accessory)
    app.activate(ignoringOtherApps: true)
    var paths: [String] = []
    if args.contains("--open-dialog") {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = false
        panel.allowsMultipleSelection = args.last == "multi"
        panel.title = "Open Lightbar Assets"
        if panel.runModal() == .OK { paths = panel.urls.map(\.path) }
    } else {
        let panel = NSSavePanel()
        panel.title = "Save Lightbar Asset"
        if args.count > 2 { panel.nameFieldStringValue = args[2] }
        if args.count > 3 && !args[3].isEmpty { panel.directoryURL = URL(fileURLWithPath: args[3]) }
        if panel.runModal() == .OK, let url = panel.url { paths = [url.path] }
    }
    let data = try! JSONSerialization.data(withJSONObject: paths)
    FileHandle.standardOutput.write(data)
    exit(0)
}

final class AppDelegate: NSObject, NSApplicationDelegate, WKNavigationDelegate, WKUIDelegate, NSWindowDelegate {
    var window: NSWindow!
    var web: WKWebView!
    var backend: Process?
    var output = ""
    var loaded = false
    var bridgeInput: FileHandle?
    var lineBuffer = ""
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.setActivationPolicy(.regular)
        makeMenus()
        let config = WKWebViewConfiguration()
        web = WKWebView(frame: .zero, configuration: config)
        web.navigationDelegate = self
        web.uiDelegate = self
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1320, height: 880), styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.title = "Lightbar Editor"
        window.delegate = self
        window.minSize = NSSize(width: 1000, height: 680)
        window.contentView = web
        window.center()
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        startBackend()
    }
    func makeMenus() {
        let menu = NSMenu()
        let appItem = NSMenuItem(); menu.addItem(appItem)
        let appMenu = NSMenu(); appItem.submenu = appMenu
        appMenu.addItem(withTitle: "About Lightbar Editor", action: #selector(about), keyEquivalent: "")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Quit Lightbar Editor", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        let editItem = NSMenuItem(); menu.addItem(editItem)
        let edit = NSMenu(title: "Edit"); editItem.submenu = edit
        for (title, action, key) in [("Cut", "cut:", "x"), ("Copy", "copy:", "c"), ("Paste", "paste:", "v"), ("Select All", "selectAll:", "a")] {
            edit.addItem(withTitle: title, action: Selector(action), keyEquivalent: key)
        }
        NSApp.mainMenu = menu
    }
    @objc func about() {
        NSApp.orderFrontStandardAboutPanel(options: [.applicationName: "Lightbar Editor", .applicationVersion: "1.7.0 · macOS port", .credits: NSAttributedString(string: "Original editor © SaveEditors. GPL-3.0-only.\nmacOS port with native window and file dialogs.")])
    }
    func startBackend() {
        let process = Process()
        process.executableURL = Bundle.main.url(forAuxiliaryExecutable: "LightbarBackend")
        var env = ProcessInfo.processInfo.environment
        env["LIGHTBAR_NO_BROWSER"] = "1"
        env["LIGHTBAR_NATIVE_BRIDGE"] = "1"
        env["LIGHTBAR_NATIVE_HOST"] = Bundle.main.executablePath
        process.environment = env
        process.arguments = Array(CommandLine.arguments.dropFirst()).filter { !$0.hasPrefix("-psn_") }
        let input = Pipe(); process.standardInput = input; bridgeInput = input.fileHandleForWriting
        let pipe = Pipe(); process.standardOutput = pipe
        process.standardError = FileHandle.standardError
        pipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let bytes = handle.availableData
            guard !bytes.isEmpty else { handle.readabilityHandler = nil; return }
            let text = String(decoding: bytes, as: UTF8.self)
            DispatchQueue.main.async {
                guard let self = self else { return }
                self.output += text
                self.lineBuffer += text
                while let end = self.lineBuffer.firstIndex(of: "\n") {
                    let line = String(self.lineBuffer[..<end]); self.lineBuffer.removeSubrange(...end)
                    if line.hasPrefix("LIGHTBAR_DIALOG\t"), let data = String(line.dropFirst(16)).data(using: .utf8), let args = try? JSONDecoder().decode([String].self, from: data) { self.showDialog(args) }
                }
                if !self.loaded, let range = self.output.range(of: "http://127.0.0.1:[0-9]+/", options: .regularExpression), let url = URL(string: String(self.output[range])) {
                    self.loaded = true
                    self.web.load(URLRequest(url: url))
                }
            }
        }
        process.terminationHandler = { _ in DispatchQueue.main.async { NSApp.terminate(nil) } }
        do { try process.run(); backend = process }
        catch { let alert = NSAlert(error: error); alert.runModal(); NSApp.terminate(nil) }
    }
    func showDialog(_ args: [String]) {
        func reply(_ paths: [String]) {
            var data = (try? JSONSerialization.data(withJSONObject: paths)) ?? Data("[]".utf8)
            data.append(10); try? self.bridgeInput?.write(contentsOf: data)
        }
        if args.first == "--open-dialog" {
            let panel = NSOpenPanel(); panel.canChooseDirectories = false
            panel.allowsMultipleSelection = args.last == "multi"
            panel.beginSheetModal(for: window) { result in reply(result == .OK ? panel.urls.map(\.path) : []) }
        } else {
            let panel = NSSavePanel()
            if args.count > 1 { panel.nameFieldStringValue = args[1] }
            if args.count > 2 && !args[2].isEmpty { panel.directoryURL = URL(fileURLWithPath: args[2]) }
            panel.beginSheetModal(for: window) { result in reply(result == .OK ? panel.url.map { [$0.path] } ?? [] : []) }
        }
    }
    func webView(_ webView: WKWebView, decidePolicyFor action: WKNavigationAction, decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let url = action.request.url else { decisionHandler(.cancel); return }
        if url.host == "127.0.0.1" { decisionHandler(.allow) }
        else { if ["https", "http"].contains(url.scheme ?? "") { NSWorkspace.shared.open(url) }; decisionHandler(.cancel) }
    }
    func webView(_ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String, initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void) {
        let alert = NSAlert(); alert.messageText = message; alert.beginSheetModal(for: window) { _ in completionHandler() }
    }
    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String, initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void) {
        let alert = NSAlert(); alert.messageText = message; alert.addButton(withTitle: "Continue"); alert.addButton(withTitle: "Cancel")
        alert.beginSheetModal(for: window) { completionHandler($0 == .alertFirstButtonReturn) }
    }
    func webView(_ webView: WKWebView, runJavaScriptTextInputPanelWithPrompt prompt: String, defaultText: String?, initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (String?) -> Void) {
        let alert = NSAlert(); alert.messageText = prompt
        let field = NSTextField(string: defaultText ?? ""); field.frame = NSRect(x: 0, y: 0, width: 360, height: 24)
        alert.accessoryView = field; alert.addButton(withTitle: "OK"); alert.addButton(withTitle: "Cancel")
        alert.beginSheetModal(for: window) { completionHandler($0 == .alertFirstButtonReturn ? field.stringValue : nil) }
    }
    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        guard backend?.isRunning == true, loaded else { return .terminateNow }
        web.evaluateJavaScript("typeof dirtyAny === 'function' && dirtyAny()") { result, _ in
            if result as? Bool == true {
                let alert = NSAlert(); alert.messageText = "Quit with unsaved changes?"
                alert.informativeText = "Your saved files stay unchanged. Save your work before quitting to keep these edits."
                alert.addButton(withTitle: "Keep Editing"); alert.addButton(withTitle: "Quit Without Saving")
                alert.beginSheetModal(for: self.window) { NSApp.reply(toApplicationShouldTerminate: $0 == .alertSecondButtonReturn) }
            } else { NSApp.reply(toApplicationShouldTerminate: true) }
        }
        return .terminateLater
    }
    func windowShouldClose(_ sender: NSWindow) -> Bool { NSApp.terminate(nil); return false }
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }
    func applicationWillTerminate(_ notification: Notification) { backend?.terminate() }
}
let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.run()
