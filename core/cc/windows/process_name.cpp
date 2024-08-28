/*
 * Copyright (C) 2019 Google Inc.
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

#include <Shlwapi.h>

#include <cstdint>
#include <string>

namespace core {

std::string get_process_name() {
  char modulename[MAX_PATH + 1];
  memset(modulename, 0, MAX_PATH + 1);
  if (GetModuleFileNameA(NULL, modulename, MAX_PATH) == 0) {
    return "";
  }
  return PathFindFileNameA(modulename);
}

uint64_t get_process_id() {
  return static_cast<uint64_t>(GetCurrentProcessId());
}

}  // namespace core
