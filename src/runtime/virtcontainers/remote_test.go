// Copyright (c) 2023 IBM Corporation
// SPDX-License-Identifier: Apache-2.0

package virtcontainers

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kata-containers/kata-containers/src/runtime/virtcontainers/types"
	"github.com/stretchr/testify/assert"
)

func newRemoteConfig() HypervisorConfig {
	return HypervisorConfig{
		RemoteHypervisorSocket:  "/run/peerpod/hypervisor.sock",
		RemoteHypervisorTimeout: 600,
		DisableGuestSeLinux:     true,
		EnableAnnotations:       []string{},
	}
}

func TestRemoteHypervisorGenerateSocket(t *testing.T) {
	assert := assert.New(t)

	remoteHypervisor := remoteHypervisor{
		config: newRemoteConfig(),
	}
	id := "sandboxId"

	// No socketPath should error
	_, err := remoteHypervisor.GenerateSocket(id)
	assert.Error(err)

	socketPath := "socketPath"
	remoteHypervisor.agentSocketPath = socketPath

	result, err := remoteHypervisor.GenerateSocket(id)
	assert.NoError(err)

	expected := types.RemoteSock{
		SandboxID:        id,
		TunnelSocketPath: socketPath,
	}
	assert.Equal(result, expected)
}

func TestRemoteHypervisorSaveLoad(t *testing.T) {
	assert := assert.New(t)

	// Create a remote hypervisor with state
	rh := &remoteHypervisor{
		sandboxID:       remoteHypervisorSandboxID("test-sandbox-123"),
		agentSocketPath: "/run/peerpod/pods/test-sandbox-123/agent.sock",
		config:          newRemoteConfig(),
	}

	// Save the state
	savedState := rh.Save()

	// Verify saved state contains expected values
	assert.Equal("/run/peerpod/pods/test-sandbox-123/agent.sock", savedState.AgentSocketPath)
	assert.Equal("test-sandbox-123", savedState.RemoteSandboxID)

	// Create a new remote hypervisor (simulating restart)
	rh2 := &remoteHypervisor{
		config: newRemoteConfig(),
	}

	// Verify initial state is empty
	assert.Empty(rh2.agentSocketPath)
	assert.Empty(string(rh2.sandboxID))

	// Load the saved state
	rh2.Load(savedState)

	// Verify state was restored
	assert.Equal("/run/peerpod/pods/test-sandbox-123/agent.sock", rh2.agentSocketPath)
	assert.Equal(remoteHypervisorSandboxID("test-sandbox-123"), rh2.sandboxID)
}

func TestOpenRemoteServiceRetry(t *testing.T) {
	assert := assert.New(t)

	// Test 1: Non-existent socket should fail after retries
	t.Run("fails after retries on non-existent socket", func(t *testing.T) {
		start := time.Now()
		_, err := openRemoteService("/nonexistent/path/to/socket.sock")
		elapsed := time.Since(start)

		assert.Error(err)
		assert.Contains(err.Error(), "failed to connect to remote hypervisor socket")
		// Should have retried (at least some delay from retries)
		// With 5 attempts and exponential backoff (1+2+4+8=15s total wait between retries)
		// We check it took at least 1 second (first retry delay)
		assert.True(elapsed >= 1*time.Second, "should have retried with delay, elapsed: %v", elapsed)
	})

	// Test 2: Valid socket connects successfully
	t.Run("connects to valid socket", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "remote-test-*")
		assert.NoError(err)
		defer os.RemoveAll(tmpDir)

		socketPath := filepath.Join(tmpDir, "test.sock")
		listener, err := net.Listen("unix", socketPath)
		assert.NoError(err)
		defer listener.Close()

		svc, err := openRemoteService(socketPath)
		assert.NoError(err)
		assert.NotNil(svc)
		assert.NotNil(svc.conn)
		assert.NotNil(svc.client)

		err = svc.Close()
		assert.NoError(err)
	})
}
