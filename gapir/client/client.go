// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/gapid/core/app"
	"github.com/google/gapid/core/app/auth"
	"github.com/google/gapid/core/app/crash"
	"github.com/google/gapid/core/app/crash/reporting"
	"github.com/google/gapid/core/app/status"
	"github.com/google/gapid/core/context/keys"
	"github.com/google/gapid/core/data/id"
	"github.com/google/gapid/core/event/task"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/core/os/device/bind"
	"github.com/google/gapid/core/os/file"
	"github.com/google/gapid/gapir"
	replaysrv "github.com/google/gapid/gapir/replay_service"
	"github.com/google/gapid/gapis/database"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// LaunchArgsKey is the bind device property key used to control the command
	// line arguments when launching GAPIR. The property must be of type []string.
	LaunchArgsKey = "gapir-launch-args"
	// gapirAuthTokenMetaDataName is the key of the Context metadata pair that
	// contains the authentication token. This token is common knowledge shared
	// between GAPIR client (which is GAPIS) and GAPIR server (which is GAPIR
	// device).
	gapirAuthTokenMetaDataName = "gapir-auth-token"
	// gRPCConnectTimeout is the time allowed to establish a gRPC connection.
	gRPCConnectTimeout = time.Second * 10
	// heartbeatInterval is the delay between heartbeat pings.
	heartbeatInterval = time.Second * 2
)

// ReplayExecutor must be implemented by replay executors to handle some live
// interactions with a running replay.
type ReplayExecutor interface {
	// HandlePostData handles the given post data message.
	HandlePostData(context.Context, *gapir.PostData) error
	// HandleNotification handles the given notification message.
	HandleNotification(context.Context, *gapir.Notification) error
	// HandleFinished is notified when the given replay is finished.
	HandleFinished(context.Context, error) error
	// HandleFenceReadyRequest handles when the replayer is waiting for the server
	// to execute the registered FenceReadyRequestCallback for fence ID provided
	// in the FenceReadyRequest.
	HandleFenceReadyRequest(context.Context, *gapir.FenceReadyRequest) error
}

// ReplayerKey is used to uniquely identify a GAPIR instance.
type ReplayerKey struct {
	device bind.Device
	arch   device.Architecture
}

// replayerInfo stores data related to a single GAPIR instance.
type replayerInfo struct {
	device               bind.Device
	abi                  *device.ABI
	deviceConnectionInfo deviceConnectionInfo
	executor             ReplayExecutor
	conn                 *grpc.ClientConn
	rpcClient            replaysrv.GapirClient
	rpcStream            replaysrv.Gapir_ReplayClient
}

// Client handles multiple GAPIR instances identified by ReplayerKey.
type Client struct {
	// mutex prevents data races when restarting replayers.
	mutex     sync.Mutex
	replayers map[ReplayerKey]*replayerInfo
}

// New returns a new Client with no replayers.
func New(ctx context.Context) *Client {
	client := &Client{replayers: map[ReplayerKey]*replayerInfo{}}
	app.AddCleanup(ctx, func() {
		client.shutdown(ctx)
	})
	return client
}

// shutdown closes all replayer instances and makes the client invalid.
func (client *Client) shutdown(ctx context.Context) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	for _, replayer := range client.replayers {
		replayer.closeConnection(ctx)
	}
	client.replayers = nil
}

