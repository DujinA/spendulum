# Rate-provider stub

A placeholder for the stub that replaces Frankfurter in end-to-end tests. `compose.e2e.yaml`
serves this directory as `http://rates:8080`. The exchange-rates slice adds the fixed fake
rates here and points the app's rate-provider base URL setting at the stub.
