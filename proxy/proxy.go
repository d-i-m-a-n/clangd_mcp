// Package proxy implements a transparent LSP proxy with an embedded MCP server.
//
// Architecture:
//
//	IDE ──stdio──► Proxy ──stdio──► clangd
//	                 │
//	            HTTP/SSE :port (MCP)
//	                 │
//	         AI Agent (Claude, etc.)
//
// Request ID multiplexing:
//   - Requests from IDE get IDs remapped to "Q<n>" (string) before forwarding to clangd.
//     Responses are routed back to IDE stdout with the original ID restored.
//   - Requests from MCP tools get IDs "M<n>". Responses are sent to a waiting channel.
//   - Notifications pass through without ID remapping.
//   - Server-initiated requests from clangd are forwarded to IDE as-is.
//     IDE's responses (which have IDs we don't own) are forwarded to clangd directly.
package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"clangd-mcp/lsp"
)

// Config holds proxy configuration.
type Config struct {
	MCPPort   int    // HTTP port for MCP SSE server
	Workspace string // workspace root directory
}

type reqResult struct {
	result json.RawMessage
	lspErr *lsp.ResponseError
}

type pendingEntry struct {
	method     string          // LSP method (used to detect initialize)
	originalID json.RawMessage // original ID (for IDE response restoration)
	resultCh   chan reqResult   // nil for IDE entries; non-nil for MCP entries
}

// Proxy manages bidirectional LSP message forwarding and MCP integration.
type Proxy struct {
	cfg Config

	// LSP server process
	cmd   *exec.Cmd
	lspIn io.WriteCloser
	lspMu sync.Mutex // serializes writes to lspIn

	// Request tracking
	counter atomic.Int64
	mu      sync.Mutex
	pending map[string]*pendingEntry // remapped ID string → entry

	// Clangd capabilities (captured from first initialize response)
	capsOnce sync.Once
	capsCh   chan struct{}
	caps     json.RawMessage

	// stdout serialization (proxy writes IDE responses on stdout)
	outMu sync.Mutex
}

// New creates a new Proxy instance.
func New(cfg Config) *Proxy {
	return &Proxy{
		cfg:     cfg,
		pending: make(map[string]*pendingEntry),
		capsCh:  make(chan struct{}),
	}
}

// Attach connects the proxy to an already-running LSP server process.
// stdin and stdout are the pipes to/from the process.
func (p *Proxy) Attach(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.Reader) {
	p.cmd = cmd
	p.lspIn = stdin
	go p.readLSP(bufio.NewReader(stdout))
}

// RunQTC reads IDE's LSP traffic from stdin and forwards it to the LSP server.
// Returns when stdin closes.
func (p *Proxy) RunQTC() {
	r := bufio.NewReader(os.Stdin)
	for {
		msg, err := lsp.ReadMessage(r)
		if err != nil {
			if err != io.EOF {
				log.Printf("proxy: IDE read error: %v", err)
			}
			return
		}
		if err := p.handleFromIDE(msg); err != nil {
			log.Printf("proxy: IDE handler: %v", err)
		}
	}
}

// handleFromIDE processes one message arriving from the IDE.
func (p *Proxy) handleFromIDE(msg *lsp.Message) error {
	switch {
	case msg.IsResponse():
		// IDE responding to a server-initiated clangd request (e.g. workspace/configuration).
		// Forward directly without ID remapping.
		return p.writeLSPRaw(msg)

	case msg.IsNotification():
		return p.writeLSPRaw(msg)

	case msg.IsRequest():
		n := p.counter.Add(1)
		remapped := fmt.Sprintf("Q%d", n)

		p.mu.Lock()
		p.pending[remapped] = &pendingEntry{
			method:     msg.Method,
			originalID: msg.ID,
			resultCh:   nil, // response goes to IDE stdout
		}
		p.mu.Unlock()

		msg.ID = jsonString(remapped)
		return p.writeLSPRaw(msg)
	}
	return nil
}

// readLSP reads messages from the LSP server stdout and dispatches them.
func (p *Proxy) readLSP(r *bufio.Reader) {
	for {
		msg, err := lsp.ReadMessage(r)
		if err != nil {
			if err != io.EOF {
				log.Printf("proxy: LSP read error: %v", err)
			}
			return
		}
		log.Printf("←clangd  method=%q id=%s", msg.Method, truncate(string(msg.ID), 40))
		p.dispatchFromLSP(msg)
	}
}

