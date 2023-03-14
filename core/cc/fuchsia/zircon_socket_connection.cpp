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

#include <errno.h>
#include <zircon/status.h>

#include <thread>

#include "core/cc/log.h"

#define ERR_IF_COND(cond, ...) \
  if ((cond)) {                \
    GAPID_ERROR(__VA_ARGS__);  \
  }

namespace core {

ZirconSocketConnection::~ZirconSocketConnection() { mSocket.reset(); }

size_t ZirconSocketConnection::send(const void* data, size_t size) {
  size_t bytes_written = 0;
  zx_status_t status = mSocket.write(0u, data, size, &bytes_written);
  ERR_IF_COND(status != ZX_OK, "Failed to write data to Zircon socket: %s",
              zx_status_get_string(status));
  return bytes_written;
}

size_t ZirconSocketConnection::recv(void* data, const size_t size) {
  const auto p = static_cast<uint8_t*>(data);
  size_t total_bytes_read = 0;
  zx_status_t read_status = ZX_ERR_INTERNAL;
  zx_status_t wait_status = ZX_ERR_INTERNAL;
  zx_signals_t observed_signal = 0;
  do {
    size_t bytes_read = 0;
    read_status = mSocket.read(0u, p + total_bytes_read,
                               size - total_bytes_read, &bytes_read);
    switch (read_status) {
      case ZX_OK:
        total_bytes_read += bytes_read;
        break;
      case ZX_ERR_SHOULD_WAIT:
        wait_status =
            mSocket.wait_one(ZX_SOCKET_READABLE | ZX_SOCKET_PEER_CLOSED,
                             zx::time::infinite(), &observed_signal);

        if (wait_status != ZX_OK || observed_signal & ZX_SOCKET_PEER_CLOSED) {
          GAPID_ERROR("ZirconSocketConnection wait_status: %s observed: %d",
                      zx_status_get_string(wait_status), observed_signal);
          return 0;
        }
        break;
      default:
        GAPID_ERROR("ZirconSocketConnection unexpected read_status: %s",
                    zx_status_get_string(read_status));
        return 0;
    }
  } while ((read_status == ZX_OK || observed_signal & ZX_SOCKET_READABLE) &&
           total_bytes_read < size);

  ERR_IF_COND(total_bytes_read != size,
              "Failed to read %zu bytes from Zircon socket. read_status: %s",
              size, zx_status_get_string(read_status));

  return total_bytes_read;
}

std::unique_ptr<Connection> ZirconSocketConnection::accept(int timeoutMs) {
  GAPID_FATAL("Accept is not implemented for Zircon sockets.");
  return nullptr;
}

const char* ZirconSocketConnection::error() { return strerror(errno); }

void ZirconSocketConnection::close() { mSocket.reset(); }

}  // namespace core

#endif  // GAPID_TARGET_OS_FUCHSIA
