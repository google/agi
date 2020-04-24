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

package com.google.gapid.perfetto.views;

import static com.google.gapid.perfetto.TimeSpan.timeToString;
import static com.google.gapid.widgets.Widgets.createBoldLabel;
import static com.google.gapid.widgets.Widgets.createComposite;
import static com.google.gapid.widgets.Widgets.createLabel;
import static com.google.gapid.widgets.Widgets.withIndents;
import static com.google.gapid.widgets.Widgets.withLayoutData;
import static com.google.gapid.widgets.Widgets.withMargin;
import static com.google.gapid.widgets.Widgets.withSpans;

import com.google.common.collect.Iterables;
import com.google.gapid.perfetto.models.VulkanEventTrack;
import org.eclipse.swt.SWT;
import org.eclipse.swt.layout.GridData;
import org.eclipse.swt.layout.GridLayout;
import org.eclipse.swt.widgets.Composite;

/**
 * Displays information about a selected Vulkan API event.
 */
public class VulkanEventSelectionView extends Composite {
  private static final int PROPERTIES_PER_PANEL = 8;
  private static final int PANEL_INDENT = 25;

  public VulkanEventSelectionView(Composite parent, State state, VulkanEventTrack.Slices slice) {
    super(parent, SWT.NONE);
    setLayout(withMargin(new GridLayout(2, false), 0, 0));

    if (slice.count != 1) {
      throw new IllegalArgumentException("Slice count != 1. Should only use VulkanEventSelectionView for a single slice.");
    }

    Composite main = withLayoutData(createComposite(this, new GridLayout(2, false)),
        new GridData(SWT.LEFT, SWT.TOP, false, false));
    withLayoutData(createBoldLabel(main, "Slice:"), withSpans(new GridData(), 2, 1));

    createLabel(main, "Name:");
    createLabel(main, slice.names.get(0));

    createLabel(main, "Time:");
    createLabel(main, timeToString(slice.times.get(0) - state.getTraceTime().start));

    createLabel(main, "Duration:");
    createLabel(main, timeToString(slice.durs.get(0)));

    createLabel(main, "Command Buffer:");
    createLabel(main, Long.toString(slice.commandBuffers.get(0)));

    createLabel(main, "Submission ID:");
    createLabel(main, Long.toString(slice.submissionIds.get(0)));

    if (!slice.argSets.get(0).isEmpty()) {
      String[] keys = Iterables.toArray(slice.argSets.get(0).keys(), String.class);
      int panels = (keys.length + PROPERTIES_PER_PANEL - 1) / PROPERTIES_PER_PANEL;
      Composite props = withLayoutData(createComposite(this, new GridLayout(2 * panels, false)),
          withIndents(new GridData(SWT.LEFT, SWT.TOP, false, false), PANEL_INDENT, 0));
      withLayoutData(createBoldLabel(props, "Properties:"),
          withSpans(new GridData(), 2 * panels, 1));

      for (int i = 0; i < keys.length && i < PROPERTIES_PER_PANEL; i++) {
        int cols = (keys.length - i + PROPERTIES_PER_PANEL - 1) / PROPERTIES_PER_PANEL;
        for (int c = 0; c < cols; c++) {
          withLayoutData(createLabel(props, keys[i + c * PROPERTIES_PER_PANEL] + ":"),
              withIndents(new GridData(), (c == 0) ? 0 : PANEL_INDENT, 0));
          createLabel(props, String.valueOf(slice.argSets.get(0).get(keys[i + c * PROPERTIES_PER_PANEL])));
        }
        if (cols != panels) {
          withLayoutData(createLabel(props, ""), withSpans(new GridData(), 2 * (panels - cols), 1));
        }
      }
    }
  }
}
