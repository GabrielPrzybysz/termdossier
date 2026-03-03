package preprocess

import "testing"

func TestStripANSI_Colors(t *testing.T) {
	input := "\x1b[31mERROR\x1b[0m: something failed"
	want := "ERROR: something failed"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI colors: got %q, want %q", got, want)
	}
}

func TestStripANSI_CursorMovement(t *testing.T) {
	input := "\x1b[2J\x1b[HHello"
	want := "Hello"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI cursor: got %q, want %q", got, want)
	}
}

func TestStripANSI_OSC(t *testing.T) {
	input := "\x1b]0;my-title\x07some text"
	want := "some text"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI OSC: got %q, want %q", got, want)
	}
}

func TestStripANSI_CarriageReturn(t *testing.T) {
	// Simulates a progress bar that overwrites itself
	input := "Progress: 50%\rProgress: 100%"
	want := "Progress: 100%"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI CR: got %q, want %q", got, want)
	}
}

func TestStripANSI_MultiLineWithCR(t *testing.T) {
	input := "line1\roverwritten1\nline2\roverwritten2"
	want := "overwritten1\noverwritten2"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI multi-CR: got %q, want %q", got, want)
	}
}

func TestStripANSI_NoEscapes(t *testing.T) {
	input := "plain text with no escapes"
	if got := StripANSI(input); got != input {
		t.Errorf("StripANSI no-op: got %q, want %q", got, input)
	}
}

func TestStripANSI_BoldAndColors(t *testing.T) {
	input := "\x1b[1;32mPASSED\x1b[0m 5 tests"
	want := "PASSED 5 tests"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI bold+color: got %q, want %q", got, want)
	}
}

func TestStripANSI_CRLF(t *testing.T) {
	// Terminal output commonly uses \r\n line endings. The \r should not
	// cause content to be discarded — only mid-line \r (progress bars) should.
	input := "Starting Nmap 7.92\r\nNmap scan report for 10.10.10.1\r\n22/tcp open ssh\r\n"
	want := "Starting Nmap 7.92\nNmap scan report for 10.10.10.1\n22/tcp open ssh\n"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI CRLF:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestStripANSI_CRLF_WithProgressBar(t *testing.T) {
	// Mix of CRLF line endings and mid-line \r progress bar overwrites
	input := "line1\r\nProgress: 50%\rProgress: 100%\r\nline3\r\n"
	want := "line1\nProgress: 100%\nline3\n"
	if got := StripANSI(input); got != want {
		t.Errorf("StripANSI CRLF+progress:\n  got:  %q\n  want: %q", got, want)
	}
}
