# wavefront-opentracing-sdk-go [![travis build status](https://travis-ci.com/wavefrontHQ/wavefront-opentracing-sdk-go.svg?branch=master)](https://travis-ci.com/wavefrontHQ/wavefront-opentracing-sdk-go) [![OpenTracing Badge](https://img.shields.io/badge/OpenTracing-enabled-blue.svg)](http://opentracing.io)

The Wavefront by VMware OpenTracing SDK for GO is a library that provides open tracing support for Wavefront.

## Requirements and Installation

Go 1.9 or higher

## Usage

Import the senders package and create a proxy or direct sender as given below.

```go
import (
  wftracer "github.com/wavefronthq/wavefront-opentracing-sdk-go/tracer"
)
```

## Set Up a Tracer

[Tracer](https://github.com/opentracing/specification/blob/master/specification.md#tracer) is an OpenTracing [interface](https://github.com/opentracing/opentracing-java#initialization) for creating spans and propagating them across arbitrary transports.

This SDK provides a `Tracer` implementation for creating spans and sending them to Wavefront. The steps for creating a `Tracer` are:

1. Create an `ApplicationTags`, which specifies metadata about your application.
2. Create a Wavefront sender object for sending trace data to Wavefront.
3. Create a `WavefrontSpanReporter` for reporting trace data to Wavefront.
4. Create the `Tracer` instance.
5. Initialize Opentrace.

### 1. Set Up Application Tags

Application tags determine the metadata (span tags) that are included with every span reported to Wavefront. These tags enable you to filter and query trace data in Wavefront.

```go
appTags := wftracer.NewApplicationTags("app", "serv")
```

### 2. Set Up a Wavefront Sender

A "Wavefront sender" is an object that implements the low-level interface for sending data to Wavefront. You can choose to send data using either the [Wavefront proxy](https://docs.wavefront.com/proxies.html) or [direct ingestion](https://docs.wavefront.com/direct_ingestion.html).

* Follow the steps in [Set Up a Wavefront Sender](https://github.com/wavefrontHQ/wavefront-sdk-go#proxy-sender).

Direct ingestion sample:

```go
directCfg := &wf.DirectConfiguration{
  Server:               "https://INSTANCE.wavefront.com",
  Token:                "YOUR_API_TOKEN",
  BatchSize:            10000,
  MaxBufferSize:        50000,
  FlushIntervalSeconds: 1,
}

sender, err := wf.NewDirectSender(directCfg)
if err != nil {
  panic(err)
}
```

### 3. Set Up a Reporter

You must create a `WavefrontSpanReporter` to report trace data to Wavefront.

To create a `WavefrontSpanReporter`:

* Specify the Wavefront sender from [Step 2](#2-set-up-a-wavefront-sender), i.e. either `WavefrontProxyClient` or `WavefrontDirectClient`.
* Specify the ApplicationTags from [Step 1](#1-set-up-application-tags).
* (Optional) Specify a string that represents the source for the reported spans. If you omit the source, the host name is automatically used.

```GO
reporter := wftracer.NewSpanReporter(sender, appTags)
```

You can change the Source tag on your spand using the `Source` Option (the hostname is used by default):

```GO
reporter := tracer.NewSpanReporter(sender, appTags, tracer.Source("app1.foo.com"))
```

#### Create a CompositeReporter (Optional)

A CompositeReporter enables you to chain a WavefrontSpanReporter to another reporter, such as a ConsoleReporter. A console reporter is useful for debugging.

```GO
wfReporter := tracer.NewSpanReporter(sender, appTags, tracer.Source("app1.foo.com"))
clReporter := tracer.NewConsoleSpanReporter()
reporter := tracer.NewCompositeSpanReporter(wfReporter, clReporter)
```

### 4. Create the WavefrontTracer

To create a `WavefrontTracer`, you pass the `Reporter` instances you created in the previous steps:

```GO
tracer := wftracer.New(reporter)
```

### 5. Initialize Opentrace

```GO
opentracing.InitGlobalTracer(tracer)
```

**Note:** After you initialize `Opentrace` with `WavefrontTracer`(in step 5), completed spans will automatically be reported to Wavefront.
You do not need to start the reporter explicitly.
