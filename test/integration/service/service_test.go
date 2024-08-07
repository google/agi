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

package service_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"reflect"
	"testing"
	"time"

	"github.com/google/gapid/core/app/auth"
	"github.com/google/gapid/core/assert"

	"github.com/google/gapid/core/event/task"
	"github.com/google/gapid/core/log"
	"github.com/google/gapid/core/net/grpcutil"
	"github.com/google/gapid/core/os/device/bind"

	//"github.com/google/gapid/core/os/device/host"
	"github.com/google/gapid/gapis/api"
	gapis "github.com/google/gapid/gapis/client"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/replay"
	"github.com/google/gapid/gapis/server"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/path"
	"github.com/google/gapid/gapis/stringtable"
	"google.golang.org/grpc"
)

func startServerAndGetGrpcClient(ctx context.Context, config server.Config) (service.Service, error, func()) {
	l := grpcutil.NewPipeListener("pipe:servicetest")

	schan := make(chan *grpc.Server, 1)
	go server.NewWithListener(ctx, l, config, schan)
	svr := <-schan

	conn, err := grpcutil.Dial(ctx, "pipe:servicetest",
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(auth.UnaryClientInterceptor(config.AuthToken)),
		grpc.WithStreamInterceptor(auth.StreamClientInterceptor(config.AuthToken)),
		grpc.WithDialer(grpcutil.GetDialer(ctx)),
	)
	if err != nil {
		return nil, log.Err(ctx, err, "Dialing GAPIS"), nil
	}
	client := gapis.Bind(conn)

	if !deviceScanDone.Fired() {
		onDeviceScanDone(ctx)
	}
	return client, nil, func() {
		client.Close()
		svr.GracefulStop()
	}
}

func setup(t *testing.T) (context.Context, server.Server, func()) {
	ctx := log.Testing(t)
	r := bind.NewRegistry()
	ctx = bind.PutRegistry(ctx, r)
	m := replay.New(ctx)
	ctx = replay.PutManager(ctx, m)
	ctx = database.Put(ctx, database.NewInMemory(ctx))

	r.AddDevice(ctx, bind.Host(ctx))

	client, err, shutdown := startServerAndGetGrpcClient(ctx, cfg)
	assert.For(ctx, "err").ThatError(err).Succeeded()

	return ctx, client, shutdown
}

func text(text string) *stringtable.Node {
	return &stringtable.Node{Node: &stringtable.Node_Text{Text: &stringtable.Text{Text: text}}}
}