// Connect starts a GAPIR instance and return its ReplayerKey.
func (client *Client) Connect(ctx context.Context, device bind.Device, abi *device.ABI) (*ReplayerKey, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	ctx = status.Start(ctx, "Connect")
	defer status.Finish(ctx)

	if client.replayers == nil {
		return nil, log.Err(ctx, nil, "Client has been shutdown")
	}

	key := ReplayerKey{device: device, arch: abi.GetArchitecture()}

	if _, ok := client.replayers[key]; ok {
		return &key, nil
	}

	launchArgs, _ := bind.GetRegistry(ctx).DeviceProperty(ctx, device, LaunchArgsKey).([]string)
	newDeviceConnectionInfo, err := initDeviceConnection(ctx, device, abi, launchArgs)
	if err != nil {
		return nil, err
	}

	log.I(ctx, "Waiting for connection to GAPIR...")

	// Create gRPC connection
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", newDeviceConnectionInfo.port),
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(gRPCConnectTimeout),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)))
	if err != nil {
		return nil, log.Err(ctx, err, "Timeout waiting for connection")
	}
	rpcClient := replaysrv.NewGapirClient(conn)

	replayerInfo := &replayerInfo{
		deviceConnectionInfo: *newDeviceConnectionInfo,
		device:               device,
		abi:                  abi,
		conn:                 conn,
		rpcClient:            rpcClient,
	}

	crash.Go(func() { client.heartbeat(ctx, replayerInfo) })
	log.I(ctx, "Heartbeat connection setup done")

	err = replayerInfo.startReplayCommunicationHandler(ctx)
	if err != nil {
		return nil, log.Err(ctx, err, "Error in startReplayCommunicationHandler")
	}

	client.replayers[key] = replayerInfo
	return &key, nil
}

// removeConnection closes and removes a GAPIR instance.
func (client *Client) removeConnection(ctx context.Context, key ReplayerKey) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	if replayer, ok := client.replayers[key]; ok {
		replayer.closeConnection(ctx)
		delete(client.replayers, key)
	}
}

// reconnect removes the replayer before re-creating it.
func (client *Client) reconnect(ctx context.Context, replayer *replayerInfo) {
	key := ReplayerKey{device: replayer.device, arch: replayer.abi.GetArchitecture()}
	client.removeConnection(ctx, key)
	client.Connect(ctx, replayer.device, replayer.abi)
}

// heartbeat regularly sends a ping to a replayer, and restarts it when it fails to reply.
func (client *Client) heartbeat(ctx context.Context, replayer *replayerInfo) {
	for {
		select {
		case <-task.ShouldStop(ctx):
			return
		case <-time.After(heartbeatInterval):
			err := replayer.ping(ctx)
			if err != nil {
				log.E(ctx, "Error sending keep-alive ping. Error: %v", err)
				client.reconnect(ctx, replayer)
				return
			}
		}
	}
}

// getActiveReplayer returns the replayer identified by key only if this
// replayer has an active connection to a GAPIR instance.
func (client *Client) getActiveReplayer(ctx context.Context, key *ReplayerKey) (*replayerInfo, error) {
	replayer, found := client.replayers[*key]
	if !found {
		return nil, log.Errf(ctx, nil, "Cannot find replayer for this key: %v", key)
	}

	if replayer.rpcClient == nil || replayer.conn == nil || replayer.rpcStream == nil {
		return nil, log.Err(ctx, nil, "Replayer has no active connection")
	}

	return replayer, nil
}

// BeginReplay sends a replay request to the replayer identified by key.
func (client *Client) BeginReplay(ctx context.Context, key *ReplayerKey, payload string, dependent string) error {
	ctx = log.Enter(ctx, "Starting replay on gapir device")
	replayerInfo, err := client.getActiveReplayer(ctx, key)
	if err != nil {
		return err
	}

	idReq := replaysrv.ReplayRequest{
		Req: &replaysrv.ReplayRequest_Replay{
			Replay: &replaysrv.Replay{
				ReplayId:    payload,
				DependentId: dependent,
			},
		},
	}
	err = replayerInfo.rpcStream.Send(&idReq)
	if err != nil {
		return log.Err(ctx, err, "Sending replay id")
	}

	return nil
}

// SetReplayExecutor assigns a replay executor to the replayer identified by
// key. It returns a cleanup function to remove the executor once the replay
// is finished.
func (client *Client) SetReplayExecutor(ctx context.Context, key *ReplayerKey, executor ReplayExecutor) (func(), error) {
	replayerInfo, err := client.getActiveReplayer(ctx, key)
	if err != nil {
		return nil, err
	}

	if replayerInfo.executor != nil {
		return nil, log.Err(ctx, nil, "Cannot set an executor while one is already present")
	}
	replayerInfo.executor = executor
	return func() { replayerInfo.executor = nil }, nil
}

