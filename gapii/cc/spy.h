/*
 * Copyright (C) 2017 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#ifndef GAPII_SPY_H
#define GAPII_SPY_H

#include <atomic>
#include <memory>
#include <unordered_map>

#include "core/cc/thread.h"
#include "gapii/cc/vulkan_spy.h"

#if TARGET_OS == GAPID_OS_FUCHSIA
#include <fidl/fuchsia.gpu.agis/cpp/fidl.h>
#endif  // TARGET_OS == GAPID_OS_FUCHSIA

namespace gapii {
struct spy_creator;
class ConnectionStream;
class Spy : public VulkanSpy {
 public:
  // get lazily constructs and returns the singleton instance to the spy.
  static Spy* get();
  ~Spy();

  CallObserver* enter(const char* name, uint32_t api);
  void exit();

  void endTraceIfRequested() override;

  void onPreEndOfFrame(CallObserver* observer, uint8_t api) override;
  void onPostEndOfFrame() override;
  void onPostFence(CallObserver* observer) override;

  inline void RegisterSymbol(const std::string& name, void* symbol) {
    mSymbols.emplace(name, symbol);
  }

  inline void* LookupSymbol(const std::string& name) const {
    const auto symbol = mSymbols.find(name);
    return (symbol == mSymbols.end()) ? nullptr : symbol->second;
  }

 private:
  Spy();

  // observeFramebuffer captures the currently bound framebuffer's color
  // buffer, and writes it to a FramebufferObservation message.
  void observeFramebuffer(CallObserver* observer, uint8_t api);

  // getFramebufferAttachmentSize attempts to retrieve the currently bound
  // framebuffer's color buffer dimensions, returning true on success or
  // false if the dimensions could not be retrieved.
  bool getFramebufferAttachmentSize(CallObserver* observer, uint32_t& width,
                                    uint32_t& height);

  // saveInitialState serializes the current global state.
  void saveInitialState();

  template <typename T>
  void saveInitialStateForApi(const char* name);

  std::unordered_map<std::string, void*> mSymbols;

  int mNumFrames;
  // The number of frames that we want to suspend capture for before
  // we start.
  std::atomic_int mSuspendCaptureFrames;

  // The connection stream to the server
  std::shared_ptr<ConnectionStream> mConnection;
  // The number of frames that we want to capture
  // 0 for manual stop, -1 for ending the trace
  std::atomic_int mCaptureFrames;
  int mObserveFrameFrequency;
  uint64_t mFrameNumber;

  bool mIgnoreFrameBoundaryDelimiters;
  bool ignoreFrameBoundaryDelimiters() override {
    return mIgnoreFrameBoundaryDelimiters;
  }

  std::unique_ptr<core::AsyncJob> mMessageReceiverJob;

#if TARGET_OS == GAPID_OS_FUCHSIA
  // Register with agis service and retrieve the Vulkan socket.
  zx_handle_t AgisRegisterAndRetrieve(uint64_t client_id);

  fidl::SyncClient<fuchsia_gpu_agis::ComponentRegistry> mAgisComponentRegistry;
#endif

  friend struct spy_creator;
};

}  // namespace gapii

#endif  // GAPII_SPY_H