var (
	deviceScanDone, onDeviceScanDone = task.NewSignal()
	stringtables                     = []*stringtable.StringTable{
		&stringtable.StringTable{
			Info: &stringtable.Info{
				CultureCode: "animals",
			},
			Entries: map[string]*stringtable.Node{
				"fish": text("glub"),
				"dog":  text("barks"),
				"cat":  text("meows"),
				"fox":  text("?"),
			},
		},
	}
	cfg = server.Config{
		Info: &service.ServerInfo{
			Name:         "testbot2000",
			VersionMajor: 123,
			VersionMinor: 456,
			Features:     []string{"moo", "meow", "meh"},
		},
		AuthToken:      "s3Cr3t",
		StringTables:   stringtables,
		DeviceScanDone: deviceScanDone,
	}
	testCaptureData, _ = base64.StdEncoding.DecodeString("UHJvdG9QYWNrDQoyLjAKAO8BDmNhcHR1cmUuSGVhZGVyCgZIZWFkZXISIAoGZGV2aWNlGAEgASgLMhAuZGV2aWNlLkluc3RhbmNlEhgKA0FCSRgCIAEoCzILLmRldmljZS5BQkkSDwoHdmVyc2lvbhgDIAEoERISCgpzdGFydF90aW1lGAQgASgE+wEPZGV2aWNlLkluc3RhbmNlCghJbnN0YW5jZRIWCgJJRBgBIAEoCzIKLmRldmljZS5JRBIOCgZzZXJpYWwYAiABKAkSDAoEbmFtZRgDIAEoCRIsCg1jb25maWd1cmF0aW9uGAQgASgLMhUuZGV2aWNlLkNvbmZpZ3VyYXRpb243CWRldmljZS5JRAoCSUQSDAoEZGF0YRgBIAEoDOcDFGRldmljZS5Db25maWd1cmF0aW9uCg1Db25maWd1cmF0aW9uEhYKAk9TGAEgASgLMgouZGV2aWNlLk9TEiIKCGhhcmR3YXJlGAIgASgLMhAuZGV2aWNlLkhhcmR3YXJlEhkKBEFCSXMYAyADKAsyCy5kZXZpY2UuQUJJEiAKB2RyaXZlcnMYBCABKAsyDy5kZXZpY2UuRHJpdmVycxI3ChNwZXJmZXR0b19jYXBhYmlsaXR5GAUgASgLMhouZGV2aWNlLlBlcmZldHRvQ2FwYWJpbGl0eRIcCgVhbmdsZRgGIAEoCzINLmRldmljZS5BTkdMRcUCCWRldmljZS5PUwoCT1MSHAoEa2luZBgBIAEoDjIOLmRldmljZS5PU0tpbmQSDAoEbmFtZRgCIAEoCRINCgVidWlsZBgDIAEoCRIVCg1tYWpvcl92ZXJzaW9uGAQgASgFEhUKDW1pbm9yX3ZlcnNpb24YBSABKAUSFQoNcG9pbnRfdmVyc2lvbhgGIAEoBRITCgtBUElfdmVyc2lvbhgHIAEoBbcBD2RldmljZS5IYXJkd2FyZQoISGFyZHdhcmUSDAoEbmFtZRgBIAEoCRIYCgNDUFUYAiABKAsyCy5kZXZpY2UuQ1BVEhgKA0dQVRgDIAEoCzILLmRldmljZS5HUFXRAQpkZXZpY2UuQ1BVCgNDUFUSDAoEbmFtZRgBIAEoCRIOCgZ2ZW5kb3IYAiABKAkSKgoMYXJjaGl0ZWN0dXJlGAMgASgOMhQuZGV2aWNlLkFyY2hpdGVjdHVyZRINCgVjb3JlcxgEIAEoDX0KZGV2aWNlLkdQVQoDR1BVEgwKBG5hbWUYASABKAkSDgoGdmVuZG9yGAIgASgJEg8KB3ZlcnNpb24YAyABKA2lAgpkZXZpY2UuQUJJCgNBQkkSDAoEbmFtZRgBIAEoCRIaCgJPUxgCIAEoDjIOLmRldmljZS5PU0tpbmQSKgoMYXJjaGl0ZWN0dXJlGAMgASgOMhQuZGV2aWNlLkFyY2hpdGVjdHVyZRIrCg1tZW1vcnlfbGF5b3V0GAQgASgLMhQuZGV2aWNlLk1lbW9yeUxheW91dMMHE2RldmljZS5NZW1vcnlMYXlvdXQKDE1lbW9yeUxheW91dBIeCgZlbmRpYW4YASABKA4yDi5kZXZpY2UuRW5kaWFuEicKB3BvaW50ZXIYAiABKAsyFi5kZXZpY2UuRGF0YVR5cGVMYXlvdXQSJwoHaW50ZWdlchgDIAEoCzIWLmRldmljZS5EYXRhVHlwZUxheW91dBIkCgRzaXplGAQgASgLMhYuZGV2aWNlLkRhdGFUeXBlTGF5b3V0EiQKBGNoYXIYBSABKAsyFi5kZXZpY2UuRGF0YVR5cGVMYXlvdXQSIwoDaTY0GAYgASgLMhYuZGV2aWNlLkRhdGFUeXBlTGF5b3V0EiMKA2kzMhgHIAEoCzIWLmRldmljZS5EYXRhVHlwZUxheW91dBIjCgNpMTYYCCABKAsyFi5kZXZpY2UuRGF0YVR5cGVMYXlvdXQSIgoCaTgYCSABKAsyFi5kZXZpY2UuRGF0YVR5cGVMYXlvdXQSIwoDZjY0GAogASgLMhYuZGV2aWNlLkRhdGFUeXBlTGF5b3V0EiMKA2YzMhgLIAEoCzIWLmRldmljZS5EYXRhVHlwZUxheW91dBIjCgNmMTYYDCABKAsyFi5kZXZpY2UuRGF0YVR5cGVMYXlvdXSNARVkZXZpY2UuRGF0YVR5cGVMYXlvdXQKDkRhdGFUeXBlTGF5b3V0EgwKBHNpemUYASABKAUSEQoJYWxpZ25tZW50GAIgASgFew5kZXZpY2UuRHJpdmVycwoHRHJpdmVycxIkCgZ2dWxrYW4YAiABKAsyFC5kZXZpY2UuVnVsa2FuRHJpdmVy9QITZGV2aWNlLlZ1bGthbkRyaXZlcgoMVnVsa2FuRHJpdmVyEiMKBmxheWVycxgBIAMoCzITLmRldmljZS5WdWxrYW5MYXllchIpCiFpY2RfYW5kX2ltcGxpY2l0X2xheWVyX2V4dGVuc2lvbnMYAiADKAkSNgoQcGh5c2ljYWxfZGV2aWNlcxgDIAMoCzIcLmRldmljZS5WdWxrYW5QaHlzaWNhbERldmljZRIPCgd2ZXJzaW9uGAQgASgJgwESZGV2aWNlLlZ1bGthbkxheWVyCgtWdWxrYW5MYXllchIMCgRuYW1lGAEgASgJEhIKCmV4dGVuc2lvbnMYAiADKAmzAhtkZXZpY2UuVnVsa2FuUGh5c2ljYWxEZXZpY2UKFFZ1bGthblBoeXNpY2FsRGV2aWNlEhMKC2FwaV92ZXJzaW9uGAEgASgNEhYKDmRyaXZlcl92ZXJzaW9uGAIgASgNEhEKCXZlbmRvcl9pZBgDIAEoDRIRCglkZXZpY2VfaWQYBCABKA0SEwoLZGV2aWNlX25hbWUYBSABKAnvBBlkZXZpY2UuUGVyZmV0dG9DYXBhYmlsaXR5ChJQZXJmZXR0b0NhcGFiaWxpdHkSKwoNZ3B1X3Byb2ZpbGluZxgBIAEoCzIULmRldmljZS5HUFVQcm9maWxpbmcSPAoVdnVsa2FuX3Byb2ZpbGVfbGF5ZXJzGAIgASgLMh0uZGV2aWNlLlZ1bGthblByb2ZpbGluZ0xheWVycxIfChdjYW5fc3BlY2lmeV9hdHJhY2VfYXBwcxgDIAEoCBIbChNoYXNfZnJhbWVfbGlmZWN5Y2xlGAQgASgIEhYKDmhhc19wb3dlcl9yYWlsGAUgASgIEiIKGmNhbl9kb3dubG9hZF93aGlsZV90cmFjaW5nGAYgASgIEiMKG2Nhbl9wcm92aWRlX3RyYWNlX2ZpbGVfcGF0aBgHIAEoCIMDE2RldmljZS5HUFVQcm9maWxpbmcKDEdQVVByb2ZpbGluZxIWCg5oYXNSZW5kZXJTdGFnZRgBIAEoCBI8ChZncHVfY291bnRlcl9kZXNjcmlwdG9yGAIgASgLMhwuZGV2aWNlLkdwdUNvdW50ZXJEZXNjcmlwdG9yEicKH2hhc19yZW5kZXJfc3RhZ2VfcHJvZHVjZXJfbGF5ZXIYBCABKAgSGQoRaGFzX2dwdV9tZW1fdG90YWwYBSABKAhKBAgDEATJFhtkZXZpY2UuR3B1Q291bnRlckRlc2NyaXB0b3IKFEdwdUNvdW50ZXJEZXNjcmlwdG9yEjoKBXNwZWNzGAEgAygLMisuZGV2aWNlLkdwdUNvdW50ZXJEZXNjcmlwdG9yLkdwdUNvdW50ZXJTcGVjEjwKBmJsb2NrcxgCIAMoCzIsLmRldmljZS5HcHVDb3VudGVyRGVzY3JpcHRvci5HcHVDb3VudGVyQmxvY2sSHgoWbWluX3NhbXBsaW5nX3BlcmlvZF9ucxgDIAEoBBIeChZtYXhfc2FtcGxpbmdfcGVyaW9kX25zGAQgASgEEiYKHnN1cHBvcnRzX2luc3RydW1lbnRlZF9zYW1wbGluZxgFIAEoCBrzAgoOR3B1Q291bnRlclNwZWMSEgoKY291bnRlcl9pZBgBIAEoDRIMCgRuYW1lGAIgASgJEhMKC2Rlc2NyaXB0aW9uGAMgASgJEhgKDmludF9wZWFrX3ZhbHVlGAUgASgDSAASGwoRZG91YmxlX3BlYWtfdmFsdWUYBiABKAFIABJBCg9udW1lcmF0b3JfdW5pdHMYByADKA4yKC5kZXZpY2UuR3B1Q291bnRlckRlc2NyaXB0b3IuTWVhc3VyZVVuaXQSQwoRZGVub21pbmF0b3JfdW5pdHMYCCADKA4yKC5kZXZpY2UuR3B1Q291bnRlckRlc2NyaXB0b3IuTWVhc3VyZVVuaXQSGQoRc2VsZWN0X2J5X2RlZmF1bHQYCSABKAgSPAoGZ3JvdXBzGAogAygOMiwuZGV2aWNlLkdwdUNvdW50ZXJEZXNjcmlwdG9yLkdwdUNvdW50ZXJHcm91cEIMCgpwZWFrX3ZhbHVlSgQIBBAFGnMKD0dwdUNvdW50ZXJCbG9jaxIQCghibG9ja19pZBgBIAEoDRIWCg5ibG9ja19jYXBhY2l0eRgCIAEoDRIMCgRuYW1lGAMgASgJEhMKC2Rlc2NyaXB0aW9uGAQgASgJEhMKC2NvdW50ZXJfaWRzGAUgAygNInUKD0dwdUNvdW50ZXJHcm91cBIQCgxVTkNMQVNTSUZJRUQQABIKCgZTWVNURU0QARIMCghWRVJUSUNFUxACEg0KCUZSQUdNRU5UUxADEg4KClBSSU1JVElWRVMQBBIKCgZNRU1PUlkQBRILCgdDT01QVVRFEAYirAQKC01lYXN1cmVVbml0EggKBE5PTkUQABIHCgNCSVQQARILCgdLSUxPQklUEAISCwoHTUVHQUJJVBADEgsKB0dJR0FCSVQQBBILCgdURVJBQklUEAUSCwoHUEVUQUJJVBAGEggKBEJZVEUQBxIMCghLSUxPQllURRAIEgwKCE1FR0FCWVRFEAkSDAoIR0lHQUJZVEUQChIMCghURVJBQllURRALEgwKCFBFVEFCWVRFEAwSCQoFSEVSVFoQDRINCglLSUxPSEVSVFoQDhINCglNRUdBSEVSVFoQDxINCglHSUdBSEVSVFoQEBINCglURVJBSEVSVFoQERINCglQRVRBSEVSVFoQEhIOCgpOQU5PU0VDT05EEBMSDwoLTUlDUk9TRUNPTkQQFBIPCgtNSUxMSVNFQ09ORBAVEgoKBlNFQ09ORBAWEgoKBk1JTlVURRAXEggKBEhPVVIQGBIKCgZWRVJURVgQGRIJCgVQSVhFTBAaEgwKCFRSSUFOR0xFEBsSDQoJUFJJTUlUSVZFECYSDAoIRlJBR01FTlQQJxINCglNSUxMSVdBVFQQHBIICgRXQVRUEB0SDAoIS0lMT1dBVFQQHhIJCgVKT1VMRRAfEggKBFZPTFQQIBIKCgZBTVBFUkUQIRILCgdDRUxTSVVTECISDgoKRkFIUkVOSEVJVBAjEgoKBktFTFZJThAkEgsKB1BFUkNFTlQQJRIPCgtJTlNUUlVDVElPThAouwYqZGV2aWNlLkdwdUNvdW50ZXJEZXNjcmlwdG9yLkdwdUNvdW50ZXJTcGVjCg5HcHVDb3VudGVyU3BlYxISCgpjb3VudGVyX2lkGAEgASgNEgwKBG5hbWUYAiABKAkSEwoLZGVzY3JpcHRpb24YAyABKAkSGAoOaW50X3BlYWtfdmFsdWUYBSABKANIABIbChFkb3VibGVfcGVha192YWx1ZRgGIAEoAUgAEkEKD251bWVyYXRvcl91bml0cxgHIAMoDjIoLmRldmljZS5HcHVDb3VudGVyRGVzY3JpcHRvci5NZWFzdXJlVW5pdBJDChFkZW5vbWluYXRvcl91bml0cxgIIAMoDjIoLmRldmljZS5HcHVDb3VudGVyRGVzY3JpcHRvci5NZWFzdXJlVW5pdBIZChFzZWxlY3RfYnlfZGVmYXVsdBgJIAEoCBI8CgZncm91cHMYCiADKA4yLC5kZXZpY2UuR3B1Q291bnRlckRlc2NyaXB0b3IuR3B1Q291bnRlckdyb3VwQgwKCnBlYWtfdmFsdWVKBAgEEAW9AitkZXZpY2UuR3B1Q291bnRlckRlc2NyaXB0b3IuR3B1Q291bnRlckJsb2NrCg9HcHVDb3VudGVyQmxvY2sSEAoIYmxvY2tfaWQYASABKA0SFgoOYmxvY2tfY2FwYWNpdHkYAiABKA0SDAoEbmFtZRgDIAEoCRITCgtkZXNjcmlwdGlvbhgEIAEoCRITCgtjb3VudGVyX2lkcxgFIAMoDb8BHGRldmljZS5WdWxrYW5Qcm9maWxpbmdMYXllcnMKFVZ1bGthblByb2ZpbGluZ0xheWVycxISCgpjcHVfdGltaW5nGAEgASgIEhYKDm1lbW9yeV90cmFja2VyGAIgASgIawxkZXZpY2UuQU5HTEUKBUFOR0xFEg8KB3BhY2thZ2UYASABKAkSDwoHdmVyc2lvbhgCIAEoBfIGAAIK0gIKFgoUv0mAQKHM1UmPOE3HbrdKnxDJ1Q4aDkdvb2dsZSBQaXhlbCA2IqcCCkAIBBICMTQaOG9yaW9sZS1lbmcgMTQgTUFTVEVSIGVuZy5raXJpZHouMjAyMzA4MTYuMTQwMjU1IGRldi1rZXlzEhoKBm9yaW9sZRIOCgVnczEwMRIDQVJNGAIaABpVCglhcm02NC12OGEQBBgCIkQIAhIECAgQCBoECAgQCCIECAgQCCoECAEQATIECAgQCDoECAQQBEIECAIQAkoECAEQAVIECAgQCFoECAQQBGIECAIQAhpXCgthcm1lYWJpLXY3YRAEGAEiRAgCEgQIBBAEGgQIBBAEIgQIBBAEKgQIARABMgQICBAIOgQIBBAEQgQIAhACSgQIARABUgQICBAIWgQIBBAEYgQIAhACGgsKB2FybWVhYmkQBCICEgAqBhICCAEYARJVCglhcm02NC12OGEQBBgCIkQIAhIECAgQCBoECAgQCCIECAgQCCoECAEQATIECAgQCDoECAQQBEIECAIQAkoECAEQAVIECAgQCFoECAQQBGIECAIQAhgIIK/nodqSvPMCuBAAAgq1BwoWChRIbNVGdIExBD2K3gB3ng1GJzwa0BoOR29vZ2xlIFBpeGVsIDYiigcKQAgEEgIxNBo4b3Jpb2xlLWVuZyAxNCBNQVNURVIgZW5nLmtpcmlkei4yMDIzMDgxNi4xNDAyNTUgZGV2LWtleXMSGgoGb3Jpb2xlEg4KBWdzMTAxEgNBUk0YAhoAGlUKCWFybTY0LXY4YRAEGAIiRAgCEgQICBAIGgQICBAIIgQICBAIKgQIARABMgQICBAIOgQIBBAEQgQIAhACSgQIARABUgQICBAIWgQIBBAEYgQIAhACGlcKC2FybWVhYmktdjdhEAQYASJECAISBAgEEAQaBAgEEAQiBAgEEAQqBAgBEAEyBAgIEAg6BAgEEARCBAgCEAJKBAgBEAFSBAgIEAhaBAgEEARiBAgCEAIaCwoHYXJtZWFiaRAEIuQEEuEECiEKC0RlYnVnTWFya2VyEhJWS19FWFRfZGVidWdfdXRpbHMKRgobVktfTEFZRVJfS0hST05PU192YWxpZGF0aW9uEhNWS19FWFRfZGVidWdfcmVwb3J0EhJWS19FWFRfZGVidWdfdXRpbHMKDwoNTWVtb3J5VHJhY2tlcgoLCglDUFVUaW1pbmcKIwohVktfTEFZRVJfU0FNU1VOR19zd2FwY2hhaW5fcm90YXRlEg5WS19LSFJfc3VyZmFjZRIlVktfS0hSX3N1cmZhY2VfcHJvdGVjdGVkX2NhcGFiaWxpdGllcxIWVktfS0hSX2FuZHJvaWRfc3VyZmFjZRIbVktfRVhUX3N3YXBjaGFpbl9jb2xvcnNwYWNlEiBWS19LSFJfZ2V0X3N1cmZhY2VfY2FwYWJpbGl0aWVzMhIbVktfR09PR0xFX3N1cmZhY2VsZXNzX3F1ZXJ5EhtWS19FWFRfc3VyZmFjZV9tYWludGVuYW5jZTESE1ZLX0VYVF9kZWJ1Z19yZXBvcnQSHFZLX0tIUl9kZXZpY2VfZ3JvdXBfY3JlYXRpb24SIlZLX0tIUl9leHRlcm5hbF9mZW5jZV9jYXBhYmlsaXRpZXMSI1ZLX0tIUl9leHRlcm5hbF9tZW1vcnlfY2FwYWJpbGl0aWVzEiZWS19LSFJfZXh0ZXJuYWxfc2VtYXBob3JlX2NhcGFiaWxpdGllcxImVktfS0hSX2dldF9waHlzaWNhbF9kZXZpY2VfcHJvcGVydGllczIaHQjn4YACEICAgFgYtScgkICIkAkqCE1hbGktRzc4KgYSAggBGAESVQoJYXJtNjQtdjhhEAQYAiJECAISBAgIEAgaBAgIEAgiBAgIEAgqBAgBEAEyBAgIEAg6BAgEEARCBAgCEAJKBAgBEAFSBAgIEAhaBAgEEARiBAgCEAIYCCDd2ozjkrzzAg==")
	drawCmdIndex       uint64
	swapCmdIndex       uint64
)

