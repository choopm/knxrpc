/*
Copyright 2024 Christoph Hoopmann

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package knxrpc

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/choopm/knxrpc/knx/groupaddress/v1"
	"github.com/rs/zerolog"
	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/util"
)

// knxLogHandler logs everything to trace level
type knxLogHandler struct {
	util.LogTarget
	log *zerolog.Logger

	cancel context.CancelFunc
}

// Printf implements util.LogTarget
func (s *knxLogHandler) Printf(format string, args ...interface{}) {
	// Msgf does not require a newline to be present, trim it
	format = strings.TrimSuffix(format, "\n")

	s.log.Trace().Msgf(format, args...)

	// this is a hack to detect broken connections within the used knx library
	if format == "Worker exited" {
		s.log.Error().Msg("knx worker exit detected, cancelling context")
		s.cancel()
	}
}

// connectTunnel connects and sets up the KNX tunnel
func (s *Server) connectTunnel() (err error) {
	// build host:port
	hostPort := fmt.Sprintf("%s:%d",
		s.config.KNX.GatwewayHost,
		s.config.KNX.GatwewayPort)

	// Connect to the gateway.
	s.tunnel, err = knx.NewGroupTunnel(hostPort, knx.TunnelConfig{
		ResendInterval:    knx.DefaultTunnelConfig.ResendInterval,
		HeartbeatInterval: knx.DefaultTunnelConfig.HeartbeatInterval,
		ResponseTimeout:   s.config.KNX.Timeout,
		SendLocalAddress:  s.config.KNX.SendLocalAddress,
		UseTCP:            s.config.KNX.UseTCP,
	})
	if err != nil {
		return fmt.Errorf("connect tunnel: %s", err)
	}
	// s.tunnel.Close() is handled at the end of [Start]

	return nil
}

// busMessageReader starts the message reading or error
func (s *Server) busMessageReader(ctx context.Context) error {
	// infinite reader loop
	for {
		select {
		// quit when ctx is done
		case <-ctx.Done():
			return nil

		// pass any event to message dispatcher
		case event := <-s.tunnel.Inbound():
			if err := s.dispatchEvent(&event); err != nil {
				return err
			}
		}
	}
}

// dispatchEvent dispatches an event to connected streams
func (s *Server) dispatchEvent(event *knx.GroupEvent) error {
	if err := s.dispatchToSubscribers(event); err != nil {
		return err
	}
	if err := s.dispatchToSniffers(event); err != nil {
		return err
	}

	return nil
}

// dispatchToSubscribers sends the event to subscriber streams
func (s *Server) dispatchToSubscribers(event *knx.GroupEvent) error {
	s.m_subscribers.Lock()
	defer s.m_subscribers.Unlock()

	subs, ok := s.subscribers[event.Destination]
	if !ok {
		// no subscribers for this group address
		return nil
	}

	resp := toV1SubscribeResponse(event)

	for _, sub := range subs {
		if sub.req.Event != v1.Event_EVENT_UNSPECIFIED &&
			sub.req.Event != resp.Event {
			// this subscriber is not interested in this kind of event
			continue
		}

		err := sub.stream.Send(resp)
		if err != nil {
			s.log.Error().
				Err(err).
				Str("peer", sub.stream.Conn().Peer().Addr).
				Msg("unable to send response to subscriber")
			continue
		}
	}

	return nil
}

// dispatchToSniffers sends the event to sniffer streams
func (s *Server) dispatchToSniffers(event *knx.GroupEvent) error {
	s.m_sniffers.Lock()
	defer s.m_sniffers.Unlock()

	if len(s.sniffers) == 0 {
		// no sniffers connected
		return nil
	}

	resp := toV1SubscribeResponse(event)

	for _, sniffer := range s.sniffers {
		if sniffer.req.Event != v1.Event_EVENT_UNSPECIFIED &&
			sniffer.req.Event != resp.Event {
			// this sniffer is not interested in this kind of event
			continue
		}

		err := sniffer.stream.Send(resp)
		if err != nil {
			s.log.Error().
				Err(err).
				Str("peer", sniffer.stream.Conn().Peer().Addr).
				Msg("unable to send response to sniffer")
			continue
		}
	}

	return nil
}
