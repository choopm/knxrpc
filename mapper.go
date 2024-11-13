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

	v1 "github.com/choopm/knxrpc/knx/groupaddress/v1"
	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
)

// toV1SubscribeResponse returns the v1.SubscribeResponse of event
func toV1SubscribeResponse(event *knx.GroupEvent) *v1.SubscribeResponse {
	ret := &v1.SubscribeResponse{
		GroupAddress:    event.Destination.String(),
		PhysicalAddress: event.Source.String(),
		Event:           v1.Event_EVENT_UNSPECIFIED,
		Data:            event.Data,
	}

	switch event.Command {
	case knx.GroupRead:
		ret.Event = v1.Event_EVENT_READ
	case knx.GroupResponse:
		ret.Event = v1.Event_EVENT_RESPONSE
	case knx.GroupWrite:
		ret.Event = v1.Event_EVENT_WRITE
	}

	return ret
}

// fromV1PublishRequest returns the v1.SubscribeResponse of event
func fromV1PublishRequest(req *v1.PublishRequest) (*knx.GroupEvent, error) {
	// parse group address
	ga, err := cemi.NewGroupAddrString(req.GroupAddress)
	if err != nil {
		return nil, fmt.Errorf("parse groupAddress: %s", err)
	}

	event := &knx.GroupEvent{
		Destination: ga,
		Data:        req.Data,
	}

	switch req.Event {
	case v1.Event_EVENT_READ:
		event.Command = knx.GroupRead
	case v1.Event_EVENT_RESPONSE:
		event.Command = knx.GroupResponse
	case v1.Event_EVENT_WRITE:
		event.Command = knx.GroupWrite
	default:
		event.Command = knx.GroupWrite
	}

	if len(req.PhysicalAddress) == 0 {
		return event, nil
	}

	event.Source, err = cemi.NewIndividualAddrString(req.PhysicalAddress)
	if err != nil {
		return nil, fmt.Errorf("parse physicalAddress: %s", err)
	}

	return event, nil
}
