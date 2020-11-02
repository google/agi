/*
 * Copyright (C) 2017 Google Inc.
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
package com.google.gapid.views;

import org.eclipse.swt.widgets.Control;

/**
 * The content of a tab in the main UI.
 */
public interface TabContent {
  /**
   * Reinitializes this tab content from the current state of the models. Called if the tab was
   * created after the UI has already been visible for some time.
   */
  public default void reinitialize() { /* do nothing */ }

  /**
   * @return the {@link Control} for this tab content.
   */
  public Control getControl();

  /**
   * Dispose this tab content.
   */
  public default void dispose() {
    getControl().dispose();
  }

  /**
   * Returns true if tab content supports pinning.
   */
  public default boolean supportsPinning() {
    return false;
  }

  /**
   * Returns true if tab content currently can be pinned.
   */
  public default boolean isPinnable() {
    return false;
  }

  /**
   * Returns true if tab content is pinned.
   */
  public default boolean isPinned() {
    return false;
  }

  /**
   * Preserves the tab content until it is disposed.
   */
  public default void pin() { /* do nothing */ }
}
