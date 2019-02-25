# wavefront-opentracing-sdk-go [![build status][ci-img]][ci] [![Go Report Card][go-report-img]][go-report] [![GoDoc][godoc-img]][godoc] [![OpenTracing Badge](https://img.shields.io/badge/OpenTracing-enabled-blue.svg)](http://opentracing.io)

The Wavefront by VMware OpenTracing SDK for Go is a library that provides open tracing support for Wavefront.

## Requirements and Installation

Go 1.9 or higher

## Imports

Import Wavefront packages.

```go
import (
	"github.com/wavefronthq/wavefront-opentracing-sdk-go/reporter"
	"github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
	"github.com/wavefronthq/wavefront-sdk-go/application"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
)
```

## Set Up a Tracer

[Tracer](https://github.com/opentracing/specification/blob/master/specification.md#tracer) is an OpenTracing [interface](https://github.com/opentracing/opentracing-java#initialization) for creating spans and propagating them across arbitrary transports.

This SDK provides a `WavefrontTracer` that implements the `Tracer` interface. A `WaverfrontTracer`:
* Creates spans and sends them to Wavefront.
* Automatically generates and reports [RED metrics](https://github.com/wavefrontHQ/wavefront-opentracing-sdk-java/blob/master/docs/metrics.md) from your spans.

The steps for creating a `WavefrontTracer` are:

1. Create a `Tags` instance to specify metadata about your application.
2. Create a Wavefront `Sender` for managing communication with Wavefront.
3. Create a `WavefrontSpanReporter` for reporting trace data to Wavefront.
4. Create the `WavefrontTracer`.
5. Initialize the OpenTracing global tracer.

### 1. Set Up Application Tags

Application tags determine the metadata (span tags) that are included with every span reported to Wavefront. These tags enable you to filter and query trace data in Wavefront.

You encapsulate application tags in a `Tags` instance. See [Application Tags](https://github.com/wavefrontHQ/wavefront-sdk-go/blob/master/docs/apptags.md) for details.

The following example specifies values for the 2 required tags (`application` and `service`):

```go
appTags := application.New("OrderingApp", "inventory")
```


### 2. Set Up a Wavefront Sender

A "Wavefront sender" is an object that implements the low-level interface for sending data to Wavefront. You can choose to send data using either the [Wavefront proxy](https://docs.wavefront.com/proxies.html) or [direct ingestion](https://docs.wavefront.com/direct_ingestion.html).

* If you have already set up a Wavefront sender for another SDK that will run in the same process, use that one. (For details, see [Share a Wavefront Sender](https://github.com/wavefrontHQ/wavefront-sdk-go/blob/master/docs/sender.md#share-a-wavefront-sender).)

* Otherwise, follow the steps in [Set Up a Wavefront Sender](https://github.com/wavefrontHQ/wavefront-sdk-go/blob/master/docs/sender.md) to configure a proxy `Sender` or a direct `Sender`.

The following example configures a direct `Sender` with default direct ingestion properties:

```go
directCfg := &senders.DirectConfiguration{
  Server:               "https://INSTANCE.wavefront.com",
  Token:                "YOUR_API_TOKEN",
}

sender, err := senders.NewDirectSender(directCfg)
if err != nil {
  panic(err)
}
```

### 3. Set Up a Reporter

You must create a `WavefrontSpanReporter` to report trace data to Wavefront. You can optionally create a `CompositeReporter` to send data to Wavefront and print to the console.

#### Create a `WavefrontSpanReporter`

To create a `WavefrontSpanReporter`, you specify:

* The Wavefront sender from [Step 2](#2-set-up-a-wavefront-sender), i.e. either a proxy `Sender` or a direct `Sender`.
* The `Tags` instance from [Step 1](#1-set-up-application-tags).
* (Optional) A nondefault source for the reported spans.

This example creates a `WavefrontSpanReporter` that assigns the default source (the host name) to the reported spans:

```GO
reporter := reporter.New(sender, appTags)
```

This example creates a `WavefrontSpanReporter` that assigns the specified source to the reported spans:

```GO
reporter := reporter.New(sender, appTags, reporter.Source("app1.foo.com"))
```

#### Create a CompositeReporter (Optional)

A `CompositeReporter` enables you to chain a `WavefrontSpanReporter` to another reporter, such as a `ConsoleReporter`. A console reporter is useful for debugging.

```GO
wfReporter := reporter.New(sender, appTags, reporter.Source("app1.foo.com"))
clReporter := reporter.NewConsoleSpanReporter("app1.foo.com") //Specify the same source you used for the WavefrontSpanReporter
reporter := reporter.NewCompositeSpanReporter(wfReporter, clReporter)
```

### 4. Create the WavefrontTracer

To create a `WavefrontTracer`, you initialize it with the `Reporter` instance you created in the previous step:

```GO
tracer := tracer.New(reporter)
```

#### Sampling (Optional)

You can optionally create the `WavefrontTracer` with one or more sampling strategies. See the [sampling documentation](https://github.com/wavefrontHQ/wavefront-opentracing-sdk-go/blob/master/docs/sampling.md) for details.

```GO
tracer.New(reporter, WithSampler(sampler))
```

### 5. Initialize the Global Tracer

To create a global tracer, you initialize it with the `WavefrontTracer` you created in the previous step:

```GO
opentracing.InitGlobalTracer(tracer)
```

**Note:** Initializing the global tracer causes completed spans be reported to Wavefront automatically.
You do not need to start the reporter explicitly.

## Cross Process Context Propagation
See the [context propagation documentation](https://github.com/wavefrontHQ/wavefront-opentracing-sdk-go/tree/master/docs/contextpropagation.md) for details on propagating span contexts across process boundaries.


## RED Metrics
See the [RED metrics documentation](https://github.com/wavefrontHQ/wavefront-opentracing-sdk-java/blob/master/docs/metrics.md) for details on the out-of-the-box metrics and histograms that are provided.

[ci-img]: https://travis-ci.com/wavefrontHQ/wavefront-opentracing-sdk-go.svg?branch=master
[ci]: https://travis-ci.com/wavefrontHQ/wavefront-opentracing-sdk-go
[godoc]: https://godoc.org/github.com/wavefrontHQ/wavefront-opentracing-sdk-go
[godoc-img]: https://godoc.org/github.com/wavefrontHQ/wavefront-opentracing-sdk-go?status.svg
[go-report-img]: https://goreportcard.com/badge/github.com/wavefronthq/wavefront-opentracing-sdk-go
[go-report]: https://goreportcard.com/report/github.com/wavefronthq/wavefront-opentracing-sdk-go
