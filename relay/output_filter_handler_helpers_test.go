package relay

// Shared helpers for the 4 kids_mode strict output filter handler
// integration tests (compatible_handler_test.go, claude_handler_test.go,
// responses_handler_test.go, gemini_handler_test.go).
//
// Each of those tests previously only asserted the FINAL recorded body and
// gin context keys after the Helper returned. That leaves two design-doc P0
// proof items (lines ~941/943/950/951) unverified:
//
//   - outputFilterWriterSpy wraps the writer UNDERLYING outputFilterWriter
//     and counts every WriteHeader/Write/WriteString/Flush that reaches it.
//     outputFilterWriter buffers the entire response and only forwards to
//     its underlying writer from finalize() (run by restore()), so a spy
//     write count of 0 at probe time proves NO bytes have leaked to the
//     client yet.
//   - outputFilterWrapProbe.snapshot is called from inside the mock upstream
//     httptest.Server handler — i.e. DURING adaptor.DoRequest/DoResponse (or
//     chatCompletionsViaResponses' internal upstream call for the
//     compatible/claude early-return branches), which always runs strictly
//     before wrapOutputFilterWriter's restore()/finalize(). It records
//     whether c.Writer is already *outputFilterWriter and how many writes
//     the spy has seen so far, proving the filter is installed (and nothing
//     has leaked) before the response-producing call completes.

import (
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// outputFilterWriterSpy wraps a gin.ResponseWriter, counting every call that
// reaches it while still forwarding to the underlying writer.
type outputFilterWriterSpy struct {
	gin.ResponseWriter

	mu     sync.Mutex
	writes int
}

func (s *outputFilterWriterSpy) WriteHeader(code int) {
	s.mu.Lock()
	s.writes++
	s.mu.Unlock()
	s.ResponseWriter.WriteHeader(code)
}

func (s *outputFilterWriterSpy) Write(p []byte) (int, error) {
	s.mu.Lock()
	s.writes++
	s.mu.Unlock()
	return s.ResponseWriter.Write(p)
}

func (s *outputFilterWriterSpy) WriteString(p string) (int, error) {
	s.mu.Lock()
	s.writes++
	s.mu.Unlock()
	return s.ResponseWriter.WriteString(p)
}

func (s *outputFilterWriterSpy) Flush() {
	s.mu.Lock()
	s.writes++
	s.mu.Unlock()
	s.ResponseWriter.Flush()
}

func (s *outputFilterWriterSpy) writeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes
}

// outputFilterWrapProbe captures, from inside a mock upstream
// httptest.Server handler, whether c.Writer is already *outputFilterWriter
// and how many writes the spy has seen — both AT THE MOMENT the upstream
// request is being served, which is strictly before
// wrapOutputFilterWriter's restore()/finalize() can have run.
type outputFilterWrapProbe struct {
	captured    atomic.Bool
	wrapped     atomic.Bool
	writesSoFar atomic.Int32
}

func (p *outputFilterWrapProbe) snapshot(c *gin.Context, spy *outputFilterWriterSpy) {
	_, ok := c.Writer.(*outputFilterWriter)
	p.wrapped.Store(ok)
	p.writesSoFar.Store(int32(spy.writeCount()))
	p.captured.Store(true)
}

// assertOutputFilterWrapTiming runs the P0 proof assertions shared by all 4
// handler tests:
//
//   - (b) c.Writer was already *outputFilterWriter when the mock upstream
//     handler ran (wrapOutputFilterWriter installs the filter BEFORE the
//     response-producing call).
//   - (a) zero bytes had reached the underlying writer at that point
//     (outputFilterWriter buffers everything until restore()/finalize()).
//   - after Helper returns, restore() has run: c.Writer is back to the
//     pre-wrap spy (not still *outputFilterWriter), and finalize() flushed
//     the (possibly replaced) body through to the spy.
func assertOutputFilterWrapTiming(t interface {
	Helper()
	Fatal(args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
}, c *gin.Context, spy *outputFilterWriterSpy, probe *outputFilterWrapProbe) {
	t.Helper()

	if !probe.captured.Load() {
		t.Fatal("mock upstream handler was never invoked; outputFilterWrapProbe not captured")
	}
	if !probe.wrapped.Load() {
		t.Error("c.Writer was not *outputFilterWriter at upstream-call time; wrapOutputFilterWriter must install the filter before the response-producing call")
	}
	if n := probe.writesSoFar.Load(); n != 0 {
		t.Errorf("underlying writer received %d write(s) before restore()/finalize() ran; want 0", n)
	}

	if _, stillWrapped := c.Writer.(*outputFilterWriter); stillWrapped {
		t.Error("c.Writer is still *outputFilterWriter after Helper returned; restore() did not run")
	}
	if c.Writer != spy {
		t.Errorf("c.Writer = %T after Helper returned, want the original spy writer restored by restore()", c.Writer)
	}
	if spy.writeCount() == 0 {
		t.Error("underlying writer received no writes; finalize() never flushed the response")
	}
}
