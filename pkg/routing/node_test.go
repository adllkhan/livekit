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

package routing_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/livekit-server/pkg/routing"
)

// node identity is what other nodes and Redis use to reach this one, so it stays a single
// address even when several are advertised to clients as ICE candidates.
func TestNewLocalNodeUsesSingleIP(t *testing.T) {
	t.Run("node_ip wins over node_ips", func(t *testing.T) {
		conf, err := config.NewConfig(`rtc:
  node_ip: 10.1.99.215
  node_ips:
    - 89.218.81.183
    - 10.1.6.19`, true, nil, nil)
		require.NoError(t, err)

		node, err := routing.NewLocalNode(conf)
		require.NoError(t, err)
		require.Equal(t, "10.1.99.215", node.NodeIP())
	})

	t.Run("first node_ips entry is used when node_ip is unset", func(t *testing.T) {
		conf, err := config.NewConfig(`rtc:
  node_ips:
    - 89.218.81.183
    - 10.1.6.19`, true, nil, nil)
		require.NoError(t, err)

		node, err := routing.NewLocalNode(conf)
		require.NoError(t, err)
		require.Equal(t, "89.218.81.183", node.NodeIP())
	})
}
