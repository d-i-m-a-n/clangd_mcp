// clangd-mcp: transparent LSP proxy with an embedded MCP/SSE server.
//
// Usage (drop-in replacement for clangd):
//
//	clangd-mcp [clangd-flags...]
//
// clangd-mcp finds clangd (via env, config, or PATH), forwards all arguments
// to it, and starts an MCP/SSE server for AI agent access.
//
// IDE setup (e.g. QtCreator):
//
//	Preferences → C++ → Clang Code Model → Override clangd executable
//	Point it to clangd-mcp.exe; no additional arguments needed.
//
// MCP client config (Claude Desktop, etc.):
//
//	{
//	  "mcpServers": {
//	    "clangd": { "type": "sse", "url": "http://localhost:7878/sse" }
//	  }
//	}
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"clangd-mcp/config"
	mcpserver "clangd-mcp/mcp"
	"clangd-mcp/proxy"
)

const probeTimeout = 500 * time.Millisecond

func setupLogging() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	logPath := filepath.Join(filepath.Dir(exe), "clangd-mcp.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix(fmt.Sprintf("[pid:%d] ", os.Getpid()))
}

func main() {
	setupLogging()
	log.Printf("args: %q", os.Args)

	cfg := config.Load()
	log.Printf("config: clangd=%q port=%d", cfg.ClangdPath, cfg.Port)

	cmd := exec.Command(cfg.ClangdPath, os.Args[1:]...)
	cmd.Stderr = os.Stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("clangd-mcp: stdin pipe: %v", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("clangd-mcp: stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("clangd-mcp: start clangd: %v", err)
	}

	// Read clangd stdout in a goroutine, sending chunks to a buffered channel.
	// In probe mode (--version, --check) clangd exits quickly and readDone fires.
	// In proxy mode clangd keeps running and chanReader serves data to the proxy.
	const chanBuf = 1024
	dataCh := make(chan []byte, chanBuf)
	readDone := make(chan struct{})

	go func() {
		buf := make([]byte, 65536)
		for {
			n, rerr := stdoutPipe.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				dataCh <- chunk
			}
			if rerr != nil {
				close(dataCh)
				close(readDone)
				return
			}
		}
	}()

	select {
	case <-readDone:
		// Clangd exited quickly (probe/version check).
		for chunk := range dataCh {
			os.Stdout.Write(chunk)
		}
		cmd.Wait() //nolint:errcheck
		os.Exit(cmd.ProcessState.ExitCode())

	case <-time.After(probeTimeout):
		// Clangd is still running — enter proxy mode.
	}

	workspace, _ := os.Getwd()

	proxyCfg := proxy.Config{
		MCPPort:   cfg.Port,
		Workspace: workspace,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	p := proxy.New(proxyCfg)
	p.Attach(cmd, stdinPipe, &chanReader{ch: dataCh})

	go p.RunQTC()

	if hasArg(os.Args[1:], "--compile-commands-dir") {
		go startMCPServer(p, cfg.Port)
	} else {
		log.Printf("clangd-mcp: --compile-commands-dir not set, MCP server not started")
	}

	select {
	case <-ctx.Done():
	case err := <-waitAsync(p):
		if err != nil {
			log.Printf("clangd-mcp: clangd exited: %v", err)
		}
	}
}

// chanReader is an io.Reader backed by a channel of byte slices.
type chanReader struct {
	ch  <-chan []byte
	buf []byte
}

func (r *chanReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 {
		chunk, ok := <-r.ch
		if !ok {
			return 0, io.EOF
		}
		r.buf = chunk
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

// startMCPServer tries to bind the MCP port, retrying if it is still held by a
// dying previous instance (IDE launches overlapping proxy processes).
func startMCPServer(p *proxy.Proxy, port int) {
	const (
		maxRetries = 10
		retryDelay = 1 * time.Second
	)
	addr := fmt.Sprintf(":%d", port)

	for i := range maxRetries {
		sseServer := mcpserver.NewSSEServer(p, port)
		log.Printf("clangd-mcp: starting MCP SSE server on %s", addr)
		err := sseServer.Start(addr)
		if err == nil {
			return
		}
		if i < maxRetries-1 {
			log.Printf("clangd-mcp: port %d busy, retry %d/%d: %v", port, i+1, maxRetries, err)
			time.Sleep(retryDelay)
		} else {
			log.Printf("clangd-mcp: MCP server failed after %d retries: %v", maxRetries, err)
		}
	}
}

func hasArg(args []string, prefix string) bool {
	for _, a := range args {
		if a == prefix || len(a) > len(prefix) && a[:len(prefix)+1] == prefix+"=" {
			return true
		}
	}
	return false
}

func waitAsync(p *proxy.Proxy) <-chan error {
	ch := make(chan error, 1)
	go func() { ch <- p.Wait() }()
	return ch
}
