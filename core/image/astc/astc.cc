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

#include <stdio.h>
#include <string.h>

#include "astc.h"
#include "third_party/astc-encoder/Source/astcenccli_internal.h"

int init_astc_for_decode(astcenc_profile profile,
    astc_compressed_image& input_image, astcenc_config& config, loggerFunc logger) {
    unsigned int block_x = input_image.block_x;
	unsigned int block_y = input_image.block_y;
	unsigned int block_z = input_image.block_z;

    astcenc_preset preset = ASTCENC_PRE_FASTEST;
    unsigned int flags = 0;
    astcenc_error status = astcenc_config_init(profile, block_x, block_y, block_z, preset, flags, config);
    if (status == ASTCENC_ERR_BAD_BLOCK_SIZE) {
        logger("ERROR: Block size is invalid\n");
		return 1;
	} else if (status == ASTCENC_ERR_BAD_CPU_ISA) {
		logger("ERROR: Required SIMD ISA support missing on this CPU\n");
		return 1;
	} else if (status == ASTCENC_ERR_BAD_CPU_FLOAT) {
		logger("ERROR: astcenc must not be compiled with -ffast-math\n");
		return 1;
	} else if (status != ASTCENC_SUCCESS) {
        char error[255]{0};
		sprintf(error, "ERROR: Init config failed with %s\n", astcenc_get_error_string(status));
        logger(error);
		return 1;
	}
    return 0;
}

astc_compressed_image create_astc_compressed_image(uint8_t* data, uint32_t width, uint32_t height,
    uint32_t block_width, uint32_t block_height) {
    uint32_t block_x = (width + block_width - 1) / block_width;
    uint32_t block_y = (height + block_height - 1) / block_height;

    astc_compressed_image image{};
    image.dim_x = width;
    image.dim_y = height;
    image.dim_z = 1;
    image.block_x = block_width;
    image.block_y = block_height;
    image.block_z = 1;
    image.data = data;
	image.data_len = block_x * block_y * 16;
    return image;
}

void write_image(uint8_t* buf, astcenc_image* img) {
    uint8_t*** data8 = static_cast<uint8_t***>(img->data);
    for (unsigned int y = 0; y < img->dim_y; y++) {
        const uint8_t* src = data8[0][y + img->dim_pad] + (4 * img->dim_pad);
        uint8_t* dst = buf + y * img->dim_x * 4;

        for (unsigned int x = 0; x < img->dim_x; x++) {
            dst[4 * x]     = src[4 * x];
            dst[4 * x + 1] = src[4 * x + 1];
            dst[4 * x + 2] = src[4 * x + 2];
            dst[4 * x + 3] = src[4 * x + 3];
        }
    }
}

extern "C" int decompress_astc(uint8_t* input_image_raw, uint8_t* output_image_raw,
    uint32_t width, uint32_t height, uint32_t block_width, uint32_t block_height, loggerFunc logger) {

    astc_compressed_image input_image = create_astc_compressed_image(input_image_raw,
        width, height, block_width, block_height);

    astcenc_profile profile = ASTCENC_PRF_LDR;
    astcenc_config config {};
    if (init_astc_for_decode(profile, input_image, config, logger) != 0) {
        logger("ASTC initialisation failed");
        return 1;
    }

    unsigned int thread_count = get_cpu_count();
	astcenc_context* codec_context;
	astcenc_error codec_status = astcenc_context_alloc(config, thread_count, &codec_context);
	if (codec_status != ASTCENC_SUCCESS) {
        char error[255]{0};
		sprintf(error,"ERROR: Codec context alloc failed: %s\n", astcenc_get_error_string(codec_status));
		logger(error);
        return 1;
	}

    unsigned int bitness = 8;
    astcenc_image* output_image = alloc_image(bitness,
        input_image.dim_x, input_image.dim_y, input_image.dim_z, 0);

    astcenc_swizzle swz_decode{ ASTCENC_SWZ_R, ASTCENC_SWZ_G, ASTCENC_SWZ_B, ASTCENC_SWZ_A };
    codec_status = astcenc_decompress_image(codec_context, input_image.data, input_image.data_len,
		                                        *output_image, swz_decode);
    if (codec_status != ASTCENC_SUCCESS) {
        char error[255]{0};
        sprintf(error, "ERROR: Codec decompress failed: %s\n", astcenc_get_error_string(codec_status));
        logger(error);
        return 1;
    }

    write_image(output_image_raw, output_image);
    free_image(output_image);
	astcenc_context_free(codec_context);
    return 0;
}
