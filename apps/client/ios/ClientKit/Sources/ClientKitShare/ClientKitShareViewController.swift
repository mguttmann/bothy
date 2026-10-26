// SPDX-License-Identifier: Apache-2.0

import Foundation
import SwiftUI

#if os(iOS)
    import UIKit

    /// The principal class of the iOS Share Extension (brief M5-06; ADR 0056 item 2): it hosts
    /// the SwiftUI sheet of `ShareSheetFactory` and ends the extension request. The extension
    /// target subclasses it without adding anything, so all logic stays testable in ClientKit.
    open class ClientKitShareViewController: UIViewController {
        /// Builds the sheet over the input items and fills the view with it.
        override open func viewDidLoad() {
            super.viewDidLoad()
            // A swipe down would end the extension without completeRequest or cancelRequest: a
            // running send would be cut off without an inbox entry and the staged copies would
            // stay. Cancel is the way out, and it is disabled while sending.
            isModalInPresentation = true
            let bundle = Bundle(for: type(of: self))
            let content = ShareSheetFactory.content(
                bundle: bundle, inputItems: extensionContext?.inputItems ?? [], finish: { [weak self] in self?.finish($0) })
            let host = UIHostingController(rootView: ShareSheetRoot(content: content, strings: .fromBundle(bundle)))
            addChild(host)
            host.view.translatesAutoresizingMaskIntoConstraints = false
            view.addSubview(host.view)
            NSLayoutConstraint.activate([
                host.view.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                host.view.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                host.view.topAnchor.constraint(equalTo: view.topAnchor),
                host.view.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            ])
            host.didMove(toParent: self)
        }

        private func finish(_ how: ShareViewModel.Finish) {
            switch how {
            case .completed:
                extensionContext?.completeRequest(returningItems: nil, completionHandler: nil)
            case .cancelled:
                extensionContext?.cancelRequest(withError: NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
            }
        }
    }

#elseif os(macOS)
    import AppKit

    /// The principal class of the macOS Share Extension (brief M5-06; ADR 0056 item 2): it hosts
    /// the SwiftUI sheet of `ShareSheetFactory` as its view and ends the extension request. The
    /// extension target subclasses it without adding anything; there is no nib.
    open class ClientKitShareViewController: NSViewController {
        /// The size of the sheet the system shows; a layout value, not a limit.
        public static let sheetSize = NSSize(width: 420, height: 560)

        /// Builds the sheet over the input items as the controller's view.
        override open func loadView() {
            let bundle = Bundle(for: type(of: self))
            let content = ShareSheetFactory.content(
                bundle: bundle, inputItems: extensionContext?.inputItems ?? [], finish: { [weak self] in self?.finish($0) })
            let host = NSHostingView(rootView: ShareSheetRoot(content: content, strings: .fromBundle(bundle)))
            host.frame = NSRect(origin: .zero, size: Self.sheetSize)
            view = host
            preferredContentSize = Self.sheetSize
        }

        private func finish(_ how: ShareViewModel.Finish) {
            switch how {
            case .completed:
                extensionContext?.completeRequest(returningItems: nil, completionHandler: nil)
            case .cancelled:
                extensionContext?.cancelRequest(withError: NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
            }
        }
    }
#endif
