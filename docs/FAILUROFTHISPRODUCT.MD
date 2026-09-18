
### Bot Shield Product Quality Testing

We use the following setup to validate Bot Shield's detection and blocking capabilities:

* **Dummy Client Site:**
  `D:\bot-shield\hotel_flask_app.py`
  This is a local dummy hotel website that acts as the **protected client site**. Bot Shield is placed in front of this application to evaluate and protect its incoming traffic.
* **Real Bot Simulation:**
  `D:\bot-shield\test.py`
  This script generates **real automated bot traffic** against the protected client site for product-quality testing.

### Current Testing Failure

During the latest product-quality test, we ran `test.py` against the protected `hotel_flask_app.py` application.

**Result:** Bot Shield failed to detect and block the automated bot traffic.

This is considered a **detection failure** and needs to be investigated before treating the current detection logic as production-ready.

### Testing Objective

The goal is to ensure that Bot Shield can:

1. Correctly identify automated/bot traffic.
2. Distinguish bot traffic from legitimate users.
3. Apply the appropriate policy action when a bot is detected.
4. Avoid relying on a single weak detection signal.
5. Provide sufficient request-level evidence explaining why traffic was classified as automated.
6. Pass adversarial testing before the detection capability is considered production-ready.

**Important:** The test should focus on improving Bot Shield's defensive detection and blocking capability, not on making the bot harder to detect.
