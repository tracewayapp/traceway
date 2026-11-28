Session capture screenshots, captured 27 November 2026

Source: the Traceway dashboard of the working tree (vite dev on :15183) against
examples/devtesting-embedded (:18182). Light theme, Chromium, 1440 x 900 viewport,
device scale 1.5, UTC.

Project: the seeded "Android App" project. Its rules ("Quiet logs in production",
"Stream replay for the 2.4 beta", "Checkout journey", "No replay images on old devices")
and the capture of user u-4821 were created through the dashboard API for the pictures.
The rule dialog images show the New Rule dialog filled in the browser: Applies to,
Triggers (a navigation, tap and log sequence) and Signals with the Replay images select
open.

session-replay.png is a real wireframe recording: the Android SDK's wireframe gallery run
(Robolectric, the "images" screen with replay images on), uploaded as a session of the
project with OTLP session.start/session.end and POST /api/recordings, and played by the
dashboard's player. capture-dialog.png is "Capture this device" on that session's page.

No data, chart values or UI labels have been retouched. The Data Collection images are
cropped to the page's content column; the dialogs to the dialog.
