// Copyright (C) 2022 Google Inc.
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

#include "memory_remapper.h"

namespace agi {
namespace replay2 {

	ReplayAddress MemoryRemapper::AddMapping(const MemoryObservation& observation)
	{
		const CaptureAddress captureAddress = observation.captureAddress();

		auto iter = CaptureAddressRangeIter(captureAddress);
		if(iter != captureAddressRanges_.end()) {

			const CaptureAddressRange captureAddressRange = iter->first;

			const intptr_t offset = captureAddress.charPtr() -captureAddressRange.baseAddress().charPtr();
			if(offset < captureAddressRange.length()) {
				throw AddressAlreadyMappedException();
			}
		}

		const size_t mappingLength = observation.resourceGenerator()->length();

		char* replayAllocation = new char [mappingLength];
		const ReplayAddress replayAddress(replayAllocation);

		observation.resourceGenerator()->generate(replayAddress);

		const CaptureAddressRange captureAddressRange = CaptureAddressRange(captureAddress, mappingLength);
		const ReplayAddressRange replayAddressRange = ReplayAddressRange(replayAddress, mappingLength);

		const auto newMapping = std::make_pair(captureAddressRange, replayAddressRange);
		captureAddressRanges_.insert(newMapping);

		return replayAddress;
	}

	void MemoryRemapper::RemoveMapping(const CaptureAddress& captureAddress)
	{
		auto iter = CaptureAddressRangeIter(captureAddress);
		if(iter == captureAddressRanges_.end()) throw AddressNotMappedException();

		const CaptureAddressRange captureAddressRange = iter->first;
		if(captureAddressRange.baseAddress() != captureAddress) {

			const intptr_t offset = captureAddress.charPtr() -captureAddressRange.baseAddress().charPtr();
			if(offset >= captureAddressRange.length()) {
				throw AddressNotMappedException();
			}
			else {
				throw RemoveMappingOffsetAddressException();
			}
		}

		const ReplayAddressRange replayAddressRange = iter->second;

#ifndef NDEBUG
		// In debug we'll splat all released memory with 0xDEAD before releasing it to help with debugging.
		const unsigned char dead[2] = { 0xDE, 0xAD };
		for(int i = 0; i < replayAddressRange.length(); ++i) {
			replayAddressRange.baseAddress().charPtr()[i] = dead[i %2];
		}
#endif
		delete [] replayAddressRange.baseAddress().charPtr();
		captureAddressRanges_.erase(iter);
	}

	ReplayAddress MemoryRemapper::RemapCaptureAddress(const CaptureAddress& captureAddress) const
	{
		auto iter = CaptureAddressRangeIter(captureAddress);
		if(iter == captureAddressRanges_.end()) {
			throw AddressNotMappedException();
		}

		const CaptureAddressRange captureAddressRange = iter->first;
		const ReplayAddressRange replayAddressRange = iter->second;

		const intptr_t offset = captureAddress.charPtr() -captureAddressRange.baseAddress().charPtr();
		if(offset >= captureAddressRange.length()) {
			throw AddressNotMappedException();
		}

		const ReplayAddress replayAddress(replayAddressRange.baseAddress().charPtr() +offset);
		return replayAddress;
	}

	std::map<CaptureAddressRange, ReplayAddressRange>::const_iterator
	MemoryRemapper::CaptureAddressRangeIter(const CaptureAddress& captureAddress) const
	{
		auto nextIter = captureAddressRanges_.upper_bound(CaptureAddressRange(captureAddress, 0));
		if(nextIter != captureAddressRanges_.begin()) {
			return --nextIter;
		}
		return captureAddressRanges_.end();
	}

}  // namespace replay2
}  // namespace agi
