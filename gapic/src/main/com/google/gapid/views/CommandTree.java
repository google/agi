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

import static com.google.gapid.perfetto.views.StyleConstants.threadStateSleeping;
import static com.google.gapid.perfetto.views.StyleConstants.mainGradient;
import static com.google.gapid.image.Images.noAlpha;
import static com.google.gapid.models.ImagesModel.THUMB_SIZE;
import static com.google.gapid.util.Colors.getRandomColor;
import static com.google.gapid.util.Colors.lerp;
import static com.google.gapid.util.Loadable.MessageType.Error;
import static com.google.gapid.widgets.Widgets.createComposite;
import static com.google.gapid.widgets.Widgets.createLabel;
import static com.google.gapid.widgets.Widgets.createTableColumn;
import static com.google.gapid.widgets.Widgets.createButton;
import static com.google.gapid.widgets.Widgets.createTableViewer;
import static com.google.gapid.widgets.Widgets.createTextbox;
import static com.google.gapid.widgets.Widgets.packColumns;
import static com.google.gapid.widgets.Widgets.withIndents;
import static com.google.gapid.widgets.Widgets.withMargin;
import static com.google.gapid.widgets.Widgets.withLayoutData;

import com.google.common.collect.Lists;
import com.google.common.collect.Maps;
import com.google.common.collect.Sets;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.gapid.models.Analytics.View;
import com.google.gapid.models.Capture;
import com.google.gapid.models.CommandStream;
import com.google.gapid.models.CommandStream.CommandIndex;
import com.google.gapid.models.CommandStream.Node;
import com.google.gapid.models.Follower;
import com.google.gapid.models.Models;
import com.google.gapid.models.Profile;
import com.google.gapid.models.Settings;
import com.google.gapid.perfetto.canvas.Fonts;
import com.google.gapid.perfetto.canvas.Panel;
import com.google.gapid.perfetto.canvas.PanelCanvas;
import com.google.gapid.perfetto.canvas.RenderContext;
import com.google.gapid.perfetto.TimeSpan;
import com.google.gapid.perfetto.Unit;
import com.google.gapid.perfetto.models.CounterInfo;
import com.google.gapid.perfetto.views.TraceConfigDialog.GpuCountersDialog;
import com.google.gapid.proto.device.GpuProfiling;
import com.google.gapid.proto.device.GpuProfiling.GpuCounterDescriptor.GpuCounterGroup;
import com.google.gapid.proto.device.GpuProfiling.GpuCounterDescriptor.GpuCounterSpec;
import com.google.gapid.proto.service.Service;
import com.google.gapid.proto.service.Service.ClientAction;
import com.google.gapid.proto.service.api.API;
import com.google.gapid.proto.service.path.Path;
import com.google.gapid.proto.SettingsProto;
import com.google.gapid.proto.SettingsProto.UI.PerformancePreset;
import com.google.gapid.rpc.Rpc;
import com.google.gapid.rpc.RpcException;
import com.google.gapid.rpc.SingleInFlight;
import com.google.gapid.rpc.UiCallback;
import com.google.gapid.util.Events;
import com.google.gapid.util.Experimental;
import com.google.gapid.util.Loadable;
import com.google.gapid.util.Messages;
import com.google.gapid.util.MoreFutures;
import com.google.gapid.util.SelectionHandler;
import com.google.gapid.views.Formatter.Style;
import com.google.gapid.views.Formatter.StylingString;
import com.google.gapid.widgets.LinkifiedTreeWithImages;
import com.google.gapid.widgets.LoadableImage;
import com.google.gapid.widgets.LoadableImageWidget;
import com.google.gapid.widgets.LoadablePanel;
import com.google.gapid.widgets.EventsFilter;
import com.google.gapid.widgets.Theme;
import com.google.gapid.widgets.Widgets;

