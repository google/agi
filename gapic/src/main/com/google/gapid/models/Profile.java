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
package com.google.gapid.models;
import static com.google.gapid.rpc.UiErrorCallback.error;
import static com.google.gapid.rpc.UiErrorCallback.success;
import static com.google.gapid.util.Logging.throttleLogRpcError;
import static com.google.gapid.util.MoreFutures.transform;
import static java.util.Collections.binarySearch;
import static java.util.logging.Level.WARNING;
import static java.util.stream.Collectors.groupingBy;
import static java.util.stream.Collectors.mapping;
import static java.util.stream.Collectors.toList;
import static java.util.stream.Collectors.toSet;

import com.google.common.base.Objects;
import com.google.common.collect.Lists;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.gapid.perfetto.TimeSpan;
import com.google.gapid.proto.service.Service;
import com.google.gapid.proto.service.path.Path;
import com.google.gapid.rpc.Rpc;
import com.google.gapid.rpc.RpcException;
import com.google.gapid.rpc.UiErrorCallback.ResultOrError;
import com.google.gapid.server.Client;
import com.google.gapid.util.Events;
import com.google.gapid.util.Loadable;
import com.google.gapid.util.Paths;
import com.google.gapid.util.Ranges;

import org.eclipse.swt.widgets.Shell;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import java.util.logging.Logger;

