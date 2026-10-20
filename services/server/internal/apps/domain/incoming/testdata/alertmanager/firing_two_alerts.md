**FIRING** HighErrorRate (2 alerts)

- **firing** HighErrorRate, severity critical: Error rate above 5 percent on api\-1
  Runbook: <https://runbooks.example.com/high-error-rate>
  Source: <https://prometheus.example.com/graph?g0.expr=rate%28http_errors_total%5B5m%5D%29&g0.tab=1>
- **firing** HighErrorRate, severity critical: Error rate above 5 percent on api\-2
  Runbook: <https://runbooks.example.com/high-error-rate>
  Source: <https://prometheus.example.com/graph?g0.expr=rate%28http_errors_total%5B5m%5D%29&g0.tab=1>

Alertmanager: <https://alertmanager.example.com>