// PrewarmReplay requests the GAPIR device to get itself into the given state
func (client *Client) PrewarmReplay(ctx context.Context, key *ReplayerKey, payload string, cleanup string) error {
	replayerInfo, err := client.getActiveReplayer(ctx, key)
	if err != nil {
		return log.Err(ctx, err, "Getting replayer replayerInfo")
	}

	PrerunReq := replaysrv.ReplayRequest{
		Req: &replaysrv.ReplayRequest_Prewarm{
			Prewarm: &replaysrv.PrewarmRequest{
				PrerunId:  payload,
				CleanupId: cleanup,
			},
		},
	}
	err = replayerInfo.rpcStream.Send(&PrerunReq)
	if err != nil {
		return log.Err(ctx, err, "Sending replay payload")
	}
	return nil
}

// startReplayCommunicationHandler launches a background task which creates
// the Replay RPC stream and starts to listen to it.
func (replayer *replayerInfo) startReplayCommunicationHandler(ctx context.Context) error {
	connected := make(chan error)
	cctx := keys.Clone(context.Background(), ctx)
	crash.Go(func() {
		// This shouldn't be sitting on this context
		cctx := status.PutTask(cctx, nil)
		cctx = status.StartBackground(cctx, "Handle Replay Communication")
		defer status.Finish(cctx)

		// Kick the communication handler
		err := replayer.handleReplayCommunication(cctx, connected)
		if err != nil {
			log.E(cctx, "Error communication with gapir: %v", err)
		}

		if replayer.executor == nil {
			log.Err(ctx, nil, "No active replay executor to HandleFinish")
			return
		}
		err = replayer.executor.HandleFinished(ctx, err)
		if err != nil {
			log.Err(cctx, err, "In cleaning up after HandleReplayCommunication returned")
		}
	})
	err := <-connected
	return err
}

// closeConnection properly terminates the replayer
func (replayer *replayerInfo) closeConnection(ctx context.Context) {
	// Call Shutdown RCP on the replayer
	if replayer.rpcClient != nil {
		// Use a clean context, since ctx is most likely already cancelled.
		sdCtx := attachAuthToken(context.Background(), replayer.deviceConnectionInfo.authToken)
		_, err := replayer.rpcClient.Shutdown(sdCtx, &replaysrv.ShutdownRequest{})
		if err != nil {
			log.E(ctx, "Sending replayer Shutdown request: %v", err)
		}
	}
	replayer.rpcClient = nil

	if replayer.rpcStream != nil {
		replayer.rpcStream.CloseSend()
	}
	replayer.rpcStream = nil

	if replayer.conn != nil {
		replayer.conn.Close()
	}
	replayer.conn = nil

	replayer.deviceConnectionInfo.cleanupFunc()
}

// ping uses the Ping RPC to make sure a GAPIR instance is alive.
func (replayer *replayerInfo) ping(ctx context.Context) error {
	if replayer.rpcClient == nil {
		return log.Errf(ctx, nil, "cannot ping without gapir connection")
	}

	ctx = attachAuthToken(ctx, replayer.deviceConnectionInfo.authToken)
	r, err := replayer.rpcClient.Ping(ctx, &replaysrv.PingRequest{})
	if err != nil {
		return log.Err(ctx, err, "Sending ping")
	}
	if r == nil {
		return log.Err(ctx, nil, "No response for ping")
	}

	return nil
}

// attachAuthToken attaches authentication token to the context as metadata, if
// the authentication token is not empty, and returns the new context. If the
// authentication token is empty, returns the original context.
func attachAuthToken(ctx context.Context, authToken auth.Token) context.Context {
	if len(authToken) != 0 {
		return metadata.NewOutgoingContext(ctx,
			metadata.Pairs(gapirAuthTokenMetaDataName, string(authToken)))
	}
	return ctx
}