public class Profile
    extends CaptureDependentModel<Profile.Data, Profile.Source, Loadable.Message, Profile.Listener> {
  private static final Logger LOG = Logger.getLogger(Profile.class.getName());

  private final Capture capture;

  public Profile(
      Shell shell, Analytics analytics, Client client, Capture capture, Devices devices) {
    super(LOG, shell, analytics, client, Listener.class, capture, devices);
    this.capture = capture;
  }

  @Override
  protected Source getSource(Capture.Data data) {
    return new Source(data.path);
  }

  @Override
  protected boolean shouldLoad(Capture.Data data) {
    return data.isGraphics();
  }

  @Override
  protected ListenableFuture<Data> doLoad(Source source, Path.Device device) {
    return transform(client.profile(capture.getData().path, device), r -> new Data(device, r));
  }

  @Override
  protected ResultOrError<Data, Loadable.Message> processResult(Rpc.Result<Data> result) {
    try {
      return success(result.get());
    } catch (RpcException e) {
      LOG.log(WARNING, "Failed to load the GPU profile", e);
      return error(Loadable.Message.error(e));
    } catch (ExecutionException e) {
      if (!shell.isDisposed()) {
        throttleLogRpcError(LOG, "Failed to load the GPU profile", e);
      }
      return error(Loadable.Message.error("Failed to load the GPU profile"));
    }
  }

  @Override
  protected void updateError(Loadable.Message error) {
    listeners.fire().onProfileLoaded(error);
  }

  @Override
  protected void fireLoadStartEvent() {
    listeners.fire().onProfileLoadingStart();
  }

  @Override
  protected void fireLoadedEvent() {
    listeners.fire().onProfileLoaded(null);
  }

  public static class Source {
    public final Path.Capture capture;

    public Source(Path.Capture capture) {
      this.capture = capture;
    }

    @Override
    public boolean equals(Object obj) {
      if (obj == this) {
        return true;
      } else if (!(obj instanceof Source)) {
        return false;
      }
      return Objects.equal(capture, ((Source)obj).capture);
    }

    @Override
    public int hashCode() {
      return (capture == null) ? 0 : capture.hashCode();
    }
  }

  public static class Data extends DeviceDependentModel.Data {
    public final Service.ProfilingData profile;
    private final Map<Integer, List<TimeSpan>> spansByGroup;
    private final List<Service.ProfilingData.GpuSlices.Group> groups;

    public Data(Path.Device device, Service.ProfilingData profile) {
      super(device);
      this.profile = profile;
      this.spansByGroup = aggregateSliceTimeByGroup(profile);
      this.groups = getSortedGroups(profile, spansByGroup.keySet());
    }

    private static Map<Integer, List<TimeSpan>>
        aggregateSliceTimeByGroup(Service.ProfilingData profile) {
      Set<Integer> groups = profile.getSlices().getGroupsList().stream()
          .map(Service.ProfilingData.GpuSlices.Group::getId)
          .collect(toSet());
      return profile.getSlices().getSlicesList().stream()
          .filter(s -> (s.getDepth() == 0) && groups.contains(s.getGroupId()))
          .collect(groupingBy(Service.ProfilingData.GpuSlices.Slice::getGroupId,
              mapping(s -> new TimeSpan(s.getTs(), s.getTs() + s.getDur()), toList())));
    }

    private static List<Service.ProfilingData.GpuSlices.Group> getSortedGroups(
        Service.ProfilingData profile, Set<Integer> ids) {
      return profile.getSlices().getGroupsList().stream()
          .filter(g -> ids.contains(g.getId()))
          .sorted((g1, g2) -> Paths.compare(g1.getLink(), g2.getLink()))
          .collect(toList());
    }

    public boolean hasSlices() {
      return profile.getSlices().getSlicesCount() > 0 &&
          profile.getSlices().getTracksCount() > 0;
    }

    public Service.ProfilingData.GpuSlices getSlices() {
      return profile.getSlices();
    }

    public List<Service.ProfilingData.Counter> getCounters() {
      return profile.getCountersList();
    }

    public TimeSpan getSlicesTimeSpan() {
      if (!hasSlices()) {
        return TimeSpan.ZERO;
      }

      long start = Long.MAX_VALUE, end = 0;
      for (Service.ProfilingData.GpuSlices.Slice slice : profile.getSlices().getSlicesList()) {
        start = Math.min(slice.getTs(), start);
        end = Math.max(slice.getTs() + slice.getDur(), end);
      }
      return new TimeSpan(start, end);
    }

    public Duration getDuration(Path.Commands range) {
      List<TimeSpan> spans = getSpans(range);
      if (spans.size() == 0) {
        return Duration.NONE;
      }
      long start = Long.MAX_VALUE, end = Long.MIN_VALUE;
      long gpuTime = 0, wallTime = 0;
      TimeSpan last = TimeSpan.ZERO;
      for (TimeSpan span : spans) {
        start = Math.min(start, span.start);
        end = Math.max(end, span.end);
        long duration = span.getDuration();
        gpuTime += duration;
        if (span.start < last.end) {
          if (span.end <= last.end) {
            continue; // completely contained within the other, can ignore it.
          }
          duration -= last.end - span.start;
        }
        wallTime += duration;
        last = span;
      }

      TimeSpan ts = start < end ? new TimeSpan(start, end) : TimeSpan.ZERO;
      return new Duration(gpuTime, wallTime, ts);
    }

    // Return an aggregated number to denote counter performance, for a specific counter at a commands range.
    public Double getCounterAggregation(Path.Commands range, Service.ProfilingData.Counter counter) {
      // Reduce overlapped GPU slices spans size.
      List<TimeSpan> spans = getSpans(range);
      if (spans.size() == 0) {
        return Double.NaN;
      }

      // Reduce overlapped counter samples size.
      // Filter out the counter samples whose implicit range collides with `range`'s gpu time.
      long rangeStart = spans.stream().mapToLong(s -> s.start).min().getAsLong();
      long rangeEnd = spans.stream().mapToLong(s -> s.end).max().getAsLong();
      List<Long> ts = Lists.newArrayList();
      List<Double> vs = Lists.newArrayList();
      for (int i = 0; i < counter.getTimestampsCount(); i++) {
        if (i > 0 && counter.getTimestamps(i - 1) > rangeEnd) {
          break;
        }
        if (counter.getTimestamps(i) > rangeStart) {
          ts.add(counter.getTimestamps(i));
          vs.add(counter.getValues(i));
        }
      }
      if (ts.size() == 0) {
        return Double.NaN;
      }

      // Aggregate counter samples.
      // Contribution time is the overlapped time between a counter sample's implicit range and a gpu slice.
      long ctSum = 0; // Accumulation of contribution time.
      double weightedValuesum = 0; // Accumulation of (counter value * counter's contribution time).
      for (TimeSpan span : spans) {
        if (ts.get(0) > span.start) {
          long ct = Math.min(ts.get(0), span.end) - span.start;
          ctSum += ct;
          weightedValuesum += ct * vs.get(0);
        }
        for (int i = 1; i < ts.size(); i++) {
          long start = ts.get(i - 1), end = ts.get(i);
          if (end < span.start) {      // Sample earlier than GPU slice's span.
            continue;
          } else if (end < span.end) { // Sample inside GPU slice's span.
            long ct = end - Math.max(start, span.start);
            ctSum += ct;
            weightedValuesum += ct * vs.get(i);
          } else {                     // Sample later than GPU slice's span.
            long ct = Math.max(0, span.end - start);
            ctSum += ct;
            weightedValuesum += ct * vs.get(i);
            break;
          }
        }
      }
      return ctSum == 0 ? 0 : weightedValuesum / ctSum;
    }

    private List<TimeSpan> getSpans(Path.Commands range) {
      List<TimeSpan> spans = Lists.newArrayList();
      int idx = binarySearch(groups, null, (g, $) -> Ranges.compare(range, g.getLink()));
      if (idx < 0) {
        return spans;
      }
      for (int i = idx; i >= 0 && Ranges.contains(range, groups.get(i).getLink()); i--) {
        spans.addAll(spansByGroup.get(groups.get(i).getId()));
      }
      for (int i = idx + 1; i < groups.size() && Ranges.contains(range, groups.get(i).getLink()); i++) {
        spans.addAll(spansByGroup.get(groups.get(i).getId()));
      }
      Collections.sort(spans, (s1, s2) -> Long.compare(s1.start, s2.start));
      return spans;
    }
  }

  public static class Duration {
    public static final Duration NONE = new Duration(0, 0, TimeSpan.ZERO) {
      @Override
      public String formatGpuTime() {
        return "";
      }

      @Override
      public String formatWallTime() {
        return "";
      }
    };

    public final long gpuTime;
    public final long wallTime;
    public final TimeSpan timeSpan;

    public Duration(long gpuTime, long wallTime, TimeSpan timeSpan) {
      this.gpuTime = gpuTime;
      this.wallTime = wallTime;
      this.timeSpan = timeSpan;
    }

    public String formatGpuTime() {
      return String.format("%.3fms", gpuTime / 1e6);
    }

    public String formatWallTime() {
      return String.format("%.3fms", wallTime / 1e6);
    }
  }

  public static interface Listener extends Events.Listener {
    /**
     * Event indicating that profiling data is being loaded.
     */
    public default void onProfileLoadingStart() { /* empty */ }

    /**
     * Event indicating that the profiling data has finished loading.
     *
     * @param error the loading error or {@code null} if loading was successful.
     */
    public default void onProfileLoaded(Loadable.Message error) { /* empty */ }
  }
}
