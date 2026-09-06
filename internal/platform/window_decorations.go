package platform

import (
	"os"
	"strings"
)

// TitleBarMode is the user's requested or the window's effective decoration mode.
type TitleBarMode string

const (
	TitleBarModeCustom   TitleBarMode = "custom"
	TitleBarModeNative   TitleBarMode = "native"
	TitleBarModeDisabled TitleBarMode = "disabled"
)

// WindowDecorationCapabilities describes whether this session can honor a
// frameless GTK window. KDE Plasma native Wayland is the only environment
// currently known to require a native-decoration fallback.
type WindowDecorationCapabilities struct {
	FramelessSupported     bool `json:"frameless_supported"`
	NativeFallbackRequired bool `json:"native_fallback_required"`
}

// WindowDecorationStatus is shared with the frontend so it renders the same
// decision that was used to create the native window.
type WindowDecorationStatus struct {
	PreferenceMode         TitleBarMode `json:"preference_mode"`
	EffectiveMode          TitleBarMode `json:"effective_mode"`
	FramelessSupported     bool         `json:"frameless_supported"`
	NativeFallbackRequired bool         `json:"native_fallback_required"`
}

// TitleBarPreference converts the legacy persisted booleans into one explicit
// mode. A corrupt true/true pair resolves to native: showing both decorations
// is unsafe, while native is the least surprising fallback.
func TitleBarPreference(nativeTitleBar, showTitleBar bool) TitleBarMode {
	if nativeTitleBar {
		return TitleBarModeNative
	}
	if showTitleBar {
		return TitleBarModeCustom
	}
	return TitleBarModeDisabled
}

// ResolveTitleBarMode applies session capabilities without changing the saved
// user preference.
func ResolveTitleBarMode(preference TitleBarMode, caps WindowDecorationCapabilities) TitleBarMode {
	if caps.NativeFallbackRequired {
		return TitleBarModeNative
	}
	return preference
}

// DetectWindowDecorationCapabilities detects only the confirmed KDE Plasma
// native-Wayland case. DISPLAY is intentionally ignored because it may expose
// XWayland alongside a native Wayland GTK session.
func DetectWindowDecorationCapabilities(getenv func(string) string) WindowDecorationCapabilities {
	sessionType := strings.ToLower(strings.TrimSpace(getenv("XDG_SESSION_TYPE")))
	waylandDisplay := strings.TrimSpace(getenv("WAYLAND_DISPLAY"))
	desktop := strings.ToUpper(strings.TrimSpace(getenv("XDG_CURRENT_DESKTOP")))
	kdeFullSession := strings.EqualFold(strings.TrimSpace(getenv("KDE_FULL_SESSION")), "true")

	isKDE := strings.Contains(desktop, "KDE") || kdeFullSession
	isNativeWayland := sessionType == "wayland" && waylandDisplay != "" && !gdkBackendForcesX11(getenv("GDK_BACKEND"))
	if isKDE && isNativeWayland {
		return WindowDecorationCapabilities{NativeFallbackRequired: true}
	}
	return WindowDecorationCapabilities{FramelessSupported: true}
}

// CurrentWindowDecorationCapabilities is the production environment wrapper.
func CurrentWindowDecorationCapabilities() WindowDecorationCapabilities {
	return DetectWindowDecorationCapabilities(os.Getenv)
}

func gdkBackendForcesX11(value string) bool {
	backend := strings.TrimSpace(strings.ToLower(value))
	if backend == "" {
		return false
	}
	// GDK_BACKEND accepts an ordered, comma-separated backend list. Only an
	// explicit first-choice x11 backend is treated as a forced X11 session.
	return strings.TrimSpace(strings.Split(backend, ",")[0]) == "x11"
}

// NewWindowDecorationStatus combines the persisted preference and the
// effective mode chosen for this window.
func NewWindowDecorationStatus(preference, effective TitleBarMode, caps WindowDecorationCapabilities) WindowDecorationStatus {
	return WindowDecorationStatus{
		PreferenceMode:         preference,
		EffectiveMode:          effective,
		FramelessSupported:     caps.FramelessSupported,
		NativeFallbackRequired: caps.NativeFallbackRequired,
	}
}
