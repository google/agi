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

	void MarkDeadAddressRange(const ReplayAddressRange& replayAddressRange) {
		const std::byte dead[2] = { std::byte(0xDE), std::byte(0xAD) };
		for(int i = 0; i < replayAddressRange.length(); ++i) {
			replayAddressRange.baseAddress().bytePtr()[i] = dead[i %2];
		}
	}

	ReplayAddress MemoryRemapper::AddMapping(const MemoryObservation& observation) {

		const CaptureAddress& captureAddress = observation.captureAddress();

		auto iter = CaptureAddressRangeIter(captureAddress);
		if(iter != captureAddressRanges_.end()) {

			const CaptureAddressRange captureAddressRange = iter->first;

			const intptr_t offset = captureAddress.bytePtr() - captureAddressRange.baseAddress().bytePtr();
			if(offset < captureAddressRange.length()) {
				throw AddressAlreadyMappedException();
			}
		}

		const size_t mappingLength = observation.resourceGenerator()->length();

		std::byte* replayAllocation = new std::byte [mappingLength];
		const ReplayAddress replayAddress(replayAllocation);

		observation.resourceGenerator()->generate(replayAddress);

		const CaptureAddressRange captureAddressRange(captureAddress, mappingLength);
		const ReplayAddressRange replayAddressRange(replayAddress, mappingLength);

		const auto newMapping = std::make_pair(captureAddressRange, replayAddressRange);
		captureAddressRanges_.emplace(newMapping);

		return replayAddress;
	}

	void MemoryRemapper::RemoveMapping(const CaptureAddress& captureAddress) {

		auto iter = CaptureAddressRangeIter(captureAddress);
		if(iter == captureAddressRanges_.end()) throw AddressNotMappedException();

		const CaptureAddressRange captureAddressRange = iter->first;
		if(captureAddressRange.baseAddress() != captureAddress) {

			const intptr_t offset = captureAddress.bytePtr() -captureAddressRange.baseAddress().bytePtr();
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
		MarkDeadAddressRange(replayAddressRange);
#endif
		delete [] replayAddressRange.baseAddress().bytePtr();
		captureAddressRanges_.erase(iter);
	}

	ReplayAddress MemoryRemapper::RemapCaptureAddress(const CaptureAddress& captureAddress) const {

		// Get an iterator to the last address range starting before captureAddress, if one exists.
		auto iter = CaptureAddressRangeIter(captureAddress);
		if(iter == captureAddressRanges_.end()) {
			// If there are no address ranges beginning before captureAddress then captureAddress has to
			// point to unmapped memory, so throw an exception.
			throw AddressNotMappedException();
		}

		const CaptureAddressRange captureAddressRange = iter->first;
		const ReplayAddressRange replayAddressRange = iter->second;

		// Compute the offset from the start of the last address range before captureAddress to captureAddress
		// itself. If this offset is less than the size of the address range, then captureAddress points to
		// memory mapped inside that address range. If the offset is larger than the previous address range's
		// length, then captureAddress points to unmapped memory between two consecutive address ranges.
		const intptr_t offset = captureAddress.bytePtr() -captureAddressRange.baseAddress().bytePtr();
		if(offset >= captureAddressRange.length()) {
			throw AddressNotMappedException();
		}

		const ReplayAddress replayAddress(replayAddressRange.baseAddress().bytePtr() +offset);
		return replayAddress;
	}

	// CaptureAddressRangeIter returns an interator to the last address range starting before captureAddress
	// or it returns .end() in the case that no such address range exists.
	std::map<CaptureAddressRange, ReplayAddressRange>::const_iterator
	MemoryRemapper::CaptureAddressRangeIter(const CaptureAddress& captureAddress) const {

		// Get an iterator to the first address range starting after captureAddress using upper_bound().
		auto nextIter = captureAddressRanges_.upper_bound(CaptureAddressRange(captureAddress, 0));
		if(nextIter != captureAddressRanges_.begin()) {
			// If we have a valid interator to the next address range, then return the iterator one before it.
			// This is either the address range that captureAddress is part of, or the last address range
			// before captureAddress (captureAddress is between two address ranges in unmapped memory).
			return --nextIter;
		}

		// If there is no address range starting before captureAddress, then captureAddress
		// cannot be part of any active address range, and we will return an invalid pointer.
		return captureAddressRanges_.end();
	}

}  // namespace replay2
}  // namespace agi