// handleReplayCommunication handles the communication with the GAPIR device on
// a replay stream connection. It creates the replay connection stream and then
// enters a loop where it listens to messages from GAPIR and dispatches them to
// the relevant handlers.
func (replayer *replayerInfo) handleReplayCommunication(
	ctx context.Context,
	connected chan error) error {
	ctx = log.Enter(ctx, "HandleReplayCommunication")
	if replayer.conn == nil || replayer.rpcClient == nil {
		return log.Errf(ctx, nil, "Gapir not connected")
	}
	// One Connection is only supposed to be used to handle replay communication
	// in one thread. Initiating another replay communication with a connection
	// which is handling another replay communication will mess up the package
	// order.
	if replayer.rpcStream != nil {
		err := log.Errf(ctx, nil, "Replayer: %v is handling another replay communication stream in another thread. Initiating a new replay on this replayer will mess up the package order for both the existing replay and the new replay", replayer)
		connected <- err
		return err
	}

	ctx = attachAuthToken(ctx, replayer.deviceConnectionInfo.authToken)
	replayStream, err := replayer.rpcClient.Replay(ctx)
	if err != nil {
		return log.Err(ctx, err, "Getting replay stream client")
	}
	replayer.rpcStream = replayStream
	connected <- nil
	defer func() {
		if replayer.rpcStream != nil {
			replayer.rpcStream.CloseSend()
			replayer.rpcStream = nil
		}
	}()
	for {
		if replayer.rpcStream == nil {
			return log.Errf(ctx, nil, "No replayer connection stream")
		}
		r, err := replayer.rpcStream.Recv()
		if err != nil {
			return log.Errf(ctx, err, "Replayer connection lost")
		}
		switch r.Res.(type) {
		case *replaysrv.ReplayResponse_PayloadRequest:
			if err := replayer.handlePayloadRequest(ctx, r.GetPayloadRequest().GetPayloadId()); err != nil {
				return log.Errf(ctx, err, "Handling replay payload request")
			}
		case *replaysrv.ReplayResponse_ResourceRequest:
			if err := replayer.handleResourceRequest(ctx, r.GetResourceRequest()); err != nil {
				return log.Errf(ctx, err, "Handling replay resource request")
			}
		case *replaysrv.ReplayResponse_CrashDump:
			if err := replayer.handleCrashDump(ctx, r.GetCrashDump()); err != nil {
				return log.Errf(ctx, err, "Handling replay crash dump")
			}
			// No valid replay response after crash dump.
			return log.Errf(ctx, nil, "Replay crash")
		case *replaysrv.ReplayResponse_PostData:
			if err := replayer.executor.HandlePostData(ctx, r.GetPostData()); err != nil {
				return log.Errf(ctx, err, "Handling post data")
			}
		case *replaysrv.ReplayResponse_Notification:
			if err := replayer.executor.HandleNotification(ctx, r.GetNotification()); err != nil {
				return log.Errf(ctx, err, "Handling notification")
			}
		case *replaysrv.ReplayResponse_Finished:
			if err := replayer.executor.HandleFinished(ctx, nil); err != nil {
				return log.Errf(ctx, err, "Handling finished")
			}
		case *replaysrv.ReplayResponse_FenceReadyRequest:
			if replayer.executor == nil {
				return log.Err(ctx, nil, "No replay executor to HandleFenceReadyRequest")
			}
			fenceReq := r.GetFenceReadyRequest()
			if err := replayer.executor.HandleFenceReadyRequest(ctx, fenceReq); err != nil {
				return log.Errf(ctx, err, "Handling replay fence ready request")
			}
			if err := replayer.sendFenceReady(ctx, fenceReq.GetId()); err != nil {
				return log.Errf(ctx, err, "connection SendFenceReady failed")
			}

		default:
			return log.Errf(ctx, nil, "Unhandled ReplayResponse type")
		}
	}
}

// sendFenceReady signals the device to continue a replay.
func (replayer *replayerInfo) sendFenceReady(ctx context.Context, id uint32) error {
	fenceReadyReq := replaysrv.ReplayRequest{
		Req: &replaysrv.ReplayRequest_FenceReady{
			FenceReady: &replaysrv.FenceReady{
				Id: id,
			},
		},
	}
	err := replayer.rpcStream.Send(&fenceReadyReq)
	if err != nil {
		return log.Errf(ctx, err, "Sending replay fence %v ready", id)
	}
	return nil
}

