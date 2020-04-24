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

import static com.google.gapid.widgets.Widgets.createTableColumn;
import static com.google.gapid.widgets.Widgets.createTableViewer;
import static com.google.gapid.widgets.Widgets.packColumns;

import com.google.gapid.perfetto.models.VulkanEventTrack;
import org.eclipse.jface.viewers.ArrayContentProvider;
import org.eclipse.jface.viewers.LabelProvider;
import org.eclipse.jface.viewers.TableViewer;
import org.eclipse.swt.SWT;
import org.eclipse.swt.layout.FillLayout;
import org.eclipse.swt.widgets.Composite;

/**
 * Displays information about a list of selected vulkan API events.
 */
public class VulkanEventsSelectionView extends Composite {
  public VulkanEventsSelectionView(Composite parent, VulkanEventTrack.Slices sel) {
    super(parent, SWT.NONE);
    setLayout(new FillLayout());

    TableViewer viewer = createTableViewer(this, SWT.NONE);
    viewer.setContentProvider(new ArrayContentProvider());
    viewer.setLabelProvider(new LabelProvider());

    createTableColumn(viewer, "Slice ID", e -> Long.toString(sel.ids.get((Integer)e)));
    createTableColumn(viewer, "Start Time", e -> Long.toString(sel.times.get((Integer)e)));
    createTableColumn(viewer, "Duration", e -> Long.toString(sel.durs.get((Integer)e)));
    createTableColumn(viewer, "Event Name", e -> sel.names.get((Integer)e));

    Integer[] rows = new Integer[sel.count];
    for (int i = 0; i < rows.length; i++) {
      rows[i] = i;
    }
    viewer.setInput(rows);
    packColumns(viewer.getTable());
  }
}
