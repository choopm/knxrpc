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
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/choopm/knxrpc/knx/groupaddress/v1"
	"github.com/vapourismo/knx-go/knx/cemi"
)

// parseGroupAddresses returns a list of parsed knx group addresses or error.
// all addresses must be in the form of "1/2/3".
func parseGroupAddresses(addresses []string) ([]cemi.GroupAddr, error) {
	ret := []cemi.GroupAddr{}

	for i, sga := range addresses {
		ga, err := cemi.NewGroupAddrString(sga)
		if err != nil {
			return nil, fmt.Errorf("parse groupAddress(%d): %s", i, err)
		}

		ret = append(ret, ga)
	}

	return ret, nil
}

// registerSubscriber adds group addresses and streams into subscribers map
func (s *Server) registerSubscriber(
	addresses []cemi.GroupAddr,
	req *v1.SubscribeRequest,
	stream *connect.ServerStream[v1.SubscribeResponse],
) {
	s.m_subscribers.Lock()
	defer s.m_subscribers.Unlock()

	for _, address := range addresses {
		// load streams slice from map
		subs, ok := s.subscribers[address]
		if !ok {
			// first subscriber for this group address
			subs = []*subscriber{}
		}

		// append ourself
		subs = append(subs, &subscriber{
			req:    req,
			stream: stream,
		})

		// put subscriber slice back into the map
		s.subscribers[address] = subs
	}
}

// unregisterSubscriber removes group addresses and streams from subscribers map
func (s *Server) unregisterSubscriber(
	addresses []cemi.GroupAddr,
	stream *connect.ServerStream[v1.SubscribeResponse],
) {
	s.m_subscribers.Lock()
	defer s.m_subscribers.Unlock()

	for _, address := range addresses {
		// load subscriber slice from map
		subs, ok := s.subscribers[address]
		if !ok {
			// this indicates a failure, ignore it
			continue
		}

		// check if we are the last subscriber, then we can simply reslice
		if len(subs) > 0 && subs[len(subs)-1].stream == stream {
			// drop the last element
			subs = subs[:len(subs)-1]

			// delete the slice if its empty now
			if len(subs) > 0 {
				s.subscribers[address] = subs
			} else {
				delete(s.subscribers, address)
			}
			continue
		}

		// otherwise we need to find ourself in the slice and move the
		// last one element into our position to stay within O(n).
		for i, s := range subs {
			if s.stream != stream {
				continue
			}

			// we found ourself, move the last element to our index
			subs[i] = subs[len(subs)-1]

			// and drop the last element
			subs = subs[:len(subs)-1]

			break
		}

		// put in back in the map
		s.subscribers[address] = subs
	}
}

// registerSniffer adds a subscriber to sniffers slice
func (s *Server) registerSniffer(
	req *v1.SubscribeRequest,
	stream *connect.ServerStream[v1.SubscribeResponse],
) {
	s.m_sniffers.Lock()
	defer s.m_sniffers.Unlock()

	s.sniffers = append(s.sniffers, &subscriber{
		req:    req,
		stream: stream,
	})
}

// unregisterSniffer removes a subscriber from sniffers slice
func (s *Server) unregisterSniffer(
	stream *connect.ServerStream[v1.SubscribeResponse],
) {
	s.m_sniffers.Lock()
	defer s.m_sniffers.Unlock()

	// check if we are the last subscriber, then we can simply reslice
	if len(s.sniffers) > 0 && s.sniffers[len(s.sniffers)-1].stream == stream {
		// drop the last element
		s.sniffers = s.sniffers[:len(s.sniffers)-1]

		return
	}

	// otherwise we need to find ourself in the slice and move the
	// last one element into our position to stay within O(n).
	for i, sub := range s.sniffers {
		if sub.stream != stream {
			continue
		}

		// we found ourself, move the last element to our index
		s.sniffers[i] = s.sniffers[len(s.sniffers)-1]

		// and drop the last element
		s.sniffers = s.sniffers[:len(s.sniffers)-1]

		return
	}
}