// handleResourceRequest sends back the requested resources.
func (replayer *replayerInfo) handleResourceRequest(ctx context.Context, req *gapir.ResourceRequest) error {
	ctx = status.Start(ctx, "Resources Request (count: %d)", len(req.GetIds()))
	defer status.Finish(ctx)

	ctx = log.Enter(ctx, "handleResourceRequest")

	// Process the request
	if req == nil {
		return log.Err(ctx, nil, "Cannot handle nil resource request")
	}
	ids := req.GetIds()
	totalExpectedSize := req.GetExpectedTotalSize()
	totalReturnedSize := uint64(0)
	response := make([]byte, 0, totalExpectedSize)
	db := database.Get(ctx)
	for _, idStr := range ids {
		rID, err := id.Parse(idStr)
		if err != nil {
			return log.Errf(ctx, err, "Failed to parse resource id: %v", idStr)
		}
		obj, err := db.Resolve(ctx, rID)
		if err != nil {
			return log.Errf(ctx, err, "Failed to parse resource id: %v", idStr)
		}
		objData := obj.([]byte)
		response = append(response, objData...)
		totalReturnedSize += uint64(len(objData))
	}
	if totalReturnedSize != totalExpectedSize {
		return log.Errf(ctx, nil, "Total resource size mismatch. expected: %v, got: %v", totalExpectedSize, totalReturnedSize)
	}

	// Send the resources
	resReq := replaysrv.ReplayRequest{
		Req: &replaysrv.ReplayRequest_Resources{
			Resources: &replaysrv.Resources{Data: response},
		},
	}
	if err := replayer.rpcStream.Send(&resReq); err != nil {
		return log.Err(ctx, err, "Sending resources")
	}
	return nil
}

// handleCrashDump uploads the received crash dump the crash tracking service.
func (replayer *replayerInfo) handleCrashDump(ctx context.Context, dump *gapir.CrashDump) error {
	if dump == nil {
		return log.Err(ctx, nil, "Nil crash dump")
	}
	filepath := dump.GetFilepath()
	crashData := dump.GetCrashData()
	OS := replayer.device.Instance().GetConfiguration().GetOS()
	// TODO(baldwinn860): get the actual version from GAPIR in case it ever goes out of sync
	if res, err := reporting.ReportMinidump(reporting.Reporter{
		AppName:    "GAPIR",
		AppVersion: app.Version.String(),
		OSName:     OS.GetName(),
		OSVersion:  fmt.Sprintf("%v %v.%v.%v", OS.GetBuild(), OS.GetMajorVersion(), OS.GetMinorVersion(), OS.GetPointVersion()),
	}, filepath, crashData); err != nil {
		return log.Err(ctx, err, "Failed to report GAPIR crash")
	} else if res != "" {
		log.I(ctx, "Crash Report Uploaded; ID: %v", res)
		file.Remove(file.Abs(filepath))
	}
	return nil
}

// handlePayloadRequest sends back the requested payload.
func (replayer *replayerInfo) handlePayloadRequest(ctx context.Context, payloadID string) error {
	ctx = status.Start(ctx, "Payload Request")
	defer status.Finish(ctx)

	pid, err := id.Parse(payloadID)
	if err != nil {
		return log.Errf(ctx, err, "Parsing payload ID")
	}
	boxed, err := database.Resolve(ctx, pid)
	if err != nil {
		return log.Errf(ctx, err, "Getting replay payload")
	}
	if payload, ok := boxed.(*gapir.Payload); ok {
		if replayer.conn == nil || replayer.rpcClient == nil {
			return log.Err(ctx, nil, "Gapir not connected")
		}
		if replayer.rpcStream == nil {
			return log.Err(ctx, nil, "Replay Communication not initiated")
		}
		payloadReq := replaysrv.ReplayRequest{
			Req: &replaysrv.ReplayRequest_Payload{
				Payload: payload,
			},
		}
		err := replayer.rpcStream.Send(&payloadReq)
		if err != nil {
			return log.Err(ctx, err, "Sending replay payload")
		}
		return nil
	}
	return log.Errf(ctx, err, "Payload type is unexpected: %T", boxed)
}
