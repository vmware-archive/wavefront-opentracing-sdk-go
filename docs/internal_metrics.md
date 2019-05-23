# Internal Diagnostic Metrics

This SDK automatically collects a set of diagnostic metrics that allow you to monitor your `WavefrontTracer` instance. These metrics are reported once per minute to Wavefront.

The following is a list of the diagnostic metrics that are collected:

|Metric Name|Metric Type|Description|
|:---|:---:|:---|
|~sdk.go.opentracing.reporter.queue.size                  |Gauge      |Spans in the in-memory reporting buffer|
|~sdk.go.opentracing.reporter.queue.remaining_capacity    |Gauge      |Remaining capacity of the in-memory reporting buffer|
|~sdk.go.opentracing.reporter.spans.received.count        |Counter    |Spans received by the reporter|
|~sdk.go.opentracing.reporter.spans.dropped.count         |Counter    |Spans dropped during reporting|
|~sdk.go.opentracing.reporter.errors.count                |Counter    |Exceptions encountered while reporting spans|
|~sdk.go.opentracing.reporter.spans.discarded.count                |Counter    |Spans that are discarded as a result of sampling|

Each of the above metrics is reported with the same source and application tags that are specified for your `WavefrontTracer` and `WavefrontSpanReporter`.
