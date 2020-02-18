/*
 * Copyright (C) 2019 Google Inc.
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

import static com.google.gapid.perfetto.views.StyleConstants.LABEL_MARGIN;
import static com.google.gapid.perfetto.views.StyleConstants.LABEL_WIDTH;
import static com.google.gapid.perfetto.views.StyleConstants.TITLE_HEIGHT;
import static com.google.gapid.perfetto.views.StyleConstants.colors;
import static com.google.gapid.util.Loadable.MessageType.Error;
import static com.google.gapid.util.Loadable.MessageType.Loading;
import static java.util.stream.Collectors.toList;

import com.google.common.collect.Lists;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.gapid.models.Capture;
import com.google.gapid.models.Models;
import com.google.gapid.models.Profile;
import com.google.gapid.models.Settings;
import com.google.gapid.perfetto.TimeSpan;
import com.google.gapid.perfetto.canvas.Area;
import com.google.gapid.perfetto.canvas.Fonts;
import com.google.gapid.perfetto.canvas.Panel;
import com.google.gapid.perfetto.canvas.RenderContext;
import com.google.gapid.perfetto.models.ArgSet;
import com.google.gapid.perfetto.models.CpuInfo;
import com.google.gapid.perfetto.models.GpuInfo;
import com.google.gapid.perfetto.models.ProcessInfo;
import com.google.gapid.perfetto.models.SliceTrack;
import com.google.gapid.perfetto.models.ThreadInfo;
import com.google.gapid.perfetto.views.GpuQueuePanel;
import com.google.gapid.perfetto.views.RootPanel;
import com.google.gapid.perfetto.views.State;
import com.google.gapid.perfetto.views.TraceComposite;
import com.google.gapid.perfetto.views.TrackPanel;
import com.google.gapid.proto.service.Service;
import com.google.gapid.util.Loadable;
import com.google.gapid.util.Messages;
import com.google.gapid.util.Scheduler;
import com.google.gapid.widgets.LoadablePanel;
import com.google.gapid.widgets.Theme;
import com.google.gapid.widgets.Widgets;

import org.eclipse.swt.SWT;
import org.eclipse.swt.layout.FillLayout;
import org.eclipse.swt.widgets.Composite;
import org.eclipse.swt.widgets.Control;

import java.util.List;

public class ProfileView extends Composite implements Tab, Capture.Listener, Profile.Listener {
  private final Models models;

  private final LoadablePanel<TraceUi> loading;
  private final TraceUi traceUi;

  public ProfileView(Composite parent, Models models, Widgets widgets) {
    super(parent, SWT.NONE);
    this.models = models;

    setLayout(new FillLayout(SWT.VERTICAL));

    loading = new LoadablePanel<TraceUi>(this, widgets, p -> new TraceUi(p, models, widgets.theme) {
      @Override
      protected Settings settings() {
        return models.settings;
      }
    });
    traceUi = loading.getContents();

    models.capture.addListener(this);
    models.profile.addListener(this);
    addListener(SWT.Dispose, e -> {
      models.capture.removeListener(this);
      models.profile.removeListener(this);
    });
  }

  @Override
  public Control getControl() {
    return this;
  }

  @Override
  public void reinitialize() {
    if (models.profile.isLoaded()) {
      loading.stopLoading();
      updateProfile(models.profile.getData());
    } else {
      loading.showMessage(
          Loading, models.capture.isLoaded() ? Messages.LOADING_CAPTURE : Messages.LOADING_PROFILE);
    }
  }

  @Override
  public void onCaptureLoadingStart(boolean maintainState) {
    loading.showMessage(Loading, Messages.LOADING_CAPTURE);
  }

  @Override
  public void onCaptureLoaded(Loadable.Message error) {
    if (error != null) {
      loading.showMessage(Error, Messages.CAPTURE_LOAD_FAILURE);
    }
  }

  @Override
  public void onProfileLoadingStart() {
    loading.showMessage(Loading, Messages.LOADING_PROFILE);
  }

  @Override
  public void onProfileLoaded(Loadable.Message error) {
    if (error != null) {
      loading.showMessage(error);
    } else {
      loading.stopLoading();
      updateProfile(models.profile.getData());
    }
  }

  private void updateProfile(Profile.Data data) {
    if (!data.hasSlices()) {
      loading.showMessage(Error, Messages.PROFILE_NO_SLICES);
      return;
    }

    traceUi.update(data);
  }

  private abstract static class TraceUi extends TraceComposite<State> {
    protected final List<Panel> panels = Lists.newArrayList();

    public TraceUi(Composite parent, Models models, Theme theme) {
      super(parent, models.analytics, theme);
    }

    public void update(Profile.Data data) {
      panels.clear();

      Service.ProfilingData.GpuSlices slices = data.getSlices();
      for (Service.ProfilingData.GpuSlices.Track track : slices.getTracksList()) {
        List<Service.ProfilingData.GpuSlices.Slice> matched = Lists.newArrayList();
        int maxDepth = 0;
        for (Service.ProfilingData.GpuSlices.Slice slice : slices.getSlicesList()) {
          if (slice.getTrackId() == track.getId()) {
            matched.add(slice);
            maxDepth = Math.max(maxDepth, slice.getDepth());
          }
        }
        panels.add(new Container(new GpuQueuePanel(state,
            new GpuInfo.Queue(track.getId(), track.getId(), maxDepth + 1),
            new GpuSliceTrack(track.getId(), matched))));
      }

      state.update(data.getSlicesTimeSpan());
    }

    @Override
    protected State createState() {
      return new State(this) {
        @Override
        public CpuInfo getCpuInfo() {
          return CpuInfo.NONE;
        }

        @Override
        public ProcessInfo getProcessInfo(long id) {
          return null;
        }

        @Override
        public ThreadInfo getThreadInfo(long id) {
          return null;
        }
      };
    }

    @Override
    protected RootPanel<State> createRootPanel() {
      return new RootPanel<State>(state, settings()) {
        @Override
        protected void createUi() {
          top.add(timeline);
          for (Panel panel : panels) {
            bottom.add(panel);
          }
        }

        @Override
        protected void preTopUiRender(RenderContext ctx, Repainter repainter) {
          // Do nothing.
        }

        @Override
        protected void preMainUiRender(RenderContext ctx, Repainter repainter) {
          // Do nothing.
        }
      };
    }

    protected abstract Settings settings();

    // TODO: dedupe with code in TrackContainer.
    private static class Container extends Panel.Base {
      private final TrackPanel<?> track;

      public Container(TrackPanel<?> panel) {
        this.track = panel;
      }

      @Override
      public double getPreferredHeight() {
        return track.getPreferredHeight();
      }

      @Override
      public void setSize(double w, double h) {
        super.setSize(w, h);
        track.setSize(w, h);
      }

      @Override
      public void render(RenderContext ctx, Repainter repainter) {
        ctx.withClip(0, 0, LABEL_WIDTH, height, () -> {
          ctx.setForegroundColor(colors().textMain);
          ctx.drawTextLeftTruncate(Fonts.Style.Normal, track.getTitle(), LABEL_MARGIN, 0,
              LABEL_WIDTH - 2 * LABEL_MARGIN, TITLE_HEIGHT);
        });

        ctx.setForegroundColor(colors().panelBorder);
        ctx.drawLine(LABEL_WIDTH - 1, 0, LABEL_WIDTH - 1, height);
        ctx.drawLine(0, height - 1, width, height - 1);
        track.render(ctx, repainter);
      }

      @Override
      public void visit(Visitor v, Area area) {
        super.visit(v, area);
        track.visit(v, area);
      }

      @Override
      public Dragger onDragStart(double x, double y, int mods) {
        return track.onDragStart(x, y, mods);
      }

      @Override
      public Hover onMouseMove(Fonts.TextMeasurer m, double x, double y, int mods) {
        return (x < LABEL_WIDTH) ? Hover.NONE : track.onMouseMove(m, x, y, mods);
      }
    }

    private static class GpuSliceTrack extends SliceTrack {
      private final List<Service.ProfilingData.GpuSlices.Slice> slices;

      protected GpuSliceTrack(long trackId, List<Service.ProfilingData.GpuSlices.Slice> slices) {
        super(trackId);
        this.slices = slices;
      }

      @Override
      public ListenableFuture<Slice> getSlice(long id) {
        return Futures.immediateFuture(toSlice(slices.get((int)id)));
      }

      @Override
      public ListenableFuture<List<Slice>> getSlices(TimeSpan ts, int minDepth, int maxDepth) {
        return Scheduler.EXECUTOR.submit(() -> slices.stream()
            .filter(s -> ts.overlaps(s.getTs(), s.getTs() + s.getDur()))
            .filter(s -> s.getDepth() >= minDepth && s.getDepth() <= maxDepth)
            .map(this::toSlice)
            .collect(toList()));
      }

      @Override
      protected ListenableFuture<?> initialize() {
        return Futures.immediateFuture(null);
      }

      @Override
      protected ListenableFuture<Data> computeData(DataRequest req) {
        return Scheduler.EXECUTOR.submit(() -> {
          List<SliceAndId> matched = Lists.newArrayList();
          for (int i = 0; i < slices.size(); i++) {
            Service.ProfilingData.GpuSlices.Slice slice = slices.get(i);
            if (req.range.overlaps(slice.getTs(), slice.getTs() + slice.getDur())) {
              matched.add(new SliceAndId(slice, i));
            }
          }

          int n = matched.size();
          Data data = new Data(req, new long[n], new long[n], new long[n], new int[n], new String[n],
              new String[n], new ArgSet[n]);
          for (int i = 0; i < n; i++) {
            SliceAndId s = matched.get(i);
            data.ids[i] = s.id;
            data.starts[i] = s.slice.getTs();
            data.ends[i] = s.slice.getTs() + s.slice.getDur();
            data.depths[i] = s.slice.getDepth();
            data.titles[i] = s.slice.getLabel();
            data.categories[i] = "";
            data.args[i] = ArgSet.EMPTY;
          }
          return data;
        });
      }

      private Slice toSlice(Service.ProfilingData.GpuSlices.Slice s) {
        return new Slice(s.getTs(), s.getDur(), "", s.getLabel(), s.getDepth(), -1, -1, ArgSet.EMPTY) {
          @Override
          public String getTitle() {
            return "GPU Render Stages";
          }
        };
      }

      private static class SliceAndId {
        public final Service.ProfilingData.GpuSlices.Slice slice;
        public final int id;

        public SliceAndId(Service.ProfilingData.GpuSlices.Slice slice, int id) {
          this.slice = slice;
          this.id = id;
        }
      }
    }
  }
}
