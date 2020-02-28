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
package com.google.gapid.perfetto.models;

import static com.google.gapid.perfetto.models.QueryEngine.createSpan;
import static com.google.gapid.perfetto.models.QueryEngine.createView;
import static com.google.gapid.perfetto.models.QueryEngine.createWindow;
import static com.google.gapid.perfetto.models.QueryEngine.dropTable;
import static com.google.gapid.perfetto.models.QueryEngine.dropView;
import static com.google.gapid.util.MoreFutures.transform;
import static com.google.gapid.util.MoreFutures.transformAsync;
import static java.lang.String.format;
import static java.util.stream.Collectors.joining;

import com.google.common.util.concurrent.ListenableFuture;
import com.google.gapid.perfetto.models.CpuTrack.Slice;
import java.util.Arrays;

/**
 * {@link Track} containing CPU usage data of all threads in a process.
 */
public class ProcessSummaryTrack extends Track.WithQueryEngine<ProcessSummaryTrack.Data> {
  // "where cpu < %numCpus%" is for performance reasons of the window table.
  private static final String PROCESS_VIEW_SQL = "select * from sched where utid in (%s)";
  private static final String SUMMARY_SQL =
      "select quantum_ts, id, utid, sum(dur)/cast(%d * %d as float) util from(" +
          "select *, first_value(row_id) over win id, first_value(utid) over win utid " +
          "from %s " +
          "window win as (partition by quantum_ts order by dur desc)) " +
      "group by quantum_ts";
  private static final String SLICES_SQL = "select ts, dur, cpu, utid, row_id from %s";

  private final int numCpus;
  private final ProcessInfo process;

  public ProcessSummaryTrack(QueryEngine qe, int numCpus, ProcessInfo process) {
    super(qe, "proc_" + process.upid + "_sum");
    this.numCpus = numCpus;
    this.process = process;
  }

  public ProcessInfo getProcess() {
    return process;
  }

  @Override
  protected ListenableFuture<?> initialize() {
    String sched = tableName("sched"), span = tableName("span"), window = tableName("window");
    String tids = process.utids.stream()
        .map(String::valueOf)
        .collect(joining(","));
    return qe.queries(
        dropTable(span),
        dropView(sched),
        dropTable(window),
        createWindow(window),
        createView(sched, format(PROCESS_VIEW_SQL, tids)),
        createSpan(span, sched + " PARTITIONED cpu, " + window));
  }

  @Override
  protected ListenableFuture<Data> computeData(DataRequest req) {
    Window window = Window.compute(req, 10);
    return transformAsync(window.update(qe, tableName("window")),
        $ -> window.quantized ? computeSummary(req, window) : computeSlices(req));
  }

  private ListenableFuture<Data> computeSummary(DataRequest req, Window w) {
    return transform(qe.query(summarySql(w.bucketSize)), result -> {
      int len = w.getNumberOfBuckets();
      long[] ids = new long[len], utids = new long[len];
      Arrays.fill(ids, -1);
      Arrays.fill(utids, -1);
      Data data = new Data(req, w.bucketSize, ids, utids, new double[len]);
      result.forEachRow(($, r) -> {
        data.ids[r.getInt(0)] = r.getLong(1);
        data.utids[r.getInt(0)] = r.getLong(2);
        data.utilizations[r.getInt(0)] = r.getDouble(3);
      });
      return data;
    });
  }

  private String summarySql(long ns) {
    return format(SUMMARY_SQL, numCpus, ns, tableName("span"));
  }

  private ListenableFuture<Data> computeSlices(DataRequest req) {
    return transform(qe.query(slicesSql()), result -> {
      int rows = result.getNumRows();
      Data data = new Data(
          req, new long[rows], new long[rows], new long[rows], new int[rows], new long[rows]);
      result.forEachRow((i, r) -> {
        long start = r.getLong(0);
        data.starts[i] = start;
        data.ends[i] = start + r.getLong(1);
        data.cpus[i] = r.getInt(2);
        data.utids[i] = r.getLong(3);
        data.ids[i] = r.getLong(4);
      });
      return data;
    });
  }

  private String slicesSql() {
    return format(SLICES_SQL, tableName("span"));
  }

  public ListenableFuture<Slice> getSlice(long id) {
    return CpuTrack.getSlice(qe, id);
  }

  public static class Data extends Track.Data {
    public final Kind kind;
    public final long[] ids;
    public final long[] utids;
    // Summary.
    public final long bucketSize;
    public final double[] utilizations;
    // Slice.
    public final long[] starts;
    public final long[] ends;
    public final int[] cpus;

    public Data(DataRequest request, long bucketSize, long[] ids, long[] utids, double[] utilizations) {
      super(request);
      this.kind = Kind.summary;
      this.ids = ids;
      this.utids = utids;
      this.bucketSize = bucketSize;
      this.utilizations = utilizations;
      this.starts = null;
      this.ends = null;
      this.cpus = null;
    }

    public Data(
        DataRequest request, long[] ids, long[] starts, long[] ends, int[] cpus, long[] utids) {
      super(request);
      this.kind = Kind.slice;
      this.ids = ids;
      this.utids = utids;
      this.starts = starts;
      this.ends = ends;
      this.cpus = cpus;
      this.bucketSize = 0;
      this.utilizations = null;
    }

    public static enum Kind {
      summary, slice;
    }
  }
}