func TestGetServerInfo(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	got, err := server.GetServerInfo(ctx)
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "got").That(got).DeepEquals(cfg.Info)
}

func TestGetAvailableStringTables(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	got, err := server.GetAvailableStringTables(ctx)
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "got").ThatSlice(got).DeepEquals([]*stringtable.Info{stringtables[0].Info})
}

func TestGetStringTable(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	got, err := server.GetStringTable(ctx, stringtables[0].Info)
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "got").That(got).DeepEquals(stringtables[0])
}

func TestImportCapture(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	got, err := server.ImportCapture(ctx, "test-capture", testCaptureData)
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "got").That(got).IsNotNil()
}

func TestGetDevices(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	got, err := server.GetDevices(ctx)
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "got").ThatSlice(got).IsNotEmpty()
}

func TestGetDevicesForReplay(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	capture, err := server.ImportCapture(ctx, "test-capture", testCaptureData)
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "capture").That(capture).IsNotNil()
	got, compatibilites, reasons, err := server.GetDevicesForReplay(ctx, capture)
	assert.For(ctx, "compatibilities").ThatInteger(len(got)).Equals(len(compatibilites))
	assert.For(ctx, "reasons").ThatInteger(len(got)).Equals(len(reasons))
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "got").ThatSlice(got).IsNotEmpty()
}

