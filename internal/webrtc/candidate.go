package webrtc

import (
	"encoding/json"
	"time"

	"github.com/HMasataka/conic/domain"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/rs/xid"
)

func OnIceCandidate(conn *websocket.Conn, pc *PeerConnection) func(*webrtc.ICECandidate) error {
	return func(candidate *webrtc.ICECandidate) error {
		if candidate == nil {
			return nil
		}

		targetID := pc.TargetID()

		candidateMsg := domain.ICECandidateMessage{
			FromID:    pc.ID(),
			ToID:      targetID,
			Candidate: candidate.ToJSON(),
		}

		data, err := json.Marshal(candidateMsg)
		if err != nil {
			return err
		}

		req := domain.Message{
			ID:        xid.New().String(),
			Type:      domain.MessageTypeCandidate,
			Timestamp: time.Now(),
			Data:      data,
		}

		msg, err := json.Marshal(req)
		if err != nil {
			return err
		}

		return conn.WriteMessage(websocket.TextMessage, msg)
	}
}