import org.eclipse.jface.dialogs.IDialogConstants;
import org.eclipse.jface.viewers.ArrayContentProvider;
import org.eclipse.jface.viewers.IStructuredSelection;
import org.eclipse.jface.viewers.LabelProvider;
import org.eclipse.jface.viewers.TableViewer;
import org.eclipse.jface.viewers.TreePath;
import org.eclipse.jface.viewers.TreeViewerColumn;
import org.eclipse.jface.window.Window;
import org.eclipse.swt.SWT;
import org.eclipse.swt.custom.SashForm;
import org.eclipse.swt.graphics.Color;
import org.eclipse.swt.graphics.ImageData;
import org.eclipse.swt.graphics.Point;
import org.eclipse.swt.graphics.RGBA;
import org.eclipse.swt.layout.FillLayout;
import org.eclipse.swt.layout.GridData;
import org.eclipse.swt.layout.GridLayout;
import org.eclipse.swt.layout.RowData;
import org.eclipse.swt.layout.RowLayout;
import org.eclipse.swt.widgets.Button;
import org.eclipse.swt.widgets.Composite;
import org.eclipse.swt.widgets.Control;
import org.eclipse.swt.widgets.Event;
import org.eclipse.swt.widgets.Label;
import org.eclipse.swt.widgets.Shell;
import org.eclipse.swt.widgets.Text;
import org.eclipse.swt.widgets.TreeItem;

import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;
import java.util.TreeSet;
import java.util.concurrent.ExecutionException;
import java.util.logging.Logger;

/**
 * API command view displaying the commands with their hierarchy grouping in a tree.
 */
