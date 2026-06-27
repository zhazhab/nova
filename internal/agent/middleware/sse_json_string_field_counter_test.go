package middleware

import "testing"

func TestJSONStringFieldCounterCountsStreamingContentChars(t *testing.T) {
	counter := newJSONStringFieldCounter("content")

	if got := counter.Write(`{"file_path":"chapters/ch01.md","content":"第一`); got != len([]rune("第一")) {
		t.Fatalf("first chunk count = %d, want %d", got, len([]rune("第一")))
	}
	if got := counter.Write(`行\n第二行"`); got != len([]rune("行\n第二行")) {
		t.Fatalf("second chunk count = %d, want %d", got, len([]rune("行\n第二行")))
	}
	if got := counter.Write(`,"other":"ignored"}`); got != 0 {
		t.Fatalf("trailing chunk count = %d, want 0", got)
	}
}

func TestJSONStringFieldCounterCountsEscapedCharsAcrossChunks(t *testing.T) {
	counter := newJSONStringFieldCounter("content")

	chunks := []string{
		`{"content":"A\`,
		`nB\"C\u`,
		`4E2D"}`,
	}
	got := 0
	for _, chunk := range chunks {
		got += counter.Write(chunk)
	}
	if want := len([]rune("A\nB\"C中")); got != want {
		t.Fatalf("escaped content count = %d, want %d", got, want)
	}
}
