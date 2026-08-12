// Copyright 2025 LiveKit, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rtc

import (
	"fmt"
	"net"
	"testing"

	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"

	"github.com/livekit/livekit-server/pkg/config"
)

type hostCandidate struct {
	address string
	port    int
}

// gatherHostIPv4Candidates gathers on a PeerConnection built from conf and returns the IPv4
// host candidates it ends up advertising.
func gatherHostIPv4Candidates(t *testing.T, yamlConf string) []hostCandidate {
	t.Helper()

	conf, err := config.NewConfig(yamlConf, true, nil, nil)
	require.NoError(t, err)

	webRTCConfig, err := NewWebRTCConfig(conf)
	require.NoError(t, err)

	api := webrtc.NewAPI(webrtc.WithSettingEngine(webRTCConfig.SettingEngine))
	pc, err := api.NewPeerConnection(webRTCConfig.Configuration)
	require.NoError(t, err)
	defer pc.Close()

	_, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
	require.NoError(t, err)

	offer, err := pc.CreateOffer(nil)
	require.NoError(t, err)

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	require.NoError(t, pc.SetLocalDescription(offer))
	<-gatherComplete

	desc := pc.LocalDescription()
	require.NotNil(t, desc)

	parsed, err := desc.Unmarshal()
	require.NoError(t, err)

	var candidates []hostCandidate
	seen := make(map[string]struct{})
	for _, media := range parsed.MediaDescriptions {
		for _, attr := range media.Attributes {
			if attr.Key != "candidate" {
				continue
			}
			candidate, err := ice.UnmarshalCandidate(attr.Value)
			require.NoError(t, err)
			if candidate.Type() != ice.CandidateTypeHost {
				continue
			}
			address := candidate.Address()
			parsedIP := net.ParseIP(address)
			if parsedIP == nil || parsedIP.To4() == nil {
				continue
			}
			key := fmt.Sprintf("%s:%d", address, candidate.Port())
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, hostCandidate{address: address, port: candidate.Port()})
		}
	}

	return candidates
}

func hostCandidateAddresses(candidates []hostCandidate) []string {
	seen := make(map[string]struct{}, len(candidates))
	addresses := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate.address]; ok {
			continue
		}
		seen[candidate.address] = struct{}{}
		addresses = append(addresses, candidate.address)
	}

	return addresses
}

// Every configured node IP has to reach the client as its own host candidate: pion keeps a
// list of external addresses per local address, and this asserts it does not stop at the first
// element of that list.
func TestNodeIPsAdvertisedAsSeparateHostCandidates(t *testing.T) {
	const baseConf = `port: 0
rtc:
  tcp_port: 0
  port_range_start: 51000
  port_range_end: 51100
  enable_loopback_candidate: true
  node_ips:
    - 89.218.81.183
    - 10.1.6.19
`

	t.Run("replaces local addresses by default", func(t *testing.T) {
		candidates := gatherHostIPv4Candidates(t, baseConf)
		require.ElementsMatch(t, []string{"89.218.81.183", "10.1.6.19"}, hostCandidateAddresses(candidates))
	})

	t.Run("keeps local addresses with advertise_internal_ip", func(t *testing.T) {
		addresses := hostCandidateAddresses(gatherHostIPv4Candidates(t, baseConf+"  advertise_internal_ip: true\n"))
		require.Subset(t, addresses, []string{"89.218.81.183", "10.1.6.19"})
		require.Greater(t, len(addresses), 2, "local addresses should be kept alongside the configured ones")
	})

	// a single muxed UDP port is the common deployment: both addresses have to be advertised
	// on that same port, since that is what the NAT and any proxy in front forward.
	t.Run("works with a muxed udp port", func(t *testing.T) {
		candidates := gatherHostIPv4Candidates(t, `port: 0
rtc:
  tcp_port: 0
  udp_port: 51750
  enable_loopback_candidate: true
  node_ips:
    - 89.218.81.183
    - 10.1.6.19
`)
		require.ElementsMatch(t, []string{"89.218.81.183", "10.1.6.19"}, hostCandidateAddresses(candidates))
		for _, candidate := range candidates {
			require.Equal(t, 51750, candidate.port, "candidate %s should keep the muxed port", candidate.address)
		}
	})

	t.Run("unset node_ips leaves candidates untouched", func(t *testing.T) {
		addresses := hostCandidateAddresses(gatherHostIPv4Candidates(t, `port: 0
rtc:
  tcp_port: 0
  port_range_start: 51000
  port_range_end: 51100
  enable_loopback_candidate: true
  node_ip: 10.1.6.19
`))
		require.NotContains(t, addresses, "89.218.81.183")
		require.NotEmpty(t, addresses)
	})
}
