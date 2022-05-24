/*
 * Copyright (C) 2022 Google Inc.
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

import static com.google.gapid.util.Logging.throttleLogRpcError;
import static com.google.gapid.widgets.Widgets.createLink;
import static com.google.gapid.widgets.Widgets.createBoldLabel;
import static com.google.gapid.widgets.Widgets.withLayoutData;
import static com.google.gapid.widgets.Widgets.withMargin;
import static com.google.gapid.widgets.Widgets.withMarginAndSpacing;
import static com.google.gapid.widgets.Widgets.withSpacing;
import static com.google.gapid.widgets.Widgets.withSpans;
import static com.google.gapid.widgets.Widgets.createButton;
import static com.google.gapid.widgets.Widgets.createComposite;
import static com.google.gapid.widgets.Widgets.createGroup;
import static com.google.gapid.widgets.Widgets.createLabel;
import static com.google.gapid.widgets.Widgets.createTextbox;
import static com.google.gapid.widgets.Widgets.removeAllSelectionListeners;
import static java.util.logging.Level.WARNING;
import static java.util.logging.Level.SEVERE;

import com.google.gapid.models.Devices.DeviceValidationResult;
import com.google.gapid.models.Devices.DeviceValidationResult.ErrorCode;
import com.google.gapid.models.Devices.DeviceCaptureInfo;
import com.google.gapid.models.Models;
import com.google.gapid.proto.device.Device;
import com.google.gapid.proto.device.Device.Instance;
import com.google.gapid.rpc.Rpc;
import com.google.gapid.rpc.RpcException;
import com.google.gapid.rpc.SingleInFlight;
import com.google.gapid.rpc.UiErrorCallback;
import com.google.gapid.widgets.LoadingIndicator;
import com.google.gapid.widgets.Widgets;
import com.google.gapid.util.Logging;
import com.google.gapid.util.Messages;
import com.google.gapid.util.OS;
import com.google.gapid.util.URLs;

import org.eclipse.swt.graphics.Point;
import org.eclipse.swt.graphics.Rectangle;
import org.eclipse.swt.layout.GridData;
import org.eclipse.swt.layout.GridLayout;
import org.eclipse.swt.program.Program;
import org.eclipse.swt.widgets.Button;
import org.eclipse.swt.widgets.Composite;
import org.eclipse.swt.widgets.Event;
import org.eclipse.swt.widgets.Group;
import org.eclipse.swt.widgets.Label;
import org.eclipse.swt.widgets.Link;
import org.eclipse.swt.widgets.Listener;
import org.eclipse.swt.widgets.Text;
import org.eclipse.swt.SWT;

import java.io.File;
import java.io.IOException;
import java.util.concurrent.ExecutionException;
import java.util.logging.Logger;

public class DeviceValidationView extends Composite {
  protected static final Logger LOG = Logger.getLogger(DeviceValidationView.class.getName());
  private static final int TRACE_VALIDATION_FAILURE_COUNT = 0;

  private final Widgets widgets;
  private final Models models;
  private final SingleInFlight rpcController = new SingleInFlight();
    
  private boolean validationPassed;
  private LoadingIndicator.Widget statusLoader;
  private Link statusText;
  private Button retryButton;

  // Separate error text from help textas a workaround to enable proper text wrapping.
  private Group errorMessageGroup;
  private Composite helpComposite;

  private int traceValidationFailureCount;

  public DeviceValidationView(Composite parent, Models models, Widgets widgets) {
    super(parent, SWT.NONE);
    this.widgets = widgets;
    this.models = models;

    validationPassed = false;

    setLayout(new GridLayout(/* numColumns= */ 3, /* makeColumnsEqualWidth= */ false));
    setLayoutData(new GridData(SWT.FILL, SWT.TOP, true, false));

    // Status icon (loader) & accompanying text
    statusLoader = widgets.loading.createWidgetWithImage(this, 
        widgets.theme.check(), widgets.theme.error());
    statusLoader.setLayoutData(new GridData(SWT.LEFT, SWT.CENTER, false, false));
    statusText = withLayoutData(createLink(this, "", e -> {
      // Intentionally empty.
    }), new GridData(SWT.FILL, SWT.CENTER, true, false));
    retryButton = withLayoutData(createButton(this, "Retry", e -> {
      // Intentionally empty.
    }), new GridData(SWT.FILL, SWT.CENTER, false, true));

    statusLoader.setVisible(false);
    statusText.setVisible(false);
    retryButton.setVisible(false);

    traceValidationFailureCount = 0;
  }

  public void ValidateDevice(DeviceCaptureInfo deviceInfo) {
    if (deviceInfo == null) {
      statusLoader.setVisible(false);
      statusText.setVisible(false);
      retryButton.setVisible(false);
      return;
    }

    ValidateDevice(deviceInfo.device);
  }

  public void ValidateDevice(Device.Instance dev) {
    statusLoader.setVisible(true);
    statusText.setVisible(true);
    retryButton.setVisible(false);
    if (errorMessageGroup != null) {
      errorMessageGroup.dispose();
      errorMessageGroup = null;
      helpComposite.dispose();
      helpComposite = null;
      requestLayout();
    }

    DeviceValidationResult cachedResult = models.devices.getCachedValidationStatus(dev);
    if (cachedResult != null) {
      if (cachedResult.passedOrSkipped()) {
        setValidationStatus(cachedResult);
        return;
      }
    }

    statusLoader.startLoading();
    statusText.setText("Device support is being validated");

    // Assign appropriate callback for retry button.
    for (Listener listener : retryButton.getListeners(SWT.Selection)) {
      retryButton.removeListener(SWT.Selection, listener);
    }
    removeAllSelectionListeners(retryButton);
    retryButton.addListener(SWT.Selection, e -> {
      ValidateDevice(dev);
    });

    models.devices.validateDevice(dev, this::setValidationStatus);
  }

  private void setValidationStatus(DeviceValidationResult result) {
    boolean passedOrSkipped = result.passedOrSkipped();
    statusLoader.stopLoading();
    statusLoader.updateStatus(passedOrSkipped);
    validationPassed = passedOrSkipped;
    statusText.setText(getResultMessage(result.errorCode));
    removeAllSelectionListeners(statusText);
    if (result.errorCode == ErrorCode.FailedPrecondition) {
      statusText.addListener(SWT.Selection, (e) -> {
        Program.launch(URLs.DEVICE_COMPATIBILITY_URL);
      });
    }
    notifyListeners(SWT.Modify, new Event());

    if (passedOrSkipped) {
      return;
    }

    // Only show the retry button for non-deterministic errors.    
    if (result.errorCode == ErrorCode.FailedTraceValidation) {
      retryButton.setVisible(true);
      traceValidationFailureCount++;
    }

    helpComposite = createComposite(this, 
    withMarginAndSpacing(new GridLayout(1, false), 0, 0, 0, 0));
    helpComposite.setLayoutData(withSpans(new GridData(), 3, 1));
    addExplanationMessage(helpComposite, result.errorCode);

    // Only show the file bug section if we encounter an unexpected error.
    if (result.errorCode == ErrorCode.Internal || result.errorCode == ErrorCode.Invalid)
      addFileBugSection(helpComposite, result.tracePath, /* addNotice= */ false);
    else if (result.errorCode == ErrorCode.FailedTraceValidation 
      && traceValidationFailureCount >= TRACE_VALIDATION_FAILURE_COUNT) {
      addFileBugSection(helpComposite, result.tracePath, /* addNotice= */ true);
    }

    errorMessageGroup = withLayoutData(createGroup(this, ""),
      withSpans(new GridData(SWT.FILL, SWT.TOP, true, false), 3, 1));
    Text errText = withLayoutData(
      createTextbox(errorMessageGroup, SWT.READ_ONLY | SWT.WRAP,  result.toString()),
      new GridData(SWT.LEFT, SWT.TOP, false, false));

    resizeWindow();
  }

  private String getResultMessage(DeviceValidationResult.ErrorCode errorCode) {
    switch(errorCode) {
      case Ok:
        return "Device successfully validated.";
      case FailedPrecondition:
        return "This device is not supported, learn more <a>here</a>.";
      case FailedTraceValidation:
        return "Failed to validate the trace of the sample application.";
      default:
        return "Encountered an unexpected error during validation.";
    }
  }

  private void addExplanationMessage(Composite parent, DeviceValidationResult.ErrorCode errorCode) {
    switch(errorCode) {
      case FailedPrecondition:
        createLabel(parent,
         "This may be caused by using an older device or an unsupported operating system.");
        break;
      case FailedTraceValidation:
        createLabel(parent, "The validation process can be non deterministic. Please retry while making sure that:");
        createLabel(parent, "\t- The screen remains unlocked");
        createLabel(parent, "\t- The sample application remains in focus");
        break;
      default:
        return;
    }
  }

  private void addFileBugSection(Composite parent, String tracePath, boolean addNotice) {
    Composite composite = createComposite(parent, 
      withMarginAndSpacing(new GridLayout(1, false), 0, 5, 0, 0));
    composite.setLayoutData(withSpans(new GridData(GridData.VERTICAL_ALIGN_END), 3, 1));

    if (addNotice) {
      withLayoutData(createBoldLabel(composite, "If this continues to persist,"),
        new GridData(GridData.VERTICAL_ALIGN_END));
    }

    Link bugLink = withLayoutData(createLink(composite, "please file a <a>bug report</a> with:", e -> {
      Program.launch(URLs.FILE_BUG_URL);
    }), new GridData(GridData.VERTICAL_ALIGN_END));
    Link serverLog = createLink(composite, "\t - The <a>server logs</a>",
      openFileAt(Logging.getServerLogFile()));
    if (tracePath.length() > 0) {
      Link traceFile = createLink(composite, "\t - The <a>trace file</a>", 
        openFileAt(new File(tracePath)));
    }
    Link clientLog = createLink(composite, "\t - The <a>client logs</a>",
      openFileAt(Logging.getClientLogFile()));
  }

  private Listener openFileAt(File file) {
    return e -> {
      try {
        OS.openFileInSystemExplorer(file);
      } catch (IOException exception) {
        LOG.log(SEVERE, "Failed to open log directory in system explorer", exception);
      }
    };
  }

  private void resizeWindow() {
    // Resize the rest of the window if needed.
    Point curr = getShell().getSize();
    Point want = getShell().computeSize(curr.x, SWT.DEFAULT);
    if (want.y > curr.y) {
      getShell().setSize(new Point(curr.x, want.y));
    }

    requestLayout();
  }

  public boolean PassesValidation() {
    return validationPassed;
  }
}
