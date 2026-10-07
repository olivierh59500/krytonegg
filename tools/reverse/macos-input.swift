import AppKit
import CoreGraphics
import Foundation

// Drive the installed emulator with normal keyboard and mouse events.
// This utility is only used for comparing the original Amiga game.
let args = CommandLine.arguments
if args.count < 2 { exit(1) }
let action = args[1]
if action == "hide" {
    let process = pid_t(args[2])!
    print(NSRunningApplication(processIdentifier: process)?.hide() ?? false)
} else if action == "windows" {
    let windows = CGWindowListCopyWindowInfo([.optionAll, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    for window in windows {
        let owner = window[kCGWindowOwnerName as String] as? String ?? ""
        if owner.contains("FS-UAE") {
            print(window)
        }
    }
} else if action == "key" {
    let code = CGKeyCode(args[2])!
    let process = pid_t(args[3])!
    CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: true)?.postToPid(process)
    Thread.sleep(forTimeInterval: 0.08)
    CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: false)?.postToPid(process)
} else if action == "click" {
    let point = CGPoint(x: Double(args[2])!, y: Double(args[3])!)
    let process = pid_t(args[4])!
    CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: point, mouseButton: .left)?.postToPid(process)
    Thread.sleep(forTimeInterval: 0.1)
    CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left)?.postToPid(process)
    Thread.sleep(forTimeInterval: 0.1)
    CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)?.postToPid(process)
}
