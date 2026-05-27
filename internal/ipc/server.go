package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// Server hosts a Unix-socket JSON-RPC listener and dispatches
// incoming verbs to a Handler.
type Server struct {
	socketPath string
	handler    Handler

	mu       sync.Mutex
	listener net.Listener
	started  bool
	stopped  bool
	wg       sync.WaitGroup
}

// New wires a Server. SocketPath must be an absolute filesystem path.
// The Handler implements the verb dispatch table.
func New(socketPath string, handler Handler) *Server {
	return &Server{socketPath: socketPath, handler: handler}
}

// SocketPath returns the configured socket path.
func (s *Server) SocketPath() string { return s.socketPath }

// Start binds the Unix socket and begins accepting connections in
// the background. It returns once the listener is bound; the accept
// loop runs until Stop is called or ctx is cancelled.
//
// Start removes any stale socket file at SocketPath before binding —
// callers expecting cooperation with another listener should pick a
// different path.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("ipc: already started")
	}
	s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o700); err != nil {
		return fmt.Errorf("ipc: mkdir socket dir: %w", err)
	}
	// Remove a stale socket file if present. Ignore ENOENT.
	if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("ipc: remove stale socket: %w", err)
	}

	l, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("ipc: listen %s: %w", s.socketPath, err)
	}
	// Owner-only access. The socket lives under the user's home dir
	// already, but be defensive.
	_ = os.Chmod(s.socketPath, 0o600)

	s.mu.Lock()
	s.listener = l
	s.started = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.acceptLoop(ctx)
	return nil
}

// Stop closes the listener and waits for in-flight connections to
// drain. Idempotent.
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.started || s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	l := s.listener
	s.mu.Unlock()

	if l != nil {
		_ = l.Close()
	}
	s.wg.Wait()
	// Best-effort cleanup of the socket file. Listener.Close on Unix
	// usually removes it, but tolerate either case.
	_ = os.Remove(s.socketPath)
	return nil
}

func (s *Server) acceptLoop(ctx context.Context) {
	defer s.wg.Done()
	// Close listener when ctx is cancelled so Accept unblocks.
	stopCtx, stopCancel := context.WithCancel(ctx)
	defer stopCancel()
	go func() {
		<-stopCtx.Done()
		s.mu.Lock()
		l := s.listener
		s.mu.Unlock()
		if l != nil {
			_ = l.Close()
		}
	}()

	for {
		s.mu.Lock()
		l := s.listener
		s.mu.Unlock()
		if l == nil {
			return
		}
		conn, err := l.Accept()
		if err != nil {
			if isClosedErr(err) {
				return
			}
			// Transient — keep accepting.
			continue
		}
		s.wg.Add(1)
		go s.serveConn(ctx, conn)
	}
}

func isClosedErr(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF)
}

// serveConn reads newline-delimited JSON requests from conn and
// writes one response per request. The connection stays open as long
// as the client keeps sending; the server hangs up on read error or
// EOF.
func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		if ctx.Err() != nil {
			return
		}
		var req Request
		if err := dec.Decode(&req); err != nil {
			if !errors.Is(err, io.EOF) {
				_ = enc.Encode(Response{Error: &ErrorBody{
					Code:    ErrCodeBadRequest,
					Message: err.Error(),
				}})
			}
			return
		}
		resp := s.dispatch(ctx, req)
		if err := enc.Encode(resp); err != nil {
			return
		}
	}
}

// dispatch routes a single request to its Handler method and packs
// the result (or error) into a Response.
func (s *Server) dispatch(ctx context.Context, req Request) Response {
	resp := Response{ID: req.ID}

	switch req.Method {
	case VerbSkillRun:
		var p SkillRunParams
		if err := unmarshalParams(req.Params, &p); err != nil {
			return errResp(req.ID, err)
		}
		out, err := s.handler.SkillRun(ctx, p)
		return packResp(req.ID, out, err)

	case VerbPipelineTrigger:
		var p PipelineTriggerParams
		if err := unmarshalParams(req.Params, &p); err != nil {
			return errResp(req.ID, err)
		}
		out, err := s.handler.PipelineTrigger(ctx, p)
		return packResp(req.ID, out, err)

	case VerbVaultQuery:
		var p VaultQueryParams
		if err := unmarshalParams(req.Params, &p); err != nil {
			return errResp(req.ID, err)
		}
		out, err := s.handler.VaultQuery(ctx, p)
		return packResp(req.ID, out, err)

	case VerbIndexRefresh:
		out, err := s.handler.IndexRefresh(ctx)
		return packResp(req.ID, out, err)

	case VerbStatus:
		out, err := s.handler.Status(ctx)
		return packResp(req.ID, out, err)

	default:
		resp.Error = &ErrorBody{
			Code:    ErrCodeUnknownMethod,
			Message: fmt.Sprintf("unknown method: %q", req.Method),
		}
		return resp
	}
}

func packResp(id string, result any, err error) Response {
	if err != nil {
		return errResp(id, err)
	}
	raw, mErr := json.Marshal(result)
	if mErr != nil {
		return errResp(id, mErr)
	}
	return Response{ID: id, Result: raw}
}

func errResp(id string, err error) Response {
	return Response{
		ID: id,
		Error: &ErrorBody{
			Code:    errorCode(err),
			Message: err.Error(),
		},
	}
}
