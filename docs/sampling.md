# Sampling

A cloud-scale web application generates a very large number of traces.
You can set up one or more sampling strategies in your application to reduce the volume of trace data
that it sends to Wavefront.

You set up a sampling strategy by configuring the `WavefrontTracer` with an implementation of the `Sampler` interface.

For example, suppose you want to allow approximately 1 out of every 5 traces.
The following snippet shows how to configure a `WavefrontTracer` with a `RateSampler`:

```GO
// Create a RateSampler with a rate of 20%
sampler := tracer.RateSampler{Rate: 20}

// Create the WavefrontTracer
tracer.New(reporter, WithSampler(sampler))
```

## Supported Sampling Strategies

The following table lists the supported sampling strategies:

| Sampler              | Description                            |
| --------------------- | -------------------------------------- |
| `DurationSampler`       | Allows a span if its duration exceeds a specified threshold. Specify the duration threshold as a `time.Duration` number of milliseconds. |
| `RateSampler`          | Allows a specified probabilistic rate of traces to be reported. Specify the rate of allowed traces as a `unint64` number between 0 and 100. |


**Note:** Regardless of the sampling strategy, the `WavefrontTracer`:
* Allows all error spans (`error=true` span tag).
* Allows all spans that have a sampling priority greater than 0 (`sampling.priority` span tag).
* Includes all spans in the [RED metrics](https://github.com/wavefrontHQ/wavefront-opentracing-sdk-java/blob/master/README.md#red-metrics) that are automatically collected and reported.


## Using Multiple Sampling Strategies

You can configure a `WavefrontTracer` with multiple sampling strategies. In this case, the `WavefrontTracer` allows a span if any of the samplers decide to allow it.

For instance, suppose you want to report approximately 10% of traces
but you also don't want to lose any spans that are over 60 seconds long.
The following code snippet shows how to configure a `WavefrontTracer` with a `RateSampler` and a `DurationSampler`:


```GO
// Create and configure the RateSampler and DurationSampler
rateSampler := tracer.RateSampler{Rate: 10}
durationSampler := tracer.DurationSampler{Duration: 60 * time.Second}

// Create the WavefrontTracer
tracer.New(reporter, WithSampler(rateSampler), WithSampler(durationSampler))
```
