<img src="brand/icon.png" alt="Eterno Mail logo" width="160">

# Eterno Mail - An Open Source Lightweight E-Mail Client
Reformulated by Weslei.

### 💰 Sponsorship
---

Donations help keep Eterno Mail updated and accelerate the development of new features for the application.

[![Donate via Pixgg](https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=https%3A%2F%2Fpixgg.com%2Fweslei)](https://pixgg.com/weslei)

[https://pixgg.com/weslei](https://pixgg.com/weslei)

> **Fork of the original project [Aerion](https://github.com/hkdb/aerion) (v0.3.3) by [hkdb / 3DF](https://3df.io).**
> This fork keeps the original Go module (`github.com/hkdb/aerion`) for internal compatibility and preserves all upstream features, while adding the custom improvements documented below.

![screenshot](docs/ss.png)


### 🔄 Changes from the original project (Aerion)
---

The following changes were made in this fork compared to the original [`hkdb/aerion`](https://github.com/hkdb/aerion) v0.3.3 repository:

#### 🏷️ Rebranding & Visual Identity

| Aspect | Original (Aerion) | This fork (Eterno Mail) |
| :--- | :--- | :--- |
| Product name | Aerion | **Eterno Mail** |
| Application identifier | `io.github.hkdb.Aerion` | `io.github.hkdb.EternoMail` |
| Window title (`main.go`) | `"Aerion"` | `"Eterno Mail"` |
| `wails.json` — `companyName` | `3DF` | `Eterno Mail` |
| `wails.json` — `productName` | `Aerion` | `Eterno Mail` |
| CNAME (GitHub Pages) | `aerion.3df.io` | `app.weslleys.com` |
| Desktop entry (`.desktop`) | `Name=Aerion` | `Name=Eterno Mail` |
| Flatpak metainfo | `<name>Aerion</name>` | `<name>Eterno Mail</name>` |
| Linux install icon | `io.github.hkdb.Aerion.png` | `io.github.hkdb.EternoMail.png` |

> **Note:** The executable name (`aerion`), Go module (`github.com/hkdb/aerion`), and frontend `package.json` (`"name": "aerion"`) were kept unchanged to preserve compatibility with internal imports and the build system.

#### 🎨 UI & Visual Improvements

The interface was significantly redesigned for a more modern and polished experience:

| Before (Aerion original) | After (Eterno Mail) |
| :---: | :---: |
| ![Aerion original](docs/ss-aerion-original.png) | ![Eterno Mail](docs/ss.png) |

Key visual changes:

- **Sidebar redesign:** Replaced the text-based sidebar with a modern, collapsible navigation for Home, Inbox, Calendar, Archived, Blocked, Drafts, Sent and Trash, plus a collapsible folder panel — cleaner and more space-efficient. Choose Compact, Medium or Large under Settings → General; each preset changes its expanded width, text, icons, row heights and spacing together, and the selected size is preserved after collapsing. Home, Inbox, Calendar and folder icons share a consistent visual axis, while inbox disclosure controls remain in their own column. Compose is a clear primary action, with account sync and Settings grouped in a persistent footer for quick access.
- **Focused folder expansion:** Expanding an account list under a folder closes the other folder groups, keeping the navigation readable.
- **Inbox Zero approach:** The email experience was restructured around the Inbox Zero methodology, helping users keep their inbox organized by encouraging archiving, categorizing, and clearing messages efficiently.
- **Inbox category cards:** Inbox messages can be grouped into People, Notifications, News, Commercial and Read cards. Each card offers a one-click completion action, expandable content and a refined “Show all” footer that highlights the full card footer on hover.
- **Quick message actions:** Hovering a conversation reveals Done, read/unread and delete controls in a fixed left rail without shifting the sender, avatar or message content.
- **Bulk selection controls:** `Ctrl/Cmd+A` toggles the current list selection; selected messages display a compact action bar for completing, archiving or clearing the selection. Read-state changes offer a visible undo action and `Ctrl/Cmd+Z` restores the last explicit read/unread change.
- **Continuous message loading:** Reaching the bottom of the message list automatically loads the next page, replacing the manual “Load more” button with a subtle loading indicator.
- **Settings redesign:** The settings panel was reformulated with a more intuitive and organized layout, making it easier to navigate and configure accounts, appearance, and preferences.
- **Sender logos in conversation list:** Company logos (Google, Discord, Carrefour, etc.) are now displayed alongside contact avatars in the message list, making it easier to visually identify senders at a glance. Falls back to colored initials when no logo is available.
- **Refined conversation list layout:** Improved spacing, typography, and visual hierarchy in the message list for better readability across density modes.
- **Folder organization:** Folders section with dedicated "Snoozed" and "More" options for better mailbox management.

#### 🐛 Bug Fixes

- **SQLite Foreign Key 787:** Fixed foreign key violation error during message upsert. The logic now preserves the existing message primary key, preventing breakage of attachment references linked by FK.
- **IMAP connection pool:** Implemented strict slot reservation in the connection pool during the handshake phase, preventing overflow when multiple connections are established in parallel (`MaxConnections=3` enforced).
- **`\Noselect` mailboxes:** Added a check to skip the `STATUS` command on mailboxes flagged as `\Noselect` (e.g., the `[Gmail]` folder), preventing sync errors.
- **Sync counters:** Fixed sync counters to report the actual number of stored headers (`failed=0` on success).
- **Folder sync state preservation:** Folder discovery upserts now retain the stored IMAP flags sync mod-sequence; a regression test protects this incremental-sync state.
- **Archive viewer handoff:** Completing, archiving, deleting or marking spam no longer leaves the conversation viewer displaying an empty removed thread while the list reloads and selects the next valid message.

#### 🚀 New Features

- **Sender Logos:** Added a complete caching and display system for company/sender logos in the conversation list:
  - New backend package: `internal/senderlogo` — fetches and stores logos based on the sender's email domain.
  - New frontend store: `frontend/src/lib/stores/senderLogos.svelte.ts` — manages logo state in Svelte.
  - New Wails binding: `app/sender_logo.go` — bridge between backend and frontend.
  - Integration in the `ConversationRow.svelte` component — displays company logo when no contact photo is available.

- **Runtime GTK icon:** Added direct embed of the application icon (`build/appicon.png`) via `//go:embed` in `main.go`, injected as `Icon: appIcon` in the Wails Linux options. This works around icon cache issues in GTK environments (Ubuntu Budgie, Plank dock).

- **Brazilian Portuguese (pt-BR) translation:** Added full localization support for Brazilian Portuguese, making the application accessible to Portuguese-speaking users.

- **Unified special folders across accounts:** Top-level Drafts, Sent, Trash, Starred, Archived, Spam/Blocked and All Mail views aggregate their matching folders from every enabled account while keeping per-account drill-down available. Gmail-style Archive is handled as a virtual archived view rather than assuming Gmail exposes a physical Archive mailbox.

- **Safer draft lifecycle:** Draft editing resolves canonical draft references, preserves the selected sender identity, keeps attachment edits on the same draft, and prevents Save & Close from closing when the latest content has not been persisted successfully. Detached composer windows hand pending draft synchronization back to the long-lived main process so quick closes do not lose or duplicate drafts.

- **Inbox display modes:** The Inbox can be switched between Priority, Categories and Chronological organization from a compact display picker.

- **Built-in release update checks:** Eterno Mail can check the official GitHub Releases feed in the background, notify when a newer release is available, let users check manually from Settings → About, and allow automatic checks to be disabled.

- **KDE Wayland/X11 titlebar compatibility:** Window decoration handling includes a conservative native-titlebar fallback for KDE/Wayland sessions while preserving the custom titlebar where it behaves correctly.

- **Viewer action tooltips:** Conversation toolbar actions show immediate localized labels on hover or keyboard focus, making icon-only controls easier to understand.

#### 🔒 Privacy & Security Improvements

- **PII redaction in logs:** Implemented `RedactEmail` and `ShortHash` functions in the `internal/logging` package that automatically mask:
  - IMAP/SMTP usernames
  - Email addresses
  - Account names
  - `Message-ID` and `Thread-ID`
  - Email subjects
  - Attachment metadata

#### 🖥️ Platform Robustness

- **Linux container support (Distrobox):** Added safe fallback for D-Bus sleep/wake signals in containerized environments. When the system D-Bus bus is unavailable (e.g., inside Distrobox), the application degrades gracefully instead of crashing.
- **Log noise reduction:**
  - Suppressed timezone push warnings when the calendar extension is disabled.
  - Silenced sender logo fetch attempts for invalid domains.
  - Consolidated FTS startup scans into a single query per folder.

#### ⚡ Performance & Observability Improvements

- **Sync instrumentation:** Added detailed phase-timing to the master sync, enabling monitoring of time spent in each phase:
  - `folder sync`, `LIST/LSUB/STATUS`, `message sync`, `pool wait`
- **Explicit CONDSTORE:** Reasons for full reconcile fallback are now explicitly logged: `below_full_reconcile_threshold`, `no_stored_modseq`, `uidvalidity_changed`, `condstore_unavailable`, `periodic_full_sweep`.
- **FTS optimization:** Full-Text Search startup queries consolidated into a single query per folder, reducing startup overhead.

#### 🗃️ Tests & Database

- **Migration tests:** Added automated tests for upgrade paths V31/V42 → current schema, including integrity and foreign key validation.

#### 📦 Dependency Updates

| Dependency | Original Version | Current Version |
| :--- | :--- | :--- |
| `wailsapp/wails/v2` | v2.13.0 | **v2.15.0** |
| `golang.org/x/crypto` | v0.51.0 | **v0.53.0** |
| `golang.org/x/net` | v0.54.0 | **v0.56.0** |
| `golang.org/x/sys` | v0.44.0 | **v0.46.0** |
| `golang.org/x/text` | v0.37.0 | **v0.39.0** |

#### 📝 Documentation Changes

- **README.md:** Full rebranding to "Eterno Mail", added "Reformulated by Weslei" credit.
- **SECURITY.md:** Product name updated to "Eterno Mail".
- **CONTRIBUTING.md:** Product references updated to "Eterno Mail".
- **CHANGELOG.md:** Specific references updated (CASA Tier 2, Flathub, refocus).
- **Makefile:** Linux install/uninstall targets updated to `io.github.hkdb.EternoMail.*`, with cleanup routine for legacy artifacts (`io.github.hkdb.Aerion.*`).


### ❓ Why?
---

I was looking for an email app similar to Spark, but I could not find one that felt right for me, so I built Eterno Mail to get the job done. It obviously does not have even 10% of Spark's features, but I enjoy the focused set of features it offers.


### 👁️‍🗨️ Summary
---

A standalone lightweight e-mail client inspired by [Geary](https://wiki.gnome.org/Apps/Geary) focused on achieving the following goals:

- Resource Efficiency - Minimal CPU, RAM, and battery consumption
- Modern UX - Clean, intuitive interface with dark mode support
- Keyboard & Mouse Friendly - Full keyboard navigation with vim-style shortcuts
- Independence - No dependency on Gnome Online Accounts or other system services
- Search That Works - Basic search that actually finds your emails

### 🖥 OS Support
---

Although Linux is a first-class citizen here, it also works on:

- MacOS
- Windows


### 🪶 Features
---

- Multiple Accounts
- Providers: (🧪 = NOT YET TESTED)
    - Generic IMAP/SMTP
    - GMail
    - Microsoft 365 / Outlook
    - Yahoo 
    - Proton Mail (via Proton Bridge)
    - iCloud Mail 
    - Mailfence
    - Murena
    - Fastmail 🧪
    - Zoho Mail 🧪
    - AOL Mail 🧪
    - GMX Mail 
    - Mail.com 🧪
    - Mailbox.org
- Unified Inbox (Color Code Accounts)
- Unified Special Folders (Drafts, Sent, Trash, Starred, Archived, Spam/Blocked and All Mail)
- Inbox Display Modes (Priority, Categories and Chronological)
- Draft Safety with Canonical Draft Editing, Sender Identity Persistence and Detached Composer Recovery
- Automatic GitHub Release Update Notifications and Manual Update Checks
- Conversation Threads
- Basic Removal of Tracking Elements in Mail Content
- WYSIWYG Detachable Composer ([TipTap Editor](https://github.com/ueberdosis/tiptap))
- WYSIWYG Signatures ([TipTap Editor](https://github.com/ueberdosis/tiptap))
- CardDav/Google/Microsoft Contact Sync for auto-complete
- Basic Local and IMAP Search
- Notification that brings focus to the e-mail when clicked
- Auto-Sync when system wakes from suspend
- Multiple color themes (More to come...)
- PGP & S/MIME support
- 1st party extension system with the following shipped:
    - Calendar (ALPHA) - Disabled by Default
    - Contacts (ALPHA) - Disabled by Default
- Custom oAuth2 support for generic IMAP, SMTP, CarDAV, and CalDAV (Tested w/ [Stalwart](https://stalw.art))
- [Keyboard Shortcuts](docs/KEYBOARD_SHORTCUTS.md)


### 🚀 Installation
---

- [Download Eterno Mail](https://github.com/wesleiaqui/EternoMail/releases)


### 📖 Documentation
---

- [Eterno Mail Documentation](https://docs.weslleys.com/)


### ⚗️ Tech Stack
---

This application was built with [Wails](https://wails.io) + [Svelte](https://svelte.dev/).

Transparency Disclaimer: This project leaveraged Claude models heavily to implement.

Eterno Mail is CASA Tier 2 Certified by Google's preferred [authorized assessor](https://appdefensealliance.dev/casa/casa-assessors): [TAC Security](https://tacsecurity.com/)


### 🗞 News & Announcments
---

- 2026-03-11 ~ Microsoft Verified
- 2026-04-16 ~ CASA Tier 2 Certified
- 2025-04-25 ~ Google Verified
- 2026-06-22 ~ Extension System + Contacts + Calendar (ALPHA)


### 🧑🏻‍💻 Roadmap
---

Confirmed future features:

- Post quantum ready encryption
- Add Mailfence and Startmail templates in add account flow for easier setup

Potential features in the future:

- Customizable shortcut keys
- Advance Search
- AI Assisted Composition (Ollama)


### 🏷️ Changelog
---

[CHANGELOG.md](CHANGELOG.md)

### 🔨 Contributing
---

Please see [CONTRIBUTING.md](CONTRIBUTING.md)


### 🙏 Issue Contributors
---

Eterno Mail is largely driven by community feedback. Big thanks to the following non-exhaustive list of contributors who submitted issues which led to meaningful improvements we all now enjoy. This project would not be the same without them!

<table>
  <tr>
    <td align="center">
      <a href="https://github.com/keithvassallomt">
        <img src="https://github.com/keithvassallomt.png" width="80"><br>
        <sub><b>keithvassallomt</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Akeithvassallomt+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>21 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/shahiljain">
        <img src="https://github.com/shahiljain.png" width="80"><br>
        <sub><b>shahiljain</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ashahiljain+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>7 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/The-Nyla">
        <img src="https://github.com/The-Nyla.png" width="80"><br>
        <sub><b>The-Nyla</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AThe-Nyla+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>6 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/lorduskordus">
        <img src="https://github.com/lorduskordus.png" width="80"><br>
        <sub><b>lorduskordus</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Alorduskordus+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>4 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/arnauda-gh">
        <img src="https://github.com/arnauda-gh.png" width="80"><br>
        <sub><b>arnauda-gh</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aarnauda-gh+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>4 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/isorropisths">
        <img src="https://github.com/isorropisths.png" width="80"><br>
        <sub><b>isorropisths</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aisorropisths+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>4 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/h-bernardo">
        <img src="https://github.com/h-bernardo.png" width="80"><br>
        <sub><b>h-bernardo</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ah-bernardo+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>3 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/slade991">
        <img src="https://github.com/slade991.png" width="80"><br>
        <sub><b>slade991</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aslade991+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>3 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/urdh">
        <img src="https://github.com/urdh.png" width="80"><br>
        <sub><b>urdh</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aurdh+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/clintre">
        <img src="https://github.com/clintre.png" width="80"><br>
        <sub><b>clintre</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aclintre+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/jeremy-niles">
        <img src="https://github.com/jeremy-niles.png" width="80"><br>
        <sub><b>jeremy-niles</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ajeremy-niles+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/onny">
        <img src="https://github.com/onny.png" width="80"><br>
        <sub><b>onny</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aonny+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/Infiniti151">
        <img src="https://github.com/Infiniti151.png" width="80"><br>
        <sub><b>Infiniti151</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AInfiniti151+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/SonGokuSSJ">
        <img src="https://github.com/SonGokuSSJ.png" width="80"><br>
        <sub><b>SonGokuSSJ</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3ASonGokuSSJ+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/pj398">
        <img src="https://github.com/pj398.png" width="80"><br>
        <sub><b>pj398</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Apj398+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>2 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/lawmanuk">
        <img src="https://github.com/lawmanuk.png" width="80"><br>
        <sub><b>lawmanuk</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Alawmanuk+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/BurningTheSky">
        <img src="https://github.com/BurningTheSky.png" width="80"><br>
        <sub><b>BurningTheSky</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3ABurningTheSky+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/az2oo1">
        <img src="https://github.com/az2oo1.png" width="80"><br>
        <sub><b>az2oo1</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aaz2oo1+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/kevbrowngb">
        <img src="https://github.com/kevbrowngb.png" width="80"><br>
        <sub><b>kevbrowngb</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Akevbrowngb+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/MMachado05">
        <img src="https://github.com/MMachado05.png" width="80"><br>
        <sub><b>MMachado05</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AMMachado05+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/CDrummond">
        <img src="https://github.com/CDrummond.png" width="80"><br>
        <sub><b>CDrummond</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3ACDrummond+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/tyderian1978">
        <img src="https://github.com/tyderian1978.png" width="80"><br>
        <sub><b>tyderian1978</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Atyderian1978+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Amjad50">
        <img src="https://github.com/Amjad50.png" width="80"><br>
        <sub><b>Amjad50</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AAmjad50+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/woolkingx">
        <img src="https://github.com/woolkingx.png" width="80"><br>
        <sub><b>woolkingx</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Awoolkingx+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/gpompeo">
        <img src="https://github.com/gpompeo.png" width="80"><br>
        <sub><b>gpompeo</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Agpompeo+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/in4matix">
        <img src="https://github.com/in4matix.png" width="80"><br>
        <sub><b>in4matix</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ain4matix+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/etbe">
        <img src="https://github.com/etbe.png" width="80"><br>
        <sub><b>etbe</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aetbe+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/kimusan">
        <img src="https://github.com/kimusan.png" width="80"><br>
        <sub><b>kimusan</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Akimusan+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/extremeleadprogram">
        <img src="https://github.com/extremeleadprogram.png" width="80"><br>
        <sub><b>extremeleadprogram</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aextremeleadprogram+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Gerti1972">
        <img src="https://github.com/Gerti1972.png" width="80"><br>
        <sub><b>Gerti1972</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AGerti1972+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/alfureu">
        <img src="https://github.com/alfureu.png" width="80"><br>
        <sub><b>alfureu</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aalfureu+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Kartoffelbauer">
        <img src="https://github.com/Kartoffelbauer.png" width="80"><br>
        <sub><b>Kartoffelbauer</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AKartoffelbauer+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/AdamHasma">
        <img src="https://github.com/AdamHasma.png" width="80"><br>
        <sub><b>AdamHasma</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AAdamHasma+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/miguelmaiquez">
        <img src="https://github.com/miguelmaiquez.png" width="80"><br>
        <sub><b>miguelmaiquez</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Amiguelmaiquez+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/cvdmint">
        <img src="https://github.com/cvdmint.png" width="80"><br>
        <sub><b>cvdmint</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Acvdmint+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/ilyonfly">
        <img src="https://github.com/ilyonfly.png" width="80"><br>
        <sub><b>ilyonfly</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ailyonfly+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/Dragonsong3k">
        <img src="https://github.com/Dragonsong3k.png" width="80"><br>
        <sub><b>Dragonsong3k</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3ADragonsong3k+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/martink1337">
        <img src="https://github.com/martink1337.png" width="80"><br>
        <sub><b>martink1337</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Amartink1337+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/bjacobs39">
        <img src="https://github.com/bjacobs39.png" width="80"><br>
        <sub><b>bjacobs39</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Abjacobs39+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/rdmrtn">
        <img src="https://github.com/rdmrtn.png" width="80"><br>
        <sub><b>rdmrtn</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ardmrtn+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/ymilly">
        <img src="https://github.com/ymilly.png" width="80"><br>
        <sub><b>ymilly</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aymilly+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/diederikh">
        <img src="https://github.com/diederikh.png" width="80"><br>
        <sub><b>diederikh</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Adiederikh+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/CreateWebNZ">
        <img src="https://github.com/CreateWebNZ.png" width="80"><br>
        <sub><b>CreateWebNZ</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3ACreateWebNZ+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/ai-mind">
        <img src="https://github.com/ai-mind.png" width="80"><br>
        <sub><b>ai-mind</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aai-mind+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/arodier">
        <img src="https://github.com/arodier.png" width="80"><br>
        <sub><b>arodier</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aarodier+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/ajclarkin">
        <img src="https://github.com/ajclarkin.png" width="80"><br>
        <sub><b>ajclarkin</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aajclarkin+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/idomusha">
        <img src="https://github.com/idomusha.png" width="80"><br>
        <sub><b>idomusha</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aidomusha+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/xJayMorex">
        <img src="https://github.com/xJayMorex.png" width="80"><br>
        <sub><b>xJayMorex</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AxJayMorex+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/hopeless65">
        <img src="https://github.com/hopeless65.png" width="80"><br>
        <sub><b>hopeless65</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ahopeless65+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/TheEmi">
        <img src="https://github.com/TheEmi.png" width="80"><br>
        <sub><b>TheEmi</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3ATheEmi+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/SantanuDatta">
        <img src="https://github.com/SantanuDatta.png" width="80"><br>
        <sub><b>SantanuDatta</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3ASantanuDatta+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/pascollin">
        <img src="https://github.com/pascollin.png" width="80"><br>
        <sub><b>pascollin</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Apascollin+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/DerKempter">
        <img src="https://github.com/DerKempter.png" width="80"><br>
        <sub><b>DerKempter</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3ADerKempter+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/initsuj">
        <img src="https://github.com/initsuj.png" width="80"><br>
        <sub><b>initsuj</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ainitsuj+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/mff47025">
        <img src="https://github.com/mff47025.png" width="80"><br>
        <sub><b>mff47025</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Amff47025+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/justin-lavelle">
        <img src="https://github.com/justin-lavelle.png" width="80"><br>
        <sub><b>justin-lavelle</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ajustin-lavelle+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/neodyme01">
        <img src="https://github.com/neodyme01.png" width="80"><br>
        <sub><b>neodyme01</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Aneodyme01+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/rdannenbring">
        <img src="https://github.com/rdannenbring.png" width="80"><br>
        <sub><b>rdannenbring</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ardannenbring+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/IvAzuara">
        <img src="https://github.com/IvAzuara.png" width="80"><br>
        <sub><b>IvAzuara</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AIvAzuara+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/robert0815">
        <img src="https://github.com/robert0815.png" width="80"><br>
        <sub><b>robert0815</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Arobert0815+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/srabette">
        <img src="https://github.com/srabette.png" width="80"><br>
        <sub><b>srabette</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Asrabette+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/frian92">
        <img src="https://github.com/frian92.png" width="80"><br>
        <sub><b>frian92</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Afrian92+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Arvid-ctrl">
        <img src="https://github.com/Arvid-ctrl.png" width="80"><br>
        <sub><b>Arvid-ctrl</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AArvid-ctrl+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Olivetti">
        <img src="https://github.com/Olivetti.png" width="80"><br>
        <sub><b>Olivetti</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AOlivetti+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/budfy">
        <img src="https://github.com/budfy.png" width="80"><br>
        <sub><b>budfy</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Abudfy+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/piresio">
        <img src="https://github.com/piresio.png" width="80"><br>
        <sub><b>piresio</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Apiresio+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/makzumi">
        <img src="https://github.com/makzumi.png" width="80"><br>
        <sub><b>makzumi</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Amakzumi+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/PeterKDunn">
        <img src="https://github.com/PeterKDunn.png" width="80"><br>
        <sub><b>PeterKDunn</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3APeterKDunn+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/mmzim05">
        <img src="https://github.com/mmzim05.png" width="80"><br>
        <sub><b>mmzim05</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ammzim05+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/m-overlund">
        <img src="https://github.com/m-overlund.png" width="80"><br>
        <sub><b>m-overlund</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Am-overlund+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/Albertmu2">
        <img src="https://github.com/Albertmu2.png" width="80"><br>
        <sub><b>Albertmu2</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AAlbertmu2+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/jakobkukla">
        <img src="https://github.com/jakobkukla.png" width="80"><br>
        <sub><b>jakobkukla</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ajakobkukla+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="https://github.com/HugoTheBoss">
        <img src="https://github.com/HugoTheBoss.png" width="80"><br>
        <sub><b>HugoTheBoss</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AHugoTheBoss+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/yuukiw">
        <img src="https://github.com/yuukiw.png" width="80"><br>
        <sub><b>yuukiw</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3Ayuukiw+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/iCzora">
        <img src="https://github.com/iCzora.png" width="80"><br>
        <sub><b>iCzora</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AiCzora+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
    <td align="center">
      <a href="https://github.com/xAptenodyte">
        <img src="https://github.com/xAptenodyte.png" width="80"><br>
        <sub><b>xAptenodyte</b></sub>
      </a><br>
      <a href="https://github.com/hkdb/aerion/issues?q=is%3Aissue+is%3Aclosed+author%3AxAptenodyte+-label%3Ainvalid+-label%3Aquestion+-label%3Aduplicate+-reason%3Aduplicate+-reason%3Anot-planned"><sub>1 closed</sub></a>
    </td>
  </tr>
</table>

*Last Updated: 2026-08-06 | Generated by gitrix


### 🌐 Translation Contributors

Special thanks to translation contributors for making Eterno Mail more accessible:


<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/lorduskordus">
        <img src="https://github.com/lorduskordus.png" width="80"><br>
        <sub><b>lorduskordus</b></sub>
      </a><br>
      <sub>Čeština (cs)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/Gerti1972">
        <img src="https://github.com/Gerti1972.png" width="80"><br>
        <sub><b>Gerti1972</b></sub>
      </a><br>
      <sub>Deutsch (de)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/dev-inside">
        <img src="https://github.com/dev-inside.png" width="80"><br>
        <sub><b>dev-inside</b></sub>
      </a><br>
      <sub>Deutsch (de)</sub><br>
      <sup><small><em>reviewer</em></small></sup>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/StefanSchroeder">
        <img src="https://github.com/StefanSchroeder.png" width="80"><br>
        <sub><b>StefanSchroeder</b></sub>
      </a><br>
      <sub>Deutsch (de)</sub><br>
      <sup><small><em>reviewer</em></small></sup>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/freemans32">
        <img src="https://github.com/freemans32.png" width="80"><br>
        <sub><b>freemans32</b></sub>
      </a><br>
      <sub>Français (fr)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/YacineBoussoufa">
        <img src="https://github.com/YacineBoussoufa.png" width="80"><br>
        <sub><b>YacineBoussoufa</b></sub>
      </a><br>
      <sub>Italiano (it)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/dexblasnoot">
        <img src="https://github.com/dexblasnoot.png" width="80"><br>
        <sub><b>dexblasnoot</b></sub>
      </a><br>
      <sub>Norsk Bokmål (nb)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/aquilapl">
        <img src="https://github.com/aquilapl.png" width="80"><br>
        <sub><b>aquilapl</b></sub>
      </a><br>
      <sub>Polski (pl)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>

<table align="left">
  <tr>
    <td align="center" width="180">
      <a href="https://github.com/0jar">
        <img src="https://github.com/0jar.png" width="80"><br>
        <sub><b>0jar</b></sub>
      </a><br>
      <sub>Tiếng Việt (vi)</sub><br>
      <sub>&nbsp;</sub>
    </td>
  </tr>
</table>


<br clear="left">


### 📑 Terms of Use & Privacy Policy
---

- [Terms of Use](docs/TERMS.md)
- [Privacy Policy](docs/PRIVACY.md)