func TestGet(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	capture, err := server.ImportCapture(ctx, "test-capture", testCaptureData)
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "capture").That(capture).IsNotNil()
	T, any := reflect.TypeOf, reflect.TypeOf(struct{}{})

	for _, test := range []struct {
		path path.Node
		ty   reflect.Type
	}{
		{capture, T((*service.Capture)(nil))},
		{capture.Commands(), T((*service.Commands)(nil))},
		{capture.Command(swapCmdIndex), T((*api.Command)(nil))},
		// TODO: box.go doesn't currently support serializing structs this big.
		// See bug https://github.com/google/gapid/issues/1761
		// panic: reflect.nameFrom: name too long
		// {capture.Command(swapCmdIndex).StateAfter(), any},
		{capture.Command(swapCmdIndex).MemoryAfter(0, 0x1000, 0x1000), T((*service.Memory)(nil))},
		{capture.Command(drawCmdIndex).Mesh(nil), T((*api.Mesh)(nil))},
		{capture.CommandTree(nil), T((*service.CommandTree)(nil))},
		{capture.Report(nil, false), T((*service.Report)(nil))},
		{capture.Resources(), T((*service.Resources)(nil))},
		{capture.Command(drawCmdIndex).FramebufferAttachmentsAfter(), T((*service.FramebufferAttachments)(nil))},
	} {
		ctx = log.V{"path": test.path}.Bind(ctx)
		got, err := server.Get(ctx, test.path.Path(), nil)
		assert.For(ctx, "err").ThatError(err).Succeeded()
		if test.ty.Kind() == reflect.Interface {
			assert.For(ctx, "got").That(got).Implements(test.ty)
		} else if test.ty != any {
			assert.For(ctx, "ty").That(reflect.TypeOf(got)).Equals(test.ty)
		}
	}
}

func TestSet(t *testing.T) {
	// TODO
}

func TestFollow(t *testing.T) {
	// TODO
}

func TestProfile(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	pprof := &bytes.Buffer{}
	trace := &bytes.Buffer{}
	stop, err := server.Profile(ctx, pprof, trace, 1)
	if assert.For(ctx, "Profile").ThatError(err).Succeeded() {
		time.Sleep(time.Second)
		err := stop()
		if assert.For(ctx, "stop").ThatError(err).Succeeded() {
			assert.For(ctx, "pprof").That(pprof.Len()).NotEquals(0)
			assert.For(ctx, "trace").That(trace.Len()).NotEquals(0)
		}
	}
}

func TestGetPerformanceCounters(t *testing.T) {
	ctx, server, shutdown := setup(t)
	defer shutdown()
	data, err := server.GetPerformanceCounters(ctx)
	assert.For(ctx, "err").ThatError(err).Succeeded()
	assert.For(ctx, "data").That(data).IsNotNil()
}
