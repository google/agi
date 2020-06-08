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

cp -r ${JAVA_HOME}/* ${DEST}/

#### Remove unnecessary files.
# The list of files to remove is obtained by comparing the list of files from
# the Zulu JDK and JRE package from
# https://www.azul.com/downloads/zulu-community. The JRE can be approximated as a subset
# of the JDK. We get the list of files to remove with:
#
# diff -r zulu11...jdk11... zulu11...jre11... | grep '^Only in' | grep jdk11 | grep -v legal
#
# Also, the macOS Zulu JDK package contains symlinks, hence the bespoke remove().
function remove() {
  rm -rf ${DEST}/$1
}

remove bin/jar
remove bin/jarsigner
remove bin/javac
remove bin/javadoc
remove bin/javap
remove bin/jcmd
remove bin/jconsole
remove bin/jdb
remove bin/jdeprscan
remove bin/jdeps
remove bin/jhsdb
remove bin/jimage
remove bin/jinfo
remove bin/jlink
remove bin/jmap
remove bin/jmod
remove bin/jps
remove bin/jshell
remove bin/jstack
remove bin/jstat
remove bin/jstatd
remove bin/rmic
remove bin/serialver
remove conf
remove demo
remove include
remove jmods
remove legal
remove lib/ct.sym
remove lib/libattach.dylib
remove lib/libsaproc.dylib
remove lib/src.zip
remove man
