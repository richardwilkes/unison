package x11

import (
	"bytes"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

// TestClipboardRoundTrip exercises the clipboard against a live X server by transferring data between two separate
// connections. Since it requires an X server and temporarily replaces the contents of the user's clipboard, it only
// runs when UNISON_X11_CLIPBOARD_TEST is set.
func TestClipboardRoundTrip(t *testing.T) {
	if os.Getenv("UNISON_X11_CLIPBOARD_TEST") == "" {
		t.Skip("set UNISON_X11_CLIPBOARD_TEST to run this test against a live X server")
	}
	setter, err := NewConn()
	if err != nil {
		t.Skipf("no X server available: %v", err)
	}
	getter, err := NewConn()
	if err != nil {
		t.Fatalf("second connection failed: %v", err)
	}

	// Preserve whatever text is currently on the user's clipboard so it can be restored at the end.
	savedText := getter.GetClipboardBytes(uti.UTF8PlainText.UTI)

	// Serve selection requests on the setter connection, as a running app's event loop would.
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			ev := setter.WaitEventsUntil(func(e Event) bool {
				_, ok := e.(*SelectionRequestEvent)
				return ok
			}, 50*time.Millisecond)
			if sre, ok := ev.(*SelectionRequestEvent); ok {
				setter.RespondToSelectionRequest(sre)
			}
		}
	}()

	text := "Héllo, wörld! 🚀"
	blob := make([]byte, 1024*1024) // large enough to exercise chunked ChangeProperty
	for i := range blob {
		blob[i] = byte(i)
	}
	setter.SetClipboardData(
		drag.Data{Type: uti.UTF8PlainText, Data: []byte(text)},
		drag.Data{Type: uti.PNG, Data: blob},
	)
	setter.Sync()

	types := getter.ClipboardDataTypes()
	t.Logf("available types: %v", types)
	if !slices.Contains(types, uti.UTF8PlainText.UTI) {
		t.Errorf("expected %q in available types", uti.UTF8PlainText.UTI)
	}
	if !slices.Contains(types, uti.PNG.UTI) {
		t.Errorf("expected %q in available types", uti.PNG.UTI)
	}

	if got := getter.GetClipboardBytes(uti.UTF8PlainText.UTI); string(got) != text {
		t.Errorf("text round trip failed: got %q", string(got))
	}
	if got := getter.GetClipboardBytes(uti.PNG.UTI); !bytes.Equal(got, blob) {
		t.Errorf("binary round trip failed: got %d bytes", len(got))
	}

	// Restore the previous clipboard contents.
	if len(savedText) != 0 {
		setter.SetClipboardData(drag.Data{Type: uti.UTF8PlainText, Data: savedText})
	}
	close(stop)
	<-done // Stop serving requests concurrently before Close, since Close pumps events itself
	setter.Close()
	getter.Close()
}
