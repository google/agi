#!/usr/bin/env python3

# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This Swarming bot test script uses gapit to perform a system_profile test.

import argparse
import botutil
import json
import os
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('agi_dir', help='Path to AGI build')
    parser.add_argument('out_dir', help='Path to output directory')
    args = parser.parse_args()

    #### Early checks and sanitization
    assert os.path.isdir(args.agi_dir)
    agi_dir = os.path.normpath(args.agi_dir)
    assert os.path.isdir(args.out_dir)
    out_dir = os.path.normpath(args.out_dir)

    #### Test parameters
    test_params = {}
    required_keys = ['apk', 'package', 'activity', 'perfetto_config']
    botutil.load_params(test_params, required_keys=required_keys)

    #### Install APK
    botutil.install_apk(test_params)

    #### Retrieve device-specific perfetto config
    p = botutil.adb(['shell', 'getprop', 'ro.product.device'])
    device = p.stdout.rstrip()
    if not device in test_params['perfetto_config'].keys():
        botutil.log('Error: no perfetto config found for device: ' + device)
        return 1
    perfetto_config = test_params['perfetto_config'][device]
    if not os.path.isfile(perfetto_config):
        botutil.log('Error: perfetto config file not found: ' + perfetto_config)
        return 1

    #### Trace the app
    gapit = os.path.join(agi_dir, 'gapit')
    perfetto_trace = os.path.join(out_dir, test_params['package'] + '.perfetto')
    cmd = [
        gapit, 'trace',
        '-api', 'perfetto',
        '-for', '5s',
        '-perfetto', perfetto_config,
        '-out', perfetto_trace
    ]

    if 'additionalargs' in test_params.keys():
        cmd += ['-additionalargs', test_params['additionalargs']]

    cmd += [test_params['package'] + '/' + test_params['activity']]

    p = botutil.runcmd(cmd)
    if p.returncode != 0:
        return 1

    #### Stop the app asap for device cool-down
    botutil.adb(['shell', 'am', 'force-stop', test_params['package']])

    #### Check perfetto trace validity by formatting it to JSON
    perfetto_json = perfetto_trace.replace('.perfetto', '.json')
    cmd = [
        gapit, 'perfetto',
        '-mode', 'metrics',
        '-format', 'json',
        '-out', perfetto_json,
        perfetto_trace
    ]
    p = botutil.runcmd(cmd)
    return p.returncode


if __name__ == '__main__':
    sys.exit(main())
