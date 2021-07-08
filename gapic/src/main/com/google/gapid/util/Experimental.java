/*
 * Copyright (C) 2020 Google Inc.
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
package com.google.gapid.util;

import com.google.common.collect.Lists;
import com.google.gapid.models.Settings;
import com.google.gapid.util.Flags.Flag;

import java.util.List;

/**
 * Command line flag definition for experimental features.
 */
public class Experimental {
  public static final Flag<Boolean> enableAll = Flags.value("experimental-enable-all", false,
      "Enable all experimental features. " +
      "Features turned on by this flag are all unstable and under development.");

  public static final Flag<Boolean> enableProfileExperiments = Flags.value("experimental-enable-profile-experiments",
      false, "Enable Profile Experiments.");

  public static final Flag<Boolean> enableUnstableFeatures = Flags.value("experimental-enable-unstable-features",
      false, "Enable various unstable features that are not ready for use yet.");

  public static List<String> getGapisFlags(boolean enableAllExperimentalFeatures) {
    List<String> args = Lists.newArrayList();
    // The --experimental-enable-all flag is a sugar flag from the UI. GAPIS knows nothing about it.
    if (enableAllExperimentalFeatures || Experimental.enableAll.get()) {
      // None at this time.
    } else {
      // None at this time.
    }
    return args;
  }

  public static boolean enableProfileExperiments(Settings settings) {
    return settings.preferences().getEnableAllExperimentalFeatures() ||
        enableAll.get() || enableProfileExperiments.get();
  }

  public static boolean enableUnstableFeatures(Settings settings) {
    return settings.preferences().getEnableAllExperimentalFeatures() ||
        enableAll.get() || enableUnstableFeatures.get();
  }
}
