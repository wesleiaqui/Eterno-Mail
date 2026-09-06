package platform

import "testing"

func TestDetectWindowDecorationCapabilities(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want WindowDecorationCapabilities
	}{
		{"KDE Wayland", map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-0", "XDG_CURRENT_DESKTOP": "KDE"}, WindowDecorationCapabilities{NativeFallbackRequired: true}},
		{"KDE Plasma variant", map[string]string{"XDG_SESSION_TYPE": " WAYLAND ", "WAYLAND_DISPLAY": "wayland-0", "XDG_CURRENT_DESKTOP": "KDE:Plasma"}, WindowDecorationCapabilities{NativeFallbackRequired: true}},
		{"KDE full session", map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-0", "KDE_FULL_SESSION": "true"}, WindowDecorationCapabilities{NativeFallbackRequired: true}},
		{"GNOME Wayland", map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-0", "XDG_CURRENT_DESKTOP": "GNOME"}, WindowDecorationCapabilities{FramelessSupported: true}},
		{"Wayland without KDE", map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-0"}, WindowDecorationCapabilities{FramelessSupported: true}},
		{"KDE X11", map[string]string{"XDG_SESSION_TYPE": "x11", "WAYLAND_DISPLAY": "wayland-0", "XDG_CURRENT_DESKTOP": "KDE"}, WindowDecorationCapabilities{FramelessSupported: true}},
		{"KDE Wayland forced X11", map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-0", "XDG_CURRENT_DESKTOP": "KDE", "GDK_BACKEND": "x11"}, WindowDecorationCapabilities{FramelessSupported: true}},
		{"KDE Wayland missing display", map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "KDE"}, WindowDecorationCapabilities{FramelessSupported: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectWindowDecorationCapabilities(func(key string) string { return tt.env[key] })
			if got != tt.want {
				t.Errorf("DetectWindowDecorationCapabilities() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestTitleBarModeResolution(t *testing.T) {
	if got := TitleBarPreference(true, true); got != TitleBarModeNative {
		t.Errorf("true,true preference = %q, want native", got)
	}
	for _, tt := range []struct {
		native, show bool
		want         TitleBarMode
	}{{false, true, TitleBarModeCustom}, {true, false, TitleBarModeNative}, {false, false, TitleBarModeDisabled}} {
		if got := TitleBarPreference(tt.native, tt.show); got != tt.want {
			t.Errorf("preference(%v, %v) = %q, want %q", tt.native, tt.show, got, tt.want)
		}
	}
	for _, caps := range []WindowDecorationCapabilities{{FramelessSupported: true}, {NativeFallbackRequired: true}} {
		for _, preference := range []TitleBarMode{TitleBarModeCustom, TitleBarModeNative, TitleBarModeDisabled} {
			want := preference
			if caps.NativeFallbackRequired {
				want = TitleBarModeNative
			}
			if got := ResolveTitleBarMode(preference, caps); got != want {
				t.Errorf("ResolveTitleBarMode(%q, %+v) = %q, want %q", preference, caps, got, want)
			}
		}
	}
}
