/*
 * Copyright (C) 2021 Google Inc.
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

#ifndef STATICANALYSIS_H_
#define STATICANALYSIS_H_

#ifdef __cplusplus
extern "C" {
#endif

#include <stddef.h>
#include <stdint.h>

typedef struct instruction_counters_t {
  uint32_t alu_instructions;
  uint32_t texture_instructions;
  uint32_t branch_instructions;
  uint32_t temp_registers;
} instruction_counters_t;

instruction_counters_t performStaticAnalysis(const uint32_t*, size_t);

#ifdef __cplusplus
}
#endif

#endif  // STATICANALYSIS_H_
