import AppKit
import ApplicationServices
import Foundation
import JangolovaMacCore
import SafariServices

@main
struct JangolovaMacExtensionCommand {
    static func main() {
        let application = NSApplication.shared
        let delegate = MenuBarDelegate()
        application.delegate = delegate
        application.setActivationPolicy(.accessory)
        application.run()
    }
}

@MainActor
final class MenuBarDelegate: NSObject, NSApplicationDelegate {
    private let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let runtime = ManagedRuntimeController()
    private let catalog = UserscriptCatalog(appGroup: ProcessInfo.processInfo.environment["JANGOLOVA_APP_GROUP"])
    private var status = ManagedRuntimeStatus.stopped

    func applicationDidFinishLaunching(_ notification: Notification) {
        statusItem.button?.title = "J"
        rebuildMenu()
    }

    private func rebuildMenu() {
        let menu = NSMenu()
        let heading = NSMenuItem(title: "Jangolova for macOS", action: nil, keyEquivalent: "")
        heading.isEnabled = false
        menu.addItem(heading)
        menu.addItem(NSMenuItem(title: runtimeLabel, action: nil, keyEquivalent: ""))
        menu.addItem(NSMenuItem(title: accessibilityLabel, action: nil, keyEquivalent: ""))
        menu.addItem(.separator())
        let managed = NSMenuItem(title: status == .stopped ? "Start Managed Cymonkey" : "Stop Managed Cymonkey", action: #selector(toggleRuntime), keyEquivalent: "")
        managed.target = self
        menu.addItem(managed)
        let userscripts = NSMenuItem(title: "Userscripts", action: nil, keyEquivalent: "")
        userscripts.submenu = NSMenu(title: "Userscripts")
        userscripts.submenu?.addItem(NSMenuItem(title: "Loading…", action: nil, keyEquivalent: ""))
        menu.addItem(userscripts)
        let safari = NSMenuItem(title: "Safari Extension Preferences…", action: #selector(openSafariPreferences), keyEquivalent: "")
        safari.target = self
        menu.addItem(safari)
        menu.addItem(.separator())
        let quit = NSMenuItem(title: "Quit Jangolova", action: #selector(quit), keyEquivalent: "q")
        quit.target = self
        menu.addItem(quit)
        statusItem.menu = menu
        Task { await populateUserscripts(userscripts.submenu) }
    }

    private var runtimeLabel: String {
        switch status {
        case .stopped: return "Cymonkey: Stopped"
        case .connecting: return "Cymonkey: Connecting"
        case .connected: return "Cymonkey: Connected"
        case .failed: return "Cymonkey: Failed"
        }
    }

    private var accessibilityLabel: String {
        "Accessibility: \(AXIsProcessTrusted() ? "Granted" : "Not Granted")"
    }

    @objc private func toggleRuntime() {
        if status != .stopped {
            Task { await runtime.stop() }
            status = .stopped
            rebuildMenu()
            return
        }
        status = .connecting
        rebuildMenu()
        Task {
            do {
                let configuration = try ManagedRuntimeConfiguration(environment: ProcessInfo.processInfo.environment)
                try await runtime.start(configuration: configuration)
                status = await runtime.status
            } catch {
                status = .failed("managed runtime configuration failed")
            }
            rebuildMenu()
        }
    }

    private func populateUserscripts(_ menu: NSMenu?) async {
        guard let menu else { return }
        let values = (try? await catalog.list()) ?? []
        menu.removeAllItems()
        if values.isEmpty {
            let empty = NSMenuItem(title: "No installed userscripts", action: nil, keyEquivalent: "")
            empty.isEnabled = false
            menu.addItem(empty)
            return
        }
        for value in values {
            let item = NSMenuItem(title: "\(value.enabled ? "✓" : "–") \(value.name)", action: nil, keyEquivalent: "")
            item.isEnabled = false
            menu.addItem(item)
        }
        let unavailable = NSMenuItem(title: "Safari execution is unavailable on this build", action: nil, keyEquivalent: "")
        unavailable.isEnabled = false
        menu.addItem(.separator())
        menu.addItem(unavailable)
    }

    @objc private func openSafariPreferences() {
        let identifier = ProcessInfo.processInfo.environment["JANGOLOVA_SAFARI_EXTENSION_ID"] ?? "dev.jangolova.macos.Extension"
        SFSafariApplication.showPreferencesForExtension(withIdentifier: identifier) { _ in }
    }

    @objc private func quit() {
        Task { await runtime.stop() }
        NSApplication.shared.terminate(nil)
    }
}
