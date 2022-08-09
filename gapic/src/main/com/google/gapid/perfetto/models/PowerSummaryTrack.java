package com.google.gapid.perfetto.models;

import static com.google.gapid.perfetto.models.QueryEngine.createSpan;
import static com.google.gapid.perfetto.models.QueryEngine.createView;
import static com.google.gapid.perfetto.models.QueryEngine.createWindow;
import static com.google.gapid.perfetto.models.QueryEngine.dropTable;
import static com.google.gapid.perfetto.models.QueryEngine.dropView;
import static com.google.gapid.perfetto.views.StyleConstants.POWER_RAIL_COUNTER_TRACK_HEIGHT;
import static com.google.gapid.perfetto.views.TrackContainer.group;
import static com.google.gapid.perfetto.views.TrackContainer.single;
import static com.google.gapid.util.MoreFutures.transform;
import static com.google.gapid.util.MoreFutures.transformAsync;
import static java.lang.String.format;

import com.google.common.collect.ImmutableListMultimap;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.gapid.models.Perfetto;
import com.google.gapid.perfetto.Unit;
import com.google.gapid.perfetto.models.TrackConfig.Group;
import com.google.gapid.perfetto.views.CounterPanel;
import com.google.gapid.perfetto.views.PowerSummaryPanel;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

/** {@link Track} summarizing the total Power usage. */
public class PowerSummaryTrack extends Track.WithQueryEngine<PowerSummaryTrack.Data> {
  private static String viewSql;
  private static int numPowerRailTracks;
  public static Unit unit;
  public static Double minValue;
  public static Double maxValue;

  private static final String VIEW_SQL_MONOTONIC =
      "select ts + 1 ts, lead(ts) over win - ts dur, lead(value) over win - value value, lead(id)"
          + " over win id from counter where track_id = ";

  public PowerSummaryTrack(QueryEngine qe, int numTracks) {
    super(qe, "power_sum");
    this.numPowerRailTracks = numTracks;
  }

  public int getNumPowerRailTracks() {
    return numPowerRailTracks;
  }

  public static Perfetto.Data.Builder enumerate(Perfetto.Data.Builder data) {
    ImmutableListMultimap<String, CounterInfo> counters = data.getCounters(CounterInfo.Type.Global);
    List<CounterInfo> powerRails =
        counters.entries().stream()
            .filter(entry -> entry.getKey().startsWith("power.rails"))
            .map(Map.Entry::getValue)
            .collect(Collectors.toList());

    if (powerRails.size() == 0) {
      return data;
    }

    // Power Group
    viewSql = buildViewSql(powerRails);
    PowerSummaryTrack powerSummaryTrack = new PowerSummaryTrack(data.qe, powerRails.size());

    // Power Rails tracks.
    for (CounterInfo powerRail : powerRails) {
      unit = powerRail.unit;
      CounterTrack powerRailTrack = new CounterTrack(data.qe, powerRail);
      data.tracks.addTrack(
          powerSummaryTrack.getId(),
          powerRailTrack.getId(),
          powerRail.name,
          single(
              state -> new CounterPanel(state, powerRailTrack, POWER_RAIL_COUNTER_TRACK_HEIGHT),
              true,
              true));
    }

    Group.UiFactory ui =
        group(
            state ->
                new PowerSummaryPanel(state, powerSummaryTrack, POWER_RAIL_COUNTER_TRACK_HEIGHT),
            false);
    data.tracks.addLabelGroup(null, powerSummaryTrack.getId(), "Power Usage", ui);
    return data;
  }

  private static String buildViewSql(List<CounterInfo> powerRails) {
    minValue = powerRails.get(0).min;
    maxValue = powerRails.get(0).max;
    StringBuilder sb =
        new StringBuilder()
            .append("select aggregateTable.ts ts, sum(aggregateTable.value) totalValue from")
            .append(" (")
            .append(VIEW_SQL_MONOTONIC)
            .append(powerRails.get(0).id)
            .append(" window win as (order by ts) ")
            .append(") as aggregateTable");
    for (int i = 1; i < numPowerRailTracks; i++) {
      sb.append("left join (")
          .append(VIEW_SQL_MONOTONIC)
          .append(powerRails.get(i).id)
          .append(" window win as (order by ts) ")
          .append(") ");
      minValue = Math.min(minValue, powerRails.get(i).min);
      maxValue = Math.max(maxValue, powerRails.get(i).max);
    }
    sb.append(" group by ts");
    minValue = Math.min(0, minValue);
    return sb.toString();
  }

  @Override
  protected ListenableFuture<?> initialize() {
    String vals = tableName("vals");
    String span = tableName("span");
    String window = tableName("window");
    return qe.queries(
        dropTable(span),
        dropTable(window),
        dropView(vals),
        createView(vals, viewSql),
        createWindow(window),
        createSpan(span, vals + ", " + window));
  }

  @Override
  protected ListenableFuture<Data> computeData(DataRequest req) {
    Window win = Window.quantized(req, 5);
    return transformAsync(win.update(qe, tableName("window")), $ -> computeData(req, win));
  }

  private ListenableFuture<Data> computeData(DataRequest req, Window win) {
    return transform(
        qe.query(sql()),
        res -> {
          int rows = res.getNumRows();
          if (rows == 0) {
            return Data.empty(req);
          }

          Data data = new Data(req, new long[rows + 1], new double[rows + 1]);
          res.forEachRow(
              (i, r) -> {
                data.ts[i] = r.getLong(0);
                data.values[i] = r.getDouble(1);
              });
          data.ts[rows] = res.getLong(rows - 1, 1, 0);
          data.values[rows] = data.values[rows - 1];
          return data;
        });
  }

  private String sql() {
    return format(viewSql, tableName("span"));
  }

  public static class Data extends Track.Data {
    public final long[] ts;
    public final double[] values;

    public Data(DataRequest request, long[] ts, double[] values) {
      super(request);
      this.ts = ts;
      this.values = values;
    }

    public static Data empty(DataRequest req) {
      return new Data(req, new long[0], new double[0]);
    }
  }
}
