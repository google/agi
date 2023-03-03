/*
 * Copyright (C) 2022 Google Inc.
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

#ifdef GAPID_TARGET_OS_FUCHSIA

#include "zircon_socket_connection.h"

#include <thread>

#include <errno.h>
#include <zircon/status.h>

#include "core/cc/log.h"

namespace core {

ZirconSocketConnection::~ZirconSocketConnection() { mSocket.reset(); }

size_t ZirconSocketConnection::send(const void* data, size_t size) {
  size_t bytes_written = 0;
  zx_status_t status = mSocket.write(0u, data, size, &bytes_written);
  if (status != ZX_OK) {
    GAPID_ERROR("Failed to write data to Zircon socket: %s",
                zx_status_get_string(status));
  }
  return bytes_written;
}

size_t ZirconSocketConnection::recv(void* data, size_t size) {
  size_t bytes_read = 0;
  int attempts = 0;
  zx_status_t status = ZX_ERR_INTERNAL;
  do  {
    status = mSocket.read(0u, data, size, &bytes_read);
    if (status != ZX_OK) {
      GAPID_ERROR("Failed to read %zu bytes from Zircon socket: %s, attempts: %d",
                   size, zx_status_get_string(status), attempts);
      attempts++;
    }
    if (status == ZX_ERR_SHOULD_WAIT) {
      GAPID_INFO("Zircon socket read: ZX_ERR_SHOULD_WAIT, waiting 6s ...");
      std::this_thread::sleep_for(std::chrono::milliseconds(6000));
    }
  } while(status == ZX_ERR_SHOULD_WAIT);
  // TODO(rosasco): release code should have a reasonable timeout for the loop above.
  return bytes_read;
}

std::unique_ptr<Connection> ZirconSocketConnection::accept(int timeoutMs) {
  GAPID_FATAL("Accept is not implemented for Zircon sockets.");
  return nullptr;
}

const char* ZirconSocketConnection::error() { return strerror(errno); }

void ZirconSocketConnection::close() { mSocket.reset(); }

}  // namespace core

#endif  // GAPID_TARGET_OS_FUCHSIA