public class CommandTree extends Composite
    implements Tab, Capture.Listener, CommandStream.Listener, Profile.Listener {
  protected static final Logger LOG = Logger.getLogger(CommandTree.class.getName());
  private static final String COMMAND_INDEX_HOVER = "Double click to copy index. Use Ctrl+G to jump to a given command index.";
  private static final String COMMAND_INDEX_DSCRP = "Command index: ";

  private final Models models;
  private final LoadablePanel<Tree> loading;
  protected final Tree tree;
  private final Label commandIdx;
  private final SelectionHandler<Control> selectionHandler;

  private final Color buttonColor;
  private Set<Integer> visibleMetrics = Sets.newHashSet();  // identified by metric.id
  private final PresetsBar presetsBar;
  private boolean showEstimate = true;

  public CommandTree(Composite parent, Models models, Widgets widgets) {
    super(parent, SWT.NONE);
    this.models = models;

    this.buttonColor = getDisplay().getSystemColor(SWT.COLOR_LIST_BACKGROUND);
    setLayout(new GridLayout(1, false));

    int numberOfButtonsPerRow = 2;
    if (Experimental.enableProfileExperiments(models.settings)) {
      numberOfButtonsPerRow = 3;
    }

    Composite buttonsComposite = createComposite(this, new GridLayout(numberOfButtonsPerRow, false));
    buttonsComposite.setLayoutData(new GridData(SWT.LEFT, SWT.TOP, true, false));

    EventsFilter eventsFilter = new EventsFilter(buttonsComposite);
    loading = LoadablePanel.create(this, widgets, p -> new Tree(p, models, widgets));
    tree = loading.getContents();
    commandIdx = createLabel(this, COMMAND_INDEX_DSCRP);
    commandIdx.setToolTipText(COMMAND_INDEX_HOVER);
    commandIdx.addListener(SWT.MouseDoubleClick, e -> {
      if (commandIdx.getText().length() > COMMAND_INDEX_DSCRP.length()) {
        widgets.copypaste.setContents(commandIdx.getText().substring(COMMAND_INDEX_DSCRP.length()));
      }
    });

    Button toggleButton = createButton(buttonsComposite, SWT.FLAT, "Estimate / Confidence Range",
        buttonColor, e -> toggleEstimateOrRange());
    toggleButton.setImage(widgets.theme.swap());
    toggleButton.setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, false, false));

    if (Experimental.enableProfileExperiments(models.settings)) {
      Button experimentsButton =createButton(buttonsComposite, SWT.FLAT, "Experiments",buttonColor,
          e -> widgets.experiments.showExperimentsPopup(getShell()));
      experimentsButton.setImage(widgets.theme.science());
      experimentsButton.setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, false, false));
    }

    Button filterButton = createButton(buttonsComposite, SWT.FLAT, "Filter Counters", buttonColor, e -> {
      GpuCountersDialog dialog = new GpuCountersDialog(
          getShell(), widgets.theme, getCounterSpecs(), Lists.newArrayList(visibleMetrics));
      if (dialog.open() == Window.OK) {
        visibleMetrics = Sets.newHashSet(dialog.getSelectedIds());
        updateTree(false);
      }
    });
    filterButton.setImage(widgets.theme.more());
    filterButton.setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, false, false));

    presetsBar = new PresetsBar(buttonsComposite, models.settings, widgets.theme);
    presetsBar.setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, true, false, numberOfButtonsPerRow, 1));

    eventsFilter.setLayoutData(new GridData(SWT.FILL, SWT.TOP, true, false));
    loading.setLayoutData(new GridData(SWT.FILL, SWT.FILL, true, true));
    commandIdx.setLayoutData(withIndents(new GridData(SWT.FILL, SWT.FILL, true, false), 3, 0));

    SashForm splitter = new SashForm(this, SWT.VERTICAL);
    Composite bottom = withLayoutData(createComposite(splitter, new FillLayout()),
        new GridData(SWT.FILL, SWT.BOTTOM, true, true));

    models.capture.addListener(this);
    models.commands.addListener(this);
    models.profile.addListener(this);
    addListener(SWT.Dispose, e -> {
      models.capture.removeListener(this);
      models.commands.removeListener(this);
      models.profile.removeListener(this);
    });

    eventsFilter.addListener(Events.FilterEvents,
      e -> filterEvents(
        (e.detail & Events.HIDE_HOST_COMMANDS) != 0,
        (e.detail & Events.HIDE_BEGIN_END) != 0,
        (e.detail & Events.HIDE_DEVICE_SYNC) != 0
      )
    );

    selectionHandler = new SelectionHandler<Control>(LOG, tree.getControl()) {
      @Override
      protected void updateModel(Event e) {
        models.analytics.postInteraction(View.Commands, ClientAction.Select);
        CommandStream.Node node = tree.getSelection();
        if (node != null) {
          CommandIndex index = node.getIndex();
          if (index == null) {
            models.commands.load(node, () -> models.commands.selectCommands(node.getIndex(), false));
          } else {
            commandIdx.setText(COMMAND_INDEX_DSCRP + node.getIndexString());
            models.commands.selectCommands(index, false);
          }
          models.profile.linkCommandToGpuGroup(node.getCommandStart());
        }
      }
    };

    CommandOptions.CreateCommandOptionsMenu(tree.getControl(), widgets, tree, this.models);

    tree.registerAsCopySource(widgets.copypaste, node -> {
      models.analytics.postInteraction(View.Commands, ClientAction.Copy);
      Service.CommandTreeNode data = node.getData();
      if (data == null) {
        // Copy before loaded. Not ideal, but this is unlikely.
        return new String[] { "Loading..." };
      }

      StringBuilder result = new StringBuilder();
      if (data.getGroup().isEmpty() && data.hasCommands()) {
        result.append(data.getCommands().getTo(0)).append(": ");
        API.Command cmd = node.getCommand();
        if (cmd == null) {
          // Copy before loaded. Not ideal, but this is unlikely.
          result.append("Loading...");
        } else {
          result.append(Formatter.toString(cmd, models.constants::getConstants));
        }
      } else {
        result.append(data.getCommands().getFrom(0)).append(": ").append(data.getGroup());
      }
      return new String[] { result.toString() };
    }, true);
  }

  private void filterEvents(Boolean hideHostCommands, Boolean hideBeginEnd, Boolean hideDeviceSync) {
    models.commands.reloadCommandTree(hideHostCommands, hideBeginEnd, hideDeviceSync);
  }

  protected void select(TreePath path) {
    models.commands.selectCommands(((CommandStream.Node)path.getLastSegment()).getIndex(), true);
  }

  @Override
  public Control getControl() {
    return this;
  }

  @Override
  public void reinitialize() {
    updateTree(false);
  }

  @Override
  public void onCaptureLoadingStart(boolean maintainState) {
    updateTree(true);
  }

  @Override
  public void onCaptureLoaded(Loadable.Message error) {
    if (error != null) {
      loading.showMessage(Error, Messages.CAPTURE_LOAD_FAILURE);
    }
  }

  @Override
  public void onCommandsLoaded() {
    updateTree(false);
  }

  @Override
  public void onCommandsSelected(CommandIndex index) {
    selectionHandler.updateSelectionFromModel(() -> models.commands.getTreePath(index.getNode()).get(), tree::setSelection);
  }

  @Override
  public void onProfileLoadingStart() {
    tree.profileLoadingError = null;
  }

  @Override
  public void onProfileLoaded(Loadable.Message error) {
    tree.profileLoadingError = error;
    tree.refresh();

    presetsBar.refresh();
    visibleMetrics = getCounterSpecs().stream()
        .mapToInt(GpuCounterSpec::getCounterId).boxed().collect(Collectors.toSet());
    updateTree(false);
  }

  private void updateTree(boolean assumeLoading) {
    if (assumeLoading || !models.commands.isLoaded()) {
      loading.startLoading();
      tree.setInput(null);
      commandIdx.setText(COMMAND_INDEX_DSCRP);
      return;
    }

    loading.stopLoading();
    tree.setInput(models.commands.getData());

    if (models.commands.getSelectedCommands() != null) {
      onCommandsSelected(models.commands.getSelectedCommands());
    }

    tree.addGpuPerformanceColumns(visibleMetrics);
    tree.packColumn();
  }

  protected static class Tree extends LinkifiedTreeWithImages<CommandStream.Node, String> {
    private static final float COLOR_INTENSITY = 0.15f;
    private static final int DURATION_WIDTH = 95;

    protected final Models models;
    private final Widgets widgets;
    private final Map<Long, Color> threadBackgroundColors = Maps.newHashMap();
    private Loadable.Message profileLoadingError;

    public Tree(Composite parent, Models models, Widgets widgets) {
      super(parent, SWT.H_SCROLL | SWT.V_SCROLL | SWT.MULTI, widgets);
      this.models = models;
      this.widgets = widgets;

      setUpStateForColumnAdding();
    }

    protected void addGpuPerformanceColumns(Set<Integer> visibleMetrics) {
      // The command tree's GPU performances are calculated from client's side.

      deleteAllColumns();
      setUpStateForColumnAdding();

      if(models.profile.getData() != null && models.profile.getData().getGpuPerformance() != null) {
        for (Service.ProfilingData.GpuCounters.Metric metric : models.profile.getData().getGpuPerformance().getMetricsList()) {
          if (visibleMetrics.contains(metric.getId())){
            addMetricColumn(metric);
          }
        }
      }
    }

    private void addMetricColumn(Service.ProfilingData.GpuCounters.Metric metric) {
      TreeViewerColumn column = addColumn(metric.getName(), node -> {
        Service.CommandTreeNode data = node.getData();
        if (data == null) {
          return "";
        } else if (profileLoadingError != null) {
          return "Profiling failed.";
        } else if (!models.profile.isLoaded()) {
          return "Profiling...";
        } else {
          return models.profile.getData().getGPUCounterForCommands(metric, data.getCommands());
        }
      }, DURATION_WIDTH);
      column.getColumn().setAlignment(SWT.RIGHT);
    }

    public void refresh() {
      refresher.refresh();
    }

    public void updateTree(TreeItem item) {
      labelProvider.updateHierarchy(item);
    }

    @Override
    protected ContentProvider<Node> createContentProvider() {
      return new ContentProvider<CommandStream.Node>() {
        @Override
        protected boolean hasChildNodes(CommandStream.Node element) {
          return element.getChildCount() > 0;
        }

        @Override
        protected CommandStream.Node[] getChildNodes(CommandStream.Node node) {
          return node.getChildren();
        }

        @Override
        protected CommandStream.Node getParentNode(CommandStream.Node child) {
          return child.getParent();
        }

        @Override
        protected boolean isLoaded(CommandStream.Node element) {
          return element.getData() != null;
        }

        @Override
        protected void load(CommandStream.Node node, Runnable callback) {
          models.commands.load(node, callback);
        }
      };
    }

    private Style getCommandStyle(Service.CommandTreeNode node, StylingString string) {
      if (node.getExperimentalCommandsCount() == 0) {
        return string.labelStyle();
      }

      final List<Path.Command> experimentCommands = node.getExperimentalCommandsList();
      if (widgets.experiments.areAllCommandsDisabled(experimentCommands)) {
        return string.disabledLabelStyle();
      }

      if (widgets.experiments.isAnyCommandDisabled(experimentCommands)) {
        return string.semiDisabledLabelStyle();
      }

      return string.labelStyle();
    }

    @Override
    protected <S extends StylingString> S format(
        CommandStream.Node element, S string, Follower.Prefetcher<String> follower) {
      Service.CommandTreeNode data = element.getData();
      if (data == null) {
        string.append("Loading...", string.structureStyle());
      } else {
        if (data.getGroup().isEmpty() && data.hasCommands()) {
          API.Command cmd = element.getCommand();
          if (cmd == null) {
            string.append("Loading...", string.structureStyle());
          } else {
            Formatter.format(cmd, models.constants::getConstants, follower::canFollow,
                string, getCommandStyle(data, string), string.identifierStyle());
          }
        } else {
          string.append(data.getGroup(), getCommandStyle(data, string));
          long count = data.getNumCommands();
          string.append(
              " (" + count + " command" + (count != 1 ? "s" : "") + ")", string.structureStyle());
        }
      }
      return string;
    }

    @Override
    protected Color getBackgroundColor(CommandStream.Node node) {
      API.Command cmd = node.getCommand();
      if (cmd == null) {
        return null;
      }

      long threadId = cmd.getThread();
      Color color = threadBackgroundColors.get(threadId);
      if (color == null) {
        Control control = getControl();
        RGBA bg = control.getBackground().getRGBA();
        color = new Color(control.getDisplay(),
            lerp(getRandomColor(getColorIndex(threadId)), bg.rgb, COLOR_INTENSITY), bg.alpha);
        threadBackgroundColors.put(threadId, color);
      }
      return color;
    }

    private static int getColorIndex(long threadId) {
      // TODO: The index should be the i'th thread in use by the capture, not a hash of the
      // thread ID. This requires using the list of threads exposed by the service.Capture.
      int hash = (int)(threadId ^ (threadId >>> 32));
      hash = hash ^ (hash >>> 16);
      hash = hash ^ (hash >>> 8);
      return hash & 0xff;
    }

    @Override
    protected boolean shouldShowImage(CommandStream.Node node) {
      return models.images.isReady() &&
          node.getData() != null && !node.getData().getGroup().isEmpty();
    }

    @Override
    protected ListenableFuture<ImageData> loadImage(CommandStream.Node node, int size) {
      return noAlpha(models.images.getThumbnail(
          node.getPath(Path.CommandTreeNode.newBuilder()).build(), size, i -> { /*noop*/ }));
    }

    @Override
    protected void createImagePopupContents(Shell shell, CommandStream.Node node) {
      LoadableImageWidget.forImage(
          shell, LoadableImage.newBuilder(widgets.loading)
              .forImageData(loadImage(node, THUMB_SIZE))
              .onErrorShowErrorIcon(widgets.theme))
      .withImageEventListener(new LoadableImage.Listener() {
        @Override
        public void onLoaded(boolean success) {
          if (success) {
            Widgets.ifNotDisposed(shell,() -> {
              Point oldSize = shell.getSize();
              Point newSize = shell.computeSize(SWT.DEFAULT, SWT.DEFAULT);
              shell.setSize(newSize);
              if (oldSize.y != newSize.y) {
                Point location = shell.getLocation();
                location.y += (oldSize.y - newSize.y) / 2;
                shell.setLocation(location);
              }
            });
          }
        }
      });
    }

    @Override
    protected Follower.Prefetcher<String> prepareFollower(CommandStream.Node node, Runnable cb) {
      return models.follower.prepare(node, cb);
    }

    @Override
    protected void follow(Path.Any path) {
      models.follower.onFollow(path);
    }

    @Override
    public void reset() {
      super.reset();
      for (Color color : threadBackgroundColors.values()) {
        color.dispose();
      }
      threadBackgroundColors.clear();
    }
  }

  private class PresetsBar extends Composite {
    private final Settings settings;
    private final Theme theme;

    public PresetsBar(Composite parent, Settings settings, Theme theme) {
      super(parent, SWT.NONE);
      this.settings = settings;
      this.theme = theme;

      RowLayout stripLayout = withMargin(new RowLayout(SWT.HORIZONTAL), 0, 0);
      stripLayout.fill = true;
      stripLayout.wrap = true;
      stripLayout.spacing = 5;
      setLayout(stripLayout);
    }

    public void refresh() {
      for (Control children : this.getChildren()) {
        children.dispose();
      }
      createPresetButtons();
      redraw();
      requestLayout();
    }

    private void createPresetButtons() {
      if (models.devices.getSelectedReplayDevice() == null) {
        return;
      }

      Button addButton = createButton(this, SWT.FLAT, "Add New Preset", buttonColor, e -> {
        AddPresetDialog dialog = new AddPresetDialog(
            getShell(), theme, getCounterSpecs(), Lists.newArrayList());
        if (dialog.open() == Window.OK) {
          Set<Integer> selectedIds = Sets.newHashSet(dialog.getSelectedIds());
          visibleMetrics = selectedIds;
          models.settings.writeUi().addPerformancePresets(SettingsProto.UI.PerformancePreset.newBuilder()
              .setPresetName(dialog.getFinalPresetName())
              .setDeviceName(models.devices.getSelectedReplayDevice().getName())
              .addAllCounterIds(selectedIds)
              .build());
          refresh();
          updateTree(false);
        }
      });
      addButton.setImage(theme.add());
      withLayoutData(new Label(this, SWT.VERTICAL | SWT.SEPARATOR), new RowData(SWT.DEFAULT, 1));

      boolean customPresetButtonCreated = false;
      for (PerformancePreset preset : settings.ui().getPerformancePresetsList()) {
        if (!preset.getDeviceName().equals(models.devices.getSelectedReplayDevice().getName())) {
          continue;
        }
        createButton(this, SWT.FLAT, preset.getPresetName(), buttonColor, e -> {
          visibleMetrics = Sets.newHashSet(preset.getCounterIdsList());
          updateTree(false);
        });
        customPresetButtonCreated = true;
      }
      if (customPresetButtonCreated) {
        withLayoutData(new Label(this, SWT.VERTICAL | SWT.SEPARATOR), new RowData(SWT.DEFAULT, 1));
      }


      for (PerformancePreset preset : getRecommendedPresets()) {
        createButton(this, SWT.FLAT, preset.getPresetName(), buttonColor, e -> {
          visibleMetrics = Sets.newHashSet(preset.getCounterIdsList());
          updateTree(false);
        });
      }
    }

    // Create and return a list of presets based on vendor provided GPU counter grouping metadata.
    private List<SettingsProto.UI.PerformancePreset> getRecommendedPresets() {
      List<SettingsProto.UI.PerformancePreset> presets = Lists.newArrayList();
      if (!models.profile.isLoaded()) {
        return presets;
      }
      Map<GpuCounterGroup, List<Integer>> groupToMetrics = Maps.newHashMap();
      // Pre-create the map entries so they go with the default order in enum definition.
      for (GpuCounterGroup group : GpuCounterGroup.values()) {
        groupToMetrics.put(group, Lists.newArrayList());
      }
      for (Service.ProfilingData.GpuCounters.Metric metric: models.profile.getData().
          getGpuPerformance().getMetricsList()) {
        for (GpuCounterGroup group : metric.getCounterGroupsList()) {
          groupToMetrics.get(group).add(metric.getId());
        }
      }
      for (GpuCounterGroup group : groupToMetrics.keySet()) {
        if (group != GpuCounterGroup.UNCLASSIFIED && groupToMetrics.get(group).size() > 0) {
          presets.add(SettingsProto.UI.PerformancePreset.newBuilder()
              .setPresetName(group.name())
              .setDeviceName(models.devices.getSelectedReplayDevice().getName())
              .addAllCounterIds(groupToMetrics.get(group))
              .build());
        }
      }
      return presets;
    }

      private class AddPresetDialog extends GpuCountersDialog {
    private Text presetNameInput;
    private String finalPresetName;
    private Label warningLabel;

    public AddPresetDialog(Shell shell, Theme theme, List<GpuCounterSpec> specs,
        List<Integer> currentIds) {
      super(shell, theme, specs, currentIds);
    }

    @Override
    protected void createButtonsForButtonBar(Composite parent) {
      Button button = createButton(parent, IDialogConstants.OPEN_ID, "Manage Presets", false);
      button.addListener(SWT.Selection,
          e-> new ManagePresetsDialog(getShell(), theme, getCounterSpecs(), Lists.newArrayList()).open());
      super.createButtonsForButtonBar(parent);
    }

    @Override
    protected Control createContents(Composite parent) {
      Control control = super.createContents(parent);
      Button okButton = getButton(IDialogConstants.OK_ID);
      okButton.setText("Add");
      okButton.setEnabled(false);
      return control;
    }

    @Override
    public String getTitle() {
      return "Create New Preset";
    }

    @Override
    protected Control createDialogArea(Composite parent) {
      Composite area = createComposite(parent, withMargin(new GridLayout(2, false),
          IDialogConstants.HORIZONTAL_MARGIN, IDialogConstants.VERTICAL_MARGIN));
      area.setLayoutData(new GridData(GridData.FILL_BOTH));

      String currentDevice = models.devices.getSelectedReplayDevice().getName();
      createLabel(area, "Current Device: ").setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, false, false));
      createLabel(area, currentDevice).setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, true, false));

      createLabel(area, "Preset Name: ").setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, false, false));
      presetNameInput = createTextbox(area, "");
      presetNameInput.setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, true, false));
      Set<String> usedNames = Sets.newHashSet();
      settings.ui().getPerformancePresetsList().stream()
          .filter(p -> p.getDeviceName().equals(currentDevice))
          .forEach(p -> usedNames.add(p.getPresetName()));
      presetNameInput.addModifyListener(e -> {
        String input = presetNameInput.getText();
        Button okButton = getButton(IDialogConstants.OK_ID);
        if (input.isEmpty() || usedNames.contains(input)) {
          okButton.setEnabled(false);
          warningLabel.setVisible(true);
        } else {
          okButton.setEnabled(true);
          warningLabel.setVisible(false);
        }
      });

      Composite tableArea = createComposite(area, new GridLayout());
      tableArea.setLayoutData(new GridData(SWT.FILL, SWT.FILL, true, true, 2, 1));
      createGpuCounterTable(tableArea);

      warningLabel = createLabel(area, "Preset name empty or already exist.");
      warningLabel.setForeground(getDisplay().getSystemColor(SWT.COLOR_DARK_RED));

      return area;
    }

    @Override
    protected void okPressed() {
      finalPresetName = presetNameInput.getText();
      super.okPressed();
    }

    public String getFinalPresetName() {
      return finalPresetName;
    }
  }

  private class ManagePresetsDialog extends GpuCountersDialog {
    private final List<PerformancePreset> removalWaitlist = Lists.newArrayList();

    public ManagePresetsDialog(Shell shell, Theme theme, List<GpuCounterSpec> specs,
        List<Integer> currentIds) {
      super(shell, theme, specs, currentIds);
    }

    @Override
    public String getTitle() {
      return "Manage Presets List";
    }

    @Override
    protected void createButtonsForButtonBar(Composite parent) {
      super.createButtonsForButtonBar(parent);
      getButton(IDialogConstants.OK_ID).setText("Save");
    }

    @Override
    protected Control createDialogArea(Composite parent) {
      Composite area = createComposite(parent, withMargin(new GridLayout(2, false),
          IDialogConstants.HORIZONTAL_MARGIN, IDialogConstants.VERTICAL_MARGIN));
      area.setLayoutData(new GridData(GridData.FILL_BOTH));

      // Create the presets listing table.
      TableViewer viewer = createTableViewer(area, SWT.NONE);
      viewer.getTable().setLayoutData(new GridData(SWT.LEFT, SWT.FILL, false, true));
      viewer.setContentProvider(new ArrayContentProvider());
      viewer.setLabelProvider(new LabelProvider());
      createTableColumn(viewer, "Device", p -> ((PerformancePreset)p).getDeviceName());
      createTableColumn(viewer, "Preset Name", p -> ((PerformancePreset)p).getPresetName());
      viewer.addSelectionChangedListener(e -> {
        IStructuredSelection selection = e.getStructuredSelection();
        // Handle an edge case after deletion.
        if (selection == null || selection.getFirstElement() == null) {
          table.setAllChecked(false);
          return;
        }
        PerformancePreset selectedPreset = (PerformancePreset)selection.getFirstElement();
        Set<Integer> counterIds = Sets.newHashSet(selectedPreset.getCounterIdsList());
        table.setCheckedElements(getSpecs().stream()
            .filter(s -> counterIds.contains(s.getCounterId()))
            .toArray(GpuCounterSpec[]::new));
      });
      List<PerformancePreset> presets = Lists.newArrayList(settings.ui().getPerformancePresetsList());
      viewer.setInput(presets);
      packColumns(viewer.getTable());

      // Create the GPU counter table, which will reflect the selected preset's containing counters.
      Composite tableArea = createComposite(area, new GridLayout());
      tableArea.setLayoutData(new GridData(SWT.FILL, SWT.FILL, true, true));
      createGpuCounterTable(tableArea);

      // Create the delete button.
      Widgets.createButton(area, SWT.FLAT, "Delete", buttonColor, e -> {
        IStructuredSelection selection = viewer.getStructuredSelection();
        if (selection == null || selection.getFirstElement() == null) {
          return;
        }
        PerformancePreset selectedPreset = (PerformancePreset)selection.getFirstElement();
        removalWaitlist.add(selectedPreset);
        presets.remove(selectedPreset);
        viewer.refresh();
      });

      return area;
    }

    @Override
    protected void okPressed() {
      SettingsProto.UI.Builder uiBuilder = models.settings.writeUi();
      // Reverse iteration, so as to avoid getting affected by index change at removal.
      for (int i = uiBuilder.getPerformancePresetsCount() - 1; i >= 0 ; i--) {
        if (removalWaitlist.contains(uiBuilder.getPerformancePresets(i))) {
          uiBuilder.removePerformancePresets(i);
        }
      }
      presetsBar.refresh();
      super.okPressed();
    }
  }
  }

  private void toggleEstimateOrRange() {
    showEstimate = !showEstimate;
    tree.packColumn();
    tree.refresh();
  }

  private List<GpuProfiling.GpuCounterDescriptor.GpuCounterSpec> getCounterSpecs() {
    List<GpuProfiling.GpuCounterDescriptor.GpuCounterSpec> specs = Lists.newArrayList();
    if (!models.profile.isLoaded()) {
      return specs;
    }
    // To reuse the existing GpuCountersDialog class for displaying, forge GpuCounterSpec instances
    // with the minimum data requirement that is needed and referenced in GpuCountersDialog.
    for (Service.ProfilingData.GpuCounters.Metric metric : models.profile.getData()
        .getGpuPerformance().getMetricsList()) {
      specs.add(GpuProfiling.GpuCounterDescriptor.GpuCounterSpec.newBuilder()
          .setName(metric.getName())
          .setDescription(metric.getDescription())
          .setCounterId(metric.getId())
          .setSelectByDefault(metric.getSelectByDefault())
          .build());
    }
    return specs;
  }
}