// dispatchFromLSP routes a message arriving from the LSP server.
func (p *Proxy) dispatchFromLSP(msg *lsp.Message) {
	switch {
	case msg.IsNotification():
		p.writeIDE(msg)

	case msg.IsResponse():
		// Parse the remapped string ID we assigned.
		var idStr string
		if err := json.Unmarshal(msg.ID, &idStr); err != nil {
			// ID is not a string — must be a response to an IDE-issued server request
			// that we forwarded without remapping. Forward back to IDE.
			p.writeIDE(msg)
			return
		}

		p.mu.Lock()
		entry, ok := p.pending[idStr]
		if ok {
			delete(p.pending, idStr)
		}
		p.mu.Unlock()

		if !ok {
			p.writeIDE(msg)
			return
		}

		// Capture capabilities from the first initialize response.
		if entry.method == "initialize" && msg.Result != nil {
			p.capsOnce.Do(func() {
				p.caps = msg.Result
				close(p.capsCh)
			})
		}

		// Restore original ID.
		msg.ID = entry.originalID

		if entry.resultCh != nil {
			// MCP tool waiting for this response.
			entry.resultCh <- reqResult{result: msg.Result, lspErr: msg.Error}
		} else {
			p.writeIDE(msg)
		}

	default:
		// Server-initiated request (e.g. workspace/configuration, window/showMessageRequest).
		p.writeIDE(msg)
	}
}

// writeLSPRaw serializes msg and sends it to the LSP server stdin.
func (p *Proxy) writeLSPRaw(msg *lsp.Message) error {
	log.Printf("→clangd  method=%q id=%s", msg.Method, truncate(string(msg.ID), 40))
	p.lspMu.Lock()
	defer p.lspMu.Unlock()
	return lsp.WriteMessage(p.lspIn, msg)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// writeIDE serializes msg and sends it to the IDE (our stdout).
func (p *Proxy) writeIDE(msg *lsp.Message) {
	p.outMu.Lock()
	defer p.outMu.Unlock()
	if err := lsp.WriteMessage(os.Stdout, msg); err != nil {
		log.Printf("proxy: write IDE error: %v", err)
	}
}

// sendRequest sends an LSP request from the proxy (or MCP tool) and waits for the response.
func (p *Proxy) sendRequest(method string, params json.RawMessage) (json.RawMessage, error) {
	n := p.counter.Add(1)
	remapped := fmt.Sprintf("M%d", n)

	ch := make(chan reqResult, 1)
	p.mu.Lock()
	p.pending[remapped] = &pendingEntry{
		method:   method,
		resultCh: ch,
	}
	p.mu.Unlock()

	msg := &lsp.Message{
		JSONRPC: "2.0",
		ID:      jsonString(remapped),
		Method:  method,
		Params:  params,
	}
	if err := p.writeLSPRaw(msg); err != nil {
		p.mu.Lock()
		delete(p.pending, remapped)
		p.mu.Unlock()
		return nil, err
	}

	select {
	case r := <-ch:
		if r.lspErr != nil {
			return nil, fmt.Errorf("LSP error %d: %s", r.lspErr.Code, r.lspErr.Message)
		}
		return r.result, nil
	case <-time.After(30 * time.Second):
		p.mu.Lock()
		delete(p.pending, remapped)
		p.mu.Unlock()
		return nil, fmt.Errorf("timeout waiting for %s", method)
	}
}

// SendRequest sends an LSP request from an MCP tool and returns the result.
func (p *Proxy) SendRequest(method string, params json.RawMessage) (json.RawMessage, error) {
	return p.sendRequest(method, params)
}

// SendNotification sends an LSP notification from an MCP tool.
func (p *Proxy) SendNotification(method string, params json.RawMessage) error {
	return p.writeLSPRaw(&lsp.Message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

// WaitReady blocks until the LSP server has responded to initialize (or timeout).
func (p *Proxy) WaitReady(timeout time.Duration) error {
	select {
	case <-p.capsCh:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for LSP initialize response")
	}
}

// Workspace returns the workspace root directory.
func (p *Proxy) Workspace() string { return p.cfg.Workspace }

// Wait waits for the LSP server process to exit.
func (p *Proxy) Wait() error { return p.cmd.Wait() }

// jsonString marshals s as a JSON string.
func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}
