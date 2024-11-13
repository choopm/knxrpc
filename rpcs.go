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
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/choopm/knxrpc/knx/groupaddress/v1"
)

// Publish implements knx.groupaddressservice.v1.Publish
func (s *Server) Publish(
	ctx context.Context,
	req *connect.Request[v1.PublishRequest],
) (*connect.Response[v1.PublishResponse], error) {
	res := &v1.PublishResponse{}

	event, err := fromV1PublishRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// write to bus
	err = s.tunnel.Send(*event)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// dispatch event aswell since we don't receive
	// a copy of our event from the gateway for subscribers.
	err = s.dispatchEvent(event)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(res), nil
}

// Subscribe implements knx.groupaddressservice.v1.Subscribe
func (s *Server) Subscribe(
	ctx context.Context,
	req *connect.Request[v1.SubscribeRequest],
	stream *connect.ServerStream[v1.SubscribeResponse],
) error {
	// parse group addresses
	addresses, err := parseGroupAddresses(req.Msg.GroupAddresses)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	if len(addresses) > 0 {
		// register group addresses to subscribe
		s.registerSubscriber(addresses, req.Msg, stream)
	} else {
		// no filtering on group_addresses -> sniffer
		s.registerSniffer(req.Msg, stream)
	}

	// block until any ctx is done
	select {
	case <-ctx.Done():
	case <-s.ctx.Done():
		return connect.NewError(connect.CodeAborted, s.ctx.Err())
	}

	if len(addresses) > 0 {
		// remove us from subscribed group addresses
		s.unregisterSubscriber(addresses, stream)
	} else {
		// no filtering on group_addresses -> sniffer
		s.unregisterSniffer(stream)
	}

	return nil
}

// SubscribeUnary implements knx.groupaddressservice.v1.SubscribeUnary
func (s *Server) SubscribeUnary(
	ctx context.Context,
	req *connect.Request[v1.SubscribeUnaryRequest],
) (*connect.Response[v1.SubscribeUnaryResponse], error) {
	// construct internal client
	client, err := NewClient(s.config.Client)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("constructing client: %v", err))
	}

	// input validation
	if req.Msg.For != "" {
		dur, err := time.ParseDuration(req.Msg.For)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parsing 'for': %v", err))
		}

		// init timer
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dur)
		defer cancel()
	}

	// resp collects all messages
	resp := connect.NewResponse(&v1.SubscribeUnaryResponse{
		Messages: []*v1.SubscribeResponse{},
	})

	// open stream
	stream, err := client.Subscribe(ctx, connect.NewRequest(req.Msg.SubscribeRequest))
	if err != nil {
		return nil, err
	}
	defer stream.Close() // nolint:errcheck

	// start receiver loop
	for stream.Receive() {
		msg := stream.Msg()
		resp.Msg.Messages = append(resp.Msg.Messages, msg)

		// if user provided no `for`, quit after first message
		if req.Msg.For == "" {
			break
		}
	}
	if connectErr := new(connect.Error); errors.As(err, &connectErr) &&
		!errors.Is(connectErr, context.Canceled) &&
		!errors.Is(connectErr, context.DeadlineExceeded) {
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("knxrpc stream closed: %s", stream.Err()))
	}

	return resp, nil
}
