/*
 * Copyright (C) 2021 Google Inc.
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

import static com.google.gapid.util.Paths.lastCommand;

import com.google.gapid.models.CommandStream;
import com.google.gapid.models.Models;
import com.google.gapid.models.Profile;
import com.google.gapid.proto.service.path.Path;
import com.google.gapid.util.Experimental;
import com.google.gapid.views.CommandTree.Tree;
import com.google.gapid.widgets.Widgets;

import org.eclipse.swt.SWT;
import org.eclipse.swt.widgets.Control;
import org.eclipse.swt.widgets.Menu;
import org.eclipse.swt.widgets.MenuItem;

import java.util.Arrays;
import java.util.Collection;
import java.util.List;
import java.util.logging.Logger;
import java.util.stream.Collectors;

public class CommandOptions {
  private static final Logger LOG = Logger.getLogger(Profile.class.getName());

  private CommandOptions() {
  }

  public static void CreateCommandOptionsMenu(Control parent, Widgets widgets, Tree tree, Models models) {
    final Menu optionsMenu = new Menu(parent);
    final Menu experimentsMenu = new Menu(optionsMenu);

    MenuItem editMenuItem = Widgets.createMenuItem(optionsMenu , "&Edit", SWT.MOD1 + 'E', e -> {
      CommandStream.Node node = tree.getSelection();
      if (node != null && node.getData() != null && node.getCommand() != null) {
        widgets.editor.showEditPopup(optionsMenu.getShell(), lastCommand(node.getData().getCommands()),
            node.getCommand(), node.device);
      }
    });

    MenuItem disableMenuItem;
    MenuItem isolateMenuItem;
    if (Experimental.enableProfileExperiments(models.settings)) {
      disableMenuItem = Widgets.createCheckMenuItem(optionsMenu, "Disable Command", SWT.MOD1 + 'D', e -> {
        CommandStream.Node node = tree.getSelection();
        if (node != null && node.getData() != null) {
          List<Path.Command> experimentalCommands = node.getData().getExperimentalCommandsList();
          if (widgets.experiments.isAnyCommandDisabled(experimentalCommands)) {
            widgets.experiments.enableCommands(experimentalCommands);
          } else {
            widgets.experiments.disableCommands(experimentalCommands);
          }
        }
      });

      isolateMenuItem = Widgets.createMenuItem(optionsMenu, "Isolate Command", SWT.MOD1 + 'I', e -> {
        CommandStream.Node node = tree.getSelection();
        if (node != null && node.getData() != null) {
         List<Path.Command> commands = getSiblings(node);
          if (widgets.experiments.isAnyCommandDisabled(commands)) {
            widgets.experiments.enableCommands(commands);
          } else {
            widgets.experiments.disableCommands(commands);
          }
        }
      });
    } else {
      disableMenuItem = null;
      isolateMenuItem = null;
    }

    tree.setPopupMenu(optionsMenu, node -> {
      if (node.getData() == null) {
        return false;
      }

      editMenuItem.setEnabled(false);
      if (node.getCommand() != null && CommandEditor.shouldShowEditPopup(node.getCommand())) {
        editMenuItem.setEnabled(true);
      }

      if (disableMenuItem != null && isolateMenuItem != null) {
        boolean CanBeDisabled = node.getData().getExperimentalCommandsCount() > 0;
        boolean hasDisabledChild = widgets.experiments.isAnyCommandDisabled(
            node.getData().getExperimentalCommandsList());
        disableMenuItem.setEnabled(CanBeDisabled);
        disableMenuItem.setSelection(hasDisabledChild);
        isolateMenuItem.setEnabled(CanBeDisabled &&
            node.getParent().getData().getExperimentalCommandsCount() > 1);
      }
      return true;
    });
  }

  private static List<Path.Command> getSiblings(CommandStream.Node node) {
    return Arrays.asList(node.getParent().getChildren()).stream()
        .filter(n -> n.getData().getExperimentalCommandsCount() > 0 && !n.equals(node))
        .map(n -> n.getData().getExperimentalCommandsList())
        .flatMap(Collection::stream)
        .collect(Collectors.toList());
  }
}
