package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"aarukanworld/internal/auth"
	"aarukanworld/internal/world"

	"github.com/gorilla/websocket"
)

const idleTimeout = 10 * time.Minute

type Server struct {
	hub      *world.Hub
	mux      *http.ServeMux
	upgrader websocket.Upgrader
}

func NewServer(hub *world.Hub) *Server {
	s := &Server{
		hub: hub,
		mux: http.NewServeMux(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				// Browser game client; tighten once origins are known.
				return true
			},
		},
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return withCORS(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /v1/worlds/{worldID}", s.handleGetWorld)
	s.mux.HandleFunc("GET /v1/worlds/{worldID}/ws", s.handleWorldWS)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetWorld(w http.ResponseWriter, r *http.Request) {
	worldID := r.PathValue("worldID")
	peers, ok := s.hub.WorldInfo(worldID)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"world_id": worldID,
			"hot":      false,
			"peers":    0,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"world_id": worldID,
		"hot":      true,
		"peers":    peers,
	})
}

func (s *Server) handleWorldWS(w http.ResponseWriter, r *http.Request) {
	worldID := r.PathValue("worldID")
	token := auth.ExtractPlayToken(r)
	claims, err := auth.Verify(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if claims.WorldID != worldID {
		writeError(w, http.StatusForbidden, "token world_id mismatch")
		return
	}

	peer, wld, err := s.hub.Attach(claims)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "err", err)
		s.hub.Detach(peer.ID)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(4 << 20)
	peer.SetConn(conn)
	defer peer.SetConn(nil)
	defer s.hub.Detach(peer.ID)

	_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))

	events := peer.Subscribe(256)
	defer peer.Unsubscribe(events)

	_ = conn.WriteJSON(world.Message{
		Type:    world.MsgWelcome,
		Nick:    peer.Nick,
		WorldID: peer.WorldID,
		Text:    "attached",
	})
	for _, nick := range wld.PeerNicks() {
		if strings.EqualFold(nick, peer.Nick) {
			continue
		}
		_ = conn.WriteJSON(world.Message{Type: world.MsgPeerJoin, Nick: nick})
	}

	errCh := make(chan error, 1)
	go func() {
		for {
			var msg world.Message
			if err := conn.ReadJSON(&msg); err != nil {
				errCh <- err
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))
			msg.Type = strings.ToLower(strings.TrimSpace(msg.Type))
			if msg.Type == "" {
				continue
			}
			wld.HandleClient(peer, msg)
		}
	}()

	for {
		select {
		case ev, open := <-events:
			if !open {
				return
			}
			if err := conn.WriteJSON(ev); err != nil {
				return
			}
		case <-errCh:
			return
		case <-r.Context().Done():
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Play-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
