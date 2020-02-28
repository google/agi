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

import com.google.gapid.perfetto.models.TrackConfig;
import com.google.gapid.widgets.DialogBase;
import com.google.gapid.widgets.Theme;

import org.eclipse.jface.viewers.CheckStateChangedEvent;
import org.eclipse.jface.viewers.ICheckStateListener;
import org.eclipse.swt.layout.FillLayout;
import org.eclipse.swt.widgets.Composite;
import org.eclipse.swt.widgets.Control;
import org.eclipse.swt.widgets.Shell;

public class FilterDialog extends DialogBase {
  private final State.ForSystemTrace state;

  public FilterDialog(Shell shell, Theme theme, State.ForSystemTrace state) {
    super(shell, theme);
    this.state = state;
  }

  @Override
  public String getTitle() {
    return "Filter Tracks";
  }

  @Override
  protected Control createDialogArea(Composite parent) {
    Composite area = (Composite)super.createDialogArea(parent);
    area.setLayout(new FillLayout());

    TrackSelector ts = new TrackSelector(area, state, theme);
    ts.tree.addCheckStateListener(new ICheckStateListener() {
      private void checkParentElement(TrackConfig.Element<?> element) {
        TrackConfig.Group parentElement = (TrackConfig.Group)element.parent;

        // Stop when root of the tree is the parent
        while (parentElement != null && !parentElement.id.equals("root")) {
          boolean childrenChecked = false;
          for (TrackConfig.Element<?> child : parentElement.tracks) {
            if (child.getChecked()) {
              childrenChecked = true;
              break;
            }
          }
          parentElement.setCheckedWithoutChildren(childrenChecked);
          parentElement = (TrackConfig.Group)parentElement.parent;
        }
      }

      @Override
      public void checkStateChanged(CheckStateChangedEvent event) {
        TrackConfig.Element<?> element = (TrackConfig.Element<?>)event.getElement();
        element.setChecked(event.getChecked());
        ts.tree.setSubtreeChecked(element, event.getChecked());
        checkParentElement(element);
        ts.tree.refresh();
      }
    });
    ts.tree.setInput(state.getTracks());
    return area;
  }

  @Override
  protected void okPressed() {
    state.update(state.getTraceTime());
    super.okPressed();
  }
}
