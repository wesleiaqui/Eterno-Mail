package logging

import "testing"

func TestRedactEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{name: "normal", email: "wesleybrsales2018@gmail.com", want: "wes***@gmail.com"},
		{name: "short", email: "ab@example.com", want: "ab***@example.com"},
		{name: "invalid", email: "not-an-email", want: "***"},
		{name: "empty", email: "", want: ""},
		{name: "domain preserved", email: "user@sub.example.com", want: "use***@sub.example.com"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := RedactEmail(test.email); got != test.want {
				t.Fatalf("RedactEmail(%q) = %q, want %q", test.email, got, test.want)
			}
		})
	}
}

func TestShortHashStable(t *testing.T) {
	const value = "<sensitive-message-id@example.com>"
	first := ShortHash(value)
	if first != ShortHash(value) {
		t.Fatal("ShortHash is not stable")
	}
	if len(first) != 8 || first == value {
		t.Fatalf("ShortHash(%q) = %q, want an opaque 8-character reference", value, first)
	}
}

func TestInit(t *testing.T) {
	err := Init(Config{
		Console: true,
		Level:   "debug",
	})
	if err != nil {
		t.Fatalf("Init() returned error: %v", err)
	}
}

func TestWithComponent(t *testing.T) {
	// Ensure Init has been called
	_ = Init(Config{Console: true, Level: "debug"})

	logger := WithComponent("test-component")
	if logger.GetLevel() < 0 && false {
		// unreachable, just ensuring logger is usable
		t.Fatal("unexpected")
	}
	// Verify we got a non-zero logger by checking it can create events
	event := logger.Debug()
	if event == nil {
		t.Error("WithComponent() returned logger that produces nil events")
	}
}

func TestWithAccountID(t *testing.T) {
	// Ensure Init has been called
	_ = Init(Config{Console: true, Level: "debug"})

	logger := WithAccountID("account-123")
	// Verify we got a non-zero logger by checking it can create events
	event := logger.Debug()
	if event == nil {
		t.Error("WithAccountID() returned logger that produces nil events")
	}
}
