#!/bin/bash
# Copyright (C) 2017 Google Inc.
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

# Creates a distributable copy of the JRE.

if [ $# -ne 1 ]; then
	echo Expected the destination folder as an argument.
	exit
fi

DEST=$1

curl -f -L -k -O -s https://github.com/AdoptOpenJDK/openjdk11-binaries/releases/download/jdk-11.0.7%2B10/OpenJDK11U-jre_x64_mac_hotspot_11.0.7_10.tar.gz
tar xzf OpenJDK11U-jre_x64_mac_hotspot_11.0.7_10.tar.gz
rm OpenJDK11U-jre_x64_mac_hotspot_11.0.7_10.tar.gz

# Create links to JRE top-level directories
pushd jdk-11.0.7+10-jre
for i in Contents/Home/* ; do
  ln -s $i
done
popd

mv jdk-11.0.7+10-jre ${DEST}
